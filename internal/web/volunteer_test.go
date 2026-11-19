package web

import (
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func TestDraftHidesSlotsAndClosedLocksDetails(t *testing.T) {
	e := newEnv(t, Config{})
	admin := e.client()
	adminPath, pub, slots := setupEvent(t, e, admin)

	expect(t, e.post(admin, adminPath+"/status", url.Values{"status": {"draft"}}), http.StatusSeeOther)
	r := e.get(e.client(), pub+"/slots/"+slots[0]+"/signup")
	expect(t, r, 404, "pas encore ouvertes")
	if strings.Contains(r.body, "Pêche aux canards") {
		t.Error("draft sign-up form shows the stand")
	}
	expect(t, e.post(e.client(), pub+"/slots/"+slots[0]+"/signup", url.Values{"name": {"Marie"}}), 404)

	expect(t, e.post(admin, adminPath+"/status", url.Values{"status": {"open"}}), http.StatusSeeOther)
	v := e.client()
	me := strings.TrimSuffix(e.post(v, pub+"/slots/"+slots[0]+"/signup", url.Values{"name": {"Marie"}}).location, "?welcome=1")
	expect(t, e.post(admin, adminPath+"/status", url.Values{"status": {"closed"}}), http.StatusSeeOther)
	expect(t, e.post(v, me, url.Values{"name": {"Someone Else"}}), http.StatusSeeOther)
	expect(t, e.get(v, me), 200, "Les inscriptions sont closes", `value="Marie"`)
}

func TestRenewPersonalLink(t *testing.T) {
	e := newEnv(t, Config{})
	_, pub, slots := setupEvent(t, e, e.client())
	v := e.client()
	old := strings.TrimSuffix(e.post(v, pub+"/slots/"+slots[0]+"/signup", url.Values{"name": {"Marie"}}).location, "?welcome=1")
	expect(t, e.get(v, old), 200, "en créer un nouveau")
	r := e.post(v, old+"/renew", nil)
	expect(t, r, http.StatusSeeOther)
	if r.location == old || !strings.HasPrefix(r.location, pub+"/me/") {
		t.Fatalf("new link %q", r.location)
	}
	expect(t, e.get(e.client(), old), 404)
	expect(t, e.get(v, r.location), 200, "nouveau lien personnel", "Marie")
	// The browser's cookie follows the new link.
	expect(t, e.get(v, pub), 200, r.location)
}

// A volunteer downloads their own slots as a calendar file.
func TestPersonalCalendarExport(t *testing.T) {
	e := newEnv(t, Config{})
	_, pub, slots := setupEvent(t, e, e.client())
	v := e.client()
	me := strings.TrimSuffix(e.post(v, pub+"/slots/"+slots[0]+"/signup", url.Values{"name": {"Marie"}}).location, "?welcome=1")
	// The cookie of v makes this a second slot of the same volunteer.
	expect(t, e.post(v, pub+"/slots/"+slots[1]+"/signup", url.Values{"name": {"Marie"}}), http.StatusSeeOther)
	// Another volunteer takes the third slot, and the first one with Marie.
	// The third slot must not be in Marie's file.
	paul := e.client()
	paulMe := strings.TrimSuffix(e.post(paul, pub+"/slots/"+slots[2]+"/signup", url.Values{"name": {"Paul"}}).location, "?welcome=1")
	expect(t, e.post(paul, pub+"/slots/"+slots[0]+"/signup", url.Values{"name": {"Paul"}}), http.StatusSeeOther)

	page := e.get(v, me)
	expect(t, page, 200, "Ajouter à mon agenda", me+"/agenda.ics")

	r := e.get(v, me+"/agenda.ics")
	expect(t, r, 200, "BEGIN:VCALENDAR\r\n", "END:VCALENDAR\r\n",
		"SUMMARY:Pêche aux canards (Fête de l'école)", "LOCATION:Cour", "URL:http", pub,
		"DTSTART:20260620T140000", "DTEND:20260620T150000",
		"DTSTART:20260620T150000", "DTEND:20260620T160000",
		"TRIGGER:-P1D", "TRIGGER:-PT1H")
	if n := strings.Count(r.body, "BEGIN:VEVENT"); n != 2 {
		t.Errorf("want 2 events, got %d:\n%s", n, r.body)
	}
	if strings.Contains(r.body, "DTSTART:20260620T160000") {
		t.Error("the file holds a slot of another volunteer")
	}
	if ct := r.header.Get("Content-Type"); ct != "text/calendar; charset=utf-8" {
		t.Errorf("Content-Type = %q", ct)
	}
	if cd := r.header.Get("Content-Disposition"); !strings.HasPrefix(cd, "attachment;") || !strings.Contains(cd, ".ics") {
		t.Errorf("Content-Disposition = %q", cd)
	}
	if cc := r.header.Get("Cache-Control"); cc != "no-store" {
		t.Errorf("Cache-Control = %q", cc)
	}
	// The secret link must not leak into a file that people can share.
	token := me[strings.LastIndex(me, "/")+1:]
	if strings.Contains(r.body, token) {
		t.Error("the calendar file contains the personal token")
	}

	// The same sign-up always has the same UID, so a second import updates
	// the calendar entry instead of adding one.
	uid := regexp.MustCompile(`UID:[^\r\n]+`)
	if a, b := uid.FindAllString(r.body, -1), uid.FindAllString(e.get(v, me+"/agenda.ics").body, -1); strings.Join(a, "|") != strings.Join(b, "|") {
		t.Errorf("UIDs change between downloads: %v / %v", a, b)
	}

	// Each sign-up has its own UID. Otherwise a calendar merges two slots of
	// one volunteer, or the slots of two volunteers, into one entry.
	marie := uid.FindAllString(r.body, -1)
	if len(marie) != 2 || marie[0] == marie[1] {
		t.Errorf("two slots of one volunteer must have two UIDs: %v", marie)
	}
	pr := e.get(paul, paulMe+"/agenda.ics")
	expect(t, pr, 200, "DTSTART:20260620T140000", "DTSTART:20260620T160000")
	for _, p := range uid.FindAllString(pr.body, -1) {
		for _, m := range marie {
			if p == m {
				t.Errorf("two volunteers share the UID %q", p)
			}
		}
	}
}

// A draft is not public, so its calendar file is not either: the sign-up form
// gives the same answer. A closed event keeps the sign-ups that it has.
func TestPersonalCalendarExportFollowsEventStatus(t *testing.T) {
	e := newEnv(t, Config{})
	admin := e.client()
	adminPath, pub, slots := setupEvent(t, e, admin)
	v := e.client()
	me := strings.TrimSuffix(e.post(v, pub+"/slots/"+slots[0]+"/signup", url.Values{"name": {"Marie"}}).location, "?welcome=1")

	expect(t, e.post(admin, adminPath+"/status", url.Values{"status": {"closed"}}), http.StatusSeeOther)
	expect(t, e.get(v, me+"/agenda.ics"), 200, "BEGIN:VEVENT")
	expect(t, e.get(v, me), 200, "Ajouter à mon agenda")

	expect(t, e.post(admin, adminPath+"/status", url.Values{"status": {"draft"}}), http.StatusSeeOther)
	expect(t, e.get(v, me+"/agenda.ics"), 404, "pas encore ouvertes")
	if body := e.get(v, me).body; strings.Contains(body, "agenda.ics") {
		t.Error("the page offers a calendar file for a draft event")
	}
}

func TestPersonalCalendarExportErrors(t *testing.T) {
	e := newEnv(t, Config{})
	_, pub, slots := setupEvent(t, e, e.client())
	// A wrong link is refused like the personal page.
	expect(t, e.get(e.client(), pub+"/me/nope/agenda.ics"), 404)

	// Without any slot there is nothing to export, and no button is shown.
	v := e.client()
	me := strings.TrimSuffix(e.post(v, pub+"/slots/"+slots[0]+"/signup", url.Values{"name": {"Marie"}}).location, "?welcome=1")
	id := regexp.MustCompile(`/signups/(\d+)/delete`).FindStringSubmatch(e.get(v, me).body)[1]
	expect(t, e.post(v, me+"/signups/"+id+"/delete", nil), http.StatusSeeOther)
	expect(t, e.get(v, me+"/agenda.ics"), 404, "aucun créneau")
	page := e.get(v, me)
	expect(t, page, 200, "êtes inscrit·e sur aucun créneau")
	if strings.Contains(page.body, "agenda.ics") {
		t.Error("the page offers a calendar file to a volunteer without slots")
	}
}

// A volunteer manages their own details and sign-ups with their link.
func TestVolunteerSelfService(t *testing.T) {
	e := newEnv(t, Config{})
	_, pub, slots := setupEvent(t, e, e.client())
	v := e.client()
	me := strings.TrimSuffix(e.post(v, pub+"/slots/"+slots[0]+"/signup", url.Values{"name": {"Marie Dupont"}}).location, "?welcome=1")

	expect(t, e.post(v, me, url.Values{"name": {"Marie Durand"}, "phone": {"06 01 02 03 04"}}), http.StatusSeeOther)
	expect(t, e.get(v, me), 200, "Vos coordonnées ont été mises à jour", `value="Marie Durand"`, `value="06 01 02 03 04"`)
	expect(t, e.post(v, me, url.Values{"name": {"Marie"}, "phone": {"call me"}}), http.StatusSeeOther)
	expect(t, e.get(v, me), 200, "ne semble pas valide", `value="Marie Durand"`)

	// Someone else's sign-up cannot be cancelled with my link.
	other := strings.TrimSuffix(e.post(e.client(), pub+"/slots/"+slots[1]+"/signup", url.Values{"name": {"Paul"}}).location, "?welcome=1")
	theirs := regexp.MustCompile(`/signups/(\d+)/delete`).FindStringSubmatch(e.get(e.client(), other).body)[1]
	expect(t, e.post(v, me+"/signups/"+theirs+"/delete", nil), 404)

	// "Ce n'est pas moi": the device forgets me, the link still works.
	expect(t, e.get(v, pub), 200, "Mes créneaux")
	expect(t, e.post(v, pub+"/forget", nil), http.StatusSeeOther)
	if body := e.get(v, pub).body; strings.Contains(body, me) {
		t.Error("still recognised after forget")
	}
	expect(t, e.get(e.client(), me), 200, "Marie Durand")
}

// Stand descriptions are on the overview (under the name, no stand tab
// any more) and in full on the sign-up form.
func TestStandDescriptions(t *testing.T) {
	e := newEnv(t, Config{})
	admin := e.client()
	adminPath, pub, slots := setupEvent(t, e, admin)
	standID := regexp.MustCompile(`id="stand-(\d+)"`).FindStringSubmatch(e.get(admin, adminPath).body)[1]
	desc := "Les enfants pêchent des canards ; un lot pour chacun."
	expect(t, e.post(admin, adminPath+"/stands/"+standID, url.Values{"name": {"Pêche aux canards"}, "description": {desc}, "color": {"#2aa3b5"}}), http.StatusSeeOther)

	r := e.get(e.client(), pub)
	expect(t, r, 200, `<span class="stand-desc">Les enfants pêchent des canards ; un lot pour chacun.</span>`,
		`<a class="stand-more" href="`+pub+`/stands/`+standID+`"`)
	if strings.Contains(r.body, "?stand=") || strings.Contains(r.body, "tabs-stands") {
		t.Error("stand tabs still shown")
	}
	// Old links to a stand tab still show the overview.
	expect(t, e.get(e.client(), pub+"?stand="+standID), 200, "stand-desc")
	expect(t, e.get(e.client(), pub+"/slots/"+slots[0]+"/signup"), 200, `<p class="desc signup-desc">Les enfants pêchent des canards ; un lot pour chacun.</p>`)
	// "Voir plus": the stand's description in a dialog (htmx) or a page.
	r = e.do(e.client(), "GET", pub+"/stands/"+standID, nil, map[string]string{"HX-Request": "true"})
	expect(t, r, 200, "<dialog", "Pêche aux canards", desc, "data-close autofocus>Fermer</button>")
	if strings.Contains(r.body, "<form") || strings.Contains(r.body, "<html") {
		t.Error("stand dialog has a form or a whole page")
	}
	expect(t, e.get(e.client(), pub+"/stands/"+standID), 200, "<html", "Tous les créneaux", desc)
	expect(t, e.get(e.client(), pub+"/stands/999999"), 404, "Stand introuvable")
	expect(t, e.post(admin, adminPath+"/status", url.Values{"status": {"draft"}}), http.StatusSeeOther)
	expect(t, e.get(e.client(), pub+"/stands/"+standID), 404, "pas encore ouvertes")
	expect(t, e.post(admin, adminPath+"/status", url.Values{"status": {"open"}}), http.StatusSeeOther)

	// The views bar only appears for a known volunteer.
	if strings.Contains(r.body, "Vue d&#39;ensemble") || strings.Contains(r.body, "Vue d'ensemble") {
		t.Error("views bar shown to an unknown visitor")
	}
}

func TestOverlapWarningAndMyPlanning(t *testing.T) {
	e := newEnv(t, Config{})
	admin := e.client()
	adminPath, publicPath, slots := setupEvent(t, e, admin) // 14h, 15h, 16h on one stand
	marie := e.client()
	form := url.Values{"name": {"Marie Dupont"}}
	r := e.post(marie, publicPath+"/slots/"+slots[0]+"/signup", form)
	expect(t, r, http.StatusSeeOther)
	meLink := strings.TrimSuffix(r.location, "?welcome=1")

	// "Mes créneaux" lists my slots only.
	r = e.get(marie, meLink)
	expect(t, r, 200, "Mes créneaux", "1 créneau · 1 h · 1 stand")
	// Only my slots: the stand's 15h and 16h slots are not listed, and mine
	// can be cancelled.
	if mine := mySlotsHTML(r.body); !strings.Contains(mine, "14h – 15h") || strings.Contains(mine, "15h – 16h") || strings.Contains(mine, "16h – 17h") {
		t.Errorf("my slots are wrong: %s", mine)
	}
	expect(t, r, 200, `class="cancel-form"`, "Annuler votre inscription : Pêche aux canards, 14h – 15h ?")
	// The next slot does not overlap: no warning.
	if strings.Contains(e.get(marie, publicPath+"/slots/"+slots[1]+"/signup").body, "même horaire") {
		t.Error("warning shown for a non-overlapping slot")
	}
	expect(t, e.post(marie, publicPath+"/slots/"+slots[1]+"/signup", form), http.StatusSeeOther)
	expect(t, e.get(marie, meLink), 200, "2 créneaux · 2 h · 1 stand")

	// A second stand with a 14:30-15:30 slot, overlapping both.
	expect(t, e.post(admin, adminPath+"/stands", url.Values{"name": {"Buvette"}}), http.StatusSeeOther)
	ids := regexp.MustCompile(`id="stand-(\d+)"`).FindAllStringSubmatch(e.get(admin, adminPath).body, -1)
	buvette := ids[len(ids)-1][1]
	expect(t, e.post(admin, adminPath+"/stands/"+buvette+"/slots", url.Values{
		"date": {"2026-06-20"}, "from": {"14:30"}, "to": {"15:30"}, "step": {"0"}, "min": {"1"},
	}), http.StatusSeeOther)
	newest := 0 // the Buvette slot is the most recent one
	for _, m := range regexp.MustCompile(`/slots/(\d+)/signup`).FindAllStringSubmatch(e.get(marie, publicPath).body, -1) {
		if n, _ := strconv.Atoi(m[1]); n > newest {
			newest = n
		}
	}
	buvetteSlot := strconv.Itoa(newest)
	r = e.get(marie, publicPath+"/slots/"+buvetteSlot+"/signup")
	expect(t, r, 200, "même horaire", "Pêche aux canards", "14h – 15h", "15h – 16h", "M'inscrire quand même")

	// Warn, never block: the sign-up goes through, and the grid flags it.
	expect(t, e.post(marie, publicPath+"/slots/"+buvetteSlot+"/signup", form), http.StatusSeeOther)
	r = e.get(marie, meLink)
	expect(t, r, 200, "3 créneaux", "2 stands", "créneaux en même temps", "my-slot clash", "⚠ En même temps")
	expect(t, e.get(marie, publicPath), 200, "mine clash", "⚠ en même temps")
	// Only my stands: a third stand where I am not signed up is not shown.
	expect(t, e.post(admin, adminPath+"/stands", url.Values{"name": {"Tombola"}}), http.StatusSeeOther)
	ids = regexp.MustCompile(`id="stand-(\d+)"`).FindAllStringSubmatch(e.get(admin, adminPath).body, -1)
	expect(t, e.post(admin, adminPath+"/stands/"+ids[len(ids)-1][1]+"/slots", url.Values{
		"date": {"2026-06-20"}, "from": {"10:00"}, "to": {"11:00"}, "step": {"0"}, "min": {"1"},
	}), http.StatusSeeOther)
	r = e.get(marie, meLink)
	if strings.Contains(mySlotsHTML(r.body), "Tombola") {
		t.Error("a stand without my sign-up is in my planning")
	}
	expect(t, e.get(marie, publicPath), 200, "Tombola")
}

// mySlotsHTML is the "Mes créneaux" list of a personal page.
func mySlotsHTML(body string) string {
	i := strings.Index(body, `id="my-slots"`)
	if i < 0 {
		return ""
	}
	body = body[i:]
	if j := strings.Index(body, "</section>"); j >= 0 {
		body = body[:j]
	}
	return body
}
