import { useEffect, useRef, useState } from 'react'
import { OMSNavigation } from './OMSNavigation.jsx'
import '../../oms-dashboard.css'
import '../../oms-redesign.css'

export const fulfillmentLabels = {
  not_ready: 'Noch nicht versandbereit',
  ready_to_pick: 'Bereit zum Picken',
  picking: 'Wird gepickt',
  packed: 'Verpackt',
  label_ready: 'Label bereit',
  held: 'Angehalten',
  handed_over: 'Übergeben',
  supplier_managed: 'Lieferantenversand'
}

const orderStatusLabels = {
  pending_payment: 'Zahlung ausstehend',
  invoice_approved: 'Rechnung freigegeben',
  paid: 'Bezahlt',
  fulfilled: 'Erfüllt',
  cancelled: 'Storniert'
}

const orderFilters = [
  ['all', 'Alle Aufträge'],
  ['open', 'Offen'],
  ['paid', 'Bezahlt / erfüllt'],
  ['pending_payment', 'Zahlung offen'],
  ['invoice_approved', 'Rechnung freigegeben'],
  ['fulfilled', 'Erfüllt'],
  ['cancelled', 'Storniert']
]

export function OMSFrame({ user, go, children, title = 'Bestellungen' }) {
  return (
    <div className="oms-app-shell">
      <OMSNavigation user={user} go={go} />
      <main className="oms-v2-main operations-main">
        <div className="oms-v2-context">
          <span>{title}</span>
          <button onClick={() => go('/oms')}>← Auftragsübersicht</button>
        </div>
        {children}
      </main>
    </div>
  )
}

export function OMSAccess({ go, error }) {
  return (
    <section className="oms-access">
      <div><b>M</b><span>Mitte OMS</span></div>
      <h1>Zugriff erforderlich.</h1>
      <p>{error}</p>
      <button onClick={() => go('/konto')}>Anmelden</button>
    </section>
  )
}

export function OMSDashboard({ user, go, api, euro, dateTime, isAuthorizationError }) {
  const [orders, setOrders] = useState([])
  const [queue, setQueue] = useState([])
  const [query, setQuery] = useState('')
  const [filter, setFilter] = useState('all')
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(25)
  const [pages, setPages] = useState(1)
  const [total, setTotal] = useState(0)
  const [summary, setSummary] = useState(null)
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(true)
  const requestID = useRef(0)

  const loadOrders = async request => {
    setLoading(true)
    const params = new URLSearchParams({
      q: query.trim(),
      status: filter,
      page: String(page),
      limit: String(pageSize)
    })

    try {
      const result = await api(`/api/operations/orders?${params}`, { cache: 'no-store' })
      if (request !== requestID.current) return
      const loadedOrders = result.orders || []
      setOrders(loadedOrders)
      setTotal(result.total || 0)
      setPages(result.pages || 1)
      setSummary(result.summary || null)
      setError('')
      loadQueue(loadedOrders.map(order => order.ID), request)
    } catch (cause) {
      if (request === requestID.current) setError(cause.message)
    } finally {
      if (request === requestID.current) setLoading(false)
    }
  }

  const loadQueue = async (orderIDs, request) => {
    if (!orderIDs.length) {
      if (request === requestID.current) setQueue([])
      return
    }
    try {
      const params = new URLSearchParams({ orderIds: orderIDs.join(',') })
      const result = await api(`/api/warehouse/fulfillments?${params}`, { cache: 'no-store' })
      if (request === requestID.current) setQueue(result.fulfillments || [])
    } catch (cause) {
      if (request === requestID.current) setError(cause.message)
    }
  }

  const refresh = () => {
    const request = ++requestID.current
    loadOrders(request)
  }

  useEffect(() => {
    const request = ++requestID.current
    const timer = setTimeout(() => loadOrders(request), query ? 250 : 0)
    return () => clearTimeout(timer)
  }, [query, filter, page, pageSize])

  if (error && !orders.length && isAuthorizationError(error)) {
    return <OMSAccess go={go} error={error} />
  }

  const fulfillmentStatusFor = orderID =>
    queue.find(item => item.OrderID === orderID)?.Status
  const kpis = summary || { open: 0, paid: 0, awaiting: 0, openValueCents: 0 }

  return (
    <OMSFrame user={user} go={go} title="Bestellungen">
      <DashboardHeader loading={loading} onRefresh={refresh} />

      {error && <div className="oms-v2-error" role="alert">{error}</div>}

      <OrderKPIs summary={kpis} loading={loading && !summary} euro={euro} />

      <section className="oms-order-board" aria-labelledby="oms-order-board-title">
        <OrderBoardHeader
          query={query}
          onQueryChange={value => {
            setQuery(value)
            setPage(1)
          }}
        />
        <OrderFilters
          filter={filter}
          total={total}
          loading={loading}
          onChange={value => {
            setFilter(value)
            setPage(1)
          }}
        />
        <OrderTable
          orders={orders}
          queueStatus={fulfillmentStatusFor}
          euro={euro}
          dateTime={dateTime}
          loading={loading}
          hasSearch={Boolean(query.trim())}
          hasFilter={filter !== 'all'}
          onOpen={orderID => go(`/oms/orders/${encodeURIComponent(orderID)}`)}
          onClear={() => {
            setQuery('')
            setFilter('all')
            setPage(1)
          }}
        />
        <OrderPagination
          page={page}
          pages={pages}
          pageSize={pageSize}
          loading={loading}
          onPageChange={setPage}
          onPageSizeChange={value => {
            setPageSize(value)
            setPage(1)
          }}
        />
      </section>
    </OMSFrame>
  )
}

