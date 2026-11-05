import { useState, useEffect } from 'react'
import { euro, csrf, api } from '../../lib/api.js'
import { DeliveryProgress } from '../../components/DeliveryProgress.jsx'

export function Cart({ cart, setCart, updateQuantity, user, go }) {
  const emptyAddress = () => ({ recipientName: '', companyName: '', addressLine1: '', addressLine2: '', postalCode: '', city: '', countryCode: 'DE' })
  const [name, setName] = useState('')
  const [email, setEmail] = useState(user?.Email || '')
  const [paymentMethod, setPaymentMethod] = useState('stripe')
  const [customerType, setCustomerType] = useState('b2c')
  const [companyName, setCompanyName] = useState('')
  const [vatId, setVatId] = useState('')
  const [buyerReference, setBuyerReference] = useState('')
  const [procurementMethod, setProcurementMethod] = useState('online_payment')
  const [note, setNote] = useState('')
  const [billingAddress, setBillingAddress] = useState(emptyAddress)
  const [shippingAddress, setShippingAddress] = useState(emptyAddress)
  const [useBillingForShipping, setUseBillingForShipping] = useState(true)
  const [busy, setBusy] = useState(false)
  const [code, setCode] = useState('')
  const [promotionMessage, setPromotionMessage] = useState('')
  const [checkoutError, setCheckoutError] = useState('')

  useEffect(() => {
    if (user?.Email) setEmail(user.Email)
  }, [user?.Email])

  const quote = cart.Quote || {}
  const subtotal = quote.SubtotalCents ?? cart.TotalCents
  const total = quote.TotalCents ?? cart.TotalCents

  const checkout = async event => {
    event.preventDefault()
    setCheckoutError('')
    setBusy(true)
    try {
      const data = await api('/api/checkout', {
        method: 'POST',
        headers: { 'X-CSRF-Token': csrf, 'Idempotency-Key': crypto.randomUUID() },
        body: JSON.stringify({
          name,
          email,
          paymentMethod,
          customerType,
          companyName,
          vatId,
          buyerReference,
          procurementMethod,
          note,
          billingAddress,
          shippingAddress,
          useBillingForShipping
        })
      })
      location.href = data.checkoutUrl
    } catch (error) {
      setCheckoutError(error.message || 'Deine Bestellung konnte gerade nicht vorbereitet werden. Bitte versuche es erneut.')
      setBusy(false)
    }
  }

  const addressFields = (title, address, setAddress, required = true) => {
    const autocompletePrefix = title === 'Lieferadresse' ? 'shipping' : 'billing'
    return <fieldset className="address-fields">
      <legend>{title}</legend>
      <label>
        Name / Ansprechpartner
        <input required={required} minLength="2" maxLength="120" autoComplete={`${autocompletePrefix} name`} value={address.recipientName} onChange={event => setAddress({ ...address, recipientName: event.target.value })} placeholder="Vor- und Nachname" />
      </label>
      {customerType === 'b2b' && <label>
        Firma an dieser Adresse
        <input maxLength="160" autoComplete={`${autocompletePrefix} organization`} value={address.companyName} onChange={event => setAddress({ ...address, companyName: event.target.value })} placeholder="Firmenname" />
      </label>}
      <label>
        Straße und Hausnummer
        <input required={required} minLength="3" maxLength="160" autoComplete={`${autocompletePrefix} address-line1`} value={address.addressLine1} onChange={event => setAddress({ ...address, addressLine1: event.target.value })} placeholder="Musterstraße 12" />
      </label>
      <label>
        <span>Adresszusatz <small>optional</small></span>
        <input maxLength="160" autoComplete={`${autocompletePrefix} address-line2`} value={address.addressLine2} onChange={event => setAddress({ ...address, addressLine2: event.target.value })} placeholder="z. B. Etage, Abteilung" />
      </label>
      <div className="address-row">
        <label>
          PLZ
          <input required={required} minLength="3" maxLength="20" autoComplete={`${autocompletePrefix} postal-code`} value={address.postalCode} onChange={event => setAddress({ ...address, postalCode: event.target.value })} placeholder="20095" />
        </label>
        <label>
          Ort
          <input required={required} minLength="2" maxLength="120" autoComplete={`${autocompletePrefix} address-level2`} value={address.city} onChange={event => setAddress({ ...address, city: event.target.value })} placeholder="Hamburg" />
        </label>
      </div>
      <label>
        Land
        <select autoComplete={`${autocompletePrefix} country`} value={address.countryCode} onChange={event => setAddress({ ...address, countryCode: event.target.value })}>
          <option value="DE">Deutschland</option>
          {title !== 'Lieferadresse' && <><option value="AT">Österreich</option><option value="CH">Schweiz</option></>}
        </select>
      </label>
      {title === 'Lieferadresse' && <small>Versand und Umsatzsteuer sind derzeit auf Deutschland beschränkt.</small>}
    </fieldset>
  }

  const applyCode = async () => {
    setPromotionMessage('')
    try {
      const data = await api('/api/cart/promotion', {
        method: 'POST',
        headers: { 'X-CSRF-Token': csrf },
        body: JSON.stringify({ code })
      })
      setCart(data.cart)
      const status = data.cart.Quote?.PromotionStatus
      setPromotionMessage(status?.startsWith('applied') ? 'Rabatt wurde angewendet.' : 'Dieser Code ist für deine Auswahl nicht verfügbar.')
    } catch (error) {
      setPromotionMessage(error.message)
    }
  }

  if (!cart.Items?.length) return <section className="cart-page">
    <p className="kicker">Deine Auswahl</p>
    <h1>Warenkorb</h1>
    <div className="empty cart-empty">
      <strong>Dein Warenkorb ist noch leer.</strong>
      <p>Entdecke Produkte aus allen Bereichen und lege deine Favoriten in den Warenkorb.</p>
      <button className="dark-button" type="button" onClick={() => go('/shop')}>Weiter einkaufen <span>↗</span></button>
    </div>
  </section>

  return <section className="cart-page checkout-page">
    <p className="kicker">Deine Auswahl · {cart.Items.length} Artikel</p>
    <h1>Warenkorb &amp; Kasse</h1>
    <DeliveryProgress quote={quote} />

    <form className="checkout-flow checkout-details" onSubmit={checkout}>
      <div className="checkout-layout">
        <div className="checkout-main">
          <section className="checkout-section checkout-items" aria-labelledby="checkout-items-title">
            <div className="checkout-section-heading">
              <div>
                <p className="checkout-step">Deine Auswahl</p>
                <h2 id="checkout-items-title">Warenkorb</h2>
              </div>
              <button className="checkout-return" type="button" onClick={() => go('/shop')}>Weiter einkaufen <span aria-hidden="true">↗</span></button>
            </div>
            <div className="cart-items">
              {cart.Items.map(item => <article key={item.Product.ID}>
                <img src={item.Product.ImageURL} alt="" />
                <div className="cart-item-copy">
                  <p>{item.Product.Category?.Name || 'MITTE'}</p>
                  <h3>{item.Product.Name}</h3>
                  <span>{euro(item.Product.PriceCents)}</span>
                  <div className="line-controls">
                    <div className="mini-quantity">
                      <button type="button" aria-label={`Menge von ${item.Product.Name} verringern`} onClick={() => updateQuantity(item.Product.ID, item.Quantity - 1)}>−</button>
                      <span>{item.Quantity}</span>
                      <button type="button" aria-label={`Menge von ${item.Product.Name} erhöhen`} onClick={() => updateQuantity(item.Product.ID, Math.min(item.Quantity + 1, 10))}>+</button>
                    </div>
                    <button type="button" className="remove-button" onClick={() => updateQuantity(item.Product.ID, 0)}>Entfernen</button>
                  </div>
                </div>
                <strong className="cart-line-total">{euro(item.Quantity * item.Product.PriceCents)}</strong>
              </article>)}
            </div>
          </section>

          <section className="checkout-section" aria-labelledby="checkout-contact-title">
            <div className="checkout-section-heading">
              <div>
                <p className="checkout-step">Kontakt</p>
                <h2 id="checkout-contact-title">Wohin dürfen wir uns melden?</h2>
              </div>
            </div>
            <div className="checkout-fields">
              <label>
                Kundengruppe
                <select value={customerType} onChange={event => setCustomerType(event.target.value)}>
                  <option value="b2c">Privatkunde</option>
                  <option value="b2b">Geschäftskunde</option>
                </select>
              </label>
              <label>
                Name
                <input required minLength="2" maxLength="120" autoComplete="name" value={name} onChange={event => setName(event.target.value)} placeholder="Vor- und Nachname" />
              </label>
              <label>
                E-Mail
                <input required type="email" maxLength="254" autoComplete="email" inputMode="email" value={email} onChange={event => setEmail(event.target.value)} placeholder="name@beispiel.de" />
              </label>
            </div>
            {customerType === 'b2b' && <fieldset className="checkout-business">
              <legend>Angaben für Geschäftskunden</legend>
              <div className="checkout-fields">
                <label>
                  Firmenname
                  <input required minLength="2" maxLength="160" autoComplete="organization" value={companyName} onChange={event => setCompanyName(event.target.value)} placeholder="Muster GmbH" />
                </label>
                <label>
                  <span>USt-IdNr. <small>optional</small></span>
                  <input maxLength="32" autoComplete="off" value={vatId} onChange={event => setVatId(event.target.value)} placeholder="DE123456789" />
                </label>
                <label>
                  <span>Leitweg-ID / Bestellreferenz <small>optional</small></span>
                  <input maxLength="120" autoComplete="off" value={buyerReference} onChange={event => setBuyerReference(event.target.value)} placeholder="z. B. 04011000-12345-67" />
                </label>
                <label>
                  Bestellweg
                  <select value={procurementMethod} onChange={event => setProcurementMethod(event.target.value)}>
                    <option value="online_payment">Online-Zahlung</option>
                    <option value="purchase_order">Bestellung mit Bestellnummer</option>
                    <option value="eprocurement">E-Procurement</option>
                  </select>
                </label>
              </div>
            </fieldset>}
          </section>

          <section className="checkout-section" aria-labelledby="checkout-address-title">
            <div className="checkout-section-heading">
              <div>
                <p className="checkout-step">Adressen</p>
                <h2 id="checkout-address-title">Wohin soll deine Bestellung?</h2>
              </div>
            </div>
            {addressFields('Rechnungsadresse', billingAddress, setBillingAddress)}
            <label className="checkbox-label checkout-address-toggle">
              <input type="checkbox" checked={useBillingForShipping} onChange={event => setUseBillingForShipping(event.target.checked)} />
              Lieferadresse entspricht Rechnungsadresse
            </label>
            {!useBillingForShipping && addressFields('Lieferadresse', shippingAddress, setShippingAddress)}
          </section>

          <section className="checkout-section" aria-labelledby="checkout-payment-title">
            <div className="checkout-section-heading">
              <div>
                <p className="checkout-step">Zahlung</p>
                <h2 id="checkout-payment-title">Wie möchtest du bezahlen?</h2>
              </div>
            </div>
            <label>
              Zahlungsart
              <select value={paymentMethod} onChange={event => setPaymentMethod(event.target.value)}>
                <option value="stripe">Karte, Apple Pay &amp; mehr via Stripe</option>
                <option value="mitte_invoice">Mitte Rechnung · Demo (30 Tage)</option>
                <option value="mitte_transfer">Überweisung an Mitte</option>
              </select>
            </label>
            {paymentMethod === 'mitte_invoice' && <p className="checkout-trust">Entwicklungs-Sandbox: simuliert eine Rechnungsfreigabe. Keine Bonitätsprüfung oder echte Forderung.</p>}
            <label className="checkout-note">
              <span>Hinweis zur Bestellung <small>optional</small></span>
              <textarea maxLength="1000" value={note} onChange={event => setNote(event.target.value)} placeholder="z. B. Lieferhinweis oder interne Referenz" />
            </label>
          </section>
        </div>

        <aside className="checkout checkout-summary" aria-labelledby="checkout-summary-title">
          <p className="checkout-step">Zusammenfassung</p>
          <h2 className="summary-title" id="checkout-summary-title">Deine Bestellung</h2>
          <div className="checkout-summary-lines">
            <div><span>Zwischensumme</span><strong>{euro(subtotal)}</strong></div>
            {quote.DiscountCents > 0 && <div className="discount"><span>{quote.DiscountLabel || 'Rabatt'}</span><strong>−{euro(quote.DiscountCents)}</strong></div>}
            <div><span>{quote.ShippingLabel || 'Versand'}</span><strong>{quote.ShippingCents === 0 ? 'Kostenlos' : euro(quote.ShippingCents || 0)}</strong></div>
            <div className="summary-total"><span>Gesamt</span><strong>{euro(total)}</strong></div>
          </div>
          <p className="checkout-tax">Inklusive {euro(quote.TaxCents || 0)} MwSt.</p>

          <details className="checkout-promo">
            <summary>Gutscheincode einlösen</summary>
            <div className="promo-form">
              <label>
                Gutscheincode
                <input maxLength="64" autoComplete="off" value={code} onChange={event => setCode(event.target.value.toUpperCase())} placeholder="z. B. MORGEN10" />
              </label>
              <button type="button" onClick={applyCode}>Einlösen</button>
            </div>
          </details>
          {promotionMessage && <p className="promo-message" role="status">{promotionMessage}</p>}

          {checkoutError && <p className="checkout-error" role="alert">{checkoutError}</p>}
          <button disabled={busy} className="coral-button checkout-submit">{busy ? 'Wird vorbereitet …' : 'Sicher zur Kasse →'}</button>
          <small className="checkout-trust">
            {paymentMethod === 'stripe' ? '🔒 Verschlüsselte Zahlung über Stripe' : paymentMethod === 'mitte_invoice' ? 'Rechnung mit simuliertem Provider-Schutz (nur Entwicklung)' : 'Überweisung mit eindeutiger Zahlungsreferenz'}
          </small>
        </aside>
      </div>
    </form>
  </section>
}
