import { useState, useEffect, useRef } from 'react'
import { euro, csrf, api, isAuthorizationError } from '../../lib/api.js'
import { dateTime, fulfillmentSteps } from '../../lib/labels.js'
import { fulfillmentLabels } from './OMSDashboard.jsx'
import { OperationsSidebar } from '../../components/operations/OperationsSidebar.jsx'

export function Operations({ user, go }) {
  const [fulfillments, setFulfillments] = useState([])
  const [selected, setSelected] = useState('')
  const [detail, setDetail] = useState(null)
  const [warehouse, setWarehouse] = useState(null)
  const [filter, setFilter] = useState('active')
  const [query, setQuery] = useState('')
  const [queueTotal, setQueueTotal] = useState(0)
  const [queuePage, setQueuePage] = useState(1)
  const [queuePages, setQueuePages] = useState(1)
  const [queueSummary, setQueueSummary] = useState(null)
  const [label, setLabel] = useState({ carrier: '', trackingNumber: '', trackingUrl: '', labelUrl: '' })
  const [note, setNote] = useState('')
  const [busy, setBusy] = useState('')
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const [loading, setLoading] = useState(true)
  const [detailLoading, setDetailLoading] = useState(false)
  const queueRequest = useRef(0)

  const load = async () => {
    const request = ++queueRequest.current
    setLoading(true)
    try {
      const params = new URLSearchParams({ status: filter, q: query.trim(), page: String(queuePage), limit: '50' })
      const warehouseData = await api(`/api/warehouse/fulfillments?${params}`, { cache: 'no-store' })
      if (request !== queueRequest.current) return
      setFulfillments(warehouseData.fulfillments || [])
      setQueueTotal(warehouseData.total || 0)
      setQueuePages(warehouseData.pages || 1)
      setQueueSummary(warehouseData.summary || null)
      setError('')
    } catch (cause) {
      if (request === queueRequest.current) setError(cause.message)
    } finally {
      if (request === queueRequest.current) setLoading(false)
    }
  }

  useEffect(() => {
    const timer = setTimeout(load, query ? 250 : 0)
    return () => {
      clearTimeout(timer)
      queueRequest.current += 1
    }
  }, [filter, query, queuePage])

  useEffect(() => {
    if (fulfillments.some(item => item.OrderID === selected)) return
    const next = fulfillments[0]?.OrderID || ''
    setSelected(next)
    if (!next) {
      setDetail(null)
      setWarehouse(null)
    }
  }, [fulfillments, selected])

  useEffect(() => {
    if (!selected) {
      setDetailLoading(false)
      return
    }
    let live = true
    setDetailLoading(true)
    setDetail(null)
    setWarehouse(null)
    Promise.all([
      api(`/api/operations/orders/${encodeURIComponent(selected)}`, { cache: 'no-store' }),
      api(`/api/warehouse/fulfillments/${encodeURIComponent(selected)}`, { cache: 'no-store' })
    ]).then(([orderData, warehouseData]) => {
      if (!live) return
      setDetail(orderData.detail)
      setWarehouse(warehouseData)
      const f = warehouseData.fulfillment
      setLabel({ carrier: f.Carrier || '', trackingNumber: f.TrackingNumber || '', trackingUrl: f.TrackingURL || '', labelUrl: f.LabelURL || '' })
    }).catch(cause => live && setError(cause.message)).finally(() => live && setDetailLoading(false))
    return () => { live = false }
  }, [selected])

  const rows = fulfillments
  const order = detail?.Order
  const fulfillment = warehouse?.fulfillment || fulfillments.find(item => item.OrderID === selected)
  const run = async (action, endpoint, payload) => {
    if (!selected) return
    setBusy(action)
    setError('')
    try {
      await api(endpoint, { method: 'POST', headers: { 'X-CSRF-Token': csrf }, body: payload ? JSON.stringify(payload) : undefined })
      setNotice(action === 'note' ? 'Interne Notiz gespeichert.' : 'Fulfillment-Status aktualisiert.')
      setNote('')
      await load()
      const [orderData, warehouseData] = await Promise.all([
        api(`/api/operations/orders/${encodeURIComponent(selected)}`, { cache: 'no-store' }),
        api(`/api/warehouse/fulfillments/${encodeURIComponent(selected)}`, { cache: 'no-store' })
      ])
      setDetail(orderData.detail)
      setWarehouse(warehouseData)
    } catch (cause) {
      setError(cause.message)
    } finally {
      setBusy('')
    }
  }

  if (error && !fulfillments.length && isAuthorizationError(error)) {
    return <section className="oms-access"><div><b>M</b><span>Mitte LAGER</span></div><h1>Zugriff erforderlich.</h1><p>{error}</p><button onClick={() => go('/konto')}>Anmelden</button></section>
  }

  return (
    <div className="operations-shell oms-shell">
      <OperationsSidebar active="fulfillment" user={user} go={go}/>
      <main className="oms-main operations-main">
        <header className="oms-topbar">
          <div><p>VERSAND &amp; FULFILLMENT</p><h1>Aufträge im Fluss.</h1></div>
          <div><button onClick={load} className="oms-refresh" disabled={loading}>{loading ? 'Wird aktualisiert …' : '↻ Aktualisieren'}</button><span className="oms-live">LIVE</span></div>
        </header>
        {(error || notice) && <div className={`oms-alert ${error ? 'error' : ''}`} role={error ? 'alert' : 'status'}>{error || notice}<button onClick={() => { setError(''); setNotice('') }}>×</button></div>}
        <section className="oms-metrics">
          <article><span>Offene Aufträge</span><strong>{loading && !queueSummary ? '—' : queueSummary?.active ?? 0}</strong><small>im Versandprozess</small></article>
          <article><span>Übergeben</span><strong>{loading && !queueSummary ? '—' : queueSummary?.handedOver ?? 0}</strong><small>versandfertig abgeschlossen</small></article>
          <article><span>In Bearbeitung</span><strong>{loading && !queueSummary ? '—' : queueSummary?.processing ?? 0}</strong><small>benötigen nächsten Schritt</small></article>
        </section>
        <section className="oms-workspace">
          <div className="oms-queue">
            <div className="oms-queue-head">
              <div><h2>Auftragsqueue</h2><span>{loading ? 'Lädt …' : `${queueTotal} Aufträge`}</span></div>
              <input aria-label="Versandaufträge nach Bestellung oder Kunde suchen" value={query} onChange={e => { setQuery(e.target.value); setQueuePage(1) }} placeholder="Bestellung oder Kunde suchen" />
            </div>
            <div className="oms-filters">
              { [['active', 'Aktiv'], ['ready_to_pick', 'Neu'], ['picking', 'Picken'], ['packed', 'Label'], ['handed_over', 'Übergeben']].map(([value, name]) =>
                <button key={value} className={filter === value ? 'active' : ''} onClick={() => { setFilter(value); setQueuePage(1) }}>{name}</button>
              ) }
            </div>
            <div className="oms-orders">
              {rows.map(f => <button key={f.OrderID} className={selected === f.OrderID ? 'selected' : ''} onClick={() => setSelected(f.OrderID)}>
                <span className={`oms-dot ${f.Status}`}/>
                <div><strong>{f.OrderID}</strong><span>{f.CustomerName || 'Kunde'}</span></div>
                <div><b>{euro(f.TotalCents || 0)}</b><small>{fulfillmentLabels[f.Status]}</small></div>
              </button>)}
              {!rows.length && <p className="oms-empty" role="status">{loading ? 'Versandaufträge werden geladen …' : error ? 'Aufträge konnten nicht geladen werden. Bitte erneut versuchen.' : 'Keine passenden Aufträge.'}</p>}
            </div>
            <nav className="oms-queue-pagination" aria-label="Versandaufträge-Seiten">
              <button disabled={loading || queuePage <= 1} onClick={() => setQueuePage(value => value - 1)}>← Zurück</button>
              <span aria-live="polite">Seite {queuePage} von {queuePages}</span>
              <button disabled={loading || queuePage >= queuePages} onClick={() => setQueuePage(value => value + 1)}>Weiter →</button>
            </nav>
          </div>
          <div className="oms-detail">
            {!order || !fulfillment ? <div className="oms-empty">{detailLoading || loading ? 'Auftragsdetails werden geladen …' : 'Wähle einen Auftrag aus.'}</div> : <>
              <div className="oms-detail-head">
                <div><button className="oms-id" onClick={() => navigator.clipboard?.writeText(order.ID)}>{order.ID} ⧉</button><h2>{order.CustomerName}</h2><span>{dateTime(order.CreatedAt)} · {euro(order.TotalCents)}</span></div>
                <span className={`oms-status ${fulfillment.Status}`}>{fulfillmentLabels[fulfillment.Status]}</span>
              </div>
              <div className="oms-progress">
                {fulfillmentSteps.map((step, index) => <div className={fulfillmentSteps.indexOf(fulfillment.Status) >= index ? 'done' : ''} key={step}><i>{fulfillmentSteps.indexOf(fulfillment.Status) > index ? '✓' : index + 1}</i><span>{step === 'ready_to_pick' ? 'Picken' : step === 'picking' ? 'Packen' : step === 'packed' ? 'Label' : step === 'label_ready' ? 'Übergabe' : 'Fertig'}</span></div>)}
              </div>
              <div className="oms-columns">
                <section>
                  <h3>Positionen <span>{(order.Items || []).reduce((sum, item) => sum + item.Quantity, 0)}</span></h3>
                  {(order.Items || []).map(item => <div className="oms-line" key={item.Name}><b>{item.Quantity}×</b><span>{item.Name}</span><small>{euro(item.UnitPriceCents * item.Quantity)}</small></div>)}
                  <h3 className="oms-section-title">Lieferadresse</h3>
                  <address>{[order.ShippingAddress?.RecipientName, order.ShippingAddress?.CompanyName, order.ShippingAddress?.AddressLine1, order.ShippingAddress?.AddressLine2, `${order.ShippingAddress?.PostalCode || ''} ${order.ShippingAddress?.City || ''}`.trim(), order.ShippingAddress?.CountryCode].filter(Boolean).map(line => <span key={line}>{line}</span>)}</address>
                </section>
                <section className="oms-action">
                  <h3>Nächster Schritt</h3>
                  {fulfillment.Status === 'ready_to_pick' && <><p>Übernimm diesen Auftrag, um mit der Kommissionierung zu starten.</p><button disabled={busy} onClick={() => run('claim', `/api/warehouse/fulfillments/${selected}/claim`)}>{busy === 'claim' ? 'Wird übernommen …' : 'Auftrag übernehmen →'}</button></>}
                  {fulfillment.Status === 'picking' && <><p>Der Auftrag ist dir zugewiesen. Sind alle Positionen verpackt?</p><button disabled={busy} onClick={() => run('pack', `/api/warehouse/fulfillments/${selected}/pack`)}>{busy === 'pack' ? 'Wird gespeichert …' : 'Als verpackt markieren →'}</button></>}
                  {fulfillment.Status === 'packed' && <form onSubmit={e => { e.preventDefault(); run('label', `/api/warehouse/fulfillments/${selected}/label`, label) }}>
                    <p>Hinterlege die Versanddaten und das Label.</p>
                    <label>Versanddienstleister<input required value={label.carrier} onChange={e => setLabel({ ...label, carrier: e.target.value })} placeholder="DHL" /></label>
                    <label>Sendungsnummer<input required value={label.trackingNumber} onChange={e => setLabel({ ...label, trackingNumber: e.target.value })} /></label>
                    <label>Tracking-URL <small>optional</small><input type="url" value={label.trackingUrl} onChange={e => setLabel({ ...label, trackingUrl: e.target.value })} placeholder="https://" /></label>
                    <label>Label-URL <small>optional</small><input type="url" value={label.labelUrl} onChange={e => setLabel({ ...label, labelUrl: e.target.value })} placeholder="https://" /></label>
                    <button disabled={busy}>{busy === 'label' ? 'Wird gespeichert …' : 'Label bestätigen →'}</button>
                  </form>}
                  {fulfillment.Status === 'label_ready' && <><p><b>{fulfillment.Carrier}</b> · {fulfillment.TrackingNumber}</p>{fulfillment.LabelURL && <a href={fulfillment.LabelURL} target="_blank" rel="noreferrer">Label öffnen ↗</a>}<button disabled={busy} onClick={() => run('handover', `/api/warehouse/fulfillments/${selected}/handover`)}>{busy === 'handover' ? 'Wird übergeben …' : 'An Versand übergeben →'}</button></>}
                  {fulfillment.Status === 'handed_over' && <p className="oms-complete">Dieser Auftrag wurde erfolgreich an den Versand übergeben.</p>}
                </section>
              </div>
              <div className="oms-history">
                <section>
                  <h3>Interne Notizen</h3>
                  <form onSubmit={e => { e.preventDefault(); if (note.trim()) run('note', `/api/operations/orders/${selected}/notes`, { body: note }) }}><textarea value={note} onChange={e => setNote(e.target.value)} placeholder="Hinweis für das Team …" /><button disabled={busy || !note.trim()}>Notiz speichern</button></form>
                  {(detail?.InternalNotes || []).map(entry => <article key={entry.ID}><b>{entry.AuthorEmail}</b><p>{entry.Body}</p><small>{dateTime(entry.CreatedAt)}</small></article>)}
                </section>
                <section>
                  <h3>Aktivität</h3>
                  {warehouse?.eventCount > (warehouse?.events || []).length && <p className="oms-history-limit">Die letzten {(warehouse?.events || []).length} von {warehouse.eventCount} Ereignissen.</p>}
                  {(warehouse?.events || []).map((event, index) => <article key={`${event.CreatedAt}-${index}`}><b>{event.Detail}</b><p>{event.ActorEmail}</p><small>{dateTime(event.CreatedAt)}</small></article>)}
                  {!(warehouse?.events || []).length && <p>Noch keine Aktivitäten.</p>}
                </section>
              </div>
            </>}
          </div>
        </section>
      </main>
    </div>
  )
}
