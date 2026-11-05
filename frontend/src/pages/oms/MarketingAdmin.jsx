import { useEffect, useState } from 'react'
import { api, csrf } from '../../lib/api.js'
import { OperationsSidebar } from '../../components/operations/OperationsSidebar.jsx'

const blankPromotion = {
  code: '', name: '', kind: 'percent', percentBps: 1500, amountCents: 0,
  minSubtotalCents: 0, categorySlug: '', productSlug: '', active: false,
  startsAt: '', endsAt: '', maxRedemptions: '', perEmailLimit: 0,
  publicCampaign: true, autoApply: true, campaignSlug: '', headline: '',
  description: '', imageUrl: '', imageAlt: '', ctaLabel: 'Jetzt entdecken',
  featured: false, demoOnly: false
}

const localDateTime = value => {
  if (!value) return ''
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  date.setMinutes(date.getMinutes() - date.getTimezoneOffset())
  return date.toISOString().slice(0, 16)
}

const eurosToCents = value => Math.round(Number(value || 0) * 100)

export function MarketingAdmin({ user, go }) {
  const [promotions, setPromotions] = useState([])
  const [categories, setCategories] = useState([])
  const [selected, setSelected] = useState(null)
  const [form, setForm] = useState(blankPromotion)
  const [loading, setLoading] = useState(true)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')

  const refresh = async () => {
    setLoading(true)
    try {
      const [promotionData, categoryData] = await Promise.all([
        api('/api/admin/promotions', { cache: 'no-store' }),
        api('/api/categories')
      ])
      setPromotions(promotionData.promotions || [])
      setCategories(categoryData.categories || [])
      setError('')
    } catch (cause) {
      setError(cause.message)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { refresh() }, [])

  const edit = promotion => {
    setSelected(promotion)
    setForm({
      ...promotion,
      startsAt: localDateTime(promotion.startsAt),
      endsAt: localDateTime(promotion.endsAt),
      maxRedemptions: promotion.maxRedemptions ?? ''
    })
    setError('')
    setNotice('')
  }

  const reset = () => {
    setSelected(null)
    setForm({ ...blankPromotion })
    setError('')
    setNotice('')
  }

  const update = (field, value) => setForm(current => ({ ...current, [field]: value }))
  const save = async event => {
    event.preventDefault()
    setBusy(true)
    setError('')
    setNotice('')
    const payload = {
      ...form,
      percentBps: form.kind === 'percent' ? Number(form.percentBps || 0) : 0,
      amountCents: form.kind === 'fixed' ? (form.amountEuro === undefined ? Number(form.amountCents || 0) : eurosToCents(form.amountEuro)) : 0,
      minSubtotalCents: form.minimumEuro === undefined ? Number(form.minSubtotalCents || 0) : eurosToCents(form.minimumEuro),
      maxRedemptions: form.maxRedemptions === '' ? null : Number(form.maxRedemptions),
      perEmailLimit: Number(form.perEmailLimit || 0),
      startsAt: form.startsAt ? new Date(form.startsAt).toISOString() : null,
      endsAt: form.endsAt ? new Date(form.endsAt).toISOString() : null,
      categorySlug: form.kind === 'free_shipping' ? '' : form.categorySlug,
      productSlug: form.kind === 'free_shipping' ? '' : form.productSlug,
      autoApply: form.publicCampaign && form.autoApply
    }
    delete payload.amountEuro
    delete payload.minimumEuro
    try {
      const path = selected ? `/api/admin/promotions/${encodeURIComponent(selected.code)}` : '/api/admin/promotions'
      const response = await api(path, {
        method: selected ? 'PATCH' : 'POST',
        headers: { 'X-CSRF-Token': csrf },
        body: JSON.stringify(payload)
      })
      setNotice(selected ? 'Angebot gespeichert.' : 'Angebot angelegt.')
      await refresh()
      const saved = (await api('/api/admin/promotions', { cache: 'no-store' })).promotions?.find(item => item.code === response.promotion.code)
      if (saved) edit(saved)
    } catch (cause) {
      setError(cause.message)
    } finally {
      setBusy(false)
    }
  }

  const publicReady = !form.publicCampaign || (
    form.campaignSlug.trim() && form.headline.trim() && form.description.trim() && form.ctaLabel.trim()
  )

  return <div className="operations-shell">
    <OperationsSidebar active="offers" user={user} go={go}/>
    <main className="operations-main">
      <section className="campaign-admin">
        <header className="campaign-admin-head">
          <div><h1>Kampagnen und Angebote</h1><p>Plane zeitlich begrenzte Rabatte, automatische Aktionen und ihre Darstellung im MITTE-Shop.</p></div>
          <div><button onClick={() => go('/angebote')}>Shop ansehen</button><button className="dark-button" onClick={reset}>+ Angebot erstellen</button></div>
        </header>
        <div className="campaign-admin-truth" role="note"><strong>Preisangaben</strong><span>Demo-Angebote müssen als simuliert markiert sein. Veröffentliche echte Preisaktionen erst nach Prüfung der Produkt- und Preisdaten.</span></div>
        {(error || notice) && <div className={`oms-alert ${error ? 'error' : ''}`} role={error ? 'alert' : 'status'}>{error || notice}</div>}
        <div className="campaign-admin-layout">
          <section className="campaign-list" aria-label="Angebote">
            <div className="campaign-list-head"><h2>Alle Angebote</h2><button disabled={loading} onClick={refresh}>↻ Aktualisieren</button></div>
            {loading && <p className="oms-empty-state" role="status">Angebote werden geladen …</p>}
            {!loading && promotions.map(promotion => {
              const scheduled = promotion.startsAt && new Date(promotion.startsAt) > new Date()
              const expired = promotion.endsAt && new Date(promotion.endsAt) < new Date()
              const status = !promotion.active ? 'Entwurf' : scheduled ? 'Geplant' : expired ? 'Abgelaufen' : 'Aktiv'
              return <button key={promotion.code} className={`campaign-list-row${selected?.code === promotion.code ? ' selected' : ''}`} onClick={() => edit(promotion)}>
              <span><strong>{promotion.headline || promotion.name}</strong><small>{promotion.code}{promotion.publicCampaign ? ` · /angebote/${promotion.campaignSlug}` : ' · Gutschein'}{promotion.demoOnly ? ' · DEMO' : ''}</small></span>
              <b className={`campaign-status ${status === 'Aktiv' ? 'active' : ''}`}>{status}</b>
            </button>})}
            {!loading && !promotions.length && !error && <p className="oms-empty-state">Noch keine Angebote angelegt.</p>}
          </section>
          <form className="campaign-editor" onSubmit={save}>
            <div className="campaign-editor-title"><div><h2>{selected ? 'Angebot bearbeiten' : 'Neues Angebot'}</h2><p>{selected ? selected.code : 'Rabatt und Kampagneninhalt gemeinsam verwalten.'}</p></div>{selected && <button type="button" onClick={reset}>Neu</button>}</div>
            <fieldset><legend>Rabatt</legend>
              <div className="campaign-form-row"><label>Code<input required maxLength="40" readOnly={Boolean(selected)} value={form.code} onChange={event => update('code', event.target.value.toUpperCase().replace(/[^A-Z0-9_-]/g, ''))} placeholder="FRUEHLING15"/></label><label>Interner Name<input required maxLength="120" value={form.name} onChange={event => update('name', event.target.value)} placeholder="Frühlingsaktion"/></label></div>
              <div className="campaign-form-row"><label>Rabattart<select value={form.kind} onChange={event => update('kind', event.target.value)}><option value="percent">Prozent</option><option value="fixed">Fester Betrag</option><option value="free_shipping">Kostenloser Versand</option></select></label>{form.kind === 'percent' ? <label>Rabatt in Prozent<input required type="number" min="0.01" max="100" step="0.01" value={form.percentBps / 100} onChange={event => update('percentBps', Math.round(Number(event.target.value) * 100))}/></label> : form.kind === 'fixed' ? <label>Rabatt in Euro<input required type="number" min="0.01" max="1000000" step="0.01" value={(Number(form.amountEuro ?? form.amountCents / 100)).toFixed(2)} onChange={event => update('amountEuro', event.target.value)}/></label> : <span className="campaign-hint">Gilt für den Versand des Warenkorbs.</span>}</div>
              <div className="campaign-form-row"><label>Mindestwarenkorb (€)<input type="number" min="0" max="1000000" step="0.01" value={(Number(form.minimumEuro ?? form.minSubtotalCents / 100)).toFixed(2)} onChange={event => update('minimumEuro', event.target.value)}/></label><label>Einlösungen pro E-Mail<input type="number" min="0" max="1000000" value={form.perEmailLimit} disabled={form.autoApply} onChange={event => update('perEmailLimit', event.target.value)}/></label></div>
              <div className="campaign-form-row"><label>Produkt-Slug (optional)<input maxLength="120" value={form.productSlug} onChange={event => update('productSlug', event.target.value)} placeholder="produkt-name"/></label><label>Kategorie (optional)<select value={form.categorySlug} onChange={event => update('categorySlug', event.target.value)}><option value="">Alle Kategorien</option>{categories.map(category => <option key={category.Slug} value={category.Slug}>{category.Name}</option>)}</select></label></div>
              <div className="campaign-form-row"><label>Beginnt am<input type="datetime-local" value={form.startsAt} onChange={event => update('startsAt', event.target.value)}/></label><label>Endet am<input type="datetime-local" value={form.endsAt} onChange={event => update('endsAt', event.target.value)}/></label></div>
              <label>Maximale Einlösungen <small>optional</small><input type="number" min="1" max="100000000" value={form.maxRedemptions ?? ''} onChange={event => update('maxRedemptions', event.target.value)}/></label>
            </fieldset>
            <fieldset><legend>Marketing im Shop</legend>
              <label className="campaign-check"><input type="checkbox" checked={form.publicCampaign} onChange={event => update('publicCampaign', event.target.checked)}/> Als Kampagne öffentlich anzeigen</label>
              {form.publicCampaign && <>
                <div className="campaign-form-row"><label>Kampagnen-URL<input required maxLength="120" value={form.campaignSlug} onChange={event => update('campaignSlug', event.target.value.toLowerCase().replace(/[^a-z0-9-]+/g, '-'))} placeholder="fruehlingsangebote"/></label><label>Überschrift<input required maxLength="120" value={form.headline} onChange={event => update('headline', event.target.value)}/></label></div>
                <label>Beschreibung<textarea required maxLength="500" rows="3" value={form.description} onChange={event => update('description', event.target.value)}/></label>
                <div className="campaign-form-row"><label>Bild-URL<input type="url" value={form.imageUrl} onChange={event => update('imageUrl', event.target.value)} placeholder="https://…"/></label><label>Bildbeschreibung<input value={form.imageAlt} onChange={event => update('imageAlt', event.target.value)} placeholder="Motiv kurz beschreiben"/></label></div>
                <div className="campaign-form-row"><label>Schaltfläche<input required maxLength="40" value={form.ctaLabel} onChange={event => update('ctaLabel', event.target.value)}/></label><label className="campaign-check"><input type="checkbox" checked={form.featured} onChange={event => update('featured', event.target.checked)}/> Auf der Startseite hervorheben</label></div>
                <label className="campaign-check"><input type="checkbox" checked={form.autoApply} onChange={event => update('autoApply', event.target.checked)}/> Rabatt automatisch im Warenkorb anwenden</label>
                {!form.autoApply && <p className="campaign-hint">Kundinnen und Kunden müssen den Angebotscode im Warenkorb eingeben.</p>}
                <label className="campaign-check campaign-demo-check"><input type="checkbox" checked={form.demoOnly} onChange={event => update('demoOnly', event.target.checked)}/> Demo-Aktion mit simulierten Preisen und Rabatten kennzeichnen</label>
              </>}
              <label className="campaign-check"><input type="checkbox" checked={form.active} onChange={event => update('active', event.target.checked)}/> Angebot aktivieren (Laufzeit wird weiterhin geprüft)</label>
              {form.demoOnly && <p className="campaign-demo-warning">Demo-Status wird öffentlich angezeigt. Diese Aktion darf nicht als echtes Preisangebot verstanden werden.</p>}
            </fieldset>
            <button className="coral-button" disabled={busy || !publicReady || !form.code.trim() || !form.name.trim()}>{busy ? 'Speichert …' : selected ? 'Angebot speichern →' : 'Angebot anlegen →'}</button>
          </form>
        </div>
      </section>
    </main>
  </div>
}