function DashboardHeader({ loading, onRefresh }) {
  return (
    <section className="oms-dashboard-hero">
      <div>
        <h1>Alle Aufträge</h1>
        <p>Filtere nach Zahlungs- und Versandstatus oder suche gezielt nach einem Auftrag.</p>
      </div>
      <button className="oms-refresh-v2" onClick={onRefresh} disabled={loading}>
        {loading ? 'Wird aktualisiert …' : '↻ Aktualisieren'}
      </button>
    </section>
  )
}

function OrderKPIs({ summary, loading, euro }) {
  const metrics = [
    { label: 'Offene Bestellungen', value: summary.open, detail: 'ohne stornierte Aufträge' },
    { label: 'Bezahlt oder erfüllt', value: summary.paid, detail: 'bestätigte Zahlungen' },
    { label: 'Zahlung ausstehend', value: summary.awaiting, detail: 'noch nicht bezahlt' },
    { label: 'Wert offener Aufträge', value: euro(summary.openValueCents), detail: 'inklusive Versandkosten' }
  ]

  return (
    <section className="oms-order-pulse" aria-label="Bestellkennzahlen">
      {metrics.map(metric => (
        <span key={metric.label}>
          <strong>{loading ? '—' : metric.value}</strong> {metric.label.toLowerCase()}
        </span>
      ))}
    </section>
  )
}

function OrderBoardHeader({ query, onQueryChange }) {
  return (
    <div className="oms-board-head">
      <div>
        <h2 id="oms-order-board-title">Aufträge</h2>
      </div>
      <label className="oms-search">
        <span aria-hidden="true">⌕</span>
        <input
          aria-label="Bestellungen durchsuchen"
          value={query}
          onChange={event => onQueryChange(event.target.value)}
          placeholder="Bestellnummer, Name, E-Mail oder Firma"
        />
      </label>
    </div>
  )
}

function OrderFilters({ filter, total, loading, onChange }) {
  return (
    <div className="oms-board-tools">
      <div aria-label="Bestellungen nach Status filtern">
        {orderFilters.map(([value, label]) => (
          <button
            key={value}
            className={filter === value ? 'active' : ''}
            aria-pressed={filter === value}
            onClick={() => onChange(value)}
          >
            {label}
          </button>
        ))}
      </div>
      <span aria-live="polite">{loading ? 'Lädt …' : `${total} Treffer`}</span>
    </div>
  )
}

function OrderTable({ orders, queueStatus, euro, dateTime, loading, hasSearch, hasFilter, onOpen, onClear }) {
  return (
    <div className="oms-order-table" role="table" aria-label="Bestellungen">
      <div className="oms-row oms-row-head" role="row">
        <span role="columnheader">Bestellung</span>
        <span role="columnheader">Kunde</span>
        <span role="columnheader">Datum</span>
        <span role="columnheader">Bearbeitung</span>
        <span role="columnheader">Betrag</span>
        <span aria-hidden="true" />
      </div>
      {orders.map(order => (
        <OrderRow
          key={order.ID}
          order={order}
          fulfillmentStatus={queueStatus(order.ID)}
          euro={euro}
          dateTime={dateTime}
          onOpen={() => onOpen(order.ID)}
        />
      ))}
      {!orders.length && (
        <div className="oms-no-results" role="status">
          <p>{loading ? 'Bestellungen werden geladen …' : hasSearch || hasFilter
            ? 'Keine Bestellungen für diese Suche oder diesen Filter.'
            : 'Noch keine Bestellungen eingegangen.'}</p>
          {!loading && (hasSearch || hasFilter) && (
            <button className="oms-clear-filters" onClick={onClear}>Suche und Filter zurücksetzen</button>
          )}
        </div>
      )}
    </div>
  )
}

function OrderRow({ order, fulfillmentStatus, euro, dateTime, onOpen }) {
  const status = fulfillmentStatus || order.Status
  const paymentLabel = orderStatusLabels[order.Status] || order.Status || 'Unbekannt'

  return (
    <button className="oms-row" role="row" onClick={onOpen}>
      <strong role="cell">{order.ID}</strong>
      <span role="cell">
        <b>{order.CustomerName || 'Gastkunde'}</b>
        {order.CompanyName && <small>{order.CompanyName}</small>}
        <small>{order.CustomerEmail || 'Keine E-Mail hinterlegt'}</small>
      </span>
      <span role="cell">{dateTime(order.CreatedAt)}</span>
      <span role="cell">
        <i className={`oms-status-dot ${status}`} />
        {fulfillmentLabels[status] || paymentLabel}
        {fulfillmentStatus && <small>Zahlung: {paymentLabel}</small>}
      </span>
      <strong role="cell">{euro(order.TotalCents)}</strong>
      <em role="cell">Details ansehen →</em>
    </button>
  )
}

function OrderPagination({ page, pages, pageSize, loading, onPageChange, onPageSizeChange }) {
  return (
    <div className="oms-pagination">
      <label>
        Zeilen pro Seite
        <select
          value={pageSize}
          onChange={event => onPageSizeChange(Number(event.target.value))}
          disabled={loading}
        >
          {[25, 50, 100].map(size => <option key={size} value={size}>{size}</option>)}
        </select>
      </label>
      <span aria-live="polite">Seite {page} von {pages}</span>
      <div>
        <button disabled={loading || page <= 1} onClick={() => onPageChange(page - 1)}>
          Zurück
        </button>
        <button disabled={loading || page >= pages} onClick={() => onPageChange(page + 1)}>
          Weiter
        </button>
      </div>
    </div>
  )
}
