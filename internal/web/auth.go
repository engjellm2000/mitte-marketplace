package web

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"net/mail"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"mitte/internal/store"

	"golang.org/x/crypto/argon2"
)

const authCookieName = "mitte_auth"
const staffCookieName = "mitte_staff"

const (
	argonMemory      uint32 = 64 * 1024
	argonIterations  uint32 = 3
	argonParallelism uint8  = 2
	argonKeyLength   uint32 = 32
)

const verificationTokenLifetime = 24 * time.Hour
const passwordResetTokenLifetime = 30 * time.Minute

func (a *App) register(w http.ResponseWriter, r *http.Request) {
	if !a.allowAuthMutation(w, r) {
		return
	}
	var input struct {
		Email        string `json:"email"`
		Password     string `json:"password"`
		CustomerType string `json:"customerType"`
	}
	if err := decodeJSON(w, r, &input, 1024); err != nil || !validAuthEmail(input.Email) || !validPassword(input.Password) {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Bitte verwende eine gültige E-Mail-Adresse und ein Passwort mit mindestens 12 Zeichen."})
		return
	}
	input.CustomerType = strings.TrimSpace(input.CustomerType)
	if input.CustomerType == "" {
		input.CustomerType = "b2c"
	}
	if !validCustomerType(input.CustomerType) {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Bitte wähle Privatperson oder Geschäftskunde aus."})
		return
	}
	passwordHash, err := hashPassword(input.Password)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	token, tokenHash, err := newAuthToken()
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	email := strings.ToLower(strings.TrimSpace(input.Email))
	body := "Bitte bestätige deine E-Mail-Adresse, indem du diesen Link öffnest:\n\n" +
		a.authEmailLink("/konto", "verify_email", token) +
		"\n\nDer Link ist 24 Stunden gültig. Falls du kein Kundenkonto erstellt hast, kannst du diese E-Mail ignorieren."
	_, err = a.store.CreateUserWithVerification(r.Context(), email, passwordHash, input.CustomerType,
		tokenHash, time.Now().UTC().Add(verificationTokenLifetime), "Bitte bestätige deine E-Mail-Adresse", body)
	if errors.Is(err, store.ErrEmailInUse) {
		respondJSON(w, http.StatusAccepted, map[string]string{"message": registrationMessage})
		return
	}
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	respondJSON(w, http.StatusAccepted, map[string]string{"message": registrationMessage})
}

const registrationMessage = "Falls du ein neues Konto erstellt hast, prüfe dein Postfach und bestätige deine E-Mail-Adresse. Bestehende Konten können sich anmelden oder ihr Passwort zurücksetzen."

func (a *App) login(w http.ResponseWriter, r *http.Request) {
	if !a.allowAuthMutation(w, r) {
		return
	}
	var input struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := decodeJSON(w, r, &input, 1024); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Ungültige Anfrage."})
		return
	}
	if !a.loginLimiter.allow(strings.ToLower(strings.TrimSpace(input.Email))) {
		w.Header().Set("Retry-After", "60")
		respondJSON(w, http.StatusTooManyRequests, map[string]string{"error": "Zu viele Anmeldeversuche. Bitte warte kurz."})
		return
	}
	user, passwordHash, err := a.store.AuthenticateUser(r.Context(), input.Email)
	if errors.Is(err, store.ErrInvalidCredentials) {
		// Keep unknown-email and bad-password requests comparably expensive.
		_, _ = hashPassword(input.Password)
		respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "E-Mail-Adresse oder Passwort ist nicht korrekt."})
		return
	}
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	if !verifyPassword(input.Password, passwordHash) {
		respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "E-Mail-Adresse oder Passwort ist nicht korrekt."})
		return
	}
	if !user.EmailVerified {
		respondJSON(w, http.StatusForbidden, map[string]string{"error": "Bitte bestätige zuerst deine E-Mail-Adresse. Du kannst dir einen neuen Bestätigungslink senden lassen."})
		return
	}
	if err := a.startUserSession(w, r, user); err != nil {
		a.serverError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"user": user})
}

