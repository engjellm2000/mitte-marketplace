import { useEffect, useRef, useState } from 'react'
import { api, isAuthorizationError } from '../../lib/api.js'
import { dateTime } from '../../lib/labels.js'
import { OMSAccess, OMSFrame } from './OMSDashboard.jsx'

const pageSize = 25
const statusLabels = {
  pending: 'Pending',
  sending: 'Sending',
  retrying: 'Retry scheduled',
  sent: 'Sent',
  cancelled: 'Cancelled'
}

export function OMSEmailDeliveryQueue({ user, go }) {
  const [deliveries, setDeliveries] = useState([])
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
        const result = await api(`/api/operations/email-deliveries?${params}`, { cache: 'no-store' })
        if (request !== requestID.current) return
        setDeliveries(result.deliveries || [])
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

  if (error && !deliveries.length && isAuthorizationError(error)) return <OMSAccess go={go} error={error} />
  const first = total ? (page - 1) * pageSize + 1 : 0
  const last = Math.min(page * pageSize, total)

  return (
    <OMSFrame user={user} go={go} title="Email delivery queue">
      <div className="oms-workspace">
        <header className="oms-page-heading">
          <div>
            <h1>Email delivery queue</h1>
            <p>Review order email delivery status, retry timing, and recorded send errors.</p>
          </div>
          <button className="oms-secondary-action" onClick={() => setRefresh(value => value + 1)} disabled={loading}>
            {loading ? 'Refreshing…' : 'Refresh data'}
          </button>
        </header>
        <p className="oms-matrix-note">This is a read-only view of delivery metadata. The mail relay owns automatic retries; message bodies and manual retry controls are not exposed here.</p>
        {error && <div className="oms-inline-error" role="alert">{error} <button onClick={() => setRefresh(value => value + 1)}>Retry</button></div>}

        <section className="oms-price-browser" aria-label="Order email deliveries">
          <div className="oms-price-filters">
            <label className="oms-search">
              <span className="oms-search-icon" aria-hidden="true">⌕</span>
              <span className="visually-hidden">Search email deliveries</span>
              <input value={query} onChange={event => { setQuery(event.target.value); setPage(1) }} placeholder="Order ID, recipient, or subject" />
              {query && <button type="button" onClick={() => { setQuery(''); setPage(1) }} aria-label="Clear search">×</button>}
            </label>
            <label className="oms-price-status">Delivery status
              <select value={status} onChange={event => { setStatus(event.target.value); setPage(1) }}>
                <option value="all">All deliveries</option>
                <option value="pending">Pending</option>
                <option value="sending">Sending</option>
                <option value="retrying">Retry scheduled</option>
                <option value="sent">Sent</option>
                <option value="cancelled">Cancelled</option>
              </select>
            </label>
          </div>

          <div className="oms-table-summary">
            <span>{total ? `Showing ${first}–${last} of ${total} deliveries` : 'No email deliveries found'}</span>
            <div>
              <button onClick={() => setPage(value => Math.max(1, value - 1))} disabled={loading || page <= 1}>Previous</button>
              <span>Page {page} of {pages}</span>
              <button onClick={() => setPage(value => Math.min(pages, value + 1))} disabled={loading || page >= pages}>Next</button>
            </div>
          </div>

          <div className="oms-table-shell">
            {loading && <p className="oms-table-state" role="status">Loading email deliveries…</p>}
            {error && <p className="oms-table-state" role="alert">{error}</p>}
            {!loading && !error && deliveries.length === 0 && <p className="oms-table-state">No deliveries match this search and status.</p>}
            <div className="oms-table-scroll">
              <table className="oms-data-table oms-email-delivery-table">
                <thead><tr><th scope="col">Order</th><th scope="col">Recipient</th><th scope="col">Subject</th><th scope="col">Delivery</th><th scope="col">Timing</th><th scope="col">Attempts / last error</th></tr></thead>
                <tbody>{!loading && deliveries.map(delivery => (
                  <tr key={delivery.ID}>
                    <td>{delivery.OrderID
                      ? <button className="oms-order-id" onClick={() => go(`/oms/orders/${encodeURIComponent(delivery.OrderID)}`)}>{delivery.OrderID}</button>
                      : <span>{delivery.CampaignName ? `Campaign · ${delivery.CampaignName}` : 'Customer account'}</span>}</td>
                    <td>{delivery.Recipient}</td>
                    <td>{delivery.Subject}</td>
                    <td><span className={`oms-price-status-tag oms-email-status-${delivery.Status}`}>{statusLabels[delivery.Status] || delivery.Status}</span></td>
                    <td>{delivery.Status === 'cancelled'
                      ? 'Cancelled'
                      : delivery.Status === 'sent'
                      ? `Sent ${dateTime(delivery.SentAt)}`
                      : delivery.Status === 'sending'
                        ? `Claimed until ${dateTime(delivery.ClaimedUntil)}`
                        : `Available ${dateTime(delivery.AvailableAt)}`}</td>
                    <td>{delivery.Attempts}{delivery.LastError && <small className="oms-email-delivery-error">{delivery.LastError}</small>}</td>
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
