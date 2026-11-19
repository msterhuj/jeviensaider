package web

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"
)

func TestDuplicateNeedsCreatePassword(t *testing.T) {
	e := newEnv(t, Config{CreatePassword: "ape2026"})
	c := e.client()
	r := e.post(c, "/events", url.Values{"title": {"Fête"}, "admin_email": {"orga@example.org"}, "password": {"ape2026"}})
	adminPath := r.location
	expect(t, e.get(c, adminPath), 200, `name="password"`)
	expect(t, e.post(c, adminPath+"/duplicate", url.Values{"title": {"Copie"}}), http.StatusSeeOther)
	expect(t, e.get(c, adminPath), 200, "Mot de passe de création incorrect")
	r = e.post(c, adminPath+"/duplicate", url.Values{"title": {"Copie"}, "password": {"ape2026"}})
	if expect(t, r, http.StatusSeeOther); r.location == adminPath {
		t.Error("duplicate with the password refused")
	}
}

// Editing stands, slots and sign-ups from the admin page.
func TestAdminEditing(t *testing.T) {
	e := newEnv(t, Config{})
	admin := e.client()
	adminPath, pub, slot := openEventWithSlot(t, e, admin, url.Values{"title": {"Fête"}, "admin_email": {"orga@example.org"}})
	expect(t, e.post(admin, adminPath+"/stands", url.Values{"name": {"Tombola"}}), http.StatusSeeOther)
	ids := regexp.MustCompile(`id="stand-(\d+)"`).FindAllStringSubmatch(e.get(admin, adminPath).body, -1)
	buvette, tombola := ids[0][1], ids[1][1]

	// Rename a stand, then move it after the other one.
	expect(t, e.post(admin, adminPath+"/stands/"+buvette, url.Values{"name": {"Buvette et crêpes"}, "color": {"#2e9e6a"}}), http.StatusSeeOther)
	expect(t, e.post(admin, adminPath+"/stands/"+buvette+"/move", url.Values{"dir": {"down"}}), http.StatusSeeOther)
	body := e.get(admin, adminPath).body
	if i, j := strings.Index(body, `id="stand-`+tombola+`"`), strings.Index(body, `id="stand-`+buvette+`"`); i < 0 || j < 0 || i > j {
		t.Error("stand not moved down")
	}
	expect(t, e.get(e.client(), pub), 200, "Buvette et crêpes", "#2e9e6a")
	expect(t, e.post(admin, adminPath+"/stands/"+buvette, url.Values{"name": {""}}), http.StatusSeeOther)
	expect(t, e.get(admin, adminPath), 200, "Le nom du stand est obligatoire")

	// Change a slot's times and capacity; bad values are refused.
	expect(t, e.post(admin, adminPath+"/slots/"+slot, url.Values{"from": {"16:00"}, "to": {"15:00"}, "min": {"2"}}), http.StatusSeeOther)
	expect(t, e.get(admin, adminPath), 200, "L&#39;heure de fin doit être après")
	expect(t, e.post(admin, adminPath+"/slots/"+slot, url.Values{"from": {"14:00"}, "to": {"15:00"}, "min": {"3"}, "max": {"2"}}), http.StatusSeeOther)
	expect(t, e.get(admin, adminPath), 200, "Le maximum doit être vide ou supérieur")
	expect(t, e.post(admin, adminPath+"/slots/"+slot, url.Values{"from": {"14:30"}, "to": {"16:00"}, "min": {"3"}}), http.StatusSeeOther)
	expect(t, e.get(e.client(), pub), 200, "14h30", "Il reste 3 places")

	// Register someone by hand, then remove them.
	expect(t, e.post(admin, adminPath+"/signups", url.Values{"slot": {slot}, "name": {"Paul Martin"}, "phone": {"0611223344"}}), http.StatusSeeOther)
	body = e.get(admin, adminPath).body
	expect(t, resp{code: 200, body: body}, 200, "Paul Martin a été inscrit·e", "0611223344", "1/3")
	signup := regexp.MustCompile(`/signups/(\d+)/delete`).FindStringSubmatch(body)[1]
	expect(t, e.post(admin, adminPath+"/signups/"+signup+"/delete", nil), http.StatusSeeOther)
	expect(t, e.get(admin, adminPath), 200, "Inscription retirée", "0/3")

	// Delete the slot, then the stand.
	expect(t, e.post(admin, adminPath+"/slots/"+slot+"/delete", nil), http.StatusSeeOther)
	expect(t, e.get(e.client(), pub+"/slots/"+slot+"/signup"), 404)
	expect(t, e.post(admin, adminPath+"/stands/"+tombola+"/delete", nil), http.StatusSeeOther)
	if body := e.get(e.client(), pub).body; strings.Contains(body, "Tombola") {
		t.Error("deleted stand still shown")
	}
	// Another event's IDs are not found.
	expect(t, e.post(admin, adminPath+"/stands/999999/delete", nil), 404)
}

func TestHealthz(t *testing.T) {
	e := newEnv(t, Config{})
	expect(t, e.get(e.client(), "/healthz"), 200, "ok")
	e.store.Close()
	r := e.get(e.client(), "/healthz")
	expect(t, r, http.StatusServiceUnavailable, "database unavailable")
	if strings.Contains(r.body, "sql") {
		t.Errorf("database error leaked: %q", r.body)
	}
}