func (a *App) resendEmailVerification(w http.ResponseWriter, r *http.Request) {
	if !a.allowAuthMutation(w, r) {
		return
	}
	var input struct {
		Email string `json:"email"`
	}
	if err := decodeJSON(w, r, &input, 512); err != nil || !validAuthEmail(input.Email) {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Bitte gib eine gültige E-Mail-Adresse ein."})
		return
	}
	email := strings.ToLower(strings.TrimSpace(input.Email))
	message := "Falls für diese Adresse ein unbestätigtes Konto besteht, senden wir einen neuen Bestätigungslink."
	if !a.loginLimiter.allow("verify:" + email) {
		respondJSON(w, http.StatusAccepted, map[string]string{"message": message})
		return
	}
	token, tokenHash, err := newAuthToken()
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	body := "Bitte bestätige deine E-Mail-Adresse, indem du diesen Link öffnest:\n\n" +
		a.authEmailLink("/konto", "verify_email", token) +
		"\n\nDer Link ist 24 Stunden gültig. Falls du kein Kundenkonto erstellt hast, kannst du diese E-Mail ignorieren."
	if _, err := a.store.IssueUserAuthToken(r.Context(), email, "verify_email", tokenHash,
		time.Now().UTC().Add(verificationTokenLifetime), "Bitte bestätige deine E-Mail-Adresse", body); err != nil {
		a.serverError(w, r, err)
		return
	}
	respondJSON(w, http.StatusAccepted, map[string]string{"message": message})
}

func (a *App) verifyEmail(w http.ResponseWriter, r *http.Request) {
	if !a.allowAuthMutation(w, r) {
		return
	}
	var input struct {
		Token string `json:"token"`
	}
	if err := decodeJSON(w, r, &input, 512); err != nil {
		return
	}
	tokenHash, ok := authTokenHash(input.Token)
	if !ok {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Der Bestätigungslink ist ungültig oder abgelaufen."})
		return
	}
	if err := a.store.VerifyUserEmail(r.Context(), tokenHash, time.Now().UTC()); errors.Is(err, store.ErrInvalidAuthToken) {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Der Bestätigungslink ist ungültig oder abgelaufen."})
		return
	} else if err != nil {
		a.serverError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"message": "Deine E-Mail-Adresse wurde bestätigt. Du kannst dich jetzt anmelden."})
}

func (a *App) requestPasswordReset(w http.ResponseWriter, r *http.Request) {
	if !a.allowAuthMutation(w, r) {
		return
	}
	var input struct {
		Email string `json:"email"`
	}
	if err := decodeJSON(w, r, &input, 512); err != nil || !validAuthEmail(input.Email) {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Bitte gib eine gültige E-Mail-Adresse ein."})
		return
	}
	email := strings.ToLower(strings.TrimSpace(input.Email))
	message := "Falls ein Konto mit dieser Adresse besteht, senden wir einen Link zum Zurücksetzen des Passworts."
	if !a.loginLimiter.allow("reset:" + email) {
		respondJSON(w, http.StatusAccepted, map[string]string{"message": message})
		return
	}
	token, tokenHash, err := newAuthToken()
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	body := "Zum Zurücksetzen deines Passworts öffne diesen Link:\n\n" +
		a.authEmailLink("/konto", "reset_password", token) +
		"\n\nDer Link ist 30 Minuten gültig. Falls du das Zurücksetzen nicht angefordert hast, kannst du diese E-Mail ignorieren."
	if _, err := a.store.IssueUserAuthToken(r.Context(), email, "password_reset", tokenHash,
		time.Now().UTC().Add(passwordResetTokenLifetime), "Passwort zurücksetzen", body); err != nil {
		a.serverError(w, r, err)
		return
	}
	respondJSON(w, http.StatusAccepted, map[string]string{"message": message})
}

