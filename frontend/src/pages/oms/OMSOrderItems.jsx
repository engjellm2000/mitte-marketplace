import { useEffect, useRef, useState } from 'react'
import { euro, api, isAuthorizationError } from '../../lib/api.js'
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

export function OMSOrderItems({ user, go }) {
  const [items, setItems] = useState([])
  const [query, setQuery] = useState('')
  const [status, setStatus] = useState('all')
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
        const params = new URLSearchParams({
          q: query.trim(),
          status,
          page: String(page),
          limit: String(pageSize)
        })
        const result = await api(`/api/operations/order-items?${params}`, { cache: 'no-store' })
        if (request !== requestID.current) return
        setItems(result.items || [])
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
  }, [query, status, page, refresh])

  if (error && !items.length && isAuthorizationError(error)) return <OMSAccess go={go} error={error} />
  const first = total ? (page - 1) * pageSize + 1 : 0
  const last = Math.min(page * pageSize, total)

  return (
    <OMSFrame user={user} go={go} title="Order items">
      <div className="oms-workspace">
        <header className="oms-page-heading">
          <div>
            <h1>Order items</h1>
            <p>Search order line snapshots across the full order history.</p>
          </div>
          <button className="oms-secondary-action" onClick={() => setRefresh(value => value + 1)} disabled={loading}>
            {loading ? 'Refreshing…' : 'Refresh data'}
          </button>
        </header>

        <section className="oms-price-browser" aria-label="Order item search">
          <div className="oms-price-filters">
            <label className="oms-search">
              <span className="oms-search-icon" aria-hidden="true">⌕</span>
              <span className="visually-hidden">Search order items</span>
              <input value={query} onChange={event => { setQuery(event.target.value); setPage(1) }} placeholder="Product, SKU, order, or customer" />
              {query && <button type="button" onClick={() => { setQuery(''); setPage(1) }} aria-label="Clear search">×</button>}
            </label>
            <label className="oms-price-status">Order status
              <select value={status} onChange={event => { setStatus(event.target.value); setPage(1) }}>
                <option value="all">All orders</option>
                <option value="open">Open</option>
                <option value="paid">Paid or fulfilled</option>
                <option value="pending_payment">Payment pending</option>
                <option value="invoice_approved">Invoice approved</option>
                <option value="fulfilled">Fulfilled</option>
                <option value="cancelled">Cancelled</option>
              </select>
            </label>
          </div>

          <div className="oms-table-summary">
            <span>{total ? `Showing ${first}–${last} of ${total} order items` : 'No order items found'}</span>
            <div>
              <button onClick={() => setPage(value => Math.max(1, value - 1))} disabled={loading || page <= 1}>Previous</button>
              <span>Page {page} of {pages}</span>
              <button onClick={() => setPage(value => Math.min(pages, value + 1))} disabled={loading || page >= pages}>Next</button>
            </div>
          </div>

          <div className="oms-table-shell">
            {loading && <p className="oms-table-state" role="status">Loading order items…</p>}
            {error && <p className="oms-table-state" role="alert">{error} <button onClick={() => setRefresh(value => value + 1)}>Retry</button></p>}
            {!loading && !error && items.length === 0 && <p className="oms-table-state">No order items match this search and status.</p>}
            <div className="oms-table-scroll">
              <table className="oms-data-table oms-order-items-table">
                <thead><tr><th scope="col">Product / SKU</th><th scope="col">Order</th><th scope="col">Customer</th><th scope="col">Order status</th><th scope="col">Ordered</th><th scope="col">Unit price</th><th scope="col">Quantity</th><th scope="col">Line total</th></tr></thead>
                <tbody>{!loading && items.map(item => (
                  <tr key={`${item.OrderID}:${item.ProductName}`}>
                    <td><strong>{item.ProductName}</strong><small>{item.SKU}</small></td>
                    <td><button className="oms-order-id" onClick={() => go(`/oms/orders/${encodeURIComponent(item.OrderID)}`)}>{item.OrderID}</button></td>
                    <td><strong>{item.CustomerName || 'Guest customer'}</strong><small>{item.CustomerEmail || '—'}</small></td>
                    <td><span className={`oms-status oms-status-${item.OrderStatus}`}>{orderStatusLabels[item.OrderStatus] || item.OrderStatus}</span></td>
                    <td>{dateTime(item.OrderCreatedAt)}</td>
                    <td>{euro(item.UnitPriceCents)}</td>
                    <td>{item.Quantity}</td>
                    <td><strong>{euro(item.UnitPriceCents * item.Quantity)}</strong></td>
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
