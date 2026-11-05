import { useEffect, useState } from 'react'
import { api, euro } from '../../lib/api.js'
import { usePageMetadata } from '../../lib/usePageMetadata.js'

function discountLabel(offer) {
  if (offer.kind === 'percent') {
    const amount = new Intl.NumberFormat('de-DE', { maximumFractionDigits: 1 }).format(offer.percentBps / 100)
    return `${amount} %`
  }
  if (offer.kind === 'fixed') return `${euro(offer.amountCents)} Rabatt`
  return 'Versand gratis'
}

function destination(offer) {
  if (offer.productSlug) return `/produkt/${encodeURIComponent(offer.productSlug)}`
  if (offer.categorySlug) return `/shop?category=${encodeURIComponent(offer.categorySlug)}`
  return '/shop'
}

export function MarketingOffers({ slug, go }) {
  const [offers, setOffers] = useState([])
  const [selected, setSelected] = useState(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    let active = true
    setLoading(true)
    setError('')
    const request = slug
      ? api(`/api/offers/${encodeURIComponent(slug)}`).then(data => ({ offer: data.offer }))
      : api('/api/offers')
    request.then(data => {
      if (!active) return
      setOffers(data.offers || [])
      setSelected(data.offer || null)
    }).catch(cause => {
      if (!active) return
      setError(cause.message)
      setSelected(null)
      setOffers([])
    }).finally(() => { if (active) setLoading(false) })
    return () => { active = false }
  }, [slug])

  usePageMetadata(
    selected ? `${selected.headline} — MITTE` : 'Angebote — MITTE',
    selected?.description || 'Entdecke aktuelle Aktionen und Angebote im MITTE Marktplatz.'
  )

  if (loading) return <div className="loading storefront-loading">Angebote werden geladen …</div>
  if (error && slug) return <section className="offer-empty"><h1>Dieses Angebot ist nicht verfügbar.</h1><p>{error}</p><button className="mitte-primary" onClick={() => go('/angebote')}>Alle Angebote ansehen →</button></section>

  if (selected) return <section className="offer-detail">
    <button className="offer-back" onClick={() => go('/angebote')}>← Alle Angebote</button>
    <div className="offer-detail-grid">
      <div className="offer-detail-art">
        {selected.imageUrl
          ? <img src={selected.imageUrl} alt={selected.imageAlt || ''}/>
          : <span aria-hidden="true">MITTE<br/>FINDET<br/>DEINS</span>}
        {selected.demoOnly && <b>DEMO-AKTION</b>}
      </div>
      <div className="offer-detail-copy">
        <p>{selected.name}</p>
        <h1>{selected.headline}</h1>
        <span>{selected.description}</span>
        <strong>{discountLabel(selected)}</strong>
        {selected.minSubtotalCents > 0 && <small>Ab {euro(selected.minSubtotalCents)} Warenkorbwert.</small>}
        {selected.autoApply
          ? <small>Der Rabatt wird automatisch im Warenkorb berücksichtigt.</small>
          : <small>Gutscheincode <b>{selected.code}</b> im Warenkorb eingeben.</small>}
        {selected.demoOnly && <div className="offer-demo-warning" role="note">Lokale Demo-Aktion: Preise, Rabatt und Ersparnis sind simuliert und nicht kaufmännisch gültig.</div>}
        <button className="mitte-primary" onClick={() => go(destination(selected))}>{selected.ctaLabel} <span>→</span></button>
      </div>
    </div>
  </section>

  return <section className="offers-page">
    <header className="offers-heading"><h1>Aktuelle Angebote</h1><p>Gute Funde aus dem MITTE Sortiment — mit einem Extra für deinen Einkauf.</p></header>
    {error && <div className="storefront-message" role="alert">Die Angebote konnten nicht geladen werden. Bitte versuche es erneut.<button onClick={() => location.reload()}>Neu laden</button></div>}
    {!error && !offers.length && <div className="offer-empty"><h2>Gerade keine aktiven Angebote.</h2><p>Schau dich in der Zwischenzeit im Sortiment um.</p><button className="mitte-primary" onClick={() => go('/shop')}>Zum Sortiment →</button></div>}
    <div className="offers-grid">
      {offers.map((offer, index) => <article className={`offer-card${index === 0 ? ' offer-card-featured' : ''}`} key={offer.campaignSlug}>
        <button className="offer-card-link" onClick={() => go(`/angebote/${encodeURIComponent(offer.campaignSlug)}`)} aria-label={`${offer.headline} ansehen`}>
          <span className="offer-card-image">
            {offer.imageUrl ? <img src={offer.imageUrl} alt={offer.imageAlt || ''} loading="lazy" decoding="async"/> : <span aria-hidden="true">MITTE<br/>FINDET<br/>DEINS</span>}
            <b>{discountLabel(offer)}</b>
          </span>
          <span className="offer-card-copy">
            <span className="offer-card-flags">{offer.demoOnly ? 'LOKALE DEMO-AKTION' : 'AKTUELLES ANGEBOT'}{offer.autoApply ? ' · AUTOMATISCH' : ' · GUTSCHEIN'}</span>
            <strong>{offer.headline}</strong>
            <span>{offer.description}</span>
            {!offer.autoApply && offer.code && <em>Code: {offer.code}</em>}
            <span className="offer-card-cta">{offer.ctaLabel} <b aria-hidden="true">↗</b></span>
          </span>
        </button>
      </article>)}
    </div>
  </section>
}