func (a *App) completePasswordReset(w http.ResponseWriter, r *http.Request) {
	if !a.allowAuthMutation(w, r) {
		return
	}
	var input struct {
		Token       string `json:"token"`
		NewPassword string `json:"newPassword"`
	}
	if err := decodeJSON(w, r, &input, 1024); err != nil || !validPassword(input.NewPassword) {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Das neue Passwort muss 12 bis 128 Zeichen lang sein."})
		return
	}
	tokenHash, ok := authTokenHash(input.Token)
	if !ok {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Der Link zum Zurücksetzen ist ungültig oder abgelaufen."})
		return
	}
	passwordHash, err := hashPassword(input.NewPassword)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	if err := a.store.ResetUserPassword(r.Context(), tokenHash, passwordHash, time.Now().UTC()); errors.Is(err, store.ErrInvalidAuthToken) {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Der Link zum Zurücksetzen ist ungültig oder abgelaufen."})
		return
	} else if err != nil {
		a.serverError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"message": "Dein Passwort wurde geändert. Du kannst dich jetzt anmelden."})
}

func (a *App) logout(w http.ResponseWriter, r *http.Request) {
	if !a.allowMutation(w, r) {
		return
	}
	if token := a.authToken(r); token != "" {
		if err := a.store.RevokeUserSession(r.Context(), token, time.Now().UTC()); err != nil {
			a.serverError(w, r, err)
			return
		}
	}
	a.clearAuthCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) authEmailLink(path, queryKey, token string) string {
	return strings.TrimRight(a.invoicePublicBaseURL, "/") + path + "?" + queryKey + "=" + url.QueryEscape(token)
}

func newAuthToken() (string, []byte, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", nil, fmt.Errorf("generate authentication token: %w", err)
	}
	digest := sha256.Sum256(value)
	return hex.EncodeToString(value), digest[:], nil
}

func authTokenHash(token string) ([]byte, bool) {
	value, err := hex.DecodeString(token)
	if err != nil || len(value) != 32 {
		return nil, false
	}
	digest := sha256.Sum256(value)
	return digest[:], true
}

func (a *App) staffLogin(w http.ResponseWriter, r *http.Request) {
	if !a.allowAuthMutation(w, r) {
		return
	}
	var input struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := decodeJSON(w, r, &input, 1024); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Ungültige Anfrage."})
		return
	}
	username := strings.ToLower(strings.TrimSpace(input.Username))
	if !a.loginLimiter.allow("staff:" + username) {
		w.Header().Set("Retry-After", "60")
		respondJSON(w, http.StatusTooManyRequests, map[string]string{"error": "Zu viele Anmeldeversuche. Bitte warte kurz."})
		return
	}
	password, exists := a.staffCredentials[username]
	validPassword := exists && staffPasswordMatches(input.Password, password)
	if !validPassword {
		respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "Benutzername oder Passwort ist nicht korrekt."})
		return
	}
	if err := a.startStaffSession(w, r, username); err != nil {
		a.serverError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"staff": a.staffIdentity(username)})
}

func staffPasswordMatches(password, credential string) bool {
	if strings.HasPrefix(credential, "$argon2id$") {
		return verifyPassword(password, credential)
	}
	// Plaintext credentials are accepted only for local development. Production
	// configuration rejects them before the web server can start.
	return subtle.ConstantTimeCompare([]byte(password), []byte(credential)) == 1
}

func (a *App) staffLogout(w http.ResponseWriter, r *http.Request) {
	if !a.allowMutation(w, r) {
		return
	}
	if token := a.staffSessionToken(r); token != "" {
		if a.sessions == nil {
			a.serverError(w, r, errors.New("operator session store is not configured"))
			return
		}
		if err := a.sessions.RevokeOperatorSession(r.Context(), token, time.Now().UTC()); err != nil {
			a.serverError(w, r, err)
			return
		}
	}
	a.clearStaffCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) staffMe(w http.ResponseWriter, r *http.Request) {
	username, err := a.currentStaffUsername(r)
	if errors.Is(err, store.ErrInvalidSession) {
		respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "Nicht als Mitarbeitende:r angemeldet."})
		return
	}
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"staff": a.staffIdentity(username)})
}

func (a *App) currentUser(w http.ResponseWriter, r *http.Request) (store.User, bool) {
	token := a.authToken(r)
	if token == "" {
		return store.User{}, false
	}
	user, err := a.store.UserBySession(r.Context(), token, time.Now().UTC())
	if errors.Is(err, store.ErrInvalidSession) {
		a.clearAuthCookie(w)
		return store.User{}, false
	}
	if err != nil {
		a.logger.Error("session lookup failed", "request_id", requestID(r.Context()), "error", err)
		return store.User{}, false
	}
	if !user.EmailVerified {
		if err := a.store.RevokeUserSession(r.Context(), token, time.Now().UTC()); err != nil {
			a.logger.Error("revoke unverified customer session", "request_id", requestID(r.Context()), "error", err)
		}
		a.clearAuthCookie(w)
		return store.User{}, false
	}
	return user, true
}

