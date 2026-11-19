package web

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"testing"

	"benevole-ape/internal/mail"
)

// fakeMailer records the messages instead of sending them.
type fakeMailer struct {
	mu   sync.Mutex
	msgs []mail.Message
}

func (f *fakeMailer) Enqueue(m mail.Message) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.msgs = append(f.msgs, m)
}

func (f *fakeMailer) take() []mail.Message {
	f.mu.Lock()
	defer f.mu.Unlock()
	m := f.msgs
	f.msgs = nil
	return m
}

func TestAdminEmailRequired(t *testing.T) {
	e := newEnv(t, Config{})
	c := e.client()
	expect(t, e.get(c, "/"), 200, `name="admin_email"`, "required")
	expect(t, e.post(c, "/events", url.Values{"title": {"Fête"}}), http.StatusBadRequest, "Indiquez votre e-mail")
	expect(t, e.post(c, "/events", url.Values{"title": {"Fête"}, "admin_email": {"x@y.fr?bcc=z@evil.fr"}}), http.StatusBadRequest, "ne semble pas valide")
}

func TestOrganiserEmails(t *testing.T) {
	fm := &fakeMailer{}
	e := newEnv(t, Config{Mailer: fm, MailEnabled: true, BaseURL: "https://jeviensaider.example"})
	admin := e.client()

	// Creation: the organiser receives the admin and public links.
	r := e.post(admin, "/events", url.Values{"title": {"Fête de l'école"}, "date": {"2026-06-20"}, "admin_email": {"orga@example.org"}, "notify": {"1"}})
	expect(t, r, http.StatusSeeOther)
	adminPath := r.location
	expect(t, e.get(admin, adminPath), 200, "il vient de vous être envoyé à orga@example.org", "E-mails de l'organisateur envoyés à orga@example.org")
	msgs := fm.take()
	if len(msgs) != 1 {
		t.Fatalf("%d messages after creation", len(msgs))
	}
	m := msgs[0]
	if m.To != "orga@example.org" || !strings.Contains(m.Subject, "Fête de l'école") ||
		!strings.Contains(m.Body, "https://jeviensaider.example"+adminPath) || !strings.Contains(m.Body, "https://jeviensaider.example/e/") {
		t.Fatalf("creation mail: %+v", m)
	}

	// Build a stand with a slot and open the event.
	expect(t, e.post(admin, adminPath+"/stands", url.Values{"name": {"Buvette"}}), http.StatusSeeOther)
	standID := regexp.MustCompile(`id="stand-(\d+)"`).FindStringSubmatch(e.get(admin, adminPath).body)[1]
	expect(t, e.post(admin, adminPath+"/stands/"+standID+"/slots", url.Values{"date": {"2026-06-20"}, "from": {"14:00"}, "to": {"15:00"}, "step": {"60"}, "min": {"2"}}), http.StatusSeeOther)
	expect(t, e.post(admin, adminPath+"/status", url.Values{"status": {"open"}}), http.StatusSeeOther)
	pub := regexp.MustCompile(`href="(/e/[^"]+)" target="_blank"`).FindStringSubmatch(e.get(admin, adminPath).body)[1]
	slot := regexp.MustCompile(`/slots/(\d+)/signup`).FindStringSubmatch(e.get(e.client(), pub).body)[1]
	fm.take()

	// Sign-up: the organiser is told who, when, and what is still missing.
	marie := e.client()
	r = e.post(marie, pub+"/slots/"+slot+"/signup", url.Values{"name": {"Marie Dupont"}, "phone": {"06 12 34 56 78"}, "comment": {"Je viens avec ma fille"}})
	expect(t, r, http.StatusSeeOther)
	msgs = fm.take()
	if len(msgs) != 1 {
		t.Fatalf("%d messages after sign-up", len(msgs))
	}
	m = msgs[0]
	for _, want := range []string{"Marie Dupont s'est inscrit·e sur « Buvette »", "06 12 34 56 78", "Je viens avec ma fille", "Ce créneau : 1/2, il reste 1 place", "https://jeviensaider.example" + adminPath} {
		if !strings.Contains(m.Body, want) {
			t.Errorf("sign-up mail lacks %q:\n%s", want, m.Body)
		}
	}
	if m.Subject != "Inscription : Marie Dupont – Buvette, 14h – 15h" {
		t.Errorf("subject %q", m.Subject)
	}

	// Cancellation.
	meLink := strings.TrimSuffix(r.location, "?welcome=1")
	signupID := regexp.MustCompile(`/signups/(\d+)/delete`).FindStringSubmatch(e.get(marie, meLink).body)[1]
	expect(t, e.post(marie, meLink+"/signups/"+signupID+"/delete", url.Values{}), http.StatusSeeOther)
	msgs = fm.take()
	if len(msgs) != 1 || !strings.HasPrefix(msgs[0].Subject, "Désinscription : Marie Dupont") || !strings.Contains(msgs[0].Body, "0/2, il reste 2 places") {
		t.Fatalf("cancellation mail: %+v", msgs)
	}

	// The organiser turns notifications off: no more messages.
	expect(t, e.post(admin, adminPath+"/event", url.Values{"title": {"Fête de l'école"}, "date": {"2026-06-20"}, "admin_email": {"orga@example.org"}}), http.StatusSeeOther)
	expect(t, e.post(e.client(), pub+"/slots/"+slot+"/signup", url.Values{"name": {"Paul"}}), http.StatusSeeOther)
	if msgs := fm.take(); len(msgs) != 0 {
		t.Fatalf("messages sent although notifications are off: %+v", msgs)
	}

	// The e-mail stays mandatory when editing the event.
	expect(t, e.post(admin, adminPath+"/event", url.Values{"title": {"Fête de l'école"}, "admin_email": {""}}), http.StatusSeeOther)
	expect(t, e.get(admin, adminPath), 200, "Indiquez votre e-mail")

	// Duplicating sends the new admin link.
	expect(t, e.post(admin, adminPath+"/duplicate", url.Values{"title": {"Fête 2027"}}), http.StatusSeeOther)
	if msgs := fm.take(); len(msgs) != 1 || !strings.Contains(msgs[0].Subject, "Fête 2027") {
		t.Fatalf("duplicate mail: %+v", msgs)
	}
}

