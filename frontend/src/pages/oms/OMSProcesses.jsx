import { useRef, useState } from 'react'
import { api, csrf, isAuthorizationError } from '../../lib/api.js'
import { OMSFrame } from './OMSDashboard.jsx'
import { unappliedDelayRows, validateDelayCSV } from '../../lib/oms-csv.js'

export function OMSProcesses({ user, go }) {
  const [fileName, setFileName] = useState('')
  const [records, setRecords] = useState([])
  const [error, setError] = useState('')
  const [results, setResults] = useState(null)
  const [running, setRunning] = useState(false)
  const [acknowledged, setAcknowledged] = useState(false)
  const fileInput = useRef(null)

  const selectFile = async event => {
    const file = event.target.files?.[0]
    if (!file) return
    setFileName(file.name)
    setError('')
    setResults(null)
    setAcknowledged(false)
    try {
      if (file.size > 1024 * 1024) throw new Error('CSV files must be 1 MB or smaller.')
      const parsed = validateDelayCSV(await file.text())
      setRecords(parsed)
    } catch (cause) {
      setRecords([])
      setError(cause.message)
    }
  }

  const apply = async () => {
    if (!acknowledged || !records.length || records.some(record => record.error) || running) return
    setRunning(true)
    setError('')
    setAcknowledged(false)
    const nextResults = results ? [...results] : Array(records.length).fill(null)
    const pending = unappliedDelayRows(records, results)
    for (let position = 0; position < pending.length; position += 1) {
      const { record, index } = pending[position]
      nextResults[index] = { status: 'running', message: 'Updating…' }
      setResults([...nextResults])
      try {
        await api(`/api/operations/orders/${encodeURIComponent(record.orderID)}/items/operation`, {
          method: 'POST',
          headers: { 'X-CSRF-Token': csrf },
          body: JSON.stringify({
            productName: record.productName,
            action: 'delay',
            detail: record.delayedDate
          })
        })
        nextResults[index] = { status: 'updated', message: 'Updated' }
      } catch (cause) {
        nextResults[index] = { status: 'failed', message: cause.message }
        if (isAuthorizationError(cause.message)) {
          for (const skipped of pending.slice(position + 1)) {
            nextResults[skipped.index] = { status: 'not_attempted', message: 'Not attempted because staff authorization failed.' }
          }
          setError(cause.message)
          break
        }
      }
      setResults([...nextResults])
    }
    setResults(nextResults)
    setRunning(false)
  }

  const invalidCount = records.filter(record => record.error).length
  const updatedCount = results?.filter(result => result?.status === 'updated').length || 0
  const remainingCount = results?.filter(result => result && result.status !== 'updated').length || 0
  const workflows = [
    user?.permissions?.includes('warehouse') && {
      title: 'Shipment fulfillment',
      description: 'Pick, pack, add carrier and tracking details, and hand over shipments.',
      action: 'Open fulfillment →',
      path: '/verwaltung'
    },
    user?.permissions?.includes('orders') && {
      title: 'Order actions',
      description: 'Review cancellations, refunds, returns, and replacement requests on each order.',
      action: 'Open orders →',
      path: '/oms/orders'
    },
    user?.permissions?.includes('orders') && {
      title: 'Returns and replacement requests',
      description: 'Review tracked item requests and open the associated order. Resolution actions are not configured.',
      action: 'Review requests →',
      path: '/oms/processes/requests'
    },
    user?.permissions?.includes('orders') && {
      title: 'Shipment delay log',
      description: 'Review item delivery dates changed through the bulk delay workflow.',
      action: 'Open delay log →',
      path: '/oms/processes/shipment-delays'
    },
    user?.permissions?.includes('payments') && {
      title: 'Payments and invoices',
      description: 'Review payment records; open issued invoices from their order details.',
      action: 'Open payments →',
      path: '/zahlungen'
    }
  ].filter(Boolean)

  return (
    <OMSFrame user={user} go={go} title="Processes">
      <div className="oms-workspace">
        <header className="oms-page-heading">
          <div>
            <h1>Order processes</h1>
            <p>Operational tools that work with the current order, payment, and fulfillment records.</p>
          </div>
        </header>

        <section className="oms-process-group" aria-labelledby="oms-existing-processes">
          <div className="oms-section-heading">
            <div><h2 id="oms-existing-processes">Available workspaces</h2><p>Continue a process in the screen that owns its workflow and data.</p></div>
          </div>
          <div className="oms-process-links">
            {workflows.map(workflow => (
              <button key={workflow.path} onClick={() => go(workflow.path)}>
                <strong>{workflow.title}</strong><span>{workflow.description}</span><b>{workflow.action}</b>
              </button>
            ))}
            {!workflows.length && <p className="oms-table-state">No additional process workspaces are assigned to this staff account.</p>}
          </div>
        </section>

        <section className="oms-process-group" aria-labelledby="oms-delay-upload">
          <div className="oms-section-heading">
            <div><h2 id="oms-delay-upload">Bulk shipment delays</h2><p>Preview a CSV and update delivery dates for specific order items.</p></div>
          </div>
          <div className="oms-process-panel">
            <p>Required columns: <code>order_id</code>, <code>product_name</code>, <code>delayed_delivery_date</code> using YYYY-MM-DD. Maximum 200 rows per file. The file is read in your browser; each valid row is saved through the order operations API.</p>
            <div className="oms-process-controls">
              <input ref={fileInput} className="visually-hidden" type="file" accept=".csv,text/csv" onChange={selectFile} />
              <button className="oms-secondary-action" onClick={() => fileInput.current?.click()} disabled={running}>Choose CSV</button>
              <span>{fileName || 'No file selected'}</span>
            </div>
            {error && <p className="oms-inline-error" role="alert">{error}</p>}
            {records.length > 0 && (
              <>
                <div className="oms-table-summary">
                  <span>{records.length} rows · {invalidCount} invalid</span>
                  {results && <span>{updatedCount} updated · {failedCount} failed</span>}
                </div>
                <div className="oms-table-shell">
                  <div className="oms-table-scroll">
                    <table className="oms-data-table">
                      <thead><tr><th scope="col">CSV row</th><th scope="col">Order</th><th scope="col">Product</th><th scope="col">New delivery date</th><th scope="col">Result</th></tr></thead>
                      <tbody>{records.map((record, index) => {
                        const result = results?.[index]
                        return <tr key={`${record.line}-${record.orderID}-${record.productName}`}>
                          <td>{record.line}</td><td>{record.orderID || '—'}</td><td>{record.productName || '—'}</td>
                          <td>{record.delayedDate || '—'}</td>
                          <td>{result?.message || record.error || 'Ready'}</td>
                        </tr>
                      })}</tbody>
                    </table>
                  </div>
                </div>
                {invalidCount === 0 && (!results || remainingCount > 0) && (
                  <div className="oms-process-confirm">
                    <label><input type="checkbox" checked={acknowledged} onChange={event => setAcknowledged(event.target.checked)} /> {results ? `I reviewed the ${remainingCount} unsaved rows and authorize retrying only those updates.` : `I reviewed these ${records.length} order-item updates and authorize saving them.`}</label>
                    <button className="oms-primary-action" onClick={apply} disabled={!acknowledged || running}>
                      {running ? `Updating…` : results ? `Retry ${remainingCount} unsaved updates` : `Apply ${records.length} delivery updates`}
                    </button>
                  </div>
                )}
                {invalidCount > 0 && <p className="oms-matrix-note">Correct the invalid rows in your CSV and choose the file again. No updates have been applied.</p>}
                {results && remainingCount > 0 && <p className="oms-matrix-note" role="status">{updatedCount} updates are saved and will not be repeated. Retry will send only the {remainingCount} rows not yet saved.</p>}
                {results && remainingCount === 0 && <p className="oms-matrix-note" role="status">All {updatedCount} delivery updates were saved.</p>}
              </>
            )}
          </div>
        </section>
      </div>
    </OMSFrame>
  )
}
