import { useEffect, useMemo, useState } from 'react'
import { api, csrf } from '../../lib/api.js'

const stateLabels = {
  order_accepted: 'Bestätigt',
  delay_reported: 'Verzögerung gemeldet',
  shipment_reported: 'Versand gemeldet',
  cancellation_requested: 'Storno angefragt'
}

const statusLabel = item => item.LatestEvent === 'cancellation_requested' && item.LatestEventResolved
  ? item.LatestResolution === 'rejected' ? 'Storno abgelehnt' : 'Anfrage bearbeitet'
  : item.LatestEvent === 'shipment_reported'
    ? item.RemainingQuantity > 0 ? `Teilversand · ${item.ShippedQuantity}/${item.Quantity}` : 'Vollständig versandt'
    : stateLabels[item.LatestEvent] || (item.LatestEvent ? item.LatestEvent : 'Neu')

const dateLabel = value => value ? new Date(value).toLocaleDateString('de-DE', { day: '2-digit', month: '2-digit', year: 'numeric' }) : '—'
const addressLines = item => [
  item.RecipientName,
  item.CompanyName,
  item.AddressLine1,
  item.AddressLine2,
  `${item.PostalCode} ${item.City}`,
  item.CountryCode
].filter(Boolean)

export function SupplierPortal({ user, checked, onAuthenticated, go }) {
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [items, setItems] = useState([])
  const [loading, setLoading] = useState(false)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const [filter, setFilter] = useState('all')
  const [selectedKey, setSelectedKey] = useState('')
  const [action, setAction] = useState('')
  const [form, setForm] = useState({ note: '', delayedDate: '', carrier: '', trackingNumber: '', trackingUrl: '' })
  const [shipmentQuantities, setShipmentQuantities] = useState({})

  const refresh = async () => {
    setLoading(true)
    setError('')
    try {
      const data = await api('/api/supplier/orders', { cache: 'no-store' })
      setItems(data.items || [])
    } catch (cause) {
      setError(cause.message)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { if (user) refresh() }, [user])

  const visibleItems = useMemo(() => items.filter(item => {
    if (filter === 'new') return !item.LatestEvent
    if (filter === 'open') return item.RemainingQuantity > 0
    if (filter === 'shipped') return item.RemainingQuantity === 0 && item.ShippedQuantity > 0
    return true
  }), [items, filter])
  const selected = visibleItems.find(item => `${item.OrderID}:${item.ProductID}` === selectedKey) || visibleItems[0]
  const cancellationPending = selected?.LatestEvent === 'cancellation_requested' && !selected.LatestEventResolved
  const counts = {
    all: items.length,
    new: items.filter(item => !item.LatestEvent).length,
    open: items.filter(item => item.RemainingQuantity > 0).length,
    shipped: items.filter(item => item.RemainingQuantity === 0 && item.ShippedQuantity > 0).length
  }
  const orderShipmentItems = selected
    ? items.filter(item => item.OrderID === selected.OrderID && item.RemainingQuantity > 0)
    : []

  const openAction = next => {
    setAction(next)
    setForm({ note: '', delayedDate: '', carrier: '', trackingNumber: '', trackingUrl: '' })
    if (next === 'shipment' && selected) {
      setShipmentQuantities(Object.fromEntries(orderShipmentItems.map(item => [item.ProductID, item.RemainingQuantity])))
    }
  }

  const login = async event => {
    event.preventDefault()
    setBusy(true)
    setError('')
    try {
      const data = await api('/api/supplier/login', {
        method: 'POST',
        headers: { 'X-CSRF-Token': csrf },
        body: JSON.stringify({ username, password })
      })
      setPassword('')
      onAuthenticated(data.supplier)
    } catch (cause) {
      setError(cause.message)
    } finally {
      setBusy(false)
    }
  }

  const logout = async () => {
    setBusy(true)
    try {
      await api('/api/supplier/logout', { method: 'POST', headers: { 'X-CSRF-Token': csrf } })
      onAuthenticated(null)
      setItems([])
      go('/lieferanten')
    } catch (cause) {
      setError(cause.message)
    } finally {
      setBusy(false)
    }
  }

  const submitAction = async event => {
    event.preventDefault()
    if (!selected) return
    setBusy(true)
    setError('')
    setNotice('')
    try {
      const shipmentItems = orderShipmentItems
        .map(item => ({ productId: item.ProductID, quantity: Number(shipmentQuantities[item.ProductID]) || 0 }))
        .filter(item => item.quantity > 0)
      if (action === 'shipment' && !shipmentItems.length) {
        setError('Bitte gib für mindestens eine Position eine Versandmenge an.')
        return
      }
      await api(action === 'shipment'
        ? `/api/supplier/orders/${encodeURIComponent(selected.OrderID)}/shipments`
        : `/api/supplier/orders/${encodeURIComponent(selected.OrderID)}/items/${selected.ProductID}/actions`, {
        method: 'POST',
        headers: { 'X-CSRF-Token': csrf },
        body: JSON.stringify(action === 'shipment' ? { ...form, items: shipmentItems } : { ...form, action })
      })
      setNotice(action === 'shipment' ? 'Sendung und Versandmengen an das OMS übermittelt.' : action === 'delay' ? 'Lieferverzögerung an das OMS übermittelt.' : action === 'cancellation_request' ? 'Stornierungsanfrage an das OMS übermittelt.' : 'Auftragsbestätigung an das OMS übermittelt.')
      setAction('')
      setForm({ note: '', delayedDate: '', carrier: '', trackingNumber: '', trackingUrl: '' })
      await refresh()
    } catch (cause) {
      setError(cause.message)
    } finally {
      setBusy(false)
    }
  }

  if (!checked) return <main className="supplier-loading" aria-live="polite">Lieferantensitzung wird geprüft …</main>
  if (!user) return <main className="supplier-login">
    <button className="supplier-login-brand" onClick={() => go('/')}><span>M</span> MITTE <small>SUPPLIER DESK</small></button>
    <section className="supplier-login-panel">
      <div className="supplier-login-mark" aria-hidden="true">SD</div>
      <h1>Lieferantenportal</h1>
      <p>Bestellungen sicher einsehen, bestätigen und Versandänderungen direkt an MITTE übermitteln.</p>
      {error && <div className="supplier-alert error" role="alert">{error}</div>}
      <form onSubmit={login}>
        <label>Benutzername<input required autoComplete="username" value={username} onChange={event => setUsername(event.target.value)} /></label>
        <label>Passwort<input required type="password" autoComplete="current-password" value={password} onChange={event => setPassword(event.target.value)} /></label>
        <button disabled={busy}>{busy ? 'Anmeldung läuft …' : 'Sicher anmelden'} <span aria-hidden="true">→</span></button>
      </form>
      <small>Dein Zugang ist nur für deinen Lieferantenbetrieb freigeschaltet.</small>
    </section>
    <footer>MITTE · Lieferantenservice</footer>
  </main>

  return <div className="supplier-app">
    <aside className="supplier-sidebar">
      <button className="supplier-brand" onClick={() => go('/')}><span>M</span><strong>MITTE<small>SUPPLIER DESK</small></strong></button>
      <div className="supplier-nav-label">LIEFERANTENPORTAL</div>
      <nav aria-label="Lieferantenbereich">
        <button className="active" aria-current="page"><span>▤</span> Bestellungen <b>{counts.open}</b></button>
      </nav>
      <div className="supplier-account"><span>{user.username.slice(0, 1).toUpperCase()}</span><div><strong>{user.supplierName}</strong><small>{user.username}</small></div></div>
      <button className="supplier-logout" disabled={busy} onClick={logout}>Abmelden <span aria-hidden="true">↗</span></button>
    </aside>
    <main className="supplier-main">
      <header className="supplier-header">
        <div><p>MITTE / LIEFERANTEN</p><h1>Lieferaufträge</h1><span>Auftragsstatus und Versanddaten werden direkt mit dem OMS synchronisiert.</span></div>
        <button disabled={loading} onClick={refresh} aria-label="Bestellungen aktualisieren">↻ <span>{loading ? 'Aktualisiert …' : 'Aktualisieren'}</span></button>
      </header>
      {(error || notice) && <div className={`supplier-alert ${error ? 'error' : ''}`} role={error ? 'alert' : 'status'}>{error || notice}</div>}
      <section className="supplier-overview" aria-label="Auftragsübersicht">
        <div><span>Positionen gesamt</span><strong>{counts.all}</strong></div>
        <div className={counts.new ? 'attention' : ''}><span>Neu zu bestätigen</span><strong>{counts.new}</strong></div>
        <div><span>In Bearbeitung</span><strong>{Math.max(0, counts.open - counts.new)}</strong></div>
        <div><span>Vollständig versandt</span><strong>{counts.shipped}</strong></div>
      </section>
      <section className="supplier-workspace">
        <div className="supplier-queue">
          <div className="supplier-queue-head"><h2>Auftragspositionen</h2><span>{visibleItems.length} Positionen</span></div>
          <div className="supplier-filters" role="group" aria-label="Bestellungen filtern">
            {[['all', 'Alle'], ['new', 'Neu'], ['open', 'Offen'], ['shipped', 'Versandt']].map(([key, label]) => <button key={key} className={filter === key ? 'active' : ''} onClick={() => setFilter(key)}>{label}<b>{counts[key]}</b></button>)}
          </div>
          <div className="supplier-list" aria-live="polite">
            {visibleItems.map(item => <button key={`${item.OrderID}:${item.ProductID}`} className={`supplier-order-row ${selected?.OrderID === item.OrderID && selected?.ProductID === item.ProductID ? 'selected' : ''}`} aria-pressed={selected?.OrderID === item.OrderID && selected?.ProductID === item.ProductID} onClick={() => { setSelectedKey(`${item.OrderID}:${item.ProductID}`); setAction('') }}>
              <span className="supplier-order-mark">{item.LatestEvent === 'shipment_reported' ? '✓' : item.LatestEvent ? '↗' : '!'}</span>
              <span className="supplier-order-copy"><strong>{item.ProductName}</strong><small>{item.OrderID} · SKU {item.SKU || '—'} · Versandt {item.ShippedQuantity}/{item.Quantity}</small></span>
              <span className={`supplier-state ${item.LatestEvent || 'new'}`}>{statusLabel(item)}</span>
              <time>{dateLabel(item.OrderCreatedAt)}</time>
            </button>)}
            {!visibleItems.length && <div className="supplier-empty">{loading ? 'Bestellungen werden geladen …' : error ? 'Die Auftragsliste konnte nicht geladen werden. Bitte erneut versuchen.' : items.length ? 'Für diesen Filter gibt es keine Positionen.' : 'Noch keine zugeordneten Lieferaufträge.'}</div>}
          </div>
        </div>
        <aside className="supplier-detail">
          {selected ? <>
            <div className="supplier-detail-head"><div><p>BESTELLUNG</p><h2>{selected.OrderID}</h2></div><span className={`supplier-state ${selected.LatestEvent || 'new'}`}>{statusLabel(selected)}</span></div>
            <div className="supplier-product-detail"><span>ARTIKEL</span><strong>{selected.ProductName}</strong><small>SKU {selected.SKU || '—'} · Versandt {selected.ShippedQuantity} von {selected.Quantity} · Offen {selected.RemainingQuantity}</small></div>
            <div className="supplier-delivery-detail"><span>LIEFERANSCHRIFT</span>{addressLines(selected).map((line, index) => <strong key={`${line}-${index}`}>{line}</strong>)}</div>
            <div className="supplier-dates">
              <div><span>Bestellt</span><strong>{dateLabel(selected.OrderCreatedAt)}</strong></div>
              <div><span>Ursprüngliche Lieferung</span><strong>{dateLabel(selected.OriginalDelivery)}</strong></div>
              <div><span>Gemeldete Lieferung</span><strong>{dateLabel(selected.DelayedDelivery)}</strong></div>
            </div>
            {selected.LatestEventNote && <div className="supplier-last-update"><span>LETZTE MELDUNG</span><p>{selected.LatestEventNote}</p>{selected.LatestCarrier && <p>{selected.LatestCarrier} · {selected.LatestTracking}</p>}{selected.LatestTrackingURL && <a href={selected.LatestTrackingURL} target="_blank" rel="noopener noreferrer">Sendungsverfolgung öffnen →</a>}{selected.LatestEventResolved && selected.LatestResolutionNote && <p>MITTE: {selected.LatestResolutionNote}</p>}</div>}
            {cancellationPending && <p className="supplier-final-state">Deine Stornierungsanfrage wird im OMS geprüft. Weitere Aktionen sind bis zur Rückmeldung gesperrt.</p>}
            {selected.OrderStatus !== 'fulfilled' && selected.RemainingQuantity > 0 && !cancellationPending && <div className="supplier-actions">
              {!selected.Accepted && <button className="supplier-primary" disabled={busy} onClick={() => openAction('accept')}>Auftrag bestätigen <span>→</span></button>}
              <div className="supplier-action-row"><button onClick={() => openAction('shipment')}>Sendung melden</button><button onClick={() => openAction('delay')}>Verzögerung</button></div>
              {selected.LatestEvent !== 'cancellation_requested' || selected.LatestEventResolved ? <button className="supplier-cancel-link" onClick={() => openAction('cancellation_request')}>Stornierung anfragen</button> : null}
            </div>}
            {selected.OrderStatus === 'fulfilled' && <p className="supplier-final-state">Die Gesamtbestellung wurde im OMS abgeschlossen.</p>}
            {selected.RemainingQuantity === 0 && selected.ShippedQuantity > 0 && selected.OrderStatus !== 'fulfilled' && <p className="supplier-final-state">Diese Position ist vollständig versandt. Weitere Lieferantenpositionen werden separat abgeschlossen.</p>}
            {action && <form className="supplier-action-form" onSubmit={submitAction}>
              <div className="supplier-action-form-head"><strong>{{ accept: 'Auftrag bestätigen', shipment: 'Versand melden', delay: 'Verzögerung melden', cancellation_request: 'Stornierung anfragen' }[action]}</strong><button type="button" onClick={() => setAction('')} aria-label="Schließen">×</button></div>
              {action === 'shipment' && <>
                <p>Trage die Mengen für diese gemeinsame Sendung ein. Nicht versendete Restmengen bleiben offen.</p>
                <div className="supplier-shipment-items">{orderShipmentItems.map(item => <label key={item.ProductID}>{item.ProductName}<small>Offen: {item.RemainingQuantity} von {item.Quantity}</small><input type="number" min="0" max={item.RemainingQuantity} step="1" value={shipmentQuantities[item.ProductID] ?? 0} onChange={event => setShipmentQuantities({ ...shipmentQuantities, [item.ProductID]: event.target.value })} /></label>)}</div>
                <label>Versanddienstleister<input required maxLength="100" value={form.carrier} onChange={event => setForm({ ...form, carrier: event.target.value })} placeholder="z. B. DHL" /></label>
                <label>Sendungsnummer<input required maxLength="160" value={form.trackingNumber} onChange={event => setForm({ ...form, trackingNumber: event.target.value })} /></label>
                <label>Tracking-Link (optional)<input type="url" value={form.trackingUrl} onChange={event => setForm({ ...form, trackingUrl: event.target.value })} placeholder="https://…" /></label>
                <label>Hinweis (optional)<textarea rows="2" maxLength="1000" value={form.note} onChange={event => setForm({ ...form, note: event.target.value })} /></label>
              </>}
              {action === 'delay' && <>
                <label>Neuer voraussichtlicher Liefertermin<input required type="date" min={new Date(Date.now() + 86400000).toISOString().slice(0, 10)} value={form.delayedDate} onChange={event => setForm({ ...form, delayedDate: event.target.value })} /></label>
                <label>Grund der Verzögerung<textarea required rows="3" maxLength="1000" value={form.note} onChange={event => setForm({ ...form, note: event.target.value })} /></label>
              </>}
              {action === 'cancellation_request' && <><p>Die Bestellung wird nicht automatisch storniert. MITTE prüft deine Anfrage im OMS.</p><label>Grund<textarea required minLength="5" rows="3" maxLength="1000" value={form.note} onChange={event => setForm({ ...form, note: event.target.value })} /></label></>}
              {action === 'accept' && <label>Hinweis (optional)<textarea rows="2" maxLength="1000" value={form.note} onChange={event => setForm({ ...form, note: event.target.value })} /></label>}
              <button className="supplier-primary" disabled={busy}>{busy ? 'Wird übermittelt …' : 'An das OMS übermitteln'} <span>→</span></button>
            </form>}
          </> : <div className="supplier-detail-empty"><span>↖</span><strong>Auftrag auswählen</strong><p>Wähle eine Position aus, um Lieferdaten zu prüfen und den nächsten Schritt an das OMS zu melden.</p></div>}
        </aside>
      </section>
      <footer className="supplier-footer">Bestell- und Lieferdaten werden vertraulich behandelt und nur für die Auftragsabwicklung verwendet.</footer>
    </main>
  </div>
}
