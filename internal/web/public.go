package web

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"benevole-ape/internal/model"
	"benevole-ape/internal/store"
)

// ---- home / event creation ----

type homePage struct {
	Common
	NeedPassword bool
	MailEnabled  bool
	Error        string
	Form         model.Event
}

func (s *Server) home(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, http.StatusOK, homeView(homePage{
		Common:       s.common(w, r, "Je viens aider"),
		NeedPassword: s.cfg.CreatePassword != "",
		MailEnabled:  s.cfg.MailEnabled,
		Form:         model.Event{NotifySignups: true},
	}))
}

func (s *Server) createEvent(w http.ResponseWriter, r *http.Request) {
	ev, err := s.readEventForm(r)
	if !s.createPasswordOK(r) {
		err = userError("Mot de passe de création incorrect.")
	}
	if err != nil {
		s.render(w, r, http.StatusBadRequest, homeView(homePage{
			Common: Common{Title: "Je viens aider"}, NeedPassword: s.cfg.CreatePassword != "", MailEnabled: s.cfg.MailEnabled, Error: err.Error(), Form: ev,
		}))
		return
	}
	if err := s.store.CreateEvent(r.Context(), &ev); err != nil {
		s.serverError(w, r, err)
		return
	}
	sent := s.sendLinks(r, &ev, false)
	msg := "Événement créé ! Ce lien d'administration est votre seule clé pour le gérer : ajoutez-le à vos favoris et ne le partagez qu'avec les organisateurs."
	if s.cfg.MailEnabled && sent {
		msg = "Événement créé ! Ce lien d'administration est votre seule clé pour le gérer : il vient de vous être envoyé à " + ev.AdminEmail + ". Ne le partagez qu'avec les organisateurs."
	}
	setFlash(w, "success", msg)
	redirect(w, r, "/a/"+ev.AdminToken)
}

// createPasswordOK checks the creation password, if one is configured.
func (s *Server) createPasswordOK(r *http.Request) bool {
	return s.cfg.CreatePassword == "" ||
		subtle.ConstantTimeCompare([]byte(r.FormValue("password")), []byte(s.cfg.CreatePassword)) == 1
}

// Messages shown when the event does not accept changes.
const (
	msgNotOpenYet = "Les inscriptions ne sont pas encore ouvertes. Revenez bientôt !"
	msgClosed     = "Les inscriptions sont closes : contactez les organisateurs pour toute modification."
)

// volunteerID is v's ID, or 0 for an unknown visitor.
func volunteerID(v *model.Volunteer) int64 {
	if v == nil {
		return 0
	}
	return v.ID
}

// ---- event loading helpers ----

func (s *Server) withEvent(h func(http.ResponseWriter, *http.Request, *model.Event)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Event pages can show the visitor's personal link and contact
		// details: never let a browser or proxy cache them.
		w.Header().Set("Cache-Control", "no-store")
		ev, err := s.store.EventBySlug(r.Context(), r.PathValue("slug"))
		if errors.Is(err, store.ErrNotFound) {
			s.renderError(w, r, http.StatusNotFound, "Cet événement n'existe pas (ou plus). Vérifiez le lien.")
			return
		}
		if err != nil {
			s.serverError(w, r, err)
			return
		}
		h(w, r, ev)
	}
}

func (s *Server) withVolunteer(h func(http.ResponseWriter, *http.Request, *model.Event, *model.Volunteer)) http.HandlerFunc {
	return s.withEvent(func(w http.ResponseWriter, r *http.Request, ev *model.Event) {
		vol, err := s.store.VolunteerByToken(r.Context(), ev.ID, r.PathValue("token"))
		if errors.Is(err, store.ErrNotFound) {
			s.renderError(w, r, http.StatusNotFound, "Ce lien personnel n'est pas (ou plus) valide.")
			return
		}
		if err != nil {
			s.serverError(w, r, err)
			return
		}
		h(w, r, ev, vol)
	})
}

