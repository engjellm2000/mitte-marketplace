import { useEffect, useRef, useState } from 'react'
import { api, csrf, isAuthorizationError } from '../../lib/api.js'
import { addressLines, dateTime } from '../../lib/labels.js'
import { OMSAccess, OMSFrame } from './OMSDashboard.jsx'

const pageSize = 25
const orderPageSize = 10
const orderStatusLabels = {
  pending_payment: 'Payment pending',
  invoice_approved: 'Invoice approved',
  paid: 'Paid',
  fulfilled: 'Fulfilled',
  cancelled: 'Cancelled'
}

export function OMSCustomers({ user, go }) {
  const [customers, setCustomers] = useState([])
  const [total, setTotal] = useState(0)
  const [pages, setPages] = useState(1)
  const [page, setPage] = useState(1)
  const [query, setQuery] = useState('')
  const [selected, setSelected] = useState(null)
  const [orders, setOrders] = useState([])
  const [orderCount, setOrderCount] = useState(0)
  const [ordersPage, setOrdersPage] = useState(1)
  const [ordersPages, setOrdersPages] = useState(1)
  const [error, setError] = useState('')
  const [orderError, setOrderError] = useState('')
  const [notice, setNotice] = useState('')
  const [fraudAction, setFraudAction] = useState('')
  const [fraudReason, setFraudReason] = useState('')
  const [fraudBusy, setFraudBusy] = useState(false)
  const [loading, setLoading] = useState(true)
  const [ordersLoading, setOrdersLoading] = useState(false)
  const [refresh, setRefresh] = useState(0)
  const requestID = useRef(0)
  const orderRequestID = useRef(0)

  useEffect(() => {
    const request = ++requestID.current
    const timer = setTimeout(async () => {
      setLoading(true)
      setError('')
      try {
        const params = new URLSearchParams({ q: query.trim(), page: String(page), limit: String(pageSize) })
        const result = await api(`/api/operations/customers?${params}`, { cache: 'no-store' })
        if (request !== requestID.current) return
        const loaded = result.customers || []
        setCustomers(loaded)
        setTotal(result.total || 0)
        setPages(result.pages || 1)
        setSelected(current => loaded.find(customer => customer.ID === current?.ID) || null)
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

  useEffect(() => {
    const request = ++orderRequestID.current
    setOrders([])
    setOrderCount(0)
    setOrderError('')
    if (!selected) {
      setOrdersLoading(false)
      return () => { orderRequestID.current += 1 }
    }
    setOrdersLoading(true)
    const params = new URLSearchParams({ page: String(ordersPage), limit: String(orderPageSize) })
    api(`/api/operations/customers/${selected.ID}/orders?${params}`, { cache: 'no-store' })
      .then(result => {
        if (request !== orderRequestID.current) return
        setOrders(result.orders || [])
        setOrderCount(result.total || 0)
        setOrdersPages(result.pages || 1)
      })
      .catch(cause => {
        if (request === orderRequestID.current) setOrderError(cause.message)
      })
      .finally(() => {
        if (request === orderRequestID.current) setOrdersLoading(false)
      })
    return () => { orderRequestID.current += 1 }
  }, [selected, ordersPage])

  const selectCustomer = customer => {
    setOrdersPage(1)
    setSelected(customer)
  }

  const updateFraudStatus = async event => {
    event.preventDefault()
    if (!selected || !fraudAction || fraudBusy) return
    setFraudBusy(true)
    setError('')
    setNotice('')
    try {
      await api(`/api/admin/customer-fraud/${selected.ID}`, {
        method: 'POST',
        headers: { 'X-CSRF-Token': csrf },
        body: JSON.stringify({ flagged: fraudAction === 'flag', reason: fraudReason.trim() })
      })
      setFraudReason('')
      setFraudAction('')
      setNotice(fraudAction === 'flag'
        ? 'Das Konto ist gesperrt. Offene und neue Bestellungen werden angehalten.'
        : 'Die Fraud-Sperre wurde aufgehoben.')
      setRefresh(current => current + 1)
    } catch (cause) {
      setError(cause.message)
    } finally {
      setFraudBusy(false)
    }
  }

  if (error && !customers.length && isAuthorizationError(error)) return <OMSAccess go={go} error={error} />
  const selectedLines = selected ? addressLines(selected.Profile?.Address) : []
  const start = total ? (page - 1) * pageSize + 1 : 0
  const end = Math.min(page * pageSize, total)

  return (
    <OMSFrame user={user} go={go} title="Customers">
      <div className="oms-workspace">
        <header className="oms-page-heading">
          <div>
            <h1>Customers</h1>
            <p>Search registered customer accounts and review their profile and linked order records.</p>
          </div>
          <span className="oms-customer-count">{total.toLocaleString()} accounts</span>
        </header>

        <p className="oms-matrix-note">This directory contains registered customer accounts only. Guest checkouts are available in order search. Trustability, courier assignment, and customer assignment tools are not configured in this system.</p>
        {error && <div className="oms-inline-error" role="alert">{error}</div>}

        <section className="oms-customer-browser" aria-label="Customer account search">
          <div className="oms-section-heading">
            <div><h2>Customer search</h2><p>Search by email, name, company, or phone number.</p></div>
          </div>
          <label className="oms-search">
            <span className="oms-search-icon" aria-hidden="true">⌕</span>
            <span className="visually-hidden">Search customer accounts</span>
            <input value={query} onChange={event => { setQuery(event.target.value); setPage(1) }} placeholder="Email, name, company, or phone" />
            {query && <button type="button" onClick={() => { setQuery(''); setPage(1) }} aria-label="Clear search">×</button>}
          </label>
          <div className="oms-table-summary">
            <span>{total ? `Showing ${start}–${end} of ${total} accounts` : 'No customer accounts found'}</span>
            <div>
              <button onClick={() => setPage(current => Math.max(1, current - 1))} disabled={loading || page <= 1}>Previous</button>
              <span>Page {page} of {pages}</span>
              <button onClick={() => setPage(current => Math.min(pages, current + 1))} disabled={loading || page >= pages}>Next</button>
            </div>
          </div>
          <div className="oms-table-shell">
            {loading && <p className="oms-table-state" role="status">Loading customer accounts…</p>}
            {error && <p className="oms-table-state" role="alert">{error} <button onClick={() => setRefresh(current => current + 1)}>Retry</button></p>}
            {!loading && !error && customers.length === 0 && <p className="oms-table-state">No registered customer accounts match this search.</p>}
            <div className="oms-table-scroll">
              <table className="oms-data-table oms-customer-table">
                <thead><tr><th scope="col">Customer</th><th scope="col">Account type</th><th scope="col">Company</th><th scope="col">Orders</th><th scope="col">Last order</th><th scope="col"><span className="visually-hidden">Profile</span></th></tr></thead>
                <tbody>{!loading && customers.map(customer => (
                  <tr key={customer.ID} className={`${selected?.ID === customer.ID ? 'oms-selected-customer' : ''} ${customer.FraudFlagged ? 'oms-fraud-customer' : ''}`}>
                    <td><strong>{customer.Profile?.FullName || 'Name not provided'}</strong><small>{customer.Email}</small>{customer.FraudFlagged && <span className="oms-fraud-label">Fraud-Sperre</span>}</td>
                    <td>{customer.Profile?.CustomerType === 'b2b' ? 'Business' : 'Private'}</td>
                    <td>{customer.Profile?.CompanyName || '—'}</td>
                    <td>{customer.OrderCount}</td>
                    <td>{customer.LastOrderAt ? dateTime(customer.LastOrderAt) : '—'}</td>
                    <td><button className="oms-row-action" onClick={() => selectCustomer(customer)} aria-expanded={selected?.ID === customer.ID}>Details</button></td>
                  </tr>
                ))}</tbody>
              </table>
            </div>
          </div>
        </section>

        {selected && (
          <section className="oms-customer-profile" aria-labelledby="oms-selected-customer">
            <div className="oms-customer-profile-heading">
              <div><h2 id="oms-selected-customer">{selected.Profile?.FullName || 'Customer profile'}</h2><a href={`mailto:${selected.Email}`}>{selected.Email}</a></div>
              <button className="oms-text-action" onClick={() => setSelected(null)}>Close profile</button>
            </div>
            {selected.FraudFlagged && <div className="oms-fraud-alert oms-v2-message error" role="alert">
              <div><strong>Fraud-Prüfung · Konto gesperrt</strong><p>{selected.FraudReason}</p>
                <small>Markiert von {selected.FraudFlaggedBy || 'Admin'} · {selected.FraudFlaggedAt ? dateTime(selected.FraudFlaggedAt) : 'Zeitpunkt nicht verfügbar'}</small>
              </div>
            </div>}
            {notice && <p className="oms-fraud-notice" role="status">{notice}</p>}
            {error && <p className="oms-inline-error" role="alert">{error}</p>}
            {user?.role === 'admin' && <div className="oms-fraud-controls">
              {!fraudAction
                ? <button className={selected.FraudFlagged ? 'oms-fraud-clear' : 'oms-fraud-mark'} onClick={() => setFraudAction(selected.FraudFlagged ? 'clear' : 'flag')}>
                  {selected.FraudFlagged ? 'Fraud-Sperre aufheben' : 'Als Fraud markieren'}
                </button>
                : <form onSubmit={updateFraudStatus}>
                  <label>
                    <span>{fraudAction === 'flag' ? 'Begründung für die Sperre' : 'Begründung für die Aufhebung'}</span>
                    <textarea value={fraudReason} onChange={event => setFraudReason(event.target.value)} minLength={5} maxLength={500} required rows={2} />
                  </label>
                  <div>
                    <button type="submit" disabled={fraudBusy}>{fraudBusy ? 'Wird gespeichert …' : fraudAction === 'flag' ? 'Sperre aktivieren' : 'Sperre aufheben'}</button>
                    <button type="button" className="oms-text-action" onClick={() => { setFraudAction(''); setFraudReason('') }} disabled={fraudBusy}>Abbrechen</button>
                  </div>
                </form>}
            </div>}
            <dl className="oms-customer-profile-facts">
              <div><dt>Account type</dt><dd>{selected.Profile?.CustomerType === 'b2b' ? 'Business' : 'Private'}</dd></div>
              <div><dt>Account created</dt><dd>{dateTime(selected.CreatedAt)}</dd></div>
              <div><dt>Phone</dt><dd>{selected.Profile?.Phone || 'Not provided'}</dd></div>
              <div><dt>Company</dt><dd>{selected.Profile?.CompanyName || '—'}</dd></div>
              <div><dt>VAT ID</dt><dd>{selected.Profile?.VATID || '—'}</dd></div>
              <div><dt>Buyer reference</dt><dd>{selected.Profile?.BuyerReference || '—'}</dd></div>
              <div className="oms-customer-profile-address"><dt>Profile address</dt><dd>{selectedLines.length ? selectedLines.join(', ') : 'No profile address provided'}</dd></div>
            </dl>
            <div className="oms-customer-orders">
              <div className="oms-section-heading"><div><h3>Linked orders</h3><p>{orderCount} matching order records</p></div></div>
              {orderError && <p className="oms-inline-error" role="alert">{orderError}</p>}
              {ordersLoading && <p className="oms-table-state" role="status">Loading linked orders…</p>}
              {!ordersLoading && !orderError && orders.length === 0 && <p className="oms-table-state">No orders are linked to this customer account.</p>}
              {orders.length > 0 && <div className="oms-recent-list">{orders.map(order => (
                <button key={order.ID} className="oms-recent-row" onClick={() => go(`/oms/orders/${encodeURIComponent(order.ID)}`)}>
                  <span className="oms-recent-primary"><strong>{order.ID}</strong><small>{order.CreatedAt ? dateTime(order.CreatedAt) : '—'}</small></span>
                  <span className={`oms-status oms-status-${order.Status || 'unknown'}`}>{orderStatusLabels[order.Status] || order.Status}</span>
                  <span className="oms-recent-arrow" aria-hidden="true">→</span>
                </button>
              ))}</div>}
              {orderCount > orderPageSize && <div className="oms-table-summary oms-customer-order-pagination">
                <span>Page {ordersPage} of {ordersPages}</span>
                <div>
                  <button onClick={() => setOrdersPage(value => Math.max(1, value - 1))} disabled={ordersLoading || ordersPage <= 1}>Previous</button>
                  <button onClick={() => setOrdersPage(value => Math.min(ordersPages, value + 1))} disabled={ordersLoading || ordersPage >= ordersPages}>Next</button>
                </div>
              </div>}
            </div>
          </section>
        )}
      </div>
    </OMSFrame>
  )
}