func (a *App) authMe(w http.ResponseWriter, r *http.Request) {
	user, ok := a.currentUser(w, r)
	if !ok {
		respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "Nicht angemeldet."})
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"user": user})
}

func (a *App) account(w http.ResponseWriter, r *http.Request) {
	user, ok := a.currentUser(w, r)
	if !ok {
		respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "Nicht angemeldet."})
		return
	}
	orders, err := a.store.OrdersForCustomer(r.Context(), user.ID)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"user": user, "orders": orders})
}

func (a *App) recommendations(w http.ResponseWriter, r *http.Request) {
	user, ok := a.currentUser(w, r)
	if !ok {
		respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "Bitte melde dich an."})
		return
	}
	feed, err := a.store.Recommendations(r.Context(), user.ID)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	respondJSON(w, http.StatusOK, map[string]any{
		"enabled":         user.RecommendationConsent,
		"recommendations": feed.Recommendations,
		"recentlyViewed":  feed.RecentlyViewed,
	})
}

func (a *App) setRecommendationConsent(w http.ResponseWriter, r *http.Request) {
	if !a.allowMutation(w, r) {
		return
	}
	user, ok := a.currentUser(w, r)
	if !ok {
		respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "Bitte melde dich an."})
		return
	}
	var input struct {
		Enabled bool `json:"enabled"`
	}
	if err := decodeJSON(w, r, &input, 256); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Bitte prüfe deine Auswahl."})
		return
	}
	if err := a.store.SetRecommendationConsent(r.Context(), user.ID, input.Enabled); err != nil {
		a.serverError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	respondJSON(w, http.StatusOK, map[string]bool{"enabled": input.Enabled})
}

func (a *App) setMarketingEmailConsent(w http.ResponseWriter, r *http.Request) {
	if !a.allowMutation(w, r) {
		return
	}
	user, ok := a.currentUser(w, r)
	if !ok {
		respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "Bitte melde dich an."})
		return
	}
	var input struct {
		Enabled bool `json:"enabled"`
	}
	if err := decodeJSON(w, r, &input, 256); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Bitte prüfe deine Auswahl."})
		return
	}
	err := a.store.SetMarketingEmailConsent(r.Context(), user.ID, input.Enabled, time.Now().UTC())
	if errors.Is(err, store.ErrEmailMarketingNotAllowed) {
		respondJSON(w, http.StatusForbidden, map[string]string{"error": "Bitte bestätige zuerst deine E-Mail-Adresse."})
		return
	}
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	respondJSON(w, http.StatusOK, map[string]bool{"enabled": input.Enabled})
}

