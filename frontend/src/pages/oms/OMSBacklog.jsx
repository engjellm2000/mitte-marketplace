import { useEffect, useRef, useState } from 'react'
import { OMSAccess, OMSFrame } from './OMSDashboard.jsx'

const stages = [
  ['all', 'Alle offenen'],
  ['payment', 'Zahlung offen'],
  ['held', 'Angehalten'],
  ['needs_fulfillment', 'Versandklärung'],
  ['in_progress', 'Fulfillment aktiv']
]

const stageLabels = {
  payment: 'Zahlung offen',
  held: 'Angehalten',
  needs_fulfillment: 'Versandklärung',
  in_progress: 'Fulfillment aktiv'
}

const orderStatusLabels = {
  pending_payment: 'Zahlung ausstehend',
  invoice_approved: 'Rechnung freigegeben',
  paid: 'Bezahlt'
}

const fulfillmentStatusLabels = {
  ready_to_pick: 'Bereit zum Picken',
  picking: 'Wird kommissioniert',
  packed: 'Verpackt',
  label_ready: 'Versandlabel bereit',
  held: 'Angehalten'
}

export function OMSBacklog({ user, go, api, euro, dateTime, isAuthorizationError }) {
  const [items, setItems] = useState([])
  const [summary, setSummary] = useState(null)
  const [query, setQuery] = useState('')
  const [stage, setStage] = useState('all')
  const [page, setPage] = useState(1)
  const [pages, setPages] = useState(1)
  const [total, setTotal] = useState(0)
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(true)
  const requestID = useRef(0)

  const load = async () => {
    const request = ++requestID.current
    setLoading(true)
    const params = new URLSearchParams({
      q: query.trim(),
      stage,
      page: String(page),
      limit: '25'
    })
    try {
      const result = await api(`/api/operations/backlog?${params}`, { cache: 'no-store' })
      if (request !== requestID.current) return
      setItems(result.items || [])
      setSummary(result.summary || null)
      setTotal(result.total || 0)
      setPages(result.pages || 1)
      setError('')
    } catch (cause) {
      if (request === requestID.current) setError(cause.message)
    } finally {
      if (request === requestID.current) setLoading(false)
    }
  }

  useEffect(() => {
    const timer = setTimeout(load, query ? 250 : 0)
    return () => {
      clearTimeout(timer)
      requestID.current += 1
    }
  }, [query, stage, page])

  if (error && !summary && isAuthorizationError(error)) return <OMSAccess go={go} error={error} />

  const counts = summary || { total: 0, payment: 0, needsFulfillment: 0, inProgress: 0, held: 0 }

  return (
    <OMSFrame user={user} go={go} title="Auftragsrückstand">
      <div className="oms-workspace oms-backlog">
        <header className="oms-page-heading oms-backlog-heading">
          <div>
            <h1>Offene Aufträge bearbeiten</h1>
            <p>Unabgeschlossene Aufträge, älteste zuerst. Angehaltene Fulfillments sind separat sichtbar und können gezielt fortgesetzt werden.</p>
          </div>
          <button className="oms-secondary-action" onClick={load} disabled={loading}>
            {loading ? 'Wird aktualisiert …' : '↻ Aktualisieren'}
          </button>
        </header>

        <section className="oms-backlog-summary" aria-label="Zusammensetzung des Rückstands">
          <div><strong>{summary ? counts.total : '—'}</strong><span>Offene Aufträge</span></div>
          <div><strong>{summary ? counts.payment : '—'}</strong><span>Warten auf Zahlung</span></div>
          <div className="attention"><strong>{summary ? counts.held : '—'}</strong><span>Fulfillment angehalten</span></div>
          <div><strong>{summary ? counts.needsFulfillment : '—'}</strong><span>Fulfillment klären</span></div>
          <div><strong>{summary ? counts.inProgress : '—'}</strong><span>Fulfillment aktiv</span></div>
        </section>

        {error && <div className="oms-inline-error" role="alert">{error}<button onClick={load}>Erneut versuchen</button></div>}

        <section className="oms-backlog-board" aria-labelledby="oms-backlog-title">
          <div className="oms-backlog-toolbar">
            <div>
              <h2 id="oms-backlog-title">Rückstandsqueue</h2>
              <p>Die ältesten Eingänge stehen oben; Status und Bestelldatum stammen aus den Auftragsdaten.</p>
            </div>
            <label className="oms-search">
              <span aria-hidden="true">⌕</span>
              <span className="visually-hidden">Rückstand durchsuchen</span>
              <input value={query} onChange={event => { setQuery(event.target.value); setPage(1) }} placeholder="Bestellnummer, Name oder E-Mail" />
              {query && <button type="button" aria-label="Suche löschen" onClick={() => { setQuery(''); setPage(1) }}>×</button>}
            </label>
          </div>
          <div className="oms-backlog-filters" aria-label="Rückstand filtern">
            {stages.map(([value, label]) => (
              <button key={value} className={stage === value ? 'active' : ''} aria-pressed={stage === value} onClick={() => { setStage(value); setPage(1) }}>
                {label}
              </button>
            ))}
            <span aria-live="polite">{loading ? 'Lädt …' : error ? 'Daten nicht verfügbar' : `${total} Aufträge`}</span>
          </div>

          <div className="oms-backlog-table" role="table" aria-label="Offene Aufträge">
            <div className="oms-backlog-row oms-backlog-table-head" role="row">
              <span role="columnheader">Bestellung</span>
              <span role="columnheader">Kunde</span>
              <span role="columnheader">Eingegangen</span>
              <span role="columnheader">Bearbeitungsstand</span>
              <span role="columnheader">Betrag</span>
              <span aria-hidden="true" />
            </div>
            {items.map(item => (
              <button className="oms-backlog-row" role="row" key={item.ID} onClick={() => go(`/oms/orders/${encodeURIComponent(item.ID)}`)}>
                <strong role="cell">{item.ID}</strong>
                <span role="cell"><b>{item.CustomerName || 'Gastkunde'}</b><small>{item.CustomerEmail || 'Keine E-Mail hinterlegt'}</small></span>
                <span role="cell">{dateTime(item.CreatedAt)}</span>
                <span className={`oms-backlog-stage ${item.Stage}`} role="cell">
                  <b>{stageLabels[item.Stage] || 'Status prüfen'}</b>
                  <small>{fulfillmentStatusLabels[item.FulfillmentStatus] || orderStatusLabels[item.Status] || item.Status}</small>
                </span>
                <strong role="cell">{euro(item.TotalCents)}</strong>
                <em role="cell">Auftrag öffnen →</em>
              </button>
            ))}
            {!items.length && (
              <div className="oms-backlog-empty" role="status">
                {loading ? 'Rückstand wird geladen …' : error ? 'Die Rückstandsqueue konnte nicht geladen werden.' : query || stage !== 'all'
                  ? 'Keine offenen Aufträge für diese Suche oder diesen Filter.'
                  : 'Keine offenen Aufträge. Neue offene Bestellungen erscheinen hier automatisch.'}
              </div>
            )}
          </div>
          <nav className="oms-backlog-pagination" aria-label="Rückstandsseiten">
            <button disabled={loading || page <= 1} onClick={() => setPage(value => value - 1)}>← Zurück</button>
            <span aria-live="polite">Seite {page} von {pages}</span>
            <button disabled={loading || page >= pages} onClick={() => setPage(value => value + 1)}>Weiter →</button>
          </nav>
        </section>
      </div>
    </OMSFrame>
  )
}
