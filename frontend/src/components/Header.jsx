import { useState, useEffect, useCallback } from 'react'
import { api } from '../lib/api.js'

const categoryOrder = ['technik', 'wohnen', 'beauty', 'mode', 'sport-outdoor', 'werkzeug-handwerk', 'kueche-haushalt', 'garten-balkon']
const categoryIndex = slug => {
  const index = categoryOrder.indexOf(slug)
  return index < 0 ? categoryOrder.length : index
}

export function Header({ count, savedCount, route, go, onCart, user }) {
  const [menuOpen, setMenuOpen] = useState(false)
  const [categories, setCategories] = useState([])
  const [categoryError, setCategoryError] = useState('')
  const [search, setSearch] = useState('')
  const navigate = path => { setMenuOpen(false); go(path) }
  const loadCategories = useCallback(() => {
    api('/api/categories')
      .then(data => { setCategories(data.categories || []); setCategoryError('') })
      .catch(cause => setCategoryError(cause.message))
  }, [])
  useEffect(() => { loadCategories() }, [loadCategories])
  useEffect(() => {
    const close = event => { if (event.key === 'Escape') setMenuOpen(false) }
    addEventListener('keydown', close)
    return () => removeEventListener('keydown', close)
  }, [])
  useEffect(() => { setMenuOpen(false) }, [route])
  const rootCategories = categories.filter(category => !category.ParentID)
    .sort((a, b) => categoryIndex(a.Slug) - categoryIndex(b.Slug) || a.Name.localeCompare(b.Name, 'de'))
    .slice(0, 5)
  const submitSearch = event => {
    event.preventDefault()
    navigate(`/shop?search=${encodeURIComponent(search.trim())}`)
  }
  return <>
    <div className="announcement" role="status"><span>MITTE — dein Marktplatz für jeden Tag</span><span>Entdecke, was zu dir passt <b aria-hidden="true">→</b></span></div>
    <header className={`storefront-header ${menuOpen ? 'menu-open' : ''}`}>
      <button className="menu-toggle" onClick={() => setMenuOpen(open => !open)} aria-expanded={menuOpen} aria-controls="primary-navigation" aria-label={menuOpen ? 'Menü schließen' : 'Menü öffnen'}><i/><i/></button>
      <button className="brand" onClick={() => navigate('/')} aria-label="MITTE Startseite"><i aria-hidden="true">M</i><span>MITTE<small>ALLES FÜR JEDEN TAG</small></span></button>
      <form className="header-search" role="search" onSubmit={submitSearch}><label className="visually-hidden" htmlFor="header-search">Produkte suchen</label><input id="header-search" value={search} onChange={event => setSearch(event.target.value)} placeholder="Wonach suchst du?" /><button aria-label="Suche starten"><svg viewBox="0 0 24 24" aria-hidden="true"><circle cx="10.8" cy="10.8" r="5.8"/><path d="m16 16 4.2 4.2"/></svg></button></form>
      <nav id="primary-navigation" aria-label="Hauptnavigation">
        {rootCategories.map(category => <button key={category.Slug} className={route.includes(`category=${category.Slug}`) ? 'active' : ''} onClick={() => navigate(`/shop?category=${encodeURIComponent(category.Slug)}`)}>{category.Name}</button>)}
        <button className={route.startsWith('/angebote') ? 'active' : ''} onClick={() => navigate('/angebote')}>Angebote</button>
        <button className={route.startsWith('/shop') && !route.includes('category=') ? 'active' : ''} onClick={() => navigate('/shop')}>Alle Kategorien</button>
      </nav>
      {categoryError && <div className="header-category-error" role="alert">Kategorien konnten nicht geladen werden. <button onClick={loadCategories}>Erneut versuchen</button></div>}
      <div className="header-actions">
        <button className={`icon-action saved-link ${route.startsWith('/merkliste') ? 'active' : ''}`} onClick={() => navigate('/merkliste')} aria-label={`Merkliste, ${savedCount} Artikel`}><svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 20.3S3.7 15.6 3.7 9.3A4.3 4.3 0 0 1 12 7.7a4.3 4.3 0 0 1 8.3 1.6c0 6.3-8.3 11-8.3 11Z"/></svg><span>Merkliste</span>{savedCount > 0 && <b>{savedCount}</b>}</button>
        <button className="account" onClick={() => navigate('/konto')}>{user ? 'Mein Konto' : 'Anmelden'}</button>
        <button className="bag" onClick={onCart} aria-label={`Warenkorb, ${count} Artikel`}><span>Auswahl</span><b>{String(count).padStart(2, '0')}</b><i aria-hidden="true">↗</i></button>
      </div>
    </header>
  </>
}