func (a *App) emailUnsubscribePage(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	tokenHash, validToken := store.EmailUnsubscribeTokenHash(token)
	if r.URL.Query().Get("success") != "1" && validToken {
		var err error
		validToken, err = a.store.EmailUnsubscribeTokenExists(r.Context(), tokenHash)
		if err != nil {
			a.serverError(w, r, err)
			return
		}
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	page := template.Must(template.New("unsubscribe").Parse(`<!doctype html>
<html lang="de"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>E-Mail-Einstellungen · MITTE</title>
<style>
body{margin:0;background:#f5f6f7;color:#142e50;font:16px/1.6 Arial,sans-serif}
main{max-width:560px;margin:10vh auto;padding:36px;background:#fff;border:1px solid #e1e5e9}
header{margin:-36px -36px 28px;padding:18px 36px;background:#193d68;color:#fff;font-weight:800;letter-spacing:.08em}
h1{margin:0 0 12px;font-size:28px;line-height:1.15}
p{color:#4e5e70}
button{min-height:46px;border:0;background:#ee5b32;padding:0 18px;color:#fff;font:700 15px Arial,sans-serif;cursor:pointer}
a{color:#193d68}
@media(max-width:640px){main{margin:24px 16px;padding:24px}header{margin:-24px -24px 24px;padding:16px 24px}}
</style></head><body><main><header>MITTE</header>
{{if .Success}}<h1>Du bist abgemeldet.</h1><p>Wir senden keine weiteren Marketing-E-Mails an diese Adresse. Bestell- und Kontonachrichten bleiben davon unberührt.</p>
{{else if .Valid}}<h1>Marketing-E-Mails abbestellen?</h1><p>Du kannst dich jederzeit in deinem Kundenkonto wieder anmelden. Bestell- und Kontonachrichten bleiben davon unberührt.</p>
<form method="post" action="/email/unsubscribe"><input type="hidden" name="token" value="{{.Token}}"><button type="submit">Marketing-E-Mails abbestellen</button></form>
{{else}}<h1>Link nicht verfügbar</h1><p>Dieser Abmeldelink ist ungültig. Du kannst deine E-Mail-Einstellungen in deinem <a href="/konto">Kundenkonto</a> verwalten.</p>{{end}}
</main></body></html>`))
	if err := page.Execute(w, struct {
		Success bool
		Valid   bool
		Token   string
	}{Success: r.URL.Query().Get("success") == "1", Valid: validToken, Token: token}); err != nil {
		a.logger.Error("render email unsubscribe page", "error", err)
	}
}

func (a *App) emailUnsubscribe(w http.ResponseWriter, r *http.Request) {
	// The opaque per-delivery token authorizes browser and RFC 8058 one-click unsubscription.
	r.Body = http.MaxBytesReader(w, r.Body, 2048)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Ungültige Abmeldeanfrage.", http.StatusBadRequest)
		return
	}
	token := r.URL.Query().Get("token")
	if token == "" {
		token = r.Form.Get("token")
	}
	tokenHash, ok := store.EmailUnsubscribeTokenHash(token)
	if !ok {
		http.Error(w, "Ungültiger Abmeldelink.", http.StatusBadRequest)
		return
	}
	err := a.store.UnsubscribeMarketingEmails(r.Context(), tokenHash, time.Now().UTC())
	if errors.Is(err, store.ErrInvalidEmailUnsubscribeToken) {
		http.Error(w, "Ungültiger Abmeldelink.", http.StatusBadRequest)
		return
	}
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	if r.URL.Path == "/email/unsubscribe" {
		http.Redirect(w, r, "/email/unsubscribe?success=1", http.StatusSeeOther)
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"message": "Die Abmeldung wurde gespeichert."})
}

func (a *App) recordProductView(w http.ResponseWriter, r *http.Request) {
	if !a.allowMutation(w, r) {
		return
	}
	user, ok := a.currentUser(w, r)
	if !ok {
		respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "Bitte melde dich an."})
		return
	}
	var input struct {
		Slug string `json:"slug"`
	}
	if err := decodeJSON(w, r, &input, 512); err != nil || len(input.Slug) > 120 || strings.TrimSpace(input.Slug) != input.Slug || input.Slug == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Ungültiges Produkt."})
		return
	}
	if err := a.store.RecordProductView(r.Context(), user.ID, input.Slug); errors.Is(err, store.ErrRecommendationConsentRequired) {
		respondJSON(w, http.StatusForbidden, map[string]string{"error": "Aktiviere persönliche Empfehlungen zuerst in deinem Konto."})
		return
	} else if errors.Is(err, store.ErrNotFound) {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "Produkt nicht gefunden."})
		return
	} else if err != nil {
		a.serverError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) updateAccountProfile(w http.ResponseWriter, r *http.Request) {
	if !a.allowMutation(w, r) {
		return
	}
	user, ok := a.currentUser(w, r)
	if !ok {
		respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "Nicht angemeldet."})
		return
	}
	var input struct {
		Email   string                `json:"email"`
		Profile store.CustomerProfile `json:"profile"`
	}
	if err := decodeJSON(w, r, &input, 8192); err != nil || !validAuthEmail(input.Email) || !validCustomerProfile(&input.Profile) {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Bitte prüfe deine Profildaten."})
		return
	}
	if strings.ToLower(strings.TrimSpace(input.Email)) != user.Email {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Die E-Mail-Adresse kann hier nicht geändert werden."})
		return
	}
	updated, err := a.store.UpdateUserProfile(r.Context(), user.ID, input.Email, input.Profile)
	if errors.Is(err, store.ErrEmailInUse) {
		respondJSON(w, http.StatusConflict, map[string]string{"error": "Diese E-Mail-Adresse wird bereits verwendet."})
		return
	}
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"user": updated})
}

