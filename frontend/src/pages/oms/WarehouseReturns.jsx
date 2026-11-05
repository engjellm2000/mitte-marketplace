import { useEffect, useState } from 'react'
import { api, csrf } from '../../lib/api.js'
import { dateTime } from '../../lib/labels.js'
import { OperationsSidebar } from '../../components/operations/OperationsSidebar.jsx'

const statusLabels = {
  expected: 'Erwartet',
  received: 'Eingetroffen · Prüfung offen',
  restocked: 'Geprüft · wieder eingelagert',
  damaged: 'Geprüft · beschädigt',
  not_restockable: 'Geprüft · nicht einlagerbar'
}

const requestLabels = {
  return: 'Rückgabe',
  replacement_with_return: 'Ersatz mit Rückgabe'
}

export function WarehouseReturns({ user, go }) {
  const [cases, setCases] = useState([])
  const [status, setStatus] = useState('all')
  const [query, setQuery] = useState('')
  const [page, setPage] = useState(1)
  const [pages, setPages] = useState(1)
  const [total, setTotal] = useState(0)
  const [references, setReferences] = useState({})
  const [busy, setBusy] = useState('')
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const [refresh, setRefresh] = useState(0)

  useEffect(() => {
    let active = true
    const timer = setTimeout(async () => {
      setLoading(true)
      setError('')
      try {
        const params = new URLSearchParams({ status, q: query.trim(), page: String(page), limit: '50' })
        const result = await api(`/api/warehouse/returns?${params}`, { cache: 'no-store' })
        if (!active) return
        setCases(result.returns || [])
        setTotal(result.total || 0)
        setPages(result.pages || 1)
      } catch (cause) {
        if (active) setError(cause.message)
      } finally {
        if (active) setLoading(false)
      }
    }, query ? 250 : 0)
    return () => { active = false; clearTimeout(timer) }
  }, [status, query, page, refresh])

  const mutate = async (item, action, payload) => {
    const key = `${item.id}:${action}`
    setBusy(key)
    setError('')
    setNotice('')
    try {
      await api(`/api/warehouse/returns/${item.id}/${action}`, {
        method: 'POST',
        headers: { 'X-CSRF-Token': csrf },
        body: JSON.stringify(payload)
      })
      setNotice(action === 'receive' ? `${item.reference} als eingetroffen erfasst.` : `${item.reference} geprüft und verbucht.`)
      setRefresh(value => value + 1)
    } catch (cause) {
      setError(cause.message)
    } finally {
      setBusy('')
    }
  }

  return <div className="operations-shell oms-shell">
    <OperationsSidebar active="returns" user={user} go={go}/>
    <main className="oms-main operations-main">
      <header className="oms-topbar">
        <div><h1>Retourenavis</h1></div>
        <div><button onClick={() => setRefresh(value => value + 1)} className="oms-refresh">↻ Aktualisieren</button></div>
      </header>
      <div className="warehouse-return-tabs" aria-label="Lagerarbeitsbereich">
        <button onClick={() => go('/verwaltung')}>Versandaufträge</button>
        <button className="active" aria-current="page">Retoureneingang</button>
      </div>
      {(error || notice) && <div className={`oms-alert ${error ? 'error' : ''}`} role={error ? 'alert' : 'status'}>{error || notice}</div>}
      <section className="warehouse-return-panel">
        <header>
          <div><h2>Angemeldete Rücksendungen</h2><p>Jede OMS-Anfrage erhält einen Retourenavis. Nur als wiederverkaufbar geprüfte Ware wird dem verfügbaren Bestand zugeschlagen.</p></div>
          <input aria-label="Retouren suchen" value={query} onChange={event => { setQuery(event.target.value); setPage(1) }} placeholder="Avis, Bestellung, SKU oder Kunde"/>
        </header>
        <nav className="warehouse-return-filters" aria-label="Retourenstatus">
          { [['all','Alle'],['expected','Erwartet'],['received','Prüfung offen'],['restocked','Eingelagert'],['damaged','Beschädigt'],['not_restockable','Nicht einlagerbar']].map(([value,label]) =>
            <button key={value} className={status === value ? 'active' : ''} onClick={() => { setStatus(value); setPage(1) }}>{label}</button>
          )}
        </nav>
        <div className="oms-table-shell">
          {error && !cases.length && <p className="oms-table-state" role="alert">{error}</p>}
          {!error && !cases.length && <p className="oms-table-state">{loading ? 'Retourenavis werden geladen …' : 'Keine Retourenavis in diesem Filter.'}</p>}
          <div className="oms-table-scroll">
            <table className="oms-data-table warehouse-return-table">
              <thead><tr><th>Avis / Bestellung</th><th>Artikel</th><th>Kunde</th><th>Anfrage</th><th>Grund / Status</th><th>Warehouse-Aktion</th></tr></thead>
              <tbody>{cases.map(item => <tr key={item.id}>
                <td><strong>{item.reference}</strong><small><button className="oms-order-id" onClick={() => go(`/oms/orders/${encodeURIComponent(item.orderId)}`)}>{item.orderId}</button> · {dateTime(item.createdAt)}</small></td>
                <td><strong>{item.productName}</strong><small>{item.sku} · {item.status === 'expected' ? `${item.quantity} Stück angemeldet` : `${item.receivedQuantity} von ${item.quantity} Stück eingetroffen`}</small></td>
                <td><strong>{item.customerName || 'Gastkunde'}</strong><small>{item.customerEmail || '—'}</small></td>
                <td>{requestLabels[item.requestType] || item.requestType}</td>
                <td><span className={`warehouse-return-status ${item.status}`}>{statusLabels[item.status] || item.status}</span><small>{item.reason}</small>{item.parcelReference && <small>Paket: {item.parcelReference}</small>}</td>
                <td className="warehouse-return-actions">
                  {item.status === 'expected' && <><label className="visually-hidden" htmlFor={`quantity-${item.id}`}>Tatsächlich eingetroffene Menge</label><input id={`quantity-${item.id}`} type="number" min="1" max={item.quantity} value={references[`quantity-${item.id}`] ?? item.quantity} onChange={event => setReferences(current => ({ ...current, [`quantity-${item.id}`]: event.target.value }))} aria-label="Eingetroffene Menge"/><label className="visually-hidden" htmlFor={`parcel-${item.id}`}>Paketreferenz</label><input id={`parcel-${item.id}`} value={references[item.id] || ''} maxLength="120" onChange={event => setReferences(current => ({ ...current, [item.id]: event.target.value }))} placeholder="Paketreferenz optional"/><button disabled={busy === `${item.id}:receive`} onClick={() => mutate(item, 'receive', { quantity: Number(references[`quantity-${item.id}`] ?? item.quantity), parcelReference: references[item.id] || '' })}>Eingang erfassen</button></>}
                  {item.status === 'received' && <><button disabled={Boolean(busy)} onClick={() => mutate(item, 'inspect', { disposition: 'restock' })}>Einlagern</button><button className="secondary" disabled={Boolean(busy)} onClick={() => mutate(item, 'inspect', { disposition: 'damaged' })}>Beschädigt</button><button className="secondary" disabled={Boolean(busy)} onClick={() => mutate(item, 'inspect', { disposition: 'not_restockable' })}>Nicht einlagerbar</button></>}
                  {!['expected','received'].includes(item.status) && <small>Erfasst: {item.receivedBy || '—'} · geprüft: {item.inspectedBy || '—'}</small>}
                </td>
              </tr>)}</tbody>
            </table>
          </div>
        </div>
        <footer className="warehouse-return-pagination"><span>{total} Avis · Seite {page} von {pages}</span><div><button disabled={page <= 1} onClick={() => setPage(value => value - 1)}>← Zurück</button><button disabled={page >= pages} onClick={() => setPage(value => value + 1)}>Weiter →</button></div></footer>
      </section>
    </main>
  </div>
}
