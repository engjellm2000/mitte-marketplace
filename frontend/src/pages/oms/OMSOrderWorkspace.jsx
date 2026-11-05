import { useState, useEffect } from 'react'
import { euro, csrf, api, isAuthorizationError } from '../../lib/api.js'
import { dateTime, itemStatusLabels, deliveryDateLabel, addressLines } from '../../lib/labels.js'
import { fulfillmentLabels, OMSAccess, OMSFrame } from './OMSDashboard.jsx'

const orderStatusLabels = {
  pending_payment: 'Zahlung ausstehend',
  invoice_approved: 'Rechnung freigegeben',
  paid: 'Bezahlt',
  fulfilled: 'Erfüllt',
  cancelled: 'Storniert'
}

export function OMSOrderWorkspace({ user, go, id }) {
  const [detail, setDetail] = useState(null)
  const [warehouse, setWarehouse] = useState(null)
  const [selectedItems, setSelectedItems] = useState([])
  const [note, setNote] = useState('')
  const [actionReason, setActionReason] = useState('')
  const [holdReason, setHoldReason] = useState('')
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const [busy, setBusy] = useState('')
  const load = async () => {
    try {
      const result = await api(`/api/operations/orders/${encodeURIComponent(id)}`, { cache: 'no-store' })
      setDetail(result.detail)
      try {
        setWarehouse(await api(`/api/warehouse/fulfillments/${encodeURIComponent(id)}`, { cache: 'no-store' }))
      } catch {
        setWarehouse(null)
      }
      setError('')
    } catch (cause) {
      setError(cause.message)
    }
  }
  useEffect(() => {
    setDetail(null)
    setSelectedItems([])
    setNotice('')
    load()
  }, [id])

  const order = detail?.Order
  const items = order?.Items || []
  const account = detail?.CustomerAccount
  const fraudBlocked = Boolean(account?.FraudFlagged)
  const operations = Object.fromEntries((detail?.ItemOperations || []).map(item => [item.ProductName, item]))
  const itemSuppliers = Object.fromEntries((detail?.ItemSuppliers || []).map(item => [item.ProductName, item.SupplierName]))
  const fulfillment = warehouse?.fulfillment
  const supplierManagedOnly = items.length > 0 && items.every(item => itemSuppliers[item.Name]?.trim())
  const fulfillmentEligible = ['paid', 'fulfilled', 'invoice_approved'].includes(order?.Status)
  const fulfillmentStatus = supplierManagedOnly && fulfillmentEligible ? 'supplier_managed' : fulfillment?.Status || (order?.Status === 'fulfilled' ? 'handed_over' : fulfillmentEligible ? 'ready_to_pick' : 'not_ready')
  const allSelected = items.length > 0 && selectedItems.length === items.length
  const wholeOrderActionAvailable = order?.Status === 'pending_payment' || (order?.Status === 'paid' && Boolean(order.StripePaymentIntentID))
  const canReversePaidOrder = user?.role === 'admin'
  const toggleItem = (name, checked) => setSelectedItems(current => checked
    ? [...new Set([...current, name])]
    : current.filter(item => item !== name))

  const mutate = async (name, endpoint, payload, success) => {
    setBusy(name)
    setError('')
    setNotice('')
    try {
      await api(endpoint, {
        method: 'POST',
        headers: { 'X-CSRF-Token': csrf },
        body: payload ? JSON.stringify(payload) : undefined
      })
      await load()
      setNotice(success)
      return true
    } catch (cause) {
      setError(cause.message)
      return false
    } finally {
      setBusy('')
    }
  }
  const applyBulkAction = async action => {
    if (!selectedItems.length) return
    if (!actionReason.trim()) {
      setError('Bitte trage zuerst einen Grund für die Aktion ein.')
      return
    }
    if (['cancel', 'refund'].includes(action)) {
      if (!allSelected) {
        setError('Stornierung und Erstattung gelten für die gesamte Bestellung. Bitte wähle alle Positionen aus.')
        return
      }
      const consequence = order.Status === 'pending_payment'
        ? 'Die gesamte unbezahlte Bestellung wird storniert und reservierter Bestand freigegeben.'
        : 'Die gesamte Bestellung wird storniert und der volle Stripe-Betrag wird erstattet.'
      if (!window.confirm(`${consequence}\n\nFortfahren?`)) return
    }
    const messages = {
      return: 'Rückgabeanfrage wurde erfasst und die Kundschaft wird per E-Mail informiert.',
      replacement: 'Ersatzanfrage wurde erfasst und die Kundschaft wird per E-Mail informiert.',
      replacement_with_return: 'Ersatz mit Rückgabe wurde erfasst. Das Lager erhält einen Retourenavis; der Ersatzversand bleibt manuell.',
      cancel: 'Bestellung wurde storniert.',
      refund: 'Vollständige Erstattung wurde ausgelöst.'
    }
    if (await mutate(`bulk-${action}`, `/api/operations/orders/${encodeURIComponent(id)}/items/action`, {
      action, productNames: selectedItems, reason: actionReason.trim()
    }, messages[action])) {
      setSelectedItems([])
      setActionReason('')
    }
  }
  const submitNote = async event => {
    event.preventDefault()
    if (!note.trim()) return
    if (await mutate('note', `/api/operations/orders/${encodeURIComponent(id)}/notes`, { body: note }, 'Interner Kommentar wurde gespeichert.')) {
      setNote('')
    }
  }
  const sendSummary = () => mutate(
    'email',
    `/api/operations/orders/${encodeURIComponent(id)}/emails/summary`,
    {},
    `Bestellübersicht wird an ${account?.Email || order.CustomerEmail} versendet.`
  )
  const updateItemStatus = (name, action) => mutate(
    `item-${name}`,
    `/api/operations/orders/${encodeURIComponent(id)}/items/operation`,
    { productName: name, action },
    'Artikelstatus wurde aktualisiert.'
  )
  const holdFulfillment = async () => {
    if (!holdReason.trim()) { setError('Bitte gib einen Grund für den Fulfillment-Stopp an.'); return }
    if (await mutate('hold', `/api/warehouse/fulfillments/${encodeURIComponent(id)}/hold`, { reason: holdReason.trim() }, 'Fulfillment wurde angehalten.')) setHoldReason('')
  }

  if (error && !order && isAuthorizationError(error)) return <OMSAccess go={go} error={error} />
  if (!order) return <OMSFrame user={user} go={go} title="Bestelldetails">{error ? <div className="oms-v2-error" role="alert">{error}<button onClick={load}>Erneut versuchen</button></div> : <p className="oms-loading" role="status">Bestellung wird geladen …</p>}</OMSFrame>

  const details = [
    ['Bestellnummer', order.ID],
    ['Bestellt am', dateTime(order.CreatedAt)],
    ['Kundenart', order.CustomerType === 'b2b' ? 'Geschäftskunde' : 'Privatkunde'],
    ['Zahlungsart', order.PaymentProvider || '—'],
    ['Zahlungsstatus', orderStatusLabels[order.Status] || order.Status],
    ['Zahlung eingegangen', dateTime(order.PaidAt)]
  ]
  if (order.CompanyName) details.push(['Unternehmen', order.CompanyName])
  if (order.VATID) details.push(['USt-IdNr.', order.VATID])
  if (order.BuyerReference) details.push(['Bestellreferenz', order.BuyerReference])
  if (order.ProcurementMethod) details.push(['Bestellweg', order.ProcurementMethod])
  if (account?.Email) details.push(['Konto-E-Mail', account.Email])
  if (account?.FullName && account.FullName !== order.CustomerName) details.push(['Name im Kundenkonto', account.FullName])
  if (account?.Phone) details.push(['Telefon im Kundenkonto', account.Phone])
  if (account?.CustomerType) details.push(['Aktueller Kontotyp', account.CustomerType === 'b2b' ? 'Geschäftskonto' : 'Privatkonto'])
  if (account?.CompanyName && account.CompanyName !== order.CompanyName) details.push(['Firma im Kundenkonto', account.CompanyName])
  const emails = detail?.Emails || []

  return <OMSFrame user={user} go={go} title={`Bestellung / ${order.ID}`}>
    <div className="oms-v2-context oms-detail-context">
      <button className="oms-back" onClick={() => go('/oms')}>← Alle Bestellungen</button>
      <button className="oms-refresh-v2" onClick={load} disabled={busy}>↻ Aktualisieren</button>
    </div>
    {(error || notice) && <div className={`oms-v2-message ${error ? 'error' : 'success'}`} role={error ? 'alert' : 'status'}>
      <span>{error || notice}</span><button onClick={() => { setError(''); setNotice('') }} aria-label="Meldung schließen">×</button>
    </div>}

    <section className="oms-customer-overview">
      <header className="oms-customer-overview-head">
        <div><h1>{order.CustomerName || 'Gastkund:in'}</h1><a href={`mailto:${order.CustomerEmail}`}>{order.CustomerEmail || 'Keine E-Mail hinterlegt'}</a></div>
        <span className={`oms-v2-badge ${order.Status}`}>{orderStatusLabels[order.Status] || order.Status}</span>
      </header>
      {account?.FraudFlagged && <div className="oms-fraud-alert oms-v2-message error" role="alert">
        <div><strong>Fraud-Prüfung · Konto gesperrt</strong><p>{account.FraudReason}</p>
          <small>Markiert von {account.FraudFlaggedBy || 'Admin'} · {account.FraudFlaggedAt ? dateTime(account.FraudFlaggedAt) : 'Zeitpunkt nicht verfügbar'}</small>
        </div>
      </div>}
      <dl className="oms-customer-facts">{details.map(([label, value]) => <div key={label}><dt>{label}</dt><dd>{value || '—'}</dd></div>)}</dl>
      {order.Note && <div className="oms-customer-note"><b>Hinweis aus der Bestellung</b><p>{order.Note}</p></div>}
    </section>

    <section className={`oms-top-grid ${account ? 'has-account' : ''}`}>
      <section className="oms-v2-card oms-address-card">
        <header><div><p className="oms-eyebrow">Rechnungsdaten</p><h2>Rechnungsadresse</h2></div></header>
        <address>{addressLines(order.BillingAddress).map((line, index) => <span key={`${line}-${index}`}>{line}</span>)}
          {!addressLines(order.BillingAddress).length && <span>Keine Rechnungsadresse hinterlegt.</span>}
        </address>
        {order.CustomerType === 'b2b' && <div className="oms-address-meta">{order.CompanyName && <span>{order.CompanyName}</span>}{order.VATID && <span>USt-IdNr. {order.VATID}</span>}</div>}
      </section>
      {account && <section className="oms-v2-card oms-address-card oms-account-address-card">
        <header><div><p className="oms-eyebrow">Aktuelles Kundenkonto</p><h2>Profiladresse</h2></div></header>
        <address>{addressLines(account.Address).map((line, index) => <span key={`${line}-${index}`}>{line}</span>)}
          {!addressLines(account.Address).length && <span>Im Kundenkonto ist keine Standardadresse hinterlegt.</span>}
        </address>
        <p className="oms-account-address-hint">Diese Angaben können nach der Bestellung geändert worden sein. Für diesen Auftrag gelten die Rechnungs- und Lieferadressen oben.</p>
      </section>}
      <section className="oms-v2-card oms-address-card">
        <header><div><p className="oms-eyebrow">Versanddaten</p><h2>Lieferadresse</h2></div></header>
        <address>{addressLines(order.ShippingAddress).map((line, index) => <span key={`${line}-${index}`}>{line}</span>)}
          {!addressLines(order.ShippingAddress).length && <span>Keine Lieferadresse hinterlegt.</span>}
        </address>
      </section>
    </section>

    <section className="oms-v2-card oms-shipment-history">
      <header><div><p className="oms-eyebrow">Sendungen</p><h2>{(detail?.Shipments || []).length} {detail?.Shipments?.length === 1 ? 'Sendung' : 'Sendungen'} · Rechnung nach vollständigem Versand</h2></div></header>
      {(detail?.Shipments || []).length ? <div className="oms-shipment-list">{detail.Shipments.map(shipment => <article key={shipment.ID}>
        <div><b>{shipment.Carrier} · {shipment.TrackingNumber}</b><span>{dateTime(shipment.ShippedAt)} · {shipment.Source === 'supplier' ? `Lieferant: ${shipment.SupplierName}` : shipment.Source === 'warehouse' ? 'Eigenes Lager' : 'OMS'}</span>
          {shipment.TrackingURL && <a href={shipment.TrackingURL} target="_blank" rel="noreferrer">Sendung verfolgen ↗</a>}</div>
        <p>{(shipment.Items || []).map(item => `${item.Quantity} × ${item.ProductName}`).join(' · ') || 'Keine Positionsdetails'}</p>
      </article>)}</div> : <p className="oms-empty-state">Noch keine Sendung gemeldet.</p>}
    </section>

    <div className="oms-detail-grid-v2 oms-detail-grid-expanded">
      <div className="oms-detail-main">
        <section className="oms-v2-card oms-email-card">
          <header><div><p className="oms-eyebrow">Kundenkommunikation</p><h2>Bestell-E-Mails</h2></div>
            <button className="oms-inline-action" disabled={busy || !order.CustomerEmail} onClick={sendSummary}>{busy === 'email' ? 'Wird vorgemerkt …' : 'Bestellübersicht senden'}</button>
          </header>
          <p className="oms-email-recipient">Bestellkontakt: <b>{order.CustomerEmail || 'Keine E-Mail-Adresse hinterlegt'}</b>{account?.Email && account.Email !== order.CustomerEmail && <> · Konto-E-Mail für die Übersicht: <b>{account.Email}</b></>}</p>
          <div className="oms-email-list">{emails.length ? emails.map(email => <article key={email.ID}>
            <span className={`oms-email-state ${email.SentAt ? 'sent' : email.LastError ? 'failed' : 'queued'}`}>{email.SentAt ? 'Versendet' : email.LastError ? 'Fehler' : 'Warteschlange'}</span>
            <div><b>{email.Subject}</b><span>{email.Recipient} · {email.SentAt ? dateTime(email.SentAt) : `Nächster Versuch ${dateTime(email.AvailableAt)}`}</span>{email.LastError && <small>{email.LastError}</small>}</div>
            <small>{email.Attempts} {email.Attempts === 1 ? 'Versuch' : 'Versuche'}</small>
          </article>) : <p className="oms-empty-state">Für diese Bestellung sind noch keine E-Mails protokolliert.</p>}</div>
        </section>

        <section className="oms-v2-card oms-activity-v2">
          <header><div><p className="oms-eyebrow">Nur intern sichtbar</p><h2>Interne Kommentare</h2></div></header>
          <form onSubmit={submitNote}><label className="oms-sr-only" htmlFor="oms-internal-comment">Interner Kommentar</label><textarea id="oms-internal-comment" value={note} maxLength={2000} onChange={event => setNote(event.target.value)} placeholder="Kontext für Kolleg:innen, Absprachen oder nächste Schritte …"/><button disabled={busy === 'note' || !note.trim()}>{busy === 'note' ? 'Wird gespeichert …' : 'Kommentar speichern'}</button></form>
          <div className="oms-events">{(detail?.InternalNotes || []).map(comment => <article key={comment.ID}><i/><div><b>{comment.AuthorEmail}</b><p>{comment.Body}</p><span>{dateTime(comment.CreatedAt)}</span></div></article>)}
            {!detail?.InternalNotes?.length && <p className="oms-empty-state">Noch keine internen Kommentare. Sie sind für Kund:innen nicht sichtbar.</p>}
          </div>
        </section>

        <section className="oms-v2-card oms-items-workspace">
          <header className="oms-items-heading"><div><p className="oms-eyebrow">Bestellpositionen</p><h2>{items.length} {items.length === 1 ? 'Artikel' : 'Artikel'} · {euro(order.TotalCents)}</h2></div><span>{fulfillmentLabels[fulfillmentStatus] || fulfillmentStatus}</span></header>
          <div className="oms-selection-toolbar">
            <label><input type="checkbox" checked={allSelected} onChange={event => setSelectedItems(event.target.checked ? items.map(item => item.Name) : [])}/> Alle auswählen</label>
            <span>{selectedItems.length} ausgewählt</span>
          </div>
          <div className="oms-item-table-v2">
            <div className="oms-item-table-head"><span>Artikel</span><span>Preisdetails</span><span>Lieferung &amp; Status</span><span>Gesamt</span><span>Auswahl</span></div>
            {items.map(item => {
              const operation = operations[item.Name] || {}
              const supplierName = itemSuppliers[item.Name]
              const status = operation.Status || (order.Status === 'paid' ? 'payment_received' : 'ready_to_process')
              const net = Math.round(item.UnitPriceCents * 100 / 119)
              return <article className="oms-item-row" key={item.Name}>
                <div className="oms-product-cell"><label className="oms-item-check"><input type="checkbox" checked={selectedItems.includes(item.Name)} onChange={event => toggleItem(item.Name, event.target.checked)} aria-label={`${item.Name} auswählen`}/></label><span className="oms-item-thumb">{item.Name.slice(0, 1)}</span><div><b>{item.Name}</b><small>SKU {item.SKU || '—'} · Menge {item.Quantity}</small></div></div>
                <div className="oms-price-details"><span>Brutto <b>{euro(item.UnitPriceCents)}</b></span><span>Netto <b>{euro(net)}</b></span><small>je Stück · inkl. 19 % MwSt.</small></div>
                <div className="oms-item-delivery"><span>Versandt {operation.ShippedQuantity || 0}/{operation.OrderedQuantity || item.Quantity}</span><span>{supplierName ? `Lieferant: ${supplierName}` : 'Eigenes Lager'}</span><span>Voraussichtlich {deliveryDateLabel(operation.DelayedDeliveryDate || operation.OriginalDeliveryDate)}</span><select aria-label={`Status für ${item.Name}`} value={status} disabled={Boolean(busy) || fraudBlocked} onChange={event => updateItemStatus(item.Name, event.target.value)}>{Object.entries(itemStatusLabels).map(([key, label]) => <option key={key} value={key}>{label}</option>)}</select></div>
                <strong className="oms-line-total">{euro(item.UnitPriceCents * item.Quantity)}</strong>
                <span className="oms-selected-label">{selectedItems.includes(item.Name) ? 'Ausgewählt' : '—'}</span>
              </article>
            })}
            {!items.length && <p className="oms-empty-state">Keine Positionen in dieser Bestellung.</p>}
          </div>
          <div className="oms-bulk-actions">
            <label className="oms-reason-field">Grund für Aktion<input value={actionReason} maxLength={500} onChange={event => setActionReason(event.target.value)} placeholder="z. B. Produkt beschädigt, Kundenwunsch …"/></label>
            <div>
              <button disabled={!selectedItems.length || Boolean(busy) || order.Status !== 'fulfilled' || fraudBlocked} onClick={() => applyBulkAction('return')}>Rückgabe eröffnen</button>
              <button disabled={!selectedItems.length || Boolean(busy) || order.Status !== 'fulfilled' || fraudBlocked} onClick={() => applyBulkAction('replacement')}>Ersatz anfragen</button>
              <button disabled={!selectedItems.length || Boolean(busy) || order.Status !== 'fulfilled' || fraudBlocked} onClick={() => applyBulkAction('replacement_with_return')}>Ersatz mit Rückgabe</button>
              <button className="oms-danger-action" disabled={!selectedItems.length || Boolean(busy) || !allSelected || !wholeOrderActionAvailable || (order.Status === 'paid' && !canReversePaidOrder)} onClick={() => applyBulkAction(order.Status === 'pending_payment' ? 'cancel' : 'refund')}>{order.Status === 'pending_payment' ? 'Unbezahlte Bestellung stornieren' : 'Voll erstatten & stornieren'}</button>
            </div>
            {order.Status !== 'fulfilled' && selectedItems.length > 0 && <small>Rückgabe und Ersatzanfrage sind erst nach erfolgreicher Versandübergabe verfügbar.</small>}
            {selectedItems.length > 0 && !allSelected && <small>Stornierung oder Erstattung erfordert die Auswahl aller Positionen.</small>}
            {order.Status === 'paid' && !order.StripePaymentIntentID && <small>Für diese Zahlungsart ist keine automatische Erstattung verfügbar.</small>}
            {order.Status === 'paid' && order.StripePaymentIntentID && !canReversePaidOrder && <small>Bezahlte Bestellungen dürfen nur von Admins storniert oder erstattet werden.</small>}
          </div>
        </section>

      </div>

      <aside className="oms-detail-side">
        <section className="oms-v2-card oms-next-step">
          <p className="oms-eyebrow">Fulfillment</p><h2>{fulfillmentLabels[fulfillmentStatus] || fulfillmentStatus}</h2>
          <p>{fulfillmentStatus === 'supplier_managed' ? order.Status === 'fulfilled' ? 'Alle Positionen wurden durch den Lieferanten versendet.' : 'Der Lieferant verantwortet Versand und Sendungsmeldungen für alle Positionen.' : fulfillmentStatus === 'not_ready' ? 'Fulfillment wird nach bestätigter Zahlung freigegeben.' : fulfillmentStatus === 'held' ? `Angehalten: ${fulfillment?.HoldReason || 'Grund nicht verfügbar'}` : fulfillmentStatus === 'ready_to_pick' ? 'Der Auftrag wartet auf Kommissionierung.' : fulfillmentStatus === 'picking' ? 'Die Kommissionierung läuft.' : fulfillmentStatus === 'packed' ? 'Der Auftrag ist verpackt und wartet auf Versanddaten.' : fulfillmentStatus === 'label_ready' ? 'Das Versandlabel ist hinterlegt.' : 'Die Sendung wurde an den Carrier übergeben.'}</p>
          {fulfillmentStatus === 'held' && <><small className="oms-hold-meta">Angehalten von {fulfillment?.HeldBy || '—'} · Fortsetzung bei {fulfillmentLabels[fulfillment?.ResumeStatus] || fulfillment?.ResumeStatus || '—'}</small><button disabled={busy || fraudBlocked} onClick={() => mutate('release', `/api/warehouse/fulfillments/${encodeURIComponent(id)}/release`, {}, 'Fulfillment wurde fortgesetzt.')}>Fulfillment fortsetzen →</button></>}
          {fulfillmentStatus === 'ready_to_pick' && <button disabled={busy || fraudBlocked} onClick={() => mutate('claim', `/api/warehouse/fulfillments/${encodeURIComponent(id)}/claim`, {}, 'Kommissionierung wurde gestartet.')}>Kommissionierung starten →</button>}
          {fulfillmentStatus === 'picking' && <button disabled={busy || fraudBlocked} onClick={() => mutate('pack', `/api/warehouse/fulfillments/${encodeURIComponent(id)}/pack`, {}, 'Auftrag wurde als verpackt markiert.')}>Als verpackt markieren →</button>}
          {fulfillmentStatus === 'packed' && <button disabled={fraudBlocked} onClick={() => go('/verwaltung')}>Versanddaten hinterlegen →</button>}
          {fulfillmentStatus === 'label_ready' && <button disabled={busy || fraudBlocked} onClick={() => mutate('handover', `/api/warehouse/fulfillments/${encodeURIComponent(id)}/handover`, {}, 'Übergabe an den Carrier wurde bestätigt.')}>An Carrier übergeben →</button>}
          {['ready_to_pick', 'picking', 'packed', 'label_ready'].includes(fulfillmentStatus) && <div className="oms-hold-control"><label>Fulfillment anhalten<input value={holdReason} maxLength={500} onChange={event => setHoldReason(event.target.value)} placeholder="Grund, z. B. Adressklärung"/></label><button className="oms-secondary-action" disabled={busy || fraudBlocked || holdReason.trim().length < 3} onClick={holdFulfillment}>Anhalten</button></div>}
        </section>
        <section className="oms-v2-card oms-totals-v2"><p className="oms-eyebrow">Zahlungsübersicht</p><dl>
          <dt>Zwischensumme</dt><dd>{euro(order.SubtotalCents)}</dd>
          {order.DiscountCents > 0 && <><dt>Rabatt</dt><dd>−{euro(order.DiscountCents)}</dd></>}
          <dt>Versand</dt><dd>{order.ShippingCents ? euro(order.ShippingCents) : 'Kostenlos'}</dd>
          <dt>Enthaltene MwSt.</dt><dd>{euro(order.TaxCents)}</dd>
          <dt className="grand">Gesamtbetrag</dt><dd className="grand">{euro(order.TotalCents)}</dd>
        </dl>{order.Invoice && <a className="oms-inline-action oms-invoice-link" href={`/api/orders/${encodeURIComponent(id)}/invoice.pdf`} target="_blank" rel="noreferrer">Rechnung {order.Invoice.Number} öffnen ↗</a>}</section>
        <section className="oms-v2-card oms-history-card"><p className="oms-eyebrow">Auftragsverlauf</p><div className="oms-events">
          {[...(detail?.StatusHistory || []).map(event => ({ text: `${event.FromStatus || 'Neu'} → ${event.ToStatus}${event.Reason ? ` · ${event.Reason}` : ''}`, who: 'System', at: event.CreatedAt })),
            ...(detail?.ItemOperationHistory || []).map(event => ({ text: `${event.ProductName}: ${itemStatusLabels[event.Action] || event.Action}${event.Detail ? ` · ${event.Detail}` : ''}`, who: event.ActorEmail, at: event.CreatedAt }))].sort((a, b) => new Date(b.at) - new Date(a.at)).slice(0, 8).map((event, index) => <article key={index}><i/><div><b>{event.text}</b><span>{event.who} · {dateTime(event.at)}</span></div></article>)}
        </div></section>
      </aside>
    </div>
  </OMSFrame>
}