func (a *App) changeAccountPassword(w http.ResponseWriter, r *http.Request) {
	if !a.allowAuthMutation(w, r) {
		return
	}
	user, ok := a.currentUser(w, r)
	if !ok {
		respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "Nicht angemeldet."})
		return
	}
	var input struct {
		CurrentPassword string `json:"currentPassword"`
		NewPassword     string `json:"newPassword"`
	}
	if err := decodeJSON(w, r, &input, 1024); err != nil || !validPassword(input.NewPassword) {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Das neue Passwort muss 12 bis 128 Zeichen lang sein."})
		return
	}
	_, passwordHash, err := a.store.AuthenticateUser(r.Context(), user.Email)
	if err != nil || !verifyPassword(input.CurrentPassword, passwordHash) {
		respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "Das aktuelle Passwort ist nicht korrekt."})
		return
	}
	newHash, err := hashPassword(input.NewPassword)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	if err := a.store.ChangeUserPassword(r.Context(), user.ID, newHash, time.Now().UTC()); err != nil {
		a.serverError(w, r, err)
		return
	}
	if err := a.startUserSession(w, r, user); err != nil {
		a.serverError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"message": "Passwort wurde geändert."})
}

func validCustomerProfile(profile *store.CustomerProfile) bool {
	profile.FullName = strings.TrimSpace(profile.FullName)
	profile.Phone = strings.TrimSpace(profile.Phone)
	profile.CompanyName = strings.TrimSpace(profile.CompanyName)
	profile.VATID = strings.TrimSpace(profile.VATID)
	profile.BuyerReference = strings.TrimSpace(profile.BuyerReference)
	profile.Address.AddressLine1 = strings.TrimSpace(profile.Address.AddressLine1)
	profile.Address.AddressLine2 = strings.TrimSpace(profile.Address.AddressLine2)
	profile.Address.PostalCode = strings.TrimSpace(profile.Address.PostalCode)
	profile.Address.City = strings.TrimSpace(profile.Address.City)
	profile.Address.CountryCode = strings.ToUpper(strings.TrimSpace(profile.Address.CountryCode))
	if profile.Address.CountryCode == "" {
		profile.Address.CountryCode = "DE"
	}
	if profile.CustomerType != "b2c" && profile.CustomerType != "b2b" {
		return false
	}
	return validProfileText(profile.FullName, 2, 160) && validProfileText(profile.Phone, 0, 64) &&
		validProfileText(profile.CompanyName, 0, 160) && validProfileText(profile.VATID, 0, 64) &&
		validProfileText(profile.BuyerReference, 0, 160) && validProfileText(profile.Address.AddressLine1, 0, 160) &&
		validProfileText(profile.Address.AddressLine2, 0, 160) && validProfileText(profile.Address.PostalCode, 0, 32) &&
		validProfileText(profile.Address.City, 0, 120) && validCountryCode(profile.Address.CountryCode)
}

func validProfileText(value string, minimum, maximum int) bool {
	if !utf8.ValidString(value) {
		return false
	}
	length := utf8.RuneCountInString(value)
	if length < minimum || length > maximum {
		return false
	}
	for _, character := range value {
		if character < 32 || character == 127 {
			return false
		}
	}
	return true
}

func validCountryCode(value string) bool {
	return len(value) == 2 && value[0] >= 'A' && value[0] <= 'Z' && value[1] >= 'A' && value[1] <= 'Z'
}

func (a *App) startUserSession(w http.ResponseWriter, r *http.Request, user store.User) error {
	token := randomToken(32)
	if err := a.store.CreateUserSession(r.Context(), user.ID, token, time.Now().UTC()); err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{Name: authCookieName, Value: token, Path: "/", MaxAge: int((7 * 24 * time.Hour).Seconds()), HttpOnly: true, Secure: a.secureCookies, SameSite: http.SameSiteLaxMode})
	return nil
}

