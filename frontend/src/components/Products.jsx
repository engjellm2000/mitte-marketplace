import { euro } from '../lib/api.js'

export function Products({ title, products = [], go, add, saved = [], toggleSaved, showEmpty = true, emptyMessage = title ? 'Neue Produkte werden vorbereitet.' : 'Keine passenden Produkte gefunden. Ändere deine Suche oder wähle einen anderen Bereich.' }) {
  return <section className="products">
    {title && <div className="section-title"><div><h2>{title}</h2></div><button className="text-button" onClick={() => go('/shop')}>Alle Produkte <span>→</span></button></div>}
    <div className="product-grid">{products.map((product, index) => {
      const isSaved = saved.some(item => item.ID === product.ID)
      const available = product.Inventory > 0
      const price = product.SalePriceCents > 0 ? product.SalePriceCents : product.PriceCents
      return <article className={`product${product.Demo ? ' product-demo' : ''}`} key={product.ID}>
        <button className="image-button" onClick={() => go(`/produkt/${product.Slug}`)} aria-label={`${product.Name} ansehen`}>
          <img loading={index < 2 ? 'eager' : 'lazy'} decoding="async" src={product.ImageURL} alt={product.ImageAlt || product.Name}/>
          <span>Produkt ansehen <b>↗</b></span>
        </button>
        {product.SalesCount > 0 && <span className="product-badge">Bestseller</span>}
        {product.Demo && <span className="product-demo-badge">Demo</span>}
        {product.SalePriceCents > 0 && <span className={`product-offer-badge${product.OfferDemo ? ' demo' : ''}`}>{product.OfferDemo ? 'Demo-Angebot' : 'Aktion'}</span>}
        <button className={`save-product ${isSaved ? 'saved' : ''}`} aria-label={`${product.Name} ${isSaved ? 'von der Merkliste entfernen' : 'merken'}`} aria-pressed={isSaved} onClick={() => toggleSaved(product)}>♡</button>
        <div className="product-info"><p>{product.Category?.Name || 'MITTE'}</p><h3><button onClick={() => go(`/produkt/${product.Slug}`)}>{product.Name}</button></h3><div><strong>{euro(price)}</strong><button disabled={!available} aria-label={available ? `${product.Name} in den Warenkorb` : `${product.Name} ist ausverkauft`} onClick={() => add(product)}>{available ? '+' : '—'}</button></div>{product.SalePriceCents > 0 && <small className="product-offer-note">{product.OfferName}{product.OfferDemo ? ' · simuliert' : ' · Rabatt im Warenkorb'}</small>}</div>
        <span className={`product-availability ${available ? '' : 'unavailable'}`}>{available ? 'VERFÜGBAR' : 'DERZEIT NICHT VERFÜGBAR'}</span>
      </article>
    })}</div>
    {showEmpty && !products.length && <div className="empty">{emptyMessage}</div>}
  </section>
}