func eventURL(ev *model.Event) string { return "/e/" + ev.Slug }

func meURL(ev *model.Event, v *model.Volunteer) string {
	return eventURL(ev) + "/me/" + v.EditToken
}

func volunteerCookie(ev *model.Event) string { return "v" + strconv.FormatInt(ev.ID, 10) }

func setVolunteerCookie(w http.ResponseWriter, ev *model.Event, v *model.Volunteer) {
	http.SetCookie(w, &http.Cookie{
		Name: volunteerCookie(ev), Value: v.EditToken, Path: eventURL(ev),
		HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: 365 * 24 * 3600,
	})
}

// cookieVolunteer returns the volunteer remembered by this browser, if any.
func (s *Server) cookieVolunteer(r *http.Request, ev *model.Event) *model.Volunteer {
	c, err := r.Cookie(volunteerCookie(ev))
	if err != nil {
		return nil
	}
	v, err := s.store.VolunteerByToken(r.Context(), ev.ID, c.Value)
	if err != nil {
		return nil
	}
	return v
}

func (s *Server) planning(ctx context.Context, ev *model.Event, myID int64) (*model.Planning, error) {
	stands, err := s.store.ListStands(ctx, ev.ID)
	if err != nil {
		return nil, err
	}
	slots, err := s.store.ListSlots(ctx, ev.ID)
	if err != nil {
		return nil, err
	}
	signups, err := s.store.ListSignups(ctx, ev.ID)
	if err != nil {
		return nil, err
	}
	return model.BuildPlanning(stands, slots, signups, myID, ev.IsOpen()), nil
}

// ---- event page (overview, "Mes créneaux") ----

type eventPage struct {
	Common
	Event    *model.Event
	P        *model.Planning
	Tab      string // grid, me
	TodoOnly bool
	EventURL string
	Me       *model.Volunteer
	MeURL    string
	MeAbsURL string
	Welcome  bool
	MyP      *model.Planning // "Mes créneaux": the planning restricted to my slots
}

func (s *Server) eventPage(w http.ResponseWriter, r *http.Request, ev *model.Event) {
	s.showEvent(w, r, ev, s.cookieVolunteer(r, ev), false)
}

func (s *Server) mePage(w http.ResponseWriter, r *http.Request, ev *model.Event, vol *model.Volunteer) {
	setVolunteerCookie(w, ev, vol)
	s.showEvent(w, r, ev, vol, true)
}

func (s *Server) showEvent(w http.ResponseWriter, r *http.Request, ev *model.Event, me *model.Volunteer, meTab bool) {
	p, err := s.planning(r.Context(), ev, volunteerID(me))
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	data := eventPage{
		Common:   s.common(w, r, ev.Title),
		Event:    ev,
		P:        p,
		Tab:      "grid",
		TodoOnly: r.URL.Query().Get("todo") == "1",
		EventURL: eventURL(ev),
		Me:       me,
	}
	if me != nil {
		data.MeURL = meURL(ev, me)
		data.MeAbsURL = s.absURL(r, data.MeURL)
		data.Welcome = r.URL.Query().Get("welcome") == "1"
	}
	if meTab {
		data.Tab = "me"
		data.MyP = p.Mine()
	}
	s.render(w, r, http.StatusOK, eventView(data))
}

// ---- sign-up ----

type signupPage struct {
	Common
	Event    *model.Event
	EventURL string
	Slot     *model.SlotView
	Me       *model.Volunteer
	Form     volunteerForm
	Error    string
	// Overlaps lists the visitor's existing sign-ups at the same time, to
	// warn them before they confirm.
	Overlaps []*model.SlotView
}

// loadSlotView returns a slot with its sign-ups, and the visitor's other
// sign-ups that overlap it.
func (s *Server) loadSlotView(ctx context.Context, ev *model.Event, slotID, myID int64) (*model.SlotView, []*model.SlotView, error) {
	p, err := s.planning(ctx, ev, myID)
	if err != nil {
		return nil, nil, err
	}
	sv := p.Slot(slotID)
	if sv == nil {
		return nil, nil, store.ErrNotFound
	}
	return sv, p.MyOverlaps(sv), nil
}

