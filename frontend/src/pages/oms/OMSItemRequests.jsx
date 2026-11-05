import { useEffect, useRef, useState } from 'react'
import { api, isAuthorizationError } from '../../lib/api.js'
import { dateTime } from '../../lib/labels.js'
import { OMSAccess, OMSFrame } from './OMSDashboard.jsx'

const pageSize = 25
const requestLabels = {
  return_open: 'Return requested',
  replacement_requested: 'Replacement requested'
}

export function OMSItemRequests({ user, go }) {
  const [requests, setRequests] = useState([])
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
        const result = await api(`/api/operations/return-requests?${params}`, { cache: 'no-store' })
        if (request !== requestID.current) return
        setRequests(result.requests || [])
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

  if (error && !requests.length && isAuthorizationError(error)) return <OMSAccess go={go} error={error} />
  const first = total ? (page - 1) * pageSize + 1 : 0
  const last = Math.min(page * pageSize, total)

  return (
    <OMSFrame user={user} go={go} title="Returns & replacement requests">
      <div className="oms-workspace">
        <header className="oms-page-heading">
          <div>
            <h1>Returns &amp; replacement requests</h1>
            <p>Review item requests recorded against fulfilled orders and open the order record.</p>
          </div>
          <button className="oms-secondary-action" onClick={() => setRefresh(value => value + 1)} disabled={loading}>
            {loading ? 'Refreshing…' : 'Refresh data'}
          </button>
        </header>
        <p className="oms-matrix-note">Return and replacement requests are logged against the order. Returns and replacements with return create a warehouse return notice (Retourenavis); refund and replacement shipment decisions remain manual.</p>

        <section className="oms-price-browser" aria-label="Return and replacement request search">
          <div className="oms-price-filters">
            <label className="oms-search">
              <span className="oms-search-icon" aria-hidden="true">⌕</span>
              <span className="visually-hidden">Search requests</span>
              <input value={query} onChange={event => { setQuery(event.target.value); setPage(1) }} placeholder="Product, SKU, order, or customer" />
              {query && <button type="button" onClick={() => { setQuery(''); setPage(1) }} aria-label="Clear search">×</button>}
            </label>
            <label className="oms-price-status">Request type
              <select value={status} onChange={event => { setStatus(event.target.value); setPage(1) }}>
                <option value="all">All requests</option>
                <option value="return_open">Returns</option>
                <option value="replacement_requested">Replacements</option>
              </select>
            </label>
          </div>

          <div className="oms-table-summary">
            <span>{total ? `Showing ${first}–${last} of ${total} requests` : 'No open requests found'}</span>
            <div>
              <button onClick={() => setPage(value => Math.max(1, value - 1))} disabled={loading || page <= 1}>Previous</button>
              <span>Page {page} of {pages}</span>
              <button onClick={() => setPage(value => Math.min(pages, value + 1))} disabled={loading || page >= pages}>Next</button>
            </div>
          </div>

          <div className="oms-table-shell">
            {loading && <p className="oms-table-state" role="status">Loading item requests…</p>}
            {error && <p className="oms-table-state" role="alert">{error} <button onClick={() => setRefresh(value => value + 1)}>Retry</button></p>}
            {!loading && !error && requests.length === 0 && <p className="oms-table-state">No tracked return or replacement requests match this search.</p>}
            <div className="oms-table-scroll">
              <table className="oms-data-table oms-item-requests-table">
                <thead><tr><th scope="col">Request</th><th scope="col">Warehouse notice</th><th scope="col">Order</th><th scope="col">Product / SKU</th><th scope="col">Customer</th><th scope="col">Reason</th><th scope="col">Requested</th><th scope="col">Recorded by</th><th scope="col"><span className="visually-hidden">Order actions</span></th></tr></thead>
                <tbody>{!loading && requests.map(item => (
                  <tr key={`${item.OrderID}:${item.ProductName}`}>
                    <td><span className={`oms-price-status-tag oms-email-status-${item.Status === 'return_open' ? 'retrying' : 'sending'}`}>{item.RequestType === 'replacement_with_return' ? 'Replacement with return' : requestLabels[item.Status] || item.Status}</span></td>
                    <td>{item.ReturnReference || '—'}</td>
                    <td><button className="oms-order-id" onClick={() => go(`/oms/orders/${encodeURIComponent(item.OrderID)}`)}>{item.OrderID}</button></td>
                    <td><strong>{item.ProductName}</strong><small>{item.SKU} · Qty {item.Quantity}</small></td>
                    <td><strong>{item.CustomerName || 'Guest customer'}</strong><small>{item.CustomerEmail || '—'}</small></td>
                    <td className="oms-request-reason">{item.Reason || 'No reason recorded'}</td>
                    <td>{dateTime(item.RequestedAt)}</td>
                    <td>{item.ActorEmail || '—'}</td>
                    <td><button className="oms-row-action" onClick={() => go(`/oms/orders/${encodeURIComponent(item.OrderID)}`)}>Open order</button></td>
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
