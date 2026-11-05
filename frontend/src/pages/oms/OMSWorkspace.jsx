import { useEffect, useRef, useState } from 'react'
import { OMSAccess, OMSFrame } from './OMSDashboard.jsx'
import { dateTime } from '../../lib/labels.js'

const matrixDefinitions = {
  fulfillment: {
    title: 'Fulfillment order matrix',
    description: 'Order payment and warehouse fulfillment state in one view.',
    mode: 'fulfillment'
  },
  'fulfillment-clarify': {
    title: 'Fulfillment Ops Clarify Matrix',
    description: 'Paid orders without a fulfillment record, surfaced for operational review.',
    mode: 'fulfillment-clarify'
  },
  payment: {
    title: 'Payment order matrix',
    description: 'Current order payment states, using the status returned by the order service.',
    mode: 'payment'
  },
  'payment-clarify': {
    title: 'Payment Ops Clarify Matrix',
    description: 'Orders not yet settled, including approved invoice orders, separated from paid and fulfilled orders.',
    mode: 'payment-clarify'
  },
  'old-orders': {
    title: 'Old order matrix',
    description: 'Completed and cancelled orders from the current order record.',
    mode: 'old-orders'
  },
  'old-ops-clarify': {
    title: 'Old Ops Clarify Matrix',
    description: 'Closed order records for status review. No legacy-only state is inferred.',
    mode: 'old-ops-clarify'
  }
}

const orderStatusLabels = {
  pending_payment: 'Payment pending',
  invoice_approved: 'Invoice approved',
  paid: 'Paid',
  fulfilled: 'Fulfilled',
  cancelled: 'Cancelled'
}

const fulfillmentStatusLabels = {
  not_ready: 'Not ready',
  ready_to_pick: 'Ready to pick',
  picking: 'Picking',
  packed: 'Packed',
  label_ready: 'Label ready',
  held: 'On hold',
  handed_over: 'Handed over'
}

function statusLabel(status, labels) {
  return labels[status] || status || '—'
}

