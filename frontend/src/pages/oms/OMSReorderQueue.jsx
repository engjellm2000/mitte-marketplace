import { useEffect, useRef, useState } from 'react'
import { OMSAccess, OMSFrame } from './OMSDashboard.jsx'

const pageSize = 50
const productStatusLabels = {
  draft: 'Draft',
  active: 'Active',
  scheduled: 'Scheduled',
  archived: 'Archived'
}

export function OMSReorderQueue({ user, go, api, euro, isAuthorizationError }) {
  const [data, setData] = useState(null)
  const [query, setQuery] = useState('')
  const [page, setPage] = useState(1)
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
          search: query.trim(),
          stock: 'low',
          page: String(page)
        })
        const result = await api(`/api/admin/inventory?${params}`, { cache: 'no-store' })
        if (request === requestID.current) setData(result)
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
  }, [query, page, refresh, api])

  if (error && !data && isAuthorizationError(error)) return <OMSAccess go={go} error={error} />
  const products = data?.products || []
  const total = data?.total || 0
  const pages = Math.max(data?.pages || 1, 1)
  const first = total ? (page - 1) * pageSize + 1 : 0
  const last = Math.min(page * pageSize, total)

  return (
    <OMSFrame user={user} go={go} title="Reorder queue">
      <div className="oms-workspace">
        <header className="oms-page-heading">
          <div>
            <h1>Reorder queue</h1>
            <p>Find products at or below their configured reorder point, including items at zero stock.</p>
          </div>
          <button className="oms-secondary-action" onClick={() => setRefresh(value => value + 1)} disabled={loading}>
            {loading ? 'Refreshing…' : 'Refresh data'}
          </button>
        </header>
        <p className="oms-matrix-note">The queue uses the inventory workspace’s existing low-stock rule. Archived products are excluded; zero-stock products are included even when no reorder point is set. Stock adjustments and receipts are recorded in the inventory workspace.</p>

        {error && <div className="oms-inline-error" role="alert">{error} <button onClick={() => setRefresh(value => value + 1)}>Retry</button></div>}

        <section className="oms-reorder-browser" aria-label="Products requiring stock review">
          <div className="oms-reorder-summary">
            <div><span>Products requiring attention</span><strong>{data ? total.toLocaleString() : '—'}</strong></div>
            <div><span>Out of stock · full catalog</span><strong>{data?.summary?.outOfStockProducts?.toLocaleString() ?? '—'}</strong></div>
          </div>
          <label className="oms-search">
            <span className="oms-search-icon" aria-hidden="true">⌕</span>
            <span className="visually-hidden">Search reorder queue</span>
            <input value={query} onChange={event => { setQuery(event.target.value); setPage(1) }} placeholder="Product name, SKU, or barcode" />
            {query && <button type="button" onClick={() => { setQuery(''); setPage(1) }} aria-label="Clear search">×</button>}
          </label>
          <div className="oms-table-summary">
            <span>{total ? `Showing ${first}–${last} of ${total} products` : 'No products require stock review'}</span>
            <div>
              <button onClick={() => setPage(value => Math.max(1, value - 1))} disabled={loading || page <= 1}>Previous</button>
              <span>Page {page} of {pages}</span>
              <button onClick={() => setPage(value => Math.min(pages, value + 1))} disabled={loading || page >= pages}>Next</button>
            </div>
          </div>
          <div className="oms-table-shell">
            {loading && <p className="oms-table-state" role="status">Loading reorder queue…</p>}
            {error && <p className="oms-table-state" role="alert">{error}</p>}
            {!loading && !error && products.length === 0 && <p className="oms-table-state">No products match this search. All other non-archived products are above their reorder point or have no stock warning.</p>}
            <div className="oms-table-scroll">
              <table className="oms-data-table oms-reorder-table">
                <thead><tr><th scope="col">Product / SKU</th><th scope="col">Category</th><th scope="col">Product status</th><th scope="col">Availability</th><th scope="col">Reserved</th><th scope="col">Reorder point</th><th scope="col">Supplier</th><th scope="col">Lead time</th><th scope="col">Unit cost</th></tr></thead>
                <tbody>{!loading && products.map(product => {
                  const outOfStock = product.Inventory === 0
                  return <tr key={product.ID}>
                    <td><strong>{product.Name}</strong><small>{product.SKU}</small></td>
                    <td>{product.Category?.Name || '—'}</td>
                    <td>{productStatusLabels[product.Status] || product.Status}</td>
                    <td><span className={`oms-price-status-tag ${outOfStock ? 'oms-reorder-out' : 'oms-reorder-low'}`}>{outOfStock ? 'Out of stock' : 'At reorder point'}</span><strong className="oms-reorder-quantity">{product.Inventory}</strong></td>
                    <td>{product.reservedStock}</td>
                    <td>{product.reorderPoint || 'Not set'}</td>
                    <td>{product.supplierName || 'Not provided'}</td>
                    <td>{product.leadTimeDays ? `${product.leadTimeDays} days` : 'Not set'}</td>
                    <td>{euro(product.unitCostCents)}</td>
                  </tr>
                })}</tbody>
              </table>
            </div>
          </div>
          <div className="oms-reorder-footer">
            <span>Need to receive stock or record an adjustment?</span>
            <button className="oms-text-action" onClick={() => go('/bestand')}>Open inventory workspace →</button>
          </div>
        </section>
      </div>
    </OMSFrame>
  )
}
