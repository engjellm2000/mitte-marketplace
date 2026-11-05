import { useEffect, useState } from 'react'
import './CookieConsent.css'

const preferencesCookie = 'mitte_cookie_preferences'
const preferencesValue = 'essential-v1'
const preferencesMaxAge = 60 * 60 * 24 * 180

function hasSavedPreferences() {
  return document.cookie.split(';').some(value => value.trim() === `${preferencesCookie}=${preferencesValue}`)
}

function saveEssentialPreferences() {
  const secure = location.protocol === 'https:' ? '; Secure' : ''
  document.cookie = `${preferencesCookie}=${preferencesValue}; Max-Age=${preferencesMaxAge}; Path=/; SameSite=Strict${secure}`
}

export function CookieConsent({ forceOpen, onClose, go }) {
  const [open, setOpen] = useState(() => !hasSavedPreferences())
  const [detailsOpen, setDetailsOpen] = useState(false)

  useEffect(() => {
    if (forceOpen) {
      setOpen(true)
      setDetailsOpen(true)
    }
  }, [forceOpen])

  const savePreferences = () => {
    saveEssentialPreferences()
    setOpen(false)
    setDetailsOpen(false)
    onClose()
  }

  const navigateTo = path => {
    setOpen(false)
    setDetailsOpen(false)
    onClose()
    go(path)
  }

  if (!open) return null

  return (
    <aside className="cookie-consent" aria-labelledby="cookie-consent-title" aria-live="polite">
      <div className="cookie-consent-intro">
        <div>
          <h2 id="cookie-consent-title">Cookies bei MITTE</h2>
          <p>Wir verwenden notwendige Cookies für Warenkorb, Anmeldung und Sicherheit. Analyse- und Werbe-Cookies setzen wir derzeit nicht ein.</p>
        </div>
        <div className="cookie-consent-actions">
          <button
            type="button"
            className="cookie-consent-secondary"
            aria-expanded={detailsOpen}
            aria-controls="cookie-consent-details"
            onClick={() => setDetailsOpen(value => !value)}
          >
            {detailsOpen ? 'Details schließen' : 'Einstellungen ansehen'}
          </button>
          <button type="button" className="cookie-consent-primary" onClick={savePreferences}>
            Nur notwendige speichern
          </button>
        </div>
      </div>
      {detailsOpen && (
        <div id="cookie-consent-details" className="cookie-consent-details">
          <p className="cookie-consent-summary">Es gibt aktuell keine optionalen Cookie-Kategorien, denen du zustimmen musst. Deine Auswahl wird 6 Monate gespeichert und lässt sich jederzeit hier erneut öffnen.</p>
          <dl className="cookie-inventory">
            <div>
              <dt>Warenkorb und Sicherheit</dt>
              <dd><code>mitte_session</code> (30 Tage) hält deinen Gast-Warenkorb bereit. <code>mitte_csrf</code> (1 Tag) schützt Änderungen vor fremden Anfragen. Beide sind nur für die Website lesbar.</dd>
            </div>
            <div>
              <dt>Anmeldung</dt>
              <dd><code>mitte_auth</code> (7 Tage) hält Kunden angemeldet. <code>mitte_staff</code> (12 Stunden) und <code>mitte_oms</code> (8 Stunden) dienen ausschließlich geschützten Mitarbeitendenbereichen.</dd>
            </div>
            <div>
              <dt>Diese Auswahl</dt>
              <dd><code>mitte_cookie_preferences</code> (6 Monate) merkt sich, dass du nur notwendige Cookies gewählt hast.</dd>
            </div>
            <div>
              <dt>Merkliste auf deinem Gerät</dt>
              <dd>Deine gespeicherten Artikel liegen im lokalen Browser-Speicher, nicht in einem Cookie. Du kannst sie in deiner <button type="button" className="cookie-consent-link" onClick={() => navigateTo('/merkliste')}>Merkliste verwalten</button>.</dd>
            </div>
            <div>
              <dt>Produktverlauf und Empfehlungen</dt>
              <dd>Angesehene Produkte werden nur nach separater Zustimmung im Kundenkonto gespeichert. Diese Funktion verwendet kein Cookie; Zustimmung und Widerruf verwaltest du <button type="button" className="cookie-consent-link" onClick={() => navigateTo('/konto')}>in deinem Konto</button>.</dd>
            </div>
          </dl>
          <p className="cookie-consent-note">Die Website lädt Produktbilder teilweise von externen Bilddiensten. Cookie-Einstellungen steuern diese Bildanfragen nicht.</p>
        </div>
      )}
    </aside>
  )
}
