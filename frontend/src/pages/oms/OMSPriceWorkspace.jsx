import { Fragment, useEffect, useRef, useState } from 'react'
import { OMSAccess, OMSFrame } from './OMSDashboard.jsx'

const pageSize = 25
const statusLabels = {
  draft: 'Draft',
  active: 'Active',
  scheduled: 'Scheduled',
  archived: 'Archived'
}

const auditActionLabels = {
  created: 'Product created',
  updated: 'Product updated',
  status_changed: 'Status changed',
  inventory_adjusted: 'Inventory adjusted',
  inventory_received: 'Inventory received'
}

function auditChangeValue(key, value, euro) {
  if (key === 'priceCents' || key === 'unitCostCents') return euro(value)
  if (typeof value === 'boolean') return value ? 'Yes' : 'No'
  if (value === null || value === undefined || value === '') return '—'
  if (typeof value === 'object') return JSON.stringify(value)
  return String(value)
}

export function OMSPriceWorkspace({ user, go, api, euro, isAuthorizationError, sellableOnly = false }) {
  const [prices, setPrices] = useState([])
  const [query, setQuery] = useState('')
  const [status, setStatus] = useState('')
  const [page, setPage] = useState(1)
  const [pages, setPages] = useState(1)
  const [total, setTotal] = useState(0)
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(true)
  const [refresh, setRefresh] = useState(0)
  const [openAuditID, setOpenAuditID] = useState(null)
  const [auditByID, setAuditByID] = useState({})
  const [auditLoadingID, setAuditLoadingID] = useState(null)
  const [auditError, setAuditError] = useState('')
  const requestID = useRef(0)
  const auditRequestID = useRef(0)

  const loadAudit = async product => {
    setAuditError('')
    const request = ++auditRequestID.current
    setAuditLoadingID(product.ID)
    try {
      const result = await api(`/api/admin/catalog/products/${product.ID}/audit`, { cache: 'no-store' })
      if (request === auditRequestID.current) {
        setAuditByID(current => ({ ...current, [product.ID]: result.audit || [] }))
      }
    } catch (cause) {
      if (request === auditRequestID.current) setAuditError(cause.message)
    } finally {
      if (request === auditRequestID.current) setAuditLoadingID(null)
    }
  }

  const toggleAudit = product => {
    if (openAuditID === product.ID) {
      auditRequestID.current += 1
      setOpenAuditID(null)
      setAuditLoadingID(null)
      setAuditError('')
      return
    }
    setOpenAuditID(product.ID)
    setAuditError('')
    if (auditByID[product.ID]) {
      auditRequestID.current += 1
      setAuditLoadingID(null)
      return
    }
    loadAudit(product)
  }

  useEffect(() => {
    const request = ++requestID.current
    const timer = setTimeout(async () => {
      setLoading(true)
      setError('')
      try {
        const params = new URLSearchParams({
          search: query.trim(),
          status: sellableOnly ? 'active' : status,
          page: String(page),
          ...(sellableOnly ? { sellable: 'true' } : {})
        })
        const result = await api(`/api/admin/catalog/prices?${params}`, { cache: 'no-store' })
        if (request !== requestID.current) return
        setPrices(result.prices || [])
        setTotal(result.total || 0)
        setPages(result.pages || 1)
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
  }, [query, status, page, refresh, api, sellableOnly])

  if (error && !prices.length && isAuthorizationError(error)) return <OMSAccess go={go} error={error} />
  const first = total ? (page - 1) * pageSize + 1 : 0
  const last = Math.min(page * pageSize, total)

  return (
    <OMSFrame user={user} go={go} title={sellableOnly ? 'Sellable queue' : 'Selling prices'}>
      <div className="oms-workspace">
        <header className="oms-page-heading">
          <div>
            <h1>{sellableOnly ? 'Sellable queue' : 'Selling prices'}</h1>
            <p>{sellableOnly
              ? 'Review active catalog products with positive available stock.'
              : 'Review catalog prices and open a product record to change its selling price.'}</p>
          </div>
          <span className="oms-customer-count">{total.toLocaleString()} products</span>
        </header>
        <p className="oms-matrix-note">{sellableOnly
          ? 'Sellable means the product is active and its currently available inventory is greater than zero. Draft, archived, and out-of-stock products are excluded; inventory reservations follow the existing stock model.'
          : 'This queue shows the current selling price stored in the catalog. Price changes are made in the product editor and use the existing catalog audit trail.'}</p>
        {error && <div className="oms-inline-error" role="alert">{error} <button onClick={() => setRefresh(value => value + 1)}>Retry</button></div>}

        <section className="oms-price-browser" aria-label="Selling price list">
          <div className="oms-price-filters">
            <label className="oms-search">
              <span className="oms-search-icon" aria-hidden="true">⌕</span>
              <span className="visually-hidden">Search priced products</span>
              <input value={query} onChange={event => { setQuery(event.target.value); setPage(1) }} placeholder="Product name, SKU, or slug" />
              {query && <button type="button" onClick={() => { setQuery(''); setPage(1) }} aria-label="Clear search">×</button>}
            </label>
            {!sellableOnly && <label className="oms-price-status">Status
              <select value={status} onChange={event => { setStatus(event.target.value); setPage(1) }}>
                <option value="">All statuses</option>
                <option value="draft">Draft</option>
                <option value="active">Active</option>
                <option value="scheduled">Scheduled</option>
                <option value="archived">Archived</option>
              </select>
            </label>}
          </div>

          <div className="oms-table-summary">
            <span>{total ? `Showing ${first}–${last} of ${total} products` : 'No products found'}</span>
            <div>
              <button onClick={() => setPage(current => Math.max(1, current - 1))} disabled={loading || page <= 1}>Previous</button>
              <span>Page {page} of {pages}</span>
              <button onClick={() => setPage(current => Math.min(pages, current + 1))} disabled={loading || page >= pages}>Next</button>
            </div>
          </div>

          <div className="oms-table-shell">
            {loading && <p className="oms-table-state" role="status">Loading selling prices…</p>}
            {error && <p className="oms-table-state" role="alert">{error}</p>}
            {!loading && !error && !prices.length && <p className="oms-table-state">No products match these filters.</p>}
            <div className="oms-table-scroll">
              <table className="oms-data-table oms-price-table">
                <thead><tr><th scope="col">Product</th><th scope="col">Category</th><th scope="col">Status</th>{sellableOnly && <th scope="col">Available</th>}<th scope="col">Selling price</th><th scope="col">Updated</th><th scope="col"><span className="visually-hidden">Product actions</span></th></tr></thead>
                <tbody>{!loading && prices.map(product => (
                  <Fragment key={product.ID}>
                    <tr>
                      <td><strong>{product.Name}</strong><small>{product.SKU}</small></td>
                      <td>{product.CategoryName}</td>
                      <td><span className={`oms-price-status-tag oms-price-status-${product.Status}`}>{statusLabels[product.Status] || product.Status}</span></td>
                      {sellableOnly && <td className="oms-price-value">{product.Available}</td>}
                      <td className="oms-price-value">{euro(product.PriceCents)}</td>
                      <td>{new Intl.DateTimeFormat('de-DE', { dateStyle: 'medium' }).format(new Date(product.UpdatedAt))}</td>
                      <td><div className="oms-price-actions">
                        <button className="oms-row-action" onClick={() => toggleAudit(product)} aria-expanded={openAuditID === product.ID}>History</button>
                        <button className="oms-row-action" onClick={() => go(`/admin/catalog?search=${encodeURIComponent(product.SKU)}`)}>Edit</button>
                      </div></td>
                    </tr>
                    {openAuditID === product.ID && (
                      <tr key={`${product.ID}-audit`} className="oms-audit-row">
                        <td colSpan={sellableOnly ? 7 : 6}>
                          <section className="oms-product-audit" aria-label={`Change history for ${product.Name}`}>
                            <div className="oms-product-audit-heading">
                              <h2>Change history</h2>
                              <span>{product.Name} · {product.SKU}</span>
                            </div>
                            {auditLoadingID === product.ID && <p className="oms-table-state" role="status">Loading recorded changes…</p>}
                            {auditError && <p className="oms-inline-error" role="alert">{auditError} <button onClick={() => loadAudit(product)}>Retry</button></p>}
                            {!auditLoadingID && !auditError && auditByID[product.ID]?.length === 0 && <p className="oms-table-state">No product changes have been recorded.</p>}
                            {!auditLoadingID && !auditError && auditByID[product.ID]?.length > 0 && (
                              <ol className="oms-audit-list">
                                {auditByID[product.ID].map((entry, index) => (
                                  <li key={`${entry.createdAt}-${index}`}>
                                    <div className="oms-audit-meta">
                                      <strong>{auditActionLabels[entry.action] || entry.action}</strong>
                                      <time dateTime={entry.createdAt}>{new Intl.DateTimeFormat('de-DE', { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(entry.createdAt))}</time>
                                      <span>{entry.actorEmail}</span>
                                    </div>
                                    {entry.changes && typeof entry.changes === 'object' && (
                                      <dl className="oms-audit-changes">
                                        {Object.entries(entry.changes).map(([key, value]) => (
                                          <div key={key}><dt>{key}</dt><dd>{auditChangeValue(key, value, euro)}</dd></div>
                                        ))}
                                      </dl>
                                    )}
                                  </li>
                                ))}
                              </ol>
                            )}
                          </section>
                        </td>
                      </tr>
                    )}
                  </Fragment>
                ))}</tbody>
              </table>
            </div>
          </div>
        </section>
      </div>
    </OMSFrame>
  )
}
