package web

import (
	"bytes"
	"embed"
	"encoding/base64"
	"fmt"
	"io/fs"
	"net/http"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/a-h/templ"

	"benevole-ape/internal/model"
)

//go:embed static
var staticFS embed.FS

func (s *Server) staticHandler() http.Handler {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		panic(err)
	}
	fsrv := http.StripPrefix("/static/", http.FileServer(http.FS(sub)))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("v") != "" {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "public, max-age=3600")
		}
		fsrv.ServeHTTP(w, r)
	})
}

// Common is embedded in every page's data.
type Common struct {
	Title string
	Flash *Flash
}

// Flash is a one-shot message carried by a cookie across a redirect.
type Flash struct {
	Kind    string // success, error, warning, info
	Message string
}

const flashCookie = "flash"

func setFlash(w http.ResponseWriter, kind, msg string) {
	http.SetCookie(w, &http.Cookie{
		Name: flashCookie, Value: base64.RawURLEncoding.EncodeToString([]byte(kind + "|" + msg)),
		Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: 60,
	})
}

func popFlash(w http.ResponseWriter, r *http.Request) *Flash {
	c, err := r.Cookie(flashCookie)
	if err != nil {
		return nil
	}
	http.SetCookie(w, &http.Cookie{Name: flashCookie, Path: "/", MaxAge: -1})
	b, err := base64.RawURLEncoding.DecodeString(c.Value)
	if err != nil {
		return nil
	}
	kind, msg, ok := strings.Cut(string(b), "|")
	if !ok {
		return nil
	}
	switch kind {
	case "success", "error", "warning", "info":
	default:
		kind = "info"
	}
	return &Flash{Kind: kind, Message: msg}
}

func (s *Server) common(w http.ResponseWriter, r *http.Request, title string) Common {
	return Common{Title: title, Flash: popFlash(w, r)}
}

func isHTMX(r *http.Request) bool { return r.Header.Get("HX-Request") == "true" }

// render writes a page (a templ component) with the given status. It is
// rendered to a buffer first, so that a failure becomes a clean 500.
func (s *Server) render(w http.ResponseWriter, r *http.Request, status int, page templ.Component) {
	var buf bytes.Buffer
	if err := page.Render(r.Context(), &buf); err != nil {
		s.log.Error("render", "path", r.URL.Path, "err", err)
		http.Error(w, "Erreur interne", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	buf.WriteTo(w)
}

type errorPage struct {
	Common
	Status  int
	Message string
}

func (s *Server) renderError(w http.ResponseWriter, r *http.Request, status int, msg string) {
	s.render(w, r, status, errorView(errorPage{Common: Common{Title: "Oups"}, Status: status, Message: msg}))
}

// absURL turns a path into an absolute URL for display and sharing.
func (s *Server) absURL(r *http.Request, path string) string {
	if s.cfg.BaseURL != "" {
		return s.cfg.BaseURL + path
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if s.cfg.TrustProxy {
		if p := r.Header.Get("X-Forwarded-Proto"); p == "http" || p == "https" {
			scheme = p
		}
	}
	return scheme + "://" + r.Host + path
}

var (
	frDays   = []string{"dimanche", "lundi", "mardi", "mercredi", "jeudi", "vendredi", "samedi"}
	frMonths = []string{"", "janvier", "février", "mars", "avril", "mai", "juin", "juillet", "août", "septembre", "octobre", "novembre", "décembre"}
)

// hm formats a time as "14h" or "14h30".
func hm(t time.Time) string {
	if t.Minute() == 0 {
		return fmt.Sprintf("%dh", t.Hour())
	}
	return fmt.Sprintf("%dh%02d", t.Hour(), t.Minute())
}

func dayFR(t time.Time) string {
	return fmt.Sprintf("%s %d %s", frDays[t.Weekday()], t.Day(), frMonths[t.Month()])
}

func dateFR(s string) string {
	t, err := time.Parse(model.DateLayout, s)
	if err != nil {
		return s
	}
	return dayFR(t) + " " + strconv.Itoa(t.Year())
}

// eventStatusLabel is the French name of an event status.
func eventStatusLabel(status string) string {
	switch status {
	case model.StatusDraft:
		return "Brouillon"
	case model.StatusOpen:
		return "Ouvertes"
	case model.StatusClosed:
		return "Closes"
	}
	return status
}

// todoURL is the event page, showing only the slots still to fill if todo.
func todoURL(base string, todo bool) string {
	if todo {
		return base + "?todo=1"
	}
	return base
}

// mailFuncs are the helpers of the e-mail templates.
func mailFuncs() template.FuncMap {
	return template.FuncMap{
		"day":    dayFR,
		"date":   dateFR,
		"span":   func(a, b time.Time) string { return hm(a) + " – " + hm(b) },
		"plural": plural,
	}
}

// durationFR formats a duration the French way: "45 min", "2 h", "2 h 30".
func durationFR(d time.Duration) string {
	m := int(d.Round(time.Minute) / time.Minute)
	switch {
	case m < 60:
		return strconv.Itoa(m) + " min"
	case m%60 == 0:
		return strconv.Itoa(m/60) + " h"
	default:
		return fmt.Sprintf("%d h %02d", m/60, m%60)
	}
}