// publicID reads the {id} of a public URL about an event's planning (a
// slot, a stand). A draft's planning is not public yet: "not found".
func (s *Server) publicID(w http.ResponseWriter, r *http.Request, ev *model.Event) (int64, bool) {
	if ev.IsDraft() {
		s.renderError(w, r, http.StatusNotFound, msgNotOpenYet)
		return 0, false
	}
	return s.pathID(w, r)
}

func (s *Server) signupForm(w http.ResponseWriter, r *http.Request, ev *model.Event) {
	id, ok := s.publicID(w, r, ev)
	if !ok {
		return
	}
	me := s.cookieVolunteer(r, ev)
	data := signupPage{Common: s.common(w, r, "Inscription"), Event: ev, EventURL: eventURL(ev), Me: me}
	if me != nil {
		data.Form = volunteerForm{Name: me.Name, Phone: me.Phone}
	}
	sv, overlaps, err := s.loadSlotView(r.Context(), ev, id, volunteerID(me))
	if s.storeError(w, r, err) {
		return
	}
	data.Slot, data.Overlaps = sv, overlaps
	if isHTMX(r) {
		s.render(w, r, http.StatusOK, signupDialog(data))
		return
	}
	s.render(w, r, http.StatusOK, signupView(data))
}

func (s *Server) signupSubmit(w http.ResponseWriter, r *http.Request, ev *model.Event) {
	id, ok := s.publicID(w, r, ev)
	if !ok {
		return
	}
	// Honeypot: bots fill every field, humans never see this one.
	if r.FormValue("website") != "" {
		redirect(w, r, eventURL(ev))
		return
	}
	me := s.cookieVolunteer(r, ev)
	form := readVolunteerForm(r)
	if err := form.validate(); err != nil {
		s.signupFailed(w, r, ev, id, me, form, http.StatusBadRequest, err.Error())
		return
	}
	req := store.SignupRequest{EventID: ev.ID, SlotID: id, Name: form.Name, Phone: form.Phone, Comment: form.Comment}
	if me != nil {
		req.EditToken = me.EditToken
	}
	res, err := s.store.SignUp(r.Context(), req)
	if msg, ok := userMessage(err); ok { // full slot, already signed up, event closed
		s.signupFailed(w, r, ev, id, me, form, http.StatusConflict, msg)
		return
	}
	if s.storeError(w, r, err) {
		return
	}

	setVolunteerCookie(w, ev, res.Volunteer)
	s.notifySignup(r.Context(), r, ev, res.Volunteer, id, form.Comment, false)
	if res.NewVolunteer {
		redirect(w, r, meURL(ev, res.Volunteer)+"?welcome=1")
		return
	}
	msg := fmt.Sprintf("C'est noté ! Vous êtes inscrit·e de %s à %s. Merci !", hm(res.Slot.StartsAt), hm(res.Slot.EndsAt))
	kind := "success"
	if len(res.Overlapping) > 0 {
		kind = "warning"
		msg += fmt.Sprintf(" Attention : ce créneau chevauche un autre de vos créneaux (%s – %s).",
			hm(res.Overlapping[0].StartsAt), hm(res.Overlapping[0].EndsAt))
	}
	setFlash(w, kind, msg)
	redirect(w, r, eventURL(ev))
}

// signupFailed shows the sign-up form again with the visitor's input and an
// error. It is always a full page: the form is posted without htmx.
func (s *Server) signupFailed(w http.ResponseWriter, r *http.Request, ev *model.Event, slotID int64, me *model.Volunteer, form volunteerForm, status int, msg string) {
	sv, overlaps, err := s.loadSlotView(r.Context(), ev, slotID, volunteerID(me))
	if s.storeError(w, r, err) {
		return
	}
	s.render(w, r, status, signupView(signupPage{
		Common: Common{Title: "Inscription"}, Event: ev, EventURL: eventURL(ev),
		Slot: sv, Overlaps: overlaps, Me: me, Form: form, Error: msg,
	}))
}

