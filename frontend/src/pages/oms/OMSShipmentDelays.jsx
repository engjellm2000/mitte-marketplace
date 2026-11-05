import { useEffect, useRef, useState } from 'react'
import { api, isAuthorizationError } from '../../lib/api.js'
import { dateTime } from '../../lib/labels.js'
import { OMSAccess, OMSFrame } from './OMSDashboard.jsx'

const pageSize = 25
const orderStatusLabels = {
  pending_payment: 'Zahlung ausstehend',
  invoice_approved: 'Rechnung freigegeben',
  paid: 'Bezahlt',
  fulfilled: 'Erfüllt',
  cancelled: 'Storniert'
}

function displayDate(value) {
  return value ? new Intl.DateTimeFormat('de-DE', { dateStyle: 'medium' }).format(new Date(`${value}T12:00:00`)) : '—'
}

export function OMSShipmentDelays({ user, go }) {
  const [delays, setDelays] = useState([])
  const [query, setQuery] = useState('')
  const [page, setPage] = useState(1)
  const [pages, setPages] = useState(1)
  const [total, setTotal] = useState(0)
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(true)
  const [refresh, setRefresh] = useState(0)
  const requestID = useRef(0)

  useEffect(() => {
    const request = ++requestID.current
    const timer = setTimeout(async () => {
      setLoading(true)
      setError('')
      try {
        const params = new URLSearchParams({ q: query.trim(), page: String(page), limit: String(pageSize) })
        const result = await api(`/api/operations/shipment-delays?${params}`, { cache: 'no-store' })
        if (request !== requestID.current) return
        setDelays(result.delays || [])
        setPages(result.pages || 1)
        setTotal(result.total || 0)
      } catch (cause) {
        if (request === requestID.current) setError(cause.message)
      } finally {
        if (request === requestID.current) setLoading(false)
      }
    }, query ? 250 : 0)
    return () => {
      clearTimeout(timer)
      requestID.current += 1
    }
  }, [query, page, refresh])

  if (error && !delays.length && isAuthorizationError(error)) return <OMSAccess go={go} error={error} />
  const first = total ? (page - 1) * pageSize + 1 : 0
  const last = Math.min(page * pageSize, total)

  return (
    <OMSFrame user={user} go={go} title="Shipment delay log">
      <div className="oms-workspace">
        <header className="oms-page-heading">
          <div>
            <h1>Shipment delay log</h1>
            <p>Review order-item delivery dates updated through the OMS delay workflow.</p>
          </div>
          <button className="oms-secondary-action" onClick={() => setRefresh(value => value + 1)} disabled={loading}>
            {loading ? 'Refreshing…' : 'Refresh data'}
          </button>
        </header>
        <p className="oms-matrix-note">This view reflects staff-entered delayed delivery dates and their last recorded editor. It does not verify carrier events or confirm delivery.</p>

        <section className="oms-price-browser" aria-label="Shipment delay records">
          <label className="oms-search">
            <span className="oms-search-icon" aria-hidden="true">⌕</span>
            <span className="visually-hidden">Search shipment delays</span>
            <input value={query} onChange={event => { setQuery(event.target.value); setPage(1) }} placeholder="Product, SKU, order, or customer" />
            {query && <button type="button" onClick={() => { setQuery(''); setPage(1) }} aria-label="Clear search">×</button>}
          </label>

          <div className="oms-table-summary">
            <span>{total ? `Showing ${first}–${last} of ${total} delayed order items` : 'No delayed order items found'}</span>
            <div>
              <button onClick={() => setPage(value => Math.max(1, value - 1))} disabled={loading || page <= 1}>Previous</button>
              <span>Page {page} of {pages}</span>
              <button onClick={() => setPage(value => Math.min(pages, value + 1))} disabled={loading || page >= pages}>Next</button>
            </div>
          </div>

          <div className="oms-table-shell">
            {loading && <p className="oms-table-state" role="status">Loading shipment delay records…</p>}
            {error && <p className="oms-table-state" role="alert">{error} <button onClick={() => setRefresh(value => value + 1)}>Retry</button></p>}
            {!loading && !error && delays.length === 0 && <p className="oms-table-state">No order items have a recorded delayed delivery date.</p>}
            <div className="oms-table-scroll">
              <table className="oms-data-table oms-shipment-delays-table">
                <thead><tr><th scope="col">Product / SKU</th><th scope="col">Order</th><th scope="col">Customer</th><th scope="col">Order status</th><th scope="col">Previous date</th><th scope="col">Delayed date</th><th scope="col">Updated</th><th scope="col">Updated by</th></tr></thead>
                <tbody>{!loading && delays.map(item => (
                  <tr key={`${item.OrderID}:${item.ProductName}`}>
                    <td><strong>{item.ProductName}</strong><small>{item.SKU} · Qty {item.Quantity}</small></td>
                    <td><button className="oms-order-id" onClick={() => go(`/oms/orders/${encodeURIComponent(item.OrderID)}`)}>{item.OrderID}</button></td>
                    <td><strong>{item.CustomerName || 'Guest customer'}</strong><small>{item.CustomerEmail || '—'}</small></td>
                    <td><span className={`oms-status oms-status-${item.OrderStatus}`}>{orderStatusLabels[item.OrderStatus] || item.OrderStatus}</span></td>
                    <td>{displayDate(item.OriginalDeliveryDate)}</td>
                    <td><strong>{displayDate(item.DelayedDeliveryDate)}</strong></td>
                    <td>{dateTime(item.UpdatedAt)}</td>
                    <td>{item.UpdatedBy || '—'}</td>
                  </tr>
                ))}</tbody>
              </table>
            </div>
          </div>
        </section>
      </div>
    </OMSFrame>
  )
}