// The organiser can have the links e-mailed again, for instance to a new
// address; the per-address limit still applies.
func TestResendLinks(t *testing.T) {
	fm := &fakeMailer{}
	e := newEnv(t, Config{Mailer: fm, MailEnabled: true, BaseURL: "https://jeviensaider.example"})
	admin := e.client()
	r := e.post(admin, "/events", url.Values{"title": {"Fête"}, "date": {"2026-06-20"}, "admin_email": {"orga@example.org"}})
	adminPath := r.location
	expect(t, e.get(admin, adminPath), 200, "Renvoyer les liens par e-mail")
	fm.take() // the creation mail

	// The organiser's address changed: the links go to the new one.
	expect(t, e.post(admin, adminPath+"/event", url.Values{"title": {"Fête"}, "date": {"2026-06-20"}, "admin_email": {"new@example.org"}}), http.StatusSeeOther)
	expect(t, e.post(admin, adminPath+"/resend", nil), http.StatusSeeOther)
	expect(t, e.get(admin, adminPath), 200, "Les liens de l&#39;événement ont été envoyés à new@example.org")
	msgs := fm.take()
	if len(msgs) != 1 {
		t.Fatalf("%d messages", len(msgs))
	}
	m := msgs[0]
	for _, want := range []string{"envoyés depuis sa page d'administration", "https://jeviensaider.example" + adminPath, "Lien public, à partager avec les bénévoles :\nhttps://jeviensaider.example/e/", "Si vous n'organisez pas cet événement"} {
		if !strings.Contains(m.Body, want) {
			t.Errorf("resent mail lacks %q:\n%s", want, m.Body)
		}
	}
	if m.To != "new@example.org" || m.Subject != "Les liens de votre événement « Fête »" || strings.Contains(m.Body, "Prochaines étapes") {
		t.Errorf("resent mail: %+v", m)
	}

	// The per-address limit (3 a day) counts resends.
	expect(t, e.post(admin, adminPath+"/resend", nil), http.StatusSeeOther)
	expect(t, e.post(admin, adminPath+"/resend", nil), http.StatusSeeOther)
	expect(t, e.post(admin, adminPath+"/resend", nil), http.StatusSeeOther)
	expect(t, e.get(admin, adminPath), 200, "Trop d&#39;envois récents vers new@example.org")
	if n := len(fm.take()); n != 2 {
		t.Errorf("%d more mails, want 2", n)
	}
}

func TestResendLinksWithoutSMTP(t *testing.T) {
	e := newEnv(t, Config{Mailer: &fakeMailer{}}) // mails are only logged
	admin := e.client()
	adminPath := e.post(admin, "/events", url.Values{"title": {"Fête"}, "admin_email": {"orga@example.org"}}).location
	if strings.Contains(e.get(admin, adminPath).body, "Renvoyer les liens") {
		t.Error("resend button shown without SMTP")
	}
	expect(t, e.post(admin, adminPath+"/resend", nil), http.StatusSeeOther)
	expect(t, e.get(admin, adminPath), 200, "pas configuré sur ce serveur")
}
