import { useState, useEffect } from 'react'
import { api } from '../../lib/api.js'
import { usePageMetadata } from '../../lib/usePageMetadata.js'
import { Products } from '../../components/Products.jsx'

const categoryOrder = ['technik', 'wohnen', 'beauty', 'mode', 'sport-outdoor', 'werkzeug-handwerk', 'kueche-haushalt', 'garten-balkon']
const categoryIndex = slug => {
  const index = categoryOrder.indexOf(slug)
  return index < 0 ? categoryOrder.length : index
}

export function Shop({ route, go, add, saved, toggleSaved }) {
  const query = new URLSearchParams(route.split('?')[1])
  const category = query.get('category') || ''
  const routeSearch = query.get('search') || ''
  const [products, setProducts] = useState([])
  const [categories, setCategories] = useState([])
  const [search, setSearch] = useState(routeSearch)
  const [sort, setSort] = useState(query.get('sort') || 'featured')
  const [page, setPage] = useState(1)
  const [hasMore, setHasMore] = useState(false)
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(true)
  const [loadingMore, setLoadingMore] = useState(false)
  const [reload, setReload] = useState(0)
  useEffect(() => { setSearch(routeSearch) }, [routeSearch])
  useEffect(() => { setPage(1) }, [category, routeSearch, sort])
  useEffect(() => {
    let active = true
    if (page === 1) {
      setLoading(true)
      setLoadingMore(false)
    }
    else setLoadingMore(true)
    setError('')
    const params = new URLSearchParams({ page: String(page), limit: '24', sort })
    if (category) params.set('category', category)
    if (routeSearch) params.set('search', routeSearch)
    const requests = page === 1
      ? Promise.all([api('/api/categories', { cache: reload ? 'no-store' : undefined }), api(`/api/products?${params}`, { cache: reload ? 'no-store' : undefined })])
      : api(`/api/products?${params}`, { cache: reload ? 'no-store' : undefined }).then(productData => [null, productData])
    requests.then(([categoryData, productData]) => {
      if (active) {
        if (categoryData) setCategories(categoryData.categories || [])
        setProducts(current => page === 1 ? (productData.products || []) : [...current, ...(productData.products || [])])
        setHasMore(Boolean(productData.hasMore))
        setError('')
      }
    }).catch(cause => { if (active) setError(cause.message) })
      .finally(() => {
        if (active) {
          if (page === 1) setLoading(false)
          else setLoadingMore(false)
        }
      })
    return () => { active = false }
  }, [category, routeSearch, reload, page, sort])
  const currentCategory = categories.find(item => item.Slug === category)
  const rootCategories = categories.filter(item => !item.ParentID)
    .sort((a, b) => categoryIndex(a.Slug) - categoryIndex(b.Slug) || a.Name.localeCompare(b.Name, 'de'))
  const subcategoriesFor = parentID => categories.filter(item => item.ParentID === parentID)
  const parentCategory = categories.find(item => item.ID === currentCategory?.ParentID)
  const activeRoot = currentCategory
    ? (parentCategory?.ParentID ? categories.find(item => item.ID === parentCategory.ParentID) : parentCategory || currentCategory)
    : null
  const activeSubcategories = activeRoot ? subcategoriesFor(activeRoot.ID) : []
  const directGrandcategories = currentCategory ? subcategoriesFor(currentCategory.ID) : []
  const activeGrandcategories = currentCategory?.ParentID
    ? directGrandcategories.length ? directGrandcategories : parentCategory?.ParentID ? subcategoriesFor(parentCategory.ID) : []
    : []
  const catalogPath = (nextCategory = category, nextSearch = routeSearch, nextSort = sort) => {
    const params = new URLSearchParams()
    if (nextCategory) params.set('category', nextCategory)
    if (nextSearch) params.set('search', nextSearch)
    if (nextSort !== 'featured') params.set('sort', nextSort)
    const queryString = params.toString()
    return `/shop${queryString ? `?${queryString}` : ''}`
  }
  usePageMetadata(currentCategory ? `${currentCategory.Name} — MITTE` : 'Alle Produkte — MITTE', currentCategory?.Description || 'Entdecke Technik, Wohnen, Beauty, Mode und mehr bei MITTE.')
  const submit = event => { event.preventDefault(); go(catalogPath(category, search.trim(), sort)) }
  return <section className="catalog">
    <div className="catalog-masthead">
      <div><p className="catalog-route"><span>MITTE</span><i>→</i><b>{currentCategory?.Name || 'ALLE PRODUKTE'}</b></p><h1>{currentCategory?.Name || 'Alles auf einen Blick.'}</h1><p>{currentCategory?.Description || 'Entdecke Produkte aus allen Bereichen – und finde schnell, was du suchst.'}</p></div>
      <div className="catalog-sign-index"><span>ZIEL</span><strong>{currentCategory ? '01' : '00'}</strong><i>↙</i></div>
    </div>
    <div className="catalog-tools">
      <form onSubmit={submit} role="search"><label htmlFor="shop-search">Wonach suchst du?</label><div><input id="shop-search" value={search} onChange={event => setSearch(event.target.value)} placeholder="Produktname oder Idee"/><button aria-label="Suche starten">Suchen <span>→</span></button></div></form>
      <label className="sort-control">Sortieren nach<select value={sort} onChange={event => { const nextSort = event.target.value; setProducts([]); setSort(nextSort); go(catalogPath(category, routeSearch, nextSort)) }} aria-label="Produkte sortieren"><option value="featured">Empfohlen</option><option value="bestselling">Am häufigsten gekauft</option><option value="price-low">Preis: niedrig zuerst</option><option value="price-high">Preis: hoch zuerst</option></select></label>
    </div>
    <nav className="catalog-destinations" aria-label="Produktbereiche"><button className={!category ? 'selected' : ''} onClick={() => go(catalogPath('', '', sort))}><span>00</span>Alles</button>{rootCategories.map((item, index) => <button key={item.Slug} className={activeRoot?.ID === item.ID ? 'selected' : ''} onClick={() => go(catalogPath(item.Slug, '', sort))}><span>{String(index + 1).padStart(2, '0')}</span>{item.Name}</button>)}</nav>
    {activeRoot && <nav className="catalog-subdestinations" aria-label={`${activeRoot.Name} Unterkategorien`}>
      {activeSubcategories.map(item => <button key={item.Slug} className={currentCategory?.ID === item.ID ? 'selected' : ''} onClick={() => go(catalogPath(item.Slug, '', sort))}>{item.Name}</button>)}
    </nav>}
    {activeGrandcategories.length > 0 && <nav className="catalog-subdestinations catalog-granddestinations" aria-label={`${currentCategory.Name} Unterkategorien`}>
      {activeGrandcategories.map(item => <button key={item.Slug} className={category === item.Slug ? 'selected' : ''} onClick={() => go(catalogPath(item.Slug, '', sort))}>{item.Name}</button>)}
    </nav>}
    {error && <div className="storefront-message" role="alert">{error}<button onClick={() => setReload(value => value + 1)}>Erneut versuchen</button></div>}
    {loading && <div className="loading storefront-loading">Sortiment wird geladen …</div>}
    {!loading && !error && <div className="catalog-result-line"><span>{products.length} {products.length === 1 ? 'Produkt' : 'Produkte'} geladen</span><span>{currentCategory?.Name || 'MITTE Sortiment'}</span></div>}
    {!loading && (products.length > 0 || !error) && <Products products={products} go={go} add={add} saved={saved} toggleSaved={toggleSaved} />}
    {hasMore && !error && <div className="catalog-pagination"><button className="route-cta" disabled={loadingMore} onClick={() => setPage(current => current + 1)}>{loadingMore ? 'Weitere Produkte werden geladen …' : 'Weitere Produkte laden →'}</button></div>}
  </section>
}