export function OMSOverview({ user, go, api, euro, isAuthorizationError }) {
  const [summary, setSummary] = useState(null)
  const [orders, setOrders] = useState([])
  const [fulfillments, setFulfillments] = useState([])
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(true)
  const [query, setQuery] = useState('')
  const requestID = useRef(0)
  const searchRef = useRef(null)

  const load = async () => {
    const request = ++requestID.current
    setLoading(true)
    try {
      const params = new URLSearchParams({ q: query.trim(), status: 'all', page: '1', limit: '8' })
      const result = await api(`/api/operations/orders?${params}`, { cache: 'no-store' })
      if (request !== requestID.current) return
      setSummary(result.summary || null)
      const loadedOrders = result.orders || []
      setOrders(loadedOrders)
      const ids = loadedOrders.map(order => order.ID).filter(Boolean)
      if (ids.length) {
        const queue = await api(`/api/warehouse/fulfillments?${new URLSearchParams({ orderIds: ids.join(',') })}`, { cache: 'no-store' })
        if (request === requestID.current) setFulfillments(queue.fulfillments || [])
      } else {
        setFulfillments([])
      }
      setError('')
    } catch (cause) {
      if (request === requestID.current) setError(cause.message)
    } finally {
      if (request === requestID.current) setLoading(false)
    }
  }

  useEffect(() => {
    const timer = setTimeout(load, query ? 250 : 0)
    return () => {
      clearTimeout(timer)
      requestID.current += 1
    }
  }, [query])

  useEffect(() => {
    const onKeyDown = event => {
      if (event.key === '/' && !event.metaKey && !event.ctrlKey && !event.altKey && document.activeElement?.tagName !== 'INPUT') {
        event.preventDefault()
        searchRef.current?.focus()
      }
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [])

  if (error && !summary && isAuthorizationError(error)) return <OMSAccess go={go} error={error} />

  const counts = summary || { open: 0, paid: 0, awaiting: 0, readyToPick: 0, fulfillmentActive: 0, openValueCents: 0 }
  const fulfillmentByOrder = new Map(fulfillments.map(item => [item.OrderID, item]))

  return (
    <OMSFrame user={user} go={go} title="Auftragsübersicht">
      <div className="oms-workspace oms-command-center">
        <header className="oms-page-heading oms-command-heading">
          <div>
            <h1>Aufträge finden und weiterbearbeiten.</h1>
            <p>Suche direkt nach einem Auftrag oder springe in die nächste operative Aufgabe.</p>
          </div>
          <div className="oms-command-actions">
            <button className="oms-secondary-action" onClick={load} disabled={loading}>{loading ? 'Aktualisiere …' : '↻ Aktualisieren'}</button>
            <button className="oms-primary-action" onClick={() => go('/oms/orders')}>Alle Aufträge →</button>
          </div>
        </header>

        <label className="oms-search oms-search-command">
          <span className="oms-search-icon" aria-hidden="true">⌕</span>
          <span className="visually-hidden">Aufträge durchsuchen</span>
          <input ref={searchRef} value={query} onChange={event => setQuery(event.target.value)} placeholder="Bestellnummer, Kund:in oder E-Mail suchen" />
          <kbd>/</kbd>
          {query && <button type="button" onClick={() => setQuery('')} aria-label="Suche löschen">×</button>}
        </label>

        <section className="oms-command-metrics" aria-label="Auftragslage">
          <article><strong>{loading && !summary ? '—' : counts.open}</strong><span>Aktive Aufträge</span><small>ohne Stornierungen</small></article>
          <article className="attention"><strong>{loading && !summary ? '—' : counts.awaiting}</strong><span>Zahlung prüfen</span><small>wartet auf Klärung</small></article>
          <article><strong>{loading && !summary ? '—' : counts.readyToPick}</strong><span>Versandbereit</span><small>bezahlt, noch nicht eingeplant</small></article>
          <article><strong>{loading && !summary ? '—' : euro(counts.openValueCents)}</strong><span>Aktiver Auftragswert</span><small>inklusive Versandkosten</small></article>
        </section>

        {error && <div className="oms-inline-error" role="alert">{error}</div>}

        <section className="oms-priority-strip" aria-label="Nächste Arbeitsschritte">
          <button onClick={() => go('/oms/backlog')}><span className="oms-priority-icon fulfillment">↗</span><span><b>Auftragsrückstand verwalten</b><small>Offene Aufträge nach Eingangsdatum bearbeiten</small></span><em>Öffnen →</em></button>
          <button onClick={() => go('/oms/matrix/payment-clarify')}><span className="oms-priority-icon payment">€</span><span><b>{counts.awaiting} Zahlungsfälle prüfen</b><small>Offene oder freigegebene Rechnungen</small></span><em>Öffnen →</em></button>
          <button onClick={() => go('/oms/matrix/fulfillment')}><span className="oms-priority-icon fulfillment">↗</span><span><b>{counts.fulfillmentActive + counts.readyToPick} Versandaufgaben</b><small>Bereit, in Bearbeitung oder labelbereit</small></span><em>Öffnen →</em></button>
        </section>

        <section className="oms-queue-section oms-command-queue" aria-labelledby="oms-queue-heading">
          <div className="oms-section-heading">
            <div><h2 id="oms-queue-heading">Zuletzt eingegangen</h2><p>Öffne einen Vorgang für Kunden-, Zahlungs- und Versanddetails.</p></div>
            <button className="oms-text-action" onClick={() => go('/oms/orders')}>Alle Aufträge <span aria-hidden="true">→</span></button>
          </div>
          <OrderRows orders={orders} loading={loading} error={error} go={go} fulfillments={fulfillmentByOrder} />
        </section>

        <section className="oms-matrix-section oms-tools-section" aria-labelledby="oms-matrix-heading">
          <div className="oms-section-heading">
            <div><h2 id="oms-matrix-heading">Weitere Arbeitsbereiche</h2><p>Gezielte Ansichten für häufige Arbeitsschritte.</p></div>
          </div>
          <div className="oms-matrix-links">
            <button className="oms-matrix-link" onClick={() => go('/oms/order-items')}><span><b>Auftragspositionen</b><small>Positionen und Ausnahmen bearbeiten</small></span><span aria-hidden="true">→</span></button>
            <button className="oms-matrix-link" onClick={() => go('/oms/customers')}><span><b>Kund:innen</b><small>Profile und Bestellhistorie</small></span><span aria-hidden="true">→</span></button>
            <button className="oms-matrix-link" onClick={() => go('/oms/processes')}><span><b>Prozesscenter</b><small>Verzögerungen, CSV und Servicefälle</small></span><span aria-hidden="true">→</span></button>
            <button className="oms-matrix-link" onClick={() => go('/oms/queues/email-deliveries')}><span><b>E-Mail-Zustellung</b><small>Warteschlange, Wiederholungen und Versandfehler</small></span><span aria-hidden="true">→</span></button>
          </div>
        </section>
      </div>
    </OMSFrame>
  )
}

export function OMSMatrix({ user, go, api, isAuthorizationError, kind }) {
  const definition = matrixDefinitions[kind] || matrixDefinitions.fulfillment
  const [orders, setOrders] = useState([])
  const [fulfillments, setFulfillments] = useState([])
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(true)
  const [query, setQuery] = useState('')
  const [page, setPage] = useState(1)
  const [pages, setPages] = useState(1)
  const [total, setTotal] = useState(0)
  const requestID = useRef(0)

  const load = async () => {
    const request = ++requestID.current
    setLoading(true)
    setError('')
    try {
      const params = new URLSearchParams({ kind, q: query.trim(), page: String(page), limit: '25' })
      const result = await api(`/api/operations/orders/matrix?${params}`, { cache: 'no-store' })
      if (request !== requestID.current) return
      const loadedOrders = result.orders || []
      setOrders(loadedOrders)
      setPages(result.pages || 1)
      setTotal(result.total || 0)
      const ids = loadedOrders.map(order => order.ID).filter(Boolean)
      if (!ids.length) {
        setFulfillments([])
      } else {
        const queueParams = new URLSearchParams({ orderIds: ids.join(',') })
        const queue = await api(`/api/warehouse/fulfillments?${queueParams}`, { cache: 'no-store' })
        if (request === requestID.current) setFulfillments(queue.fulfillments || [])
      }
      setError('')
    } catch (cause) {
      if (request === requestID.current) setError(cause.message)
    } finally {
      if (request === requestID.current) setLoading(false)
    }
  }

  useEffect(() => {
    const timer = setTimeout(load, query ? 250 : 0)
    return () => {
      clearTimeout(timer)
      requestID.current += 1
    }
  }, [query, kind, page])

  const fulfillmentByOrder = new Map(fulfillments.map(item => [item.OrderID, item]))

  if (error && isAuthorizationError(error)) return <OMSAccess go={go} error={error} />

  return (
    <OMSFrame user={user} go={go} title={definition.title}>
      <div className="oms-workspace">
        <header className="oms-page-heading">
          <div>
            <h1>{definition.title}</h1>
            <p>{definition.description}</p>
          </div>
          <button className="oms-secondary-action" onClick={load} disabled={loading}>
            {loading ? 'Refreshing…' : 'Refresh data'}
          </button>
        </header>
        {error && <div className="oms-inline-error" role="alert">{error}</div>}
        <label className="oms-search">
          <span className="oms-search-icon" aria-hidden="true">⌕</span>
          <span className="visually-hidden">Search this matrix</span>
          <input value={query} onChange={event => { setQuery(event.target.value); setPage(1) }} placeholder="Search order number or customer" />
          {query && <button type="button" onClick={() => { setQuery(''); setPage(1) }} aria-label="Clear search">×</button>}
        </label>
        {(definition.mode === 'fulfillment-clarify' || definition.mode === 'payment-clarify') && (
          <p className="oms-matrix-note">{definition.mode === 'payment-clarify'
            ? 'Includes payment-pending and invoice-approved orders. Invoice approval is not payment settlement; check the payment record before taking action.'
            : 'This view uses only the current order and fulfillment states returned by the existing services. Review the record before taking action.'}</p>
        )}
        <div className="oms-table-summary">
          <span>{total ? `${total} matching orders` : 'No orders'}</span>
          <div>
            <button onClick={() => setPage(current => Math.max(1, current - 1))} disabled={loading || page <= 1}>Previous</button>
            <span>Page {page} of {pages}</span>
            <button onClick={() => setPage(current => Math.min(pages, current + 1))} disabled={loading || page >= pages}>Next</button>
          </div>
        </div>
        <section className="oms-table-shell" aria-label={definition.title}>
          {loading && <p className="oms-table-state" role="status">Loading order records…</p>}
          {error && <p className="oms-table-state" role="alert">{error} <button onClick={load}>Retry</button></p>}
          {!loading && !error && orders.length === 0 && <p className="oms-table-state">No orders match this view.</p>}
          <div className="oms-table-scroll">
            <table className="oms-data-table">
              <thead><tr>
                <th scope="col">Order</th><th scope="col">Customer</th><th scope="col">Order state</th>
                {(definition.mode === 'fulfillment' || definition.mode === 'fulfillment-clarify') && <th scope="col">Fulfillment</th>}
                {(definition.mode === 'payment' || definition.mode === 'payment-clarify') && <th scope="col">Payment provider</th>}
                <th scope="col">Created</th><th scope="col"><span className="visually-hidden">Open</span></th>
              </tr></thead>
              <tbody>
                {!loading && !error && orders.map(order => {
                  const fulfillment = fulfillmentByOrder.get(order.ID)
                  return (
                    <tr key={order.ID}>
                      <td><button className="oms-order-id" onClick={() => go(`/oms/orders/${encodeURIComponent(order.ID)}`)}>{order.ID}</button></td>
                      <td><strong>{order.CustomerName || 'Guest customer'}</strong><small>{order.CustomerEmail || '—'}</small></td>
                      <td><span className={`oms-status oms-status-${order.Status || 'unknown'}`}>{statusLabel(order.Status, orderStatusLabels)}</span></td>
                      {(definition.mode === 'fulfillment' || definition.mode === 'fulfillment-clarify') && (
                        <td>{fulfillment ? statusLabel(fulfillment.Status, fulfillmentStatusLabels) : 'No fulfillment record'}</td>
                      )}
                      {(definition.mode === 'payment' || definition.mode === 'payment-clarify') && <td>{order.PaymentProvider || '—'}</td>}
                      <td>{order.CreatedAt ? dateTime(order.CreatedAt) : '—'}</td>
                      <td><button className="oms-row-action" onClick={() => go(`/oms/orders/${encodeURIComponent(order.ID)}`)} aria-label={`Open order ${order.ID}`}>Open →</button></td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>
        </section>
      </div>
    </OMSFrame>
  )
}

function OrderRows({ orders, loading, error, go, fulfillments }) {
  if (loading && !orders.length) return <p className="oms-table-state" role="status">Loading orders…</p>
  if (error && !orders.length) return <p className="oms-table-state">Orders could not be loaded. Retry or open the full order list.</p>
  if (!orders.length) return <p className="oms-table-state">No matching orders.</p>

  return (
    <div className="oms-recent-list">
      {orders.map(order => (
        <button key={order.ID} className="oms-recent-row" onClick={() => go(`/oms/orders/${encodeURIComponent(order.ID)}`)}>
          <span className="oms-recent-primary"><strong>{order.ID}</strong><small>{order.CustomerName || order.CustomerEmail || 'Gastkund:in'} · {dateTime(order.CreatedAt)}</small></span>
          <span className={`oms-status oms-status-${order.Status || 'unknown'}`}>{statusLabel(order.Status, orderStatusLabels)}</span>
          {fulfillments.get(order.ID) && <span className="oms-fulfillment-state">{statusLabel(fulfillments.get(order.ID).Status, fulfillmentStatusLabels)}</span>}
          <span className="oms-recent-arrow" aria-hidden="true">→</span>
        </button>
      ))}
    </div>
  )
}