// ---- stand details ----

type standPage struct {
	Common
	EventURL string
	Stand    *model.Stand
}

// standInfo shows a stand's full description ("Voir plus" in the overview),
// as a dialog with htmx or as a page.
func (s *Server) standInfo(w http.ResponseWriter, r *http.Request, ev *model.Event) {
	id, ok := s.publicID(w, r, ev)
	if !ok {
		return
	}
	stands, err := s.store.ListStands(r.Context(), ev.ID)
	if s.storeError(w, r, err) {
		return
	}
	var st *model.Stand
	for i := range stands {
		if stands[i].ID == id {
			st = &stands[i]
		}
	}
	if st == nil {
		s.renderError(w, r, http.StatusNotFound, "Stand introuvable.")
		return
	}
	data := standPage{Common: s.common(w, r, st.Name), EventURL: eventURL(ev), Stand: st}
	if isHTMX(r) {
		s.render(w, r, http.StatusOK, standDialog(data))
		return
	}
	s.render(w, r, http.StatusOK, standView(data))
}

// ---- volunteer self-service ----

func (s *Server) meUpdate(w http.ResponseWriter, r *http.Request, ev *model.Event, vol *model.Volunteer) {
	if !ev.IsOpen() {
		setFlash(w, "error", msgClosed)
		redirect(w, r, meURL(ev, vol))
		return
	}
	form := readVolunteerForm(r)
	if err := form.validate(); err != nil {
		setFlash(w, "error", err.Error())
		redirect(w, r, meURL(ev, vol))
		return
	}
	vol.Name, vol.Phone = form.Name, form.Phone
	if s.storeError(w, r, s.store.UpdateVolunteer(r.Context(), vol)) {
		return
	}
	setFlash(w, "success", "Vos coordonnées ont été mises à jour.")
	redirect(w, r, meURL(ev, vol))
}

func (s *Server) meCancel(w http.ResponseWriter, r *http.Request, ev *model.Event, vol *model.Volunteer) {
	id, ok := s.pathID(w, r)
	if !ok {
		return
	}
	if !ev.IsOpen() {
		setFlash(w, "error", msgClosed)
		redirect(w, r, meURL(ev, vol))
		return
	}
	slotID, err := s.store.CancelSignup(r.Context(), vol.ID, id)
	if s.storeError(w, r, err) {
		return
	}
	s.notifySignup(r.Context(), r, ev, vol, slotID, "", true)
	setFlash(w, "info", "Votre inscription a été annulée.")
	redirect(w, r, meURL(ev, vol))
}

// meRenew replaces the volunteer's personal link, for when it was shared
// by mistake: the old link stops working at once.
func (s *Server) meRenew(w http.ResponseWriter, r *http.Request, ev *model.Event, vol *model.Volunteer) {
	if s.storeError(w, r, s.store.RenewVolunteerToken(r.Context(), vol)) {
		return
	}
	setVolunteerCookie(w, ev, vol)
	setFlash(w, "success", "Voici votre nouveau lien personnel : l'ancien ne fonctionne plus. Pensez à le garder.")
	redirect(w, r, meURL(ev, vol))
}

// forget clears the volunteer cookie ("ce n'est pas moi" on a shared device).
func (s *Server) forget(w http.ResponseWriter, r *http.Request, ev *model.Event) {
	http.SetCookie(w, &http.Cookie{Name: volunteerCookie(ev), Path: eventURL(ev), MaxAge: -1})
	setFlash(w, "info", "Cet appareil ne vous reconnaît plus. Votre lien personnel reste valable.")
	redirect(w, r, eventURL(ev))
}
