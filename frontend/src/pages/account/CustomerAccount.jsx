import { useEffect, useState } from 'react'
import { euro, csrf, api } from '../../lib/api.js'
import { dateTime, paymentLabel, deliveryLabel, addressText } from '../../lib/labels.js'

const emptyProfile = {
  FullName: '',
  Phone: '',
  CustomerType: 'b2c',
  CompanyName: '',
  VATID: '',
  BuyerReference: '',
  Address: { AddressLine1: '', AddressLine2: '', PostalCode: '', City: '', CountryCode: 'DE' },
}

export function CustomerAccount({ user, setUser, logout, logoutBusy }) {
  const [account, setAccount] = useState(null)
  const [profile, setProfile] = useState(emptyProfile)
  const [notice, setNotice] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const [recommendationBusy, setRecommendationBusy] = useState(false)
  const [marketingEmailBusy, setMarketingEmailBusy] = useState(false)
  const [expanded, setExpanded] = useState(null)

  const refresh = async () => {
    try {
      const data = await api('/api/account', { cache: 'no-store' })
      setAccount(data)
      setUser(data.user)
      setProfile(data.user.Profile || emptyProfile)
    } catch (cause) {
      setError(cause.message)
    }
  }

  useEffect(() => { refresh() }, [])

  const updateProfile = async event => {
    event.preventDefault()
    setBusy(true)
    setError('')
    setNotice('')
    try {
      const data = await api('/api/account/profile', {
        method: 'PUT',
        headers: { 'X-CSRF-Token': csrf },
        body: JSON.stringify({ email: user.Email, profile }),
      })
      setUser(data.user)
      setProfile(data.user.Profile)
      setNotice('Deine Kontodaten wurden gespeichert.')
    } catch (cause) {
      setError(cause.message)
    } finally {
      setBusy(false)
    }
  }

  const changePassword = async event => {
    event.preventDefault()
    const form = new FormData(event.currentTarget)
    const currentPassword = form.get('currentPassword')
    const newPassword = form.get('newPassword')
    setBusy(true)
    setError('')
    setNotice('')
    try {
      await api('/api/account/password', {
        method: 'POST',
        headers: { 'X-CSRF-Token': csrf },
        body: JSON.stringify({ currentPassword, newPassword }),
      })
      event.currentTarget.reset()
      setNotice('Dein Passwort wurde geändert.')
    } catch (cause) {
      setError(cause.message)
    } finally {
      setBusy(false)
    }
  }

  const updateRecommendationConsent = async enabled => {
    setRecommendationBusy(true)
    setError('')
    setNotice('')
    try {
      const data = await api('/api/account/recommendations/consent', {
        method: 'POST',
        headers: { 'X-CSRF-Token': csrf },
        body: JSON.stringify({ enabled }),
      })
      setUser(current => ({ ...current, RecommendationConsent: data.enabled }))
      setAccount(current => current ? { ...current, user: { ...current.user, RecommendationConsent: data.enabled } } : current)
      setNotice(data.enabled
        ? 'Persönliche Empfehlungen sind aktiviert.'
        : 'Empfehlungen sind deaktiviert und dein gespeicherter Produktverlauf wurde gelöscht.')
    } catch (cause) {
      setError(cause.message)
    } finally {
      setRecommendationBusy(false)
    }
  }

  const updateMarketingEmailConsent = async enabled => {
    setMarketingEmailBusy(true)
    setError('')
    setNotice('')
    try {
      await api('/api/account/marketing-email-consent', {
        method: 'POST',
        headers: { 'X-CSRF-Token': csrf },
        body: JSON.stringify({ enabled }),
      })
      setUser(current => ({ ...current, MarketingEmailConsent: enabled }))
      setAccount(current => current
        ? { ...current, user: { ...current.user, MarketingEmailConsent: enabled } }
        : current)
      setNotice(enabled
        ? 'Du erhältst ab jetzt Marketing-E-Mails von MITTE.'
        : 'Marketing-E-Mails sind abbestellt. Bestell- und Kontonachrichten bleiben davon unberührt.')
    } catch (cause) {
      setError(cause.message)
    } finally {
      setMarketingEmailBusy(false)
    }
  }

  const setAddress = patch => setProfile(current => ({
    ...current,
    Address: { ...(current.Address || emptyProfile.Address), ...patch },
  }))
  const orders = account?.orders || []

  return (
    <section className="account-page">
      <div className="account-hero">
        <div>
          <p className="kicker">Mein Konto</p>
          <h1>Alles an einem Ort.</h1>
          <p>Bestellungen, Zahlungen und deine Angaben – klar und sicher verwaltet.</p>
        </div>
        <div className="account-email">
          <small>ANGEMELDET ALS</small>
          <strong>{user.Email}</strong>
          <button disabled={logoutBusy} className="text-button" onClick={logout}>Abmelden →</button>
        </div>
      </div>
      {(error || notice) && <p className={error ? 'account-message error' : 'account-message'} role="status">{error || notice}</p>}
      <div className="account-grid">
        <div className="account-main">
          <section className="account-section">
            <div className="account-section-head">
              <div>
                <p className="kicker">Bestellübersicht</p>
                <h2>Deine Bestellungen</h2>
              </div>
              <span>{orders.length} {orders.length === 1 ? 'Bestellung' : 'Bestellungen'}</span>
            </div>
            {!account ? (
              <p className="loading">Konto wird geladen …</p>
            ) : !orders.length ? (
              <p className="account-empty">Noch keine Bestellungen in diesem Konto. Zukünftige Bestellungen erscheinen hier automatisch.</p>
            ) : (
              <div className="order-list">
                {orders.map(order => (
                  <article className="account-order" key={order.ID}>
                    <button
                      className="order-summary"
                      onClick={() => setExpanded(expanded === order.ID ? null : order.ID)}
                      aria-expanded={expanded === order.ID}
                    >
                      <div>
                        <small>BESTELLUNG</small>
                        <strong>{order.ID}</strong>
                        <span>{dateTime(order.CreatedAt)}</span>
                      </div>
                      <div><small>GESAMT</small><strong>{euro(order.TotalCents)}</strong></div>
                      <div><small>ZAHLUNG</small><b className={`status-${order.Status}`}>{paymentLabel(order.Status)}</b></div>
                      <span className="order-chevron">{expanded === order.ID ? '−' : '+'}</span>
                    </button>
                    {expanded === order.ID && (
                      <div className="order-detail">
                        <div className="order-statuses">
                          <div><small>Zahlung</small><strong className={`status-${order.Status}`}>{paymentLabel(order.Status)}</strong></div>
                          <div><small>Bestellstatus</small><strong>{deliveryLabel(order.Status)}</strong></div>
                          {order.PaidAt && <div><small>Bezahlt am</small><strong>{dateTime(order.PaidAt)}</strong></div>}
                        </div>
                        <div className="order-detail-grid">
                          <div>
                            <h3>Artikel</h3>
                            {(order.Items || []).map(item => (
                              <p key={item.Name}>
                                <span>{item.Quantity} × {item.Name}</span>
                                <strong>{euro(item.Quantity * item.UnitPriceCents)}</strong>
                              </p>
                            ))}
                            <p><span>Zwischensumme</span><strong>{euro(order.SubtotalCents)}</strong></p>
                            {order.DiscountCents > 0 && (
                              <p><span>Rabatt{order.PromotionCode ? ` (${order.PromotionCode})` : ''}</span><strong>−{euro(order.DiscountCents)}</strong></p>
                            )}
                            <p><span>Versand</span><strong>{order.ShippingCents ? euro(order.ShippingCents) : 'Kostenlos'}</strong></p>
                            <p className="order-total"><span>Gesamt</span><strong>{euro(order.TotalCents)}</strong></p>
                          </div>
                          <div>
                            <h3>Lieferadresse</h3>
                            {addressText(order.ShippingAddress).map(line => <p key={line}>{line}</p>) || <p>Keine Adresse hinterlegt.</p>}
                          </div>
                          <div>
                            <h3>Rechnungsdetails</h3>
                            <p>{order.CustomerType === 'b2b' ? 'Geschäftskonto' : 'Privatkonto'}</p>
                            {order.CompanyName && <p>{order.CompanyName}</p>}
                            {order.VATID && <p>USt-IdNr.: {order.VATID}</p>}
                            {order.BuyerReference && <p>Referenz: {order.BuyerReference}</p>}
                            {order.Invoice ? (
                              <>
                                <p>Rechnung {order.Invoice.Number} · {dateTime(order.Invoice.IssuedAt)}</p>
                                <a
                                  className="account-order-link"
                                  href={`/api/orders/${encodeURIComponent(order.ID)}/invoice.pdf`}
                                  target="_blank"
                                  rel="noreferrer"
                                >
                                  Rechnung als PDF öffnen →
                                </a>
                              </>
                            ) : ['paid', 'invoice_approved'].includes(order.Status) ? (
                              <p>Die Rechnung wird nach vollständigem Versand bereitgestellt.</p>
                            ) : null}
                          </div>
                        </div>
                      </div>
                    )}
                  </article>
                ))}
              </div>
            )}
          </section>
        </div>
        <aside className="account-side">
          <section className="account-section">
            <p className="kicker">Persönliche Daten</p>
            <h2>Profil bearbeiten</h2>
            <form className="account-form" onSubmit={updateProfile}>
              <label>Name<input required minLength="2" maxLength="160" value={profile.FullName || ''} onChange={event => setProfile({ ...profile, FullName: event.target.value })} /></label>
              <label>E-Mail-Adresse<input required type="email" maxLength="254" value={user.Email} readOnly /><small>Die E-Mail-Adresse kann hier nicht geändert werden.</small></label>
              <label>Telefon <small>optional</small><input type="tel" maxLength="64" value={profile.Phone || ''} onChange={event => setProfile({ ...profile, Phone: event.target.value })} /></label>
              <label>Kontotyp<select value={profile.CustomerType || 'b2c'} onChange={event => setProfile({ ...profile, CustomerType: event.target.value })}><option value="b2c">Privatkunde</option><option value="b2b">Geschäftskunde</option></select></label>
              {profile.CustomerType === 'b2b' && (
                <>
                  <label>Firma<input maxLength="160" value={profile.CompanyName || ''} onChange={event => setProfile({ ...profile, CompanyName: event.target.value })} /></label>
                  <label>USt-IdNr. <small>optional</small><input maxLength="64" value={profile.VATID || ''} onChange={event => setProfile({ ...profile, VATID: event.target.value })} /></label>
                  <label>Bestellreferenz <small>optional</small><input maxLength="160" value={profile.BuyerReference || ''} onChange={event => setProfile({ ...profile, BuyerReference: event.target.value })} /></label>
                </>
              )}
              <fieldset>
                <legend>Standardadresse</legend>
                <label>Straße und Hausnummer<input maxLength="160" value={profile.Address?.AddressLine1 || ''} onChange={event => setAddress({ AddressLine1: event.target.value })} /></label>
                <label>Adresszusatz <small>optional</small><input maxLength="160" value={profile.Address?.AddressLine2 || ''} onChange={event => setAddress({ AddressLine2: event.target.value })} /></label>
                <div className="account-address-row">
                  <label>PLZ<input maxLength="32" value={profile.Address?.PostalCode || ''} onChange={event => setAddress({ PostalCode: event.target.value })} /></label>
                  <label>Ort<input maxLength="120" value={profile.Address?.City || ''} onChange={event => setAddress({ City: event.target.value })} /></label>
                </div>
                <label>Land<select value={profile.Address?.CountryCode || 'DE'} onChange={event => setAddress({ CountryCode: event.target.value })}><option value="DE">Deutschland</option><option value="AT">Österreich</option><option value="CH">Schweiz</option></select></label>
              </fieldset>
              <button disabled={busy} className="coral-button">{busy ? 'Wird gespeichert …' : 'Daten speichern →'}</button>
            </form>
          </section>
          <section className="account-section password-section">
            <p className="kicker">Sicherheit</p>
            <h2>Passwort ändern</h2>
            <form className="account-form" onSubmit={changePassword}>
              <label>Aktuelles Passwort<input required name="currentPassword" type="password" autoComplete="current-password" /></label>
              <label>Neues Passwort<input required name="newPassword" type="password" minLength="12" maxLength="128" autoComplete="new-password" /><small>Mindestens 12 Zeichen.</small></label>
              <button disabled={busy} className="dark-button">Passwort ändern →</button>
            </form>
          </section>
          <section className="account-section recommendation-section">
            <p className="kicker">Deine Empfehlungen</p>
            <h2>Persönlich für dich</h2>
            <p className="recommendation-description">Wenn du zustimmst, verwenden wir angesehene Produkte und deine Bestellungen, um passende Empfehlungen in deinem Konto zu zeigen. Ohne Zustimmung speichern wir keinen Produktverlauf für Empfehlungen.</p>
            <label className="recommendation-toggle">
              <input
                type="checkbox"
                checked={Boolean(user.RecommendationConsent)}
                disabled={recommendationBusy}
                onChange={event => updateRecommendationConsent(event.target.checked)}
              />
              <span>Meine Daten für persönliche Empfehlungen verwenden</span>
            </label>
            <small className="recommendation-footnote">Du kannst deine Zustimmung jederzeit hier widerrufen. Dabei wird dein gespeicherter Produktverlauf gelöscht.</small>
          </section>
          <section className="account-section recommendation-section">
            <p className="kicker">E-Mail-Einstellungen</p>
            <h2>Angebote von MITTE</h2>
            <p className="recommendation-description">Erhalte gelegentlich Neuigkeiten und Angebote per E-Mail. Diese freiwillige Auswahl betrifft keine Bestell- oder Kontonachrichten.</p>
            <label className="recommendation-toggle">
              <input
                type="checkbox"
                checked={Boolean(user.MarketingEmailConsent)}
                disabled={marketingEmailBusy || !user.EmailVerified}
                onChange={event => updateMarketingEmailConsent(event.target.checked)}
              />
              <span>Ich möchte Marketing-E-Mails von MITTE erhalten</span>
            </label>
            <small className="recommendation-footnote">
              Du kannst deine Auswahl jederzeit hier ändern oder dich über den Abmeldelink in jeder Marketing-E-Mail abmelden.
              {!user.EmailVerified && ' Bestätige zuerst deine E-Mail-Adresse, um dich anzumelden.'}
            </small>
          </section>
        </aside>
      </div>
    </section>
  )
}
