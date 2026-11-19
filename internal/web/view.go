package web

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/a-h/templ"

	"benevole-ape/internal/model"
)

//go:generate go tool templ generate

// assetVersion changes whenever a cached static file changes, so that
// browsers fetch the new one (see asset).
var assetVersion = func() string {
	sum := sha256.New()
	for _, f := range []string{"static/app.css", "static/app.js", "static/htmx.min.js"} {
		b, err := staticFS.ReadFile(f)
		if err != nil {
			panic(err)
		}
		sum.Write(b)
	}
	return hex.EncodeToString(sum.Sum(nil))[:10]
}()

// asset is the URL of a static file, versioned for long caching.
func asset(name string) string { return "/static/" + name + "?v=" + assetVersion }

// cssColor is a stand's colour if it is a valid one, for use in CSS.
func cssColor(c string) string {
	if validColor(c) {
		return c
	}
	return defaultColor
}

// itoa is strconv.Itoa, short for templates.
func itoa(n int) string { return strconv.Itoa(n) }

// plural picks one or many depending on n.
func plural(n int, one, many string) string {
	if n > 1 {
		return many
	}
	return one
}

// placesLeft says, positively, how many people are still needed:
// "Il reste 1 place", "Il reste 3 places".
func placesLeft(n int) string {
	return "Il reste " + itoa(n) + " " + plural(n, "place", "places")
}

// slotKind is how a slot looks: "need" (people still needed), "room"
// (enough people, more welcome), "full" (no more room).
func slotKind(s *model.SlotView) string {
	switch {
	case s.Missing > 0:
		return "need"
	case s.Full:
		return "full"
	}
	return "room"
}

// slotStatus says in words where a slot stands, without a bare fraction.
func slotStatus(s *model.SlotView) string {
	switch slotKind(s) {
	case "need":
		return placesLeft(s.Missing)
	case "full":
		return "Complet"
	}
	if s.MaxPeople > 0 {
		left := s.MaxPeople - s.Count
		return "Assez de monde, encore " + itoa(left) + " " + plural(left, "place", "places")
	}
	return "Assez de monde, vous pouvez venir"
}

// standStatus sums up a stand for the organiser.
func standStatus(st *model.StandView) string {
	switch {
	case len(st.Slots) == 0:
		return "Aucun créneau"
	case st.Missing > 0:
		return placesLeft(st.Missing)
	}
	return "Complet"
}

// slotRule describes a slot's capacity: "2 personnes", "3 personnes, 4 au maximum".
func slotRule(s *model.SlotView) string {
	r := itoa(s.MinPeople) + " " + plural(s.MinPeople, "personne", "personnes")
	if s.MaxPeople > 0 {
		r += ", " + itoa(s.MaxPeople) + " au maximum"
	}
	return r
}

// publicNames lists who is on a slot as the public sees them ("Marie D.").
func publicNames(signups []model.SignupRow) string {
	names := make([]string, len(signups))
	for i, p := range signups {
		names[i] = model.DisplayName(p.Name)
	}
	return strings.Join(names, ", ")
}

// urgentSlots are the slots that need people most, for the top of the
// event page: those the visitor can sign up on and that are under their
// minimum, the emptiest first, then the earliest.
func urgentSlots(p *model.Planning, n int) []*model.SlotView {
	var out []*model.SlotView
	for _, st := range p.Stands {
		for _, s := range st.Slots {
			if s.CanSignUp && s.Missing > 0 {
				out = append(out, s)
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Missing != out[j].Missing {
			return out[i].Missing > out[j].Missing
		}
		return out[i].StartsAt.Before(out[j].StartsAt)
	})
	if len(out) > n {
		out = out[:n]
	}
	return out
}

// shareMessage is the text organisers paste in a group chat.
func shareMessage(ev *model.Event, publicURL string) string {
	var b strings.Builder
	b.WriteString("Bonjour à tous ! Pour « " + ev.Title + " »")
	if ev.Date != "" {
		b.WriteString(" du " + dateFR(ev.Date))
	}
	b.WriteString(", on cherche des bénévoles. Inscrivez-vous en quelques secondes, sans compte : " + publicURL + " Merci !")
	return b.String()
}

// pct is count as a percentage of needed, capped at 100.
func pct(count, needed int) int {
	if needed <= 0 || count >= needed {
		return 100
	}
	return count * 100 / needed
}

func i64(n int64) string { return strconv.FormatInt(n, 10) }

func boolStr(b bool) string { return strconv.FormatBool(b) }

func lower(s string) string { return strings.ToLower(s[:1]) + s[1:] }

// capitalize upper-cases the first letter: "samedi 19 juin" -> "Samedi 19 juin".
func capitalize(s string) string {
	if s == "" {
		return s
	}
	r, n := utf8.DecodeRuneInString(s)
	return string(unicode.ToUpper(r)) + s[n:]
}

// standStyle sets the --stand CSS variable to a stand's colour.
func standStyle(color string) templ.SafeCSS {
	return templ.SafeCSS("--stand: " + cssColor(color))
}

func signupURL(eventURL string, s *model.SlotView) templ.SafeURL {
	return templ.SafeURL(eventURL + "/slots/" + i64(s.ID) + "/signup")
}

// slotWhen is a slot's time, with its day when the event spans several.
func slotWhen(p *model.Planning, s *model.SlotView) string {
	t := hm(s.StartsAt) + " – " + hm(s.EndsAt)
	if p.MultiDay {
		return dayFR(s.StartsAt) + ", " + t
	}
	return t
}

// urgentText encourages the first volunteers of an empty slot.
func urgentText(s *model.SlotView) string {
	if s.Count == 0 {
		return itoa(s.Missing) + " " + plural(s.Missing, "place libre", "places libres") + ", soyez le premier"
	}
	return placesLeft(s.Missing)
}

// withOthers names who else is on one of the visitor's slots.
func withOthers(s *model.SlotView) string {
	var others []model.SignupRow
	for _, p := range s.Signups {
		if p.ID != s.MySignupID {
			others = append(others, p)
		}
	}
	if len(others) == 0 {
		return "Pour l'instant, personne d'autre"
	}
	return "Avec " + publicNames(others)
}

// groupShown says whether a time group has something to show under the
// "places à prendre" filter.
func groupShown(d eventPage, g model.TimeGroup) bool {
	if !d.TodoOnly {
		return true
	}
	for _, s := range g.Slots {
		if s.Missing > 0 {
			return true
		}
	}
	return false
}

// maxValue is a slot's maximum for an input: empty when there is none.
func maxValue(max int) string {
	if max <= 0 {
		return ""
	}
	return itoa(max)
}

func deleteSlotConfirm(s *model.SlotView) string {
	msg := "Supprimer ce créneau"
	if s.Count > 0 {
		msg += " et ses " + itoa(s.Count) + " " + plural(s.Count, "inscription", "inscriptions")
	}
	return msg + " ?"
}

// notifyNote completes "E-mails envoyés à …" when sign-ups are not notified.
func notifyNote(notify bool) string {
	if notify {
		return ""
	}
	return " (inscriptions non notifiées)"
}
