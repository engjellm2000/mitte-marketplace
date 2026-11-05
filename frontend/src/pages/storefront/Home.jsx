import { useEffect, useState } from 'react'
import { api } from '../../lib/api.js'
import { usePageMetadata } from '../../lib/usePageMetadata.js'
import { Products } from '../../components/Products.jsx'

const categoryArt = {
  technik: 'https://images.unsplash.com/photo-1517336714731-489689fd1ca8?auto=format&fit=crop&w=1000&q=80',
  wohnen: 'https://images.unsplash.com/photo-1618220179428-22790b461013?auto=format&fit=crop&w=1000&q=80',
  beauty: 'https://images.unsplash.com/photo-1556228720-195a672e8a03?auto=format&fit=crop&w=1000&q=80',
  mode: 'https://images.unsplash.com/photo-1445205170230-053b83016050?auto=format&fit=crop&w=1000&q=80',
  'sport-outdoor': 'https://images.unsplash.com/photo-1517836357463-d25dfeac3438?auto=format&fit=crop&w=1000&q=80',
  'kueche-haushalt': 'https://images.unsplash.com/photo-1556911220-bff31c812dba?auto=format&fit=crop&w=1000&q=80',
  'garten-balkon': 'https://images.unsplash.com/photo-1416879595882-3373a0480b5b?auto=format&fit=crop&w=1000&q=80',
  'werkzeug-handwerk': 'https://images.unsplash.com/photo-1504148455328-c376907d081c?auto=format&fit=crop&w=1000&q=80',
  'baby-kinder': 'https://images.unsplash.com/photo-1519689680058-324335c77eba?auto=format&fit=crop&w=1000&q=80',
  'buero-schreibwaren': 'https://images.unsplash.com/photo-1497366754035-f200968a6e72?auto=format&fit=crop&w=1000&q=80',
  'tierbedarf': 'https://images.unsplash.com/photo-1543466835-00a7907e9de1?auto=format&fit=crop&w=1000&q=80',
  lebensmittel: 'https://images.unsplash.com/photo-1442512595331-e89e73853f31?auto=format&fit=crop&w=1000&q=80',
  'auto-mobilitaet': 'https://images.unsplash.com/photo-1503376780353-7e6692767b70?auto=format&fit=crop&w=1000&q=80',
  'reisen-freizeit': 'https://images.unsplash.com/photo-1476514525535-07fb3b4ae5f1?auto=format&fit=crop&w=1000&q=80'
}
const categoryOrder = ['technik', 'wohnen', 'beauty', 'mode', 'sport-outdoor', 'werkzeug-handwerk', 'kueche-haushalt', 'garten-balkon']
const categoryIndex = slug => {
  const index = categoryOrder.indexOf(slug)
  return index < 0 ? categoryOrder.length : index
}