func (a *App) authToken(r *http.Request) string {
	cookie, err := r.Cookie(authCookieName)
	if err != nil || len(cookie.Value) != 64 {
		return ""
	}
	return cookie.Value
}

func (a *App) clearAuthCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: authCookieName, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: a.secureCookies, SameSite: http.SameSiteLaxMode})
}

func (a *App) startStaffSession(w http.ResponseWriter, r *http.Request, username string) error {
	token, err := store.NewOperatorSessionToken()
	if err != nil {
		return err
	}
	if a.sessions == nil {
		return errors.New("operator session store is not configured")
	}
	expiresAt := time.Now().UTC().Add(12 * time.Hour)
	if err := a.sessions.CreateOperatorSession(r.Context(), token, "staff:"+username, expiresAt); err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{Name: staffCookieName, Value: token, Path: "/", Expires: expiresAt, MaxAge: int((12 * time.Hour).Seconds()), HttpOnly: true, Secure: a.secureCookies, SameSite: http.SameSiteStrictMode})
	return nil
}

func (a *App) currentStaffUsername(r *http.Request) (string, error) {
	token := a.staffSessionToken(r)
	if token == "" {
		return "", store.ErrInvalidSession
	}
	if a.sessions == nil {
		return "", errors.New("operator session store is not configured")
	}
	principal, err := a.sessions.OperatorSessionPrincipal(r.Context(), token, time.Now().UTC())
	if err != nil {
		return "", err
	}
	username, found := strings.CutPrefix(principal, "staff:")
	if !found || username == "" {
		return "", store.ErrInvalidSession
	}
	if _, exists := a.staffCredentials[username]; !exists {
		return "", store.ErrInvalidSession
	}
	return username, nil
}

func (a *App) staffSessionToken(r *http.Request) string {
	cookie, err := r.Cookie(staffCookieName)
	if err != nil || len(cookie.Value) != 64 {
		return ""
	}
	return cookie.Value
}

func (a *App) staffIdentity(username string) map[string]any {
	return map[string]any{
		"username":    username,
		"role":        a.staffRoles[username],
		"permissions": staffPermissionsForRole(a.staffRoles[username]),
	}
}

func (a *App) clearStaffCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: staffCookieName, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: a.secureCookies, SameSite: http.SameSiteStrictMode})
}

func (a *App) allowAuthMutation(w http.ResponseWriter, r *http.Request) bool {
	if !a.authLimiter.allow(a.clientIP(r)) {
		w.Header().Set("Retry-After", "60")
		respondJSON(w, http.StatusTooManyRequests, map[string]string{"error": "Zu viele Anmeldeversuche. Bitte warte kurz."})
		return false
	}
	if !a.validCSRF(r) {
		respondJSON(w, http.StatusForbidden, map[string]string{"error": "Sicherheitsprüfung fehlgeschlagen. Bitte lade die Seite neu."})
		return false
	}
	return true
}

func validAuthEmail(email string) bool {
	normalized := strings.ToLower(strings.TrimSpace(email))
	address, err := mail.ParseAddress(normalized)
	return err == nil && address.Address == normalized && len(normalized) <= 254
}

func validCustomerType(customerType string) bool {
	return customerType == "b2c" || customerType == "b2b"
}

func validPassword(password string) bool {
	length := utf8.RuneCountInString(password)
	return length >= 12 && length <= 128
}

func hashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("password salt: %w", err)
	}
	hash := argon2.IDKey([]byte(password), salt, argonIterations, argonMemory, argonParallelism, argonKeyLength)
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s", argonMemory, argonIterations, argonParallelism,
		base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(hash)), nil
}

func verifyPassword(password, encoded string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != "v=19" || parts[3] != "m=65536,t=3,p=2" {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) != 16 {
		return false
	}
	expected, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(expected) != int(argonKeyLength) {
		return false
	}
	actual := argon2.IDKey([]byte(password), salt, argonIterations, argonMemory, argonParallelism, argonKeyLength)
	return subtle.ConstantTimeCompare(actual, expected) == 1
}
