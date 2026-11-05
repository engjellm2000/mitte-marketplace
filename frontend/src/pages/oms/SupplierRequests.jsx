import { useEffect, useState } from 'react'
import { api, csrf } from '../../lib/api.js'
import { OMSNavigation } from './OMSNavigation.jsx'

const dateTime = value => value ? new Date(value).toLocaleString('de-DE', { dateStyle: 'medium', timeStyle: 'short' }) : '—'

export function SupplierRequests({ user, go }) {
  const [requests, setRequests] = useState([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [busyID, setBusyID] = useState(0)
  const [notes, setNotes] = useState({})
  const refresh = async () => {
    setLoading(true)
    try {
      const data = await api('/api/operations/supplier-requests', { cache: 'no-store' })
      setRequests(data.requests || [])
      setError('')
    } catch (cause) {
      setError(cause.message)
    } finally {
      setLoading(false)
    }
  }
  useEffect(() => { refresh() }, [])
  const review = async (request, status) => {
    const note = (notes[request.ID] || '').trim()
    if (!note) {
      setError('Bitte ergänze eine interne Notiz zur Entscheidung.')
      return
    }
    setBusyID(request.ID)
    setError('')
    try {
      await api(`/api/operations/supplier-requests/${request.ID}/review`, {
        method: 'POST',
        headers: { 'X-CSRF-Token': csrf },
        body: JSON.stringify({ status, note })
      })
      await refresh()
    } catch (cause) {
      setError(cause.message)
    } finally {
      setBusyID(0)
    }
  }

  return <div className="oms-app-shell"><OMSNavigation user={user} go={go} /><main className="oms-main">
    <section className="oms-command-center">
      <div className="oms-command-heading"><div><h1>Lieferantenanfragen</h1><p>Stornierungswünsche aus dem Lieferantenportal, die im OMS geprüft werden müssen.</p></div><button className="oms-secondary-action" onClick={refresh} disabled={loading}>{loading ? 'Wird geladen …' : 'Aktualisieren'}</button></div>
      {error && <div className="oms-alert error" role="alert">{error}</div>}
      <div className="supplier-request-summary"><strong>{requests.length}</strong><span>Offene Stornierungsanfragen</span></div>
      <div className="oms-table-wrap"><table className="oms-table"><thead><tr><th>Auftrag</th><th>Lieferant</th><th>Artikel</th><th>Menge</th><th>Grund</th><th>Eingegangen</th><th>Prüfung</th></tr></thead><tbody>
        {requests.map(request => <tr key={request.ID}><td><button className="supplier-request-order" onClick={() => go(`/oms/orders/${encodeURIComponent(request.OrderID)}`)}>{request.OrderID}</button><small>{request.OrderStatus}</small></td><td>{request.SupplierName}<small>{request.ActorUsername}</small></td><td>{request.ProductName}<small>{request.SKU}</small></td><td>{request.Quantity}</td><td>{request.Note || '—'}</td><td>{dateTime(request.CreatedAt)}</td><td><div className="supplier-request-actions"><input aria-label={`Interne Notiz für ${request.OrderID}`} value={notes[request.ID] || ''} onChange={event => setNotes({ ...notes, [request.ID]: event.target.value })} placeholder="Interne Notiz" maxLength="1000" /><button className="oms-secondary-action" disabled={busyID === request.ID} onClick={() => review(request, 'handled')}>Bearbeitet</button><button className="supplier-request-reject" disabled={busyID === request.ID} onClick={() => review(request, 'rejected')}>Ablehnen</button><button className="supplier-request-order" onClick={() => go(`/oms/orders/${encodeURIComponent(request.OrderID)}`)}>Auftrag öffnen →</button></div></td></tr>)}
      </tbody></table>
      {!requests.length && <p className="oms-empty-state">{loading ? 'Anfragen werden geladen …' : error ? 'Anfragen konnten nicht geladen werden.' : 'Keine offenen Lieferanten-Stornierungsanfragen.'}</p>}</div>
      <p className="supplier-request-caveat">Eine Anfrage storniert weder die Kundenbestellung noch löst sie eine Erstattung aus. Bitte die Zahlungs- und Auftragsentscheidung im OMS abschließen.</p>
    </section>
  </main></div>
}
