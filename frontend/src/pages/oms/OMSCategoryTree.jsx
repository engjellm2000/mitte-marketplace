import { useEffect, useMemo, useState } from 'react'
import { OMSAccess, OMSFrame } from './OMSDashboard.jsx'

function categoryForest(categories) {
  const nodes = categories.map(category => ({ ...category, children: [] }))
  const byID = new Map(nodes.map(category => [category.ID, category]))
  const roots = []
  for (const category of nodes) {
    const parent = byID.get(category.ParentID)
    if (parent && parent.ID !== category.ID) parent.children.push(category)
    else roots.push(category)
  }
  const sort = list => {
    list.sort((a, b) => a.Name.localeCompare(b.Name, 'de'))
    list.forEach(category => sort(category.children))
  }
  sort(roots)
  return roots
}

function filterTree(category, query) {
  const children = category.children.map(child => filterTree(child, query)).filter(Boolean)
  const matches = `${category.Name} ${category.Slug} ${category.Description}`.toLocaleLowerCase('de').includes(query)
  if (!query || matches) return { ...category, children: matches && query ? category.children : children }
  return children.length ? { ...category, children } : null
}

function CategoryNode({ category, depth, query }) {
  return (
    <li className="oms-category-node">
      <details open={Boolean(query) || depth === 0}>
        <summary>
          <span className="oms-category-name">{category.Name}</span>
          <span className="oms-category-slug">{category.Slug}</span>
          <span className="oms-category-id">ID {category.ID}</span>
        </summary>
        {category.Description && <p>{category.Description}</p>}
        {category.children.length > 0 && (
          <ul>{category.children.map(child => <CategoryNode key={child.ID} category={child} depth={depth + 1} query={query} />)}</ul>
        )}
      </details>
    </li>
  )
}

export function OMSCategoryTree({ user, go, api, isAuthorizationError }) {
  const [categories, setCategories] = useState([])
  const [query, setQuery] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(true)
  const [refresh, setRefresh] = useState(0)

  useEffect(() => {
    let current = true
    setLoading(true)
    api('/api/categories', { cache: 'no-store' })
      .then(result => {
        if (!current) return
        setCategories(result.categories || [])
        setError('')
      })
      .catch(cause => {
        if (current) setError(cause.message)
      })
      .finally(() => {
        if (current) setLoading(false)
      })
    return () => { current = false }
  }, [api, refresh])

  const normalizedQuery = query.trim().toLocaleLowerCase('de')
  const visible = useMemo(() => categoryForest(categories)
    .map(category => filterTree(category, normalizedQuery))
    .filter(Boolean), [categories, normalizedQuery])

  if (error && isAuthorizationError(error)) return <OMSAccess go={go} error={error} />

  return (
    <OMSFrame user={user} go={go} title="Category tree">
      <div className="oms-workspace">
        <header className="oms-page-heading">
          <div>
            <h1>Category tree</h1>
            <p>Explore the category hierarchy currently used by the product catalog.</p>
          </div>
          <span className="oms-customer-count">{categories.length.toLocaleString()} categories</span>
        </header>
        <p className="oms-matrix-note">This is a read-only view of live catalog categories. Category creation, reassignment, and SEO controls are not available through the current staff API.</p>
        {error && <div className="oms-inline-error" role="alert">{error} <button onClick={() => setRefresh(value => value + 1)}>Retry</button></div>}
        <section className="oms-category-browser" aria-label="Category hierarchy">
          <label className="oms-search">
            <span className="oms-search-icon" aria-hidden="true">⌕</span>
            <span className="visually-hidden">Search categories</span>
            <input value={query} onChange={event => setQuery(event.target.value)} placeholder="Search category name, slug, or description" />
            {query && <button type="button" onClick={() => setQuery('')} aria-label="Clear search">×</button>}
          </label>
          {loading && <p className="oms-table-state" role="status">Loading category tree…</p>}
          {!loading && !error && categories.length === 0 && <p className="oms-table-state">No categories are available.</p>}
          {!loading && !error && categories.length > 0 && visible.length === 0 && <p className="oms-table-state">No categories match this search.</p>}
          {!loading && !error && visible.length > 0 && <ul className="oms-category-tree">{visible.map(category => <CategoryNode key={category.ID} category={category} depth={0} query={normalizedQuery} />)}</ul>}
        </section>
      </div>
    </OMSFrame>
  )
}
