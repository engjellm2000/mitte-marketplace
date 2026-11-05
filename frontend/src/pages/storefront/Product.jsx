import { useState, useEffect } from 'react'
import { euro, api, csrf } from '../../lib/api.js'
import { usePageMetadata } from '../../lib/usePageMetadata.js'

export function Product({ slug, go, add, saved, toggleSaved, user }) {
  const [product, setProduct] = useState(null)
  const [quantity, setQuantity] = useState(1)
  const [error, setError] = useState('')
  const [historyError, setHistoryError] = useState('')
  useEffect(() => {
    let active = true
    api(`/api/products/${encodeURIComponent(slug)}`).then(data => { if (active) setProduct(data.product) }).catch(cause => { if (active) setError(cause.message) })
    return () => { active = false }
  }, [slug])
  useEffect(() => {
    setHistoryError('')
    if (!product || !user?.RecommendationConsent) return
    let active = true
    api('/api/account/recommendations/views', {
      method: 'POST',
      headers: { 'X-CSRF-Token': csrf },
      body: JSON.stringify({ slug: product.Slug }),
    }).catch(cause => { if (active) setHistoryError(cause.message) })
    return () => { active = false }
  }, [product?.Slug, user?.RecommendationConsent])
  usePageMetadata(product ? `${product.Name} — MITTE` : 'Produkt — MITTE', product?.ShortDescription || 'Entdecke Produkte aus dem MITTE Sortiment.')
  if (error) return   <section className="product-not-found"><p>PRODUKT / NICHT GEFUNDEN</p><h1>Hier ist gerade nichts.</h1><span>{error}</span><button className="route-cta" onClick={() => go('/shop')}>Zurück zum Sortiment <b>→</b></button></section>
  if (!product) return <div className="loading storefront-loading">Produkt wird geladen …</div>
  const isSaved = saved.some(item => item.ID === product.ID)
  const available = product.Inventory > 0
  return <section className="detail">
    <nav className="detail-breadcrumb" aria-label="Brotkrumennavigation"><button onClick={() => go('/')}>Start</button><span>→</span><button onClick={() => go(`/shop?category=${encodeURIComponent(product.Category?.Slug || '')}`)}>{product.Category?.Name || 'Produkte'}</button><span>→</span><b>{product.Name}</b></nav>
    <div className="detail-image"><img src={product.ImageURL} alt={product.ImageAlt || product.Name}/><span>PRODUKT / ANSICHT</span><b aria-hidden="true">↗</b></div>
    <div className="detail-copy">
      <p className="product-route">{product.Category?.Name || 'MITTE'} <span> / </span> PRODUKTDETAILS</p>
      <h1>{product.Name}</h1><strong className="price">{euro(product.SalePriceCents > 0 ? product.SalePriceCents : product.PriceCents)}</strong>
      {product.SalePriceCents > 0 && <p className="product-offer-note">{product.OfferName} · {product.OfferDemo ? 'Demo-Rabatt, simuliert und nicht kaufmännisch gültig.' : 'Der Rabatt wird im Warenkorb berücksichtigt.'}</p>}
      {product.Demo && <p className="detail-demo-note">Demoprodukt – Produktdaten, Preis und Bestand sind nicht als kaufmännische Angaben verifiziert.</p>}
      <p className="lead">{product.ShortDescription}</p><p className="detail-description">{product.Description}</p>
      <p className={`stock ${available ? '' : 'out-of-stock'}`}><i aria-hidden="true"/>{available ? `${product.Inventory} ${product.Inventory === 1 ? 'Stück' : 'Stück'} verfügbar` : 'Derzeit nicht verfügbar'}</p>
      {historyError && <p className="account-message" role="status">Dein Verlauf konnte nicht aktualisiert werden: {historyError}</p>}
      <div className="detail-purchase">
        <div className="quantity" aria-label="Menge wählen"><button aria-label="Menge verringern" onClick={() => setQuantity(Math.max(1, quantity - 1))}>−</button><span>{quantity}</span><button aria-label="Menge erhöhen" disabled={!available || quantity >= Math.min(product.Inventory, 10)} onClick={() => setQuantity(Math.min(product.Inventory, 10, quantity + 1))}>+</button></div>
        <div className="product-actions"><button className="route-cta" disabled={!available} onClick={() => add(product, quantity)}>{available ? 'In den Warenkorb' : 'Nicht verfügbar'} <span>→</span></button><button className={`save-detail ${isSaved ? 'saved' : ''}`} onClick={() => toggleSaved(product)} aria-label={isSaved ? 'Von der Merkliste entfernen' : 'Auf die Merkliste'} aria-pressed={isSaved}>♡</button></div>
      </div>
      <dl className="detail-facts"><div><dt>Bereich</dt><dd>{product.Category?.Name || 'MITTE'}</dd></div><div><dt>Im Sortiment</dt><dd>{available ? 'Verfügbar' : 'Derzeit ausverkauft'}</dd></div><div><dt>Versand</dt><dd>Verfügbare Versandoptionen siehst du im Warenkorb.</dd></div></dl>
      <button className="detail-back" onClick={() => go('/shop')}>← Zurück zu allen Produkten</button>
    </div>
  </section>
}