export function Home({ go, add, saved, toggleSaved, user }) {
  const [products, setProducts] = useState([])
  const [bestSellers, setBestSellers] = useState([])
  const [categories, setCategories] = useState([])
  const [personal, setPersonal] = useState(null)
  const [offers, setOffers] = useState([])
  const [offerError, setOfferError] = useState('')
  const [error, setError] = useState('')
  const [personalError, setPersonalError] = useState('')

  useEffect(() => {
    let active = true
    Promise.all([
      api('/api/products?limit=100'),
      api('/api/categories'),
      api('/api/products?sort=bestselling&limit=4')
    ]).then(([productData, categoryData, bestsellerData]) => {
      if (!active) return
      setProducts(productData.products || [])
      setCategories(categoryData.categories || [])
      setBestSellers(bestsellerData.products || [])
      setError('')
    }).catch(cause => {
      if (active) setError(cause.message)
    })
    return () => { active = false }
  }, [])

  useEffect(() => {
    let active = true
    api('/api/offers')
      .then(data => { if (active) { setOffers(data.offers || []); setOfferError('') } })
      .catch(cause => { if (active) setOfferError(cause.message) })
    return () => { active = false }
  }, [])

  useEffect(() => {
    let active = true
    setPersonal(null)
    setPersonalError('')
    if (!user?.RecommendationConsent) return () => { active = false }
    api('/api/account/recommendations', { cache: 'no-store' })
      .then(data => { if (active) setPersonal(data) })
      .catch(cause => { if (active) setPersonalError(cause.message) })
    return () => { active = false }
  }, [user?.ID, user?.RecommendationConsent])

  const rootCategories = categories.filter(category => !category.ParentID).sort((a, b) => categoryIndex(a.Slug) - categoryIndex(b.Slug) || a.Name.localeCompare(b.Name, 'de'))
  const subcategoriesFor = parentID => categories.filter(category => category.ParentID === parentID)
  const popularCategories = rootCategories.slice(0, 6)
  const discoveryProducts = products.filter(product => product.Featured).slice(0, 4)
  usePageMetadata('MITTE — Alles für deinen Alltag', 'Entdecke Technik, Wohnen, Beauty, Mode und mehr bei MITTE.')

  return <>
    {error && <div className="storefront-message" role="alert">{error}<button onClick={() => location.reload()}>Erneut laden</button></div>}
    <section className="mitte-hero">
      <div className="mitte-hero-copy">
        <h1>Alles, was du suchst.<br/><span>Mitten im Leben.</span></h1>
        <p>Finde deinen nächsten Favoriten – von Technik und Werkzeug bis zu Beauty, Wohnen und allem dazwischen.</p>
        <div className="mitte-hero-actions">
          <button className="mitte-primary" onClick={() => go('/shop')}>Sortiment entdecken <span>→</span></button>
          <button className="mitte-secondary" onClick={() => go('/shop?sort=bestselling')}>Bestseller ansehen</button>
        </div>
        <div className="mitte-hero-links" aria-label="Beliebte Bereiche">
          {popularCategories.slice(0, 4).map(category => <button key={category.Slug} onClick={() => go(`/shop?category=${encodeURIComponent(category.Slug)}`)}>{category.Name} <span>↗</span></button>)}
        </div>
      </div>
      <div className="mitte-hero-art" aria-label="Beliebte Produktbereiche">
        {popularCategories.slice(0, 3).map((category, index) => <button
          className={`mitte-hero-tile mitte-hero-tile-${index + 1}`}
          key={category.Slug}
          onClick={() => go(`/shop?category=${encodeURIComponent(category.Slug)}`)}
        >
          <img src={categoryArt[category.Slug] || products[index]?.ImageURL} alt="" fetchPriority={index === 0 ? 'high' : 'auto'}/>
          <span>{category.Name}<b>↗</b></span>
        </button>)}
        <span className="mitte-hero-stamp" aria-hidden="true">MITTE<br/>FINDET<br/>DEINS</span>
      </div>
    </section>

    {offers[0] && <section className="mitte-featured-offer">
      {offers[0].imageUrl && <img src={offers[0].imageUrl} alt={offers[0].imageAlt || ''} loading="lazy" decoding="async"/>}
      <div><p>{offers[0].demoOnly ? 'LOKALE DEMO-AKTION' : 'AKTUELL BEI MITTE'}</p><h2>{offers[0].headline}</h2><span>{offers[0].description}</span><button onClick={() => go(`/angebote/${encodeURIComponent(offers[0].campaignSlug)}`)}>{offers[0].ctaLabel} <b aria-hidden="true">→</b></button></div>
    </section>}
    {offerError && <div className="storefront-message" role="status">Aktuelle Angebote konnten nicht geladen werden.</div>}

    <section className="mitte-category-section">
      <div className="mitte-section-heading">
        <div><h2>Womit können wir helfen?</h2></div>
        <button className="mitte-text-link" onClick={() => go('/shop')}>Alle Kategorien <span>→</span></button>
      </div>
      <div className="mitte-category-grid">
        {rootCategories.map((category, index) => {
          const art = categoryArt[category.Slug]
          const children = subcategoriesFor(category.ID)
          return <button className={`mitte-category-card mitte-category-${index % 4}`} key={category.Slug} onClick={() => go(`/shop?category=${encodeURIComponent(category.Slug)}`)}>
            <span className="mitte-category-art">{art && <img src={art} loading="lazy" decoding="async" alt=""/>}</span>
            <span className="mitte-category-copy"><strong>{category.Name}</strong><small>{children.slice(0, 2).map(child => child.Name).join(' · ')}</small></span>
            <span className="mitte-category-arrow" aria-hidden="true">↗</span>
          </button>
        })}
      </div>
      {!rootCategories.length && !error && <div className="loading">Kategorien werden geladen …</div>}
    </section>

    <section className="mitte-product-section mitte-bestsellers">
      <div className="mitte-section-heading">
        <div><h2>Am häufigsten gekauft</h2><p>Sortiert nach bestätigten Bestellungen.</p></div>
        <button className="mitte-text-link" onClick={() => go('/shop?sort=bestselling')}>Alle Bestseller <span>→</span></button>
      </div>
      <Products products={bestSellers} go={go} add={add} saved={saved} toggleSaved={toggleSaved} emptyMessage="Hier erscheinen Produkte, sobald bestätigte Verkäufe vorliegen." />
    </section>

    {user?.RecommendationConsent && <section className="mitte-product-section mitte-personal">
      <div className="mitte-section-heading">
        <div><h2>Passt zu deinen Interessen</h2><p>Auf Basis deiner angesehenen und gekauften Produkte.</p></div>
        <button className="mitte-text-link" onClick={() => go('/konto')}>Einstellungen <span>→</span></button>
      </div>
      {personalError && <div className="storefront-message" role="alert">{personalError}</div>}
      {personal?.recommendations?.length > 0
        ? <Products products={personal.recommendations} go={go} add={add} saved={saved} toggleSaved={toggleSaved} />
        : !personalError && <p className="mitte-personal-empty">Sieh dir Produkte an oder kaufe etwas ein – wir nutzen deine Daten nur mit deiner aktiven Zustimmung.</p>}
      {personal?.recentlyViewed?.length > 0 && <>
          <div className="mitte-section-heading mitte-recent-heading"><div><h2>Zuletzt angesehen</h2></div></div>
        <Products products={personal.recentlyViewed} go={go} add={add} saved={saved} toggleSaved={toggleSaved} />
      </>}
    </section>}

    <section className="mitte-product-section">
      <div className="mitte-section-heading">
        <div><h2>Das könnte dich interessieren</h2></div>
        <button className="mitte-text-link" onClick={() => go('/shop')}>Alles entdecken <span>→</span></button>
      </div>
      <Products products={discoveryProducts.length ? discoveryProducts : products.slice(0, 4)} go={go} add={add} saved={saved} toggleSaved={toggleSaved} showEmpty={!error} />
    </section>
    <section className="mitte-endcap">
      <div><h2>Der nächste gute Fund<br/>ist gleich um die Ecke.</h2></div>
      <button className="mitte-primary" onClick={() => go('/shop')}>Jetzt stöbern <span>→</span></button>
    </section>
  </>
}
