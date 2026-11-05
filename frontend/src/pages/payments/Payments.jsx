import React, { useEffect, useMemo, useState } from 'react'
import '../../billie.css'
import { OperationsSidebar } from '../../components/operations/OperationsSidebar.jsx'
import { formatEuro } from '../../lib/format.js'

const euro = cents => formatEuro(Number(cents) || 0)
const csrf = document.querySelector('meta[name="csrf-token"]')?.content
const api = async (path, options = {}) => {
  const response = await fetch(path, { ...options, cache: 'no-store', headers: { 'Content-Type': 'application/json', ...(options.headers || {}) } })
  const payload = await response.json().catch(() => ({}))
  if (!response.ok) throw new Error(payload.error || 'Etwas ist schiefgelaufen.')
  return payload
}
const dateTime = value => value ? new Intl.DateTimeFormat('de-DE', { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(value)) : '—'
const dateOnly = value => value ? new Intl.DateTimeFormat('de-DE', { dateStyle: 'medium' }).format(new Date(value)) : '—'
const hay = (...values) => values.filter(Boolean).join(' ').toLowerCase()
const matches = (query, ...values) => !query.trim() || hay(...values).includes(query.trim().toLowerCase())
const addr = value => value || {}
const addrLine = address => [address.RecipientName || address.recipientName, address.CompanyName || address.companyName, address.AddressLine1 || address.addressLine1, address.AddressLine2 || address.addressLine2, `${address.PostalCode || address.postalCode || ''} ${address.City || address.city || ''}`.trim(), address.CountryCode || address.countryCode].filter(Boolean)
const statusClass = label => ({
  Erstellt: 'erstellt', Versendet: 'versendet', Überfällig: 'ueberfaellig', Abgeschlossen: 'abgeschlossen',
  Ausgezahlt: 'ausgezahlt', Storniert: 'storniert', Abgelehnt: 'abgelehnt', Abgebrochen: 'abgebrochen',
  Bezahlt: 'bezahlt', Offen: 'offen', 'Teilweise zugeordnet': 'teilweise', Zugeordnet: 'abgeschlossen', 'Nicht zugeordnet': 'offen'
}[label] || 'erstellt')

const receivableLabel = (order, payment) => {
  if (order?.Status === 'cancelled' || payment?.ReceivableStatus === 'cancelled') return 'Storniert'
  const rec = payment?.ReceivableStatus
  if (rec === 'overdue') return 'Überfällig'
  if (rec === 'declined') return 'Abgelehnt'
  if (rec === 'paid_out') return 'Ausgezahlt'
  if (rec === 'completed' || rec === 'paid') return 'Abgeschlossen'
  if (rec === 'sent') return 'Versendet'
  if (order?.Status === 'fulfilled' || order?.Shipment) return 'Versendet'
  if (rec === 'created' || order?.Status === 'invoice_approved' || order?.Status === 'pending_payment') return 'Erstellt'
  if (order?.Status === 'paid') return 'Abgeschlossen'
  return rec || order?.Status || 'Erstellt'
}

const receiptStatus = receipt => {
  if (!receipt.AllocatedCents) return 'Nicht zugeordnet'
  if (receipt.UnallocatedCents > 0) return 'Teilweise zugeordnet'
  return 'Zugeordnet'
}

function parseRoute(route) {
  const parts = (route.split('?')[0] || '').replace(/\/+$/, '').split('/').filter(Boolean)
  const section = parts[1] || 'bestellungen'
  const id = parts.slice(2).join('/')
  if (!parts[1] || section === 'bestellungen') return { view: 'orders', id: section === 'bestellungen' ? id : '' }
  if (section === 'bestellung') return { view: 'orders', id }
  if (section === 'kunden') return { view: 'customers', id: id ? decodeURIComponent(id) : '' }
  if (section === 'eingang') return { view: 'payments', id }
  if (section === 'forderung') return { view: 'payments', claim: id }
  return { view: 'orders', id: section }
}

function StatusPill({ label }) {
  return <span className={`billie-status ${statusClass(label)}`}>{label}</span>
}

function AddressBlock({ address }) {
  const lines = addrLine(addr(address))
  if (!lines.length) return <span>—</span>
  return <address>{lines.map(line => <span key={line}>{line}</span>)}</address>
}

function Shell({ user, go, view, search, setSearch, hits, counts, children }) {
  return (
    <div className="operations-shell enterprise">
      <OperationsSidebar active="payments" user={user} go={go}/>
      <main className="enterprise-main operations-main">
        <div className="payments-quick-tools">
          <label><span>Bereichsübergreifende Suche</span><input value={search} onChange={e => setSearch(e.target.value)} placeholder="Bestellung, Kunde oder Zahlung suchen …" /></label>
          {hits}
        </div>
        <nav className="payments-subnav" aria-label="Zahlungsbereich">
          <button className={view === 'orders' ? 'active' : ''} onClick={() => go('/zahlungen')}>Rechnungen <span>{counts.orders}</span></button>
          <button className={view === 'customers' ? 'active' : ''} onClick={() => go('/zahlungen/kunden')}>Kunden <span>{counts.customers}</span></button>
          <button className={view === 'payments' ? 'active' : ''} onClick={() => go('/zahlungen/eingang')}>Zahlungseingänge <span>{counts.payments}</span></button>
        </nav>
        {children}
      </main>
    </div>
  )
}

export function Payments({ user, go, route, onSwitchAccount }) {
  const parsed = parseRoute(route)
  const [orders, setOrders] = useState([])
  const [transactions, setTransactions] = useState([])
  const [receipts, setReceipts] = useState([])
  const [bank, setBank] = useState({})
  const [detail, setDetail] = useState(null)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const [loading, setLoading] = useState(true)
  const [busy, setBusy] = useState('')
  const [search, setSearch] = useState('')
  const [listQuery, setListQuery] = useState('')
  const [filter, setFilter] = useState('all')
  const [drafts, setDrafts] = useState({})
  const [newReceipt, setNewReceipt] = useState({ amount: '', payerName: '', ibanLast4: '', note: '' })

  const paymentByOrder = useMemo(() => Object.fromEntries(transactions.map(item => [item.OrderID, item])), [transactions])
  const customers = useMemo(() => {
    const map = new Map()
    for (const order of orders) {
      const key = (order.CustomerEmail || order.CustomerName || order.ID).toLowerCase()
      const current = map.get(key) || {
        key, name: order.CustomerName, email: order.CustomerEmail, company: order.CompanyName,
        type: order.CustomerType, vat: order.VATID, orders: [], totalCents: 0, outstandingCents: 0
      }
      current.orders.push(order)
      current.totalCents += order.TotalCents || 0
      current.outstandingCents += paymentByOrder[order.ID]?.OutstandingCents || 0
      if (!current.company && order.CompanyName) current.company = order.CompanyName
      map.set(key, current)
    }
    return [...map.values()].sort((a, b) => a.name.localeCompare(b.name, 'de'))
  }, [orders, paymentByOrder])

  const load = async () => {
    setLoading(true)
    try {
      const [orderData, paymentData, receiptData] = await Promise.all([
        api('/api/operations/orders'),
        api('/api/payments'),
        api('/api/payment-receipts')
      ])
      setOrders(orderData.orders || [])
      setTransactions(paymentData.transactions || [])
      setBank(paymentData.bank || {})
      setReceipts(receiptData.receipts || [])
      setError('')
    } catch (cause) {
      setError(cause.message)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { load() }, [])
  useEffect(() => { setListQuery(''); setFilter('all') }, [parsed.view])
  useEffect(() => {
    if (parsed.view !== 'orders' || !parsed.id) { setDetail(null); return }
    let live = true
    api(`/api/operations/orders/${encodeURIComponent(parsed.id)}`)
      .then(data => { if (live) setDetail(data.detail) })
      .catch(cause => { if (live) setError(cause.message) })
    return () => { live = false }
  }, [parsed.view, parsed.id])

  const invoices = transactions.filter(payment => payment.Provider === 'mitte_invoice')
  const selectedOrder = detail?.Order || orders.find(item => item.ID === parsed.id)
  const selectedCustomer = customers.find(item => item.key === parsed.id)
  const selectedReceipt = receipts.find(item => String(item.ID) === String(parsed.id))
  const selectedClaim = transactions.find(item => item.OrderID === parsed.claim)

  const filteredOrders = orders.filter(order => {
    const payment = paymentByOrder[order.ID]
    const label = receivableLabel(order, payment)
    const okFilter = filter === 'all' || label === filter
    return okFilter && matches(listQuery, order.ID, order.CustomerName, order.CustomerEmail, order.CompanyName, payment?.InvoiceNumber, payment?.Reference)
  })
  const filteredCustomers = customers.filter(item => matches(listQuery, item.name, item.email, item.company, item.vat, ...item.orders.map(order => order.ID)))
  const filteredReceipts = receipts.filter(item => matches(listQuery, item.Reference, item.PayerName, item.Note, item.PayerIBANLast4, ...(item.Allocations || []).flatMap(row => [row.OrderID, row.InvoiceNumber])))
  const filteredClaims = transactions.filter(item => matches(listQuery, item.OrderID, item.Reference, item.InvoiceNumber, item.PayerName, item.Note, paymentByOrder[item.OrderID]?.InvoiceNumber) && (filter === 'all' || (filter === 'Offen' ? item.OutstandingCents > 0 : receivableLabel(orders.find(order => order.ID === item.OrderID), item) === filter)))

  const globalHits = search.trim() ? [
    ...orders.filter(order => matches(search, order.ID, order.CustomerName, paymentByOrder[order.ID]?.InvoiceNumber, paymentByOrder[order.ID]?.Reference)).slice(0, 5).map(order => ({ kind: 'Bestellung', label: `${order.ID} · ${order.CustomerName || 'Kunde'}`, path: `/zahlungen/bestellung/${order.ID}` })),
    ...customers.filter(item => matches(search, item.name, item.email, item.company)).slice(0, 5).map(item => ({ kind: 'Kunde', label: `${item.name} · ${item.email}`, path: `/zahlungen/kunden/${encodeURIComponent(item.key)}` })),
    ...receipts.filter(item => matches(search, item.Reference, item.PayerName, item.Note)).slice(0, 5).map(item => ({ kind: 'Zahlung', label: `${item.Reference} · ${item.PayerName}`, path: `/zahlungen/eingang/${item.ID}` })),
    ...transactions.filter(item => matches(search, item.OrderID, item.Reference, item.InvoiceNumber, item.PayerName)).slice(0, 5).map(item => ({ kind: 'Forderung', label: `${item.InvoiceNumber || item.Reference} · ${euro(item.AmountCents)}`, path: `/zahlungen/forderung/${item.OrderID}` }))
  ] : []

  const createReceipt = async event => {
    event.preventDefault()
    const amountCents = Math.round(Number(newReceipt.amount.replace(',', '.')) * 100)
    setBusy('new'); setError(''); setNotice('')
    try {
      await api('/api/payment-receipts', { method: 'POST', headers: { 'X-CSRF-Token': csrf }, body: JSON.stringify({ ...newReceipt, amountCents }) })
      setNewReceipt({ amount: '', payerName: '', ibanLast4: '', note: '' })
      setNotice('Zahlungseingang erfasst. Ordne ihn einer oder mehreren Rechnungen zu.')
      await load()
    } catch (cause) { setError(cause.message) } finally { setBusy('') }
  }

  const rowsFor = receipt => drafts[receipt.ID] || (receipt.Allocations || []).map(row => ({ orderId: row.OrderID, amountCents: row.AmountCents }))
  const saveAllocations = async receipt => {
    const allocations = rowsFor(receipt).filter(row => row.orderId && Number(row.amountCents) > 0).map(row => ({ orderId: row.orderId, amountCents: Number(row.amountCents) }))
    setBusy(`receipt-${receipt.ID}`); setError(''); setNotice('')
    try {
      await api(`/api/payment-receipts/${receipt.ID}/allocations`, { method: 'PUT', headers: { 'X-CSRF-Token': csrf }, body: JSON.stringify({ allocations }) })
      setNotice('Zuordnung gespeichert.')
      setDrafts(current => { const copy = { ...current }; delete copy[receipt.ID]; return copy })
      await load()
    } catch (cause) { setError(cause.message) } finally { setBusy('') }
  }

  if (error && !loading && !orders.length && !transactions.length && /autorisiert|nicht autorisiert|forbidden|unauthorized|403|401/i.test(error)) {
    const denied = /autorisiert|nicht autorisiert|forbidden/i.test(error)
    return <section className="oms-access"><div><b>M</b><span>Mitte ZAHLUNGEN</span></div><h1>{denied && user ? 'Zugriff nicht freigeschaltet.' : 'Zugriff erforderlich.'}</h1><p>{denied && user ? `Angemeldet als Konto ${user.ID}. Wenn dies Ihr verifiziertes Mitarbeiterkonto ist, setzen Sie STAFF_USER_IDS=${user.ID} in der lokalen .env (oder im Secret Manager) und starten Sie den Server neu. Eine E-Mail-Adresse allein erteilt keinen Zugriff.` : error}</p>{denied && user && onSwitchAccount ? <button onClick={onSwitchAccount}>Zur Anmeldung wechseln</button> : <button onClick={() => go('/konto')}>Anmelden</button>}{denied && user && <button className="text-button" onClick={() => go('/')}>Zum Shop</button>}</section>
  }

  const hits = globalHits.length ? <div className="billie-hits">{globalHits.map(hit => <button key={hit.path + hit.label} onClick={() => { go(hit.path); setSearch('') }}><small>{hit.kind}</small><span>{hit.label}</span></button>)}</div> : (search.trim() ? <div className="billie-hits"><button type="button"><small>Suche</small><span>Keine Treffer</span></button></div> : null)

  return (
    <Shell user={user} go={go} view={parsed.view} search={search} setSearch={setSearch} hits={hits} counts={{ orders: orders.length, customers: customers.length, payments: receipts.length + transactions.length }}>
      {error && <p className="billie-notice error" role="alert">{error}<button type="button" onClick={load}>Erneut versuchen</button></p>}
      {notice && <p className="billie-notice">{notice}</p>}
      {parsed.view === 'orders' && !parsed.id && <OrdersList go={go} orders={filteredOrders} paymentByOrder={paymentByOrder} query={listQuery} setQuery={setListQuery} filter={filter} setFilter={setFilter} loading={loading} />}
      {parsed.view === 'orders' && parsed.id && <OrderDetail go={go} order={selectedOrder} payment={paymentByOrder[parsed.id]} bank={bank} detail={detail} receipts={receipts.filter(item => (item.Allocations || []).some(row => row.OrderID === parsed.id))} />}
      {parsed.view === 'customers' && !parsed.id && <CustomersList go={go} customers={filteredCustomers} query={listQuery} setQuery={setListQuery} loading={loading} />}
      {parsed.view === 'customers' && parsed.id && <CustomerDetail go={go} customer={selectedCustomer} paymentByOrder={paymentByOrder} />}
      {parsed.view === 'payments' && !parsed.id && !parsed.claim && <PaymentsList go={go} receipts={filteredReceipts} claims={filteredClaims} orders={orders} query={listQuery} setQuery={setListQuery} filter={filter} setFilter={setFilter} newReceipt={newReceipt} setNewReceipt={setNewReceipt} createReceipt={createReceipt} busy={busy} loading={loading} />}
      {parsed.view === 'payments' && selectedReceipt && <ReceiptDetail go={go} receipt={selectedReceipt} invoices={invoices} rows={rowsFor(selectedReceipt)} setRows={rows => setDrafts(current => ({ ...current, [selectedReceipt.ID]: rows }))} save={() => saveAllocations(selectedReceipt)} busy={busy} />}
      {parsed.view === 'payments' && parsed.claim && <ClaimDetail go={go} claim={selectedClaim} order={orders.find(item => item.ID === parsed.claim)} receipts={receipts.filter(item => (item.Allocations || []).some(row => row.OrderID === parsed.claim))} />}
    </Shell>
  )
}

function OrdersList({ go, orders, paymentByOrder, query, setQuery, filter, setFilter, loading }) {
  const filters = ['all', 'Erstellt', 'Versendet', 'Überfällig', 'Abgeschlossen', 'Storniert']
  return (
    <>
      <header className="billie-toolbar">
        <div><p>Sales / Bestellungen</p><h1>Bestellungen</h1></div>
        <div className="billie-toolbar-actions">
          <input className="billie-search" aria-label="Bestell- oder Rechnungsnummer suchen" value={query} onChange={e => setQuery(e.target.value)} placeholder="Bestell- oder Rechnungsnummer suchen" />
        </div>
      </header>
      <div className="billie-filters">{filters.map(value => <button key={value} className={filter === value ? 'active' : ''} onClick={() => setFilter(value)}>{value === 'all' ? 'Alle' : value}</button>)}</div>
      <div className="billie-table-wrap">
        <table className="billie-table">
          <thead><tr><th>Bestellnummer</th><th>Kunde</th><th>Bestelldatum</th><th>Fälligkeit</th><th className="right">Betrag</th><th>Status</th></tr></thead>
          <tbody>
            {orders.map(order => {
              const payment = paymentByOrder[order.ID]
              const label = receivableLabel(order, payment)
              return (
                <tr key={order.ID} role="link" tabIndex="0" aria-label={`Rechnung für ${order.ID} öffnen`} onClick={() => go(`/zahlungen/bestellung/${order.ID}`)} onKeyDown={event => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); go(`/zahlungen/bestellung/${order.ID}`) } }}>
                  <td><strong>{order.ID}</strong><div className="muted">{payment?.InvoiceNumber || 'Rechnung ausstehend'}</div></td>
                  <td>{order.CustomerName || '—'}<div className="muted">{order.CompanyName || order.CustomerEmail}</div></td>
                  <td>{dateOnly(order.CreatedAt)}</td>
                  <td>{dateOnly(payment?.DueAt || order.Invoice?.DueAt)}</td>
                  <td className="right">{euro(order.TotalCents)}</td>
                  <td><StatusPill label={label} /></td>
                </tr>
              )
            })}
          </tbody>
        </table>
        {!orders.length && <p className="billie-empty">{loading ? 'Rechnungen werden geladen …' : query ? 'Keine Rechnungen für diese Suche gefunden.' : 'Keine Rechnungen in dieser Ansicht.'}</p>}
      </div>
    </>
  )
}

function OrderDetail({ go, order, payment, bank, detail, receipts }) {
  if (!order) return <p className="billie-empty">Bestellung nicht gefunden.</p>
  const label = receivableLabel(order, payment)
  return (
    <>
      <button className="billie-back" onClick={() => go('/zahlungen')}>← Zurück zur Bestellübersicht</button>
      <header className="billie-toolbar">
        <div><p>Sales / Bestellungen / Bestelldetails</p><h1>Bestellung {order.ID}</h1></div>
        <StatusPill label={label} />
      </header>
      <section className="enterprise-customer">
        <h2>♟ Kunde</h2>
        <div>
          <article><small>Kunde</small><strong>{order.CustomerName || '—'}</strong><a href={`mailto:${order.CustomerEmail}`}>{order.CustomerEmail}</a><span>{order.CustomerType === 'b2b' ? 'Geschäftskunde' : 'Privatkunde'}</span></article>
          <article><small>Unternehmen</small><strong>{order.CompanyName || '—'}</strong><span>{order.VATID || 'Keine USt-IdNr.'}</span></article>
          <article><small>Rechnungsadresse</small><AddressBlock address={order.BillingAddress} /></article>
          <article><small>Lieferadresse</small><AddressBlock address={order.ShippingAddress} /></article>
          <article><small>Offen</small><strong>{euro(payment?.OutstandingCents || 0)}</strong><span>von {euro(order.TotalCents)}</span></article>
        </div>
      </section>
      <div className="billie-cards">
        <section className="enterprise-card">
          <h2>⌁ Bestellinformationen</h2>
          <dl className="billie-kv">
            <dt>Bestellnummer</dt><dd>{order.ID}</dd>
            <dt>Rechnungsnummer</dt><dd>{payment?.InvoiceNumber || order.Invoice?.Number || 'Noch nicht ausgestellt'}</dd>
            <dt>Bestelldatum</dt><dd>{dateTime(order.CreatedAt)}</dd>
            <dt>Versanddatum</dt><dd>{dateTime(order.Shipment?.ShippedAt)}</dd>
            <dt>Fälligkeit</dt><dd>{dateOnly(payment?.DueAt || order.Invoice?.DueAt)}</dd>
            <dt>Zahlungsart</dt><dd>{order.PaymentProvider === 'mitte_invoice' ? 'Rechnungskauf' : order.PaymentProvider || '—'}</dd>
            <dt>Referenz</dt><dd>{payment?.Reference || '—'}</dd>
            <dt>Positionen</dt><dd>{(order.Items || []).map(item => <span key={item.Name}>{item.Quantity}× {item.Name}</span>)}</dd>
          </dl>
        </section>
        <section className="enterprise-card">
          <h2>€ Bankinformationen</h2>
          <div className="billie-bank">
            <small>Empfänger</small>
            <strong>{bank.recipient || 'Mitte'}</strong>
            <small>IBAN</small>
            <strong>{bank.iban || '—'}</strong>
            <small>Verwendungszweck</small>
            <strong>{payment?.Reference || order.ID}</strong>
            <p>{bank.terms || 'Bitte mit exaktem Verwendungszweck überweisen.'}</p>
          </div>
        </section>
        <section className="enterprise-card totals">
          <h2>▣ Betrag</h2>
          <dl>
            <dt>Zwischensumme</dt><dd>{euro(order.SubtotalCents)}</dd>
            {order.DiscountCents > 0 && <><dt>Rabatt</dt><dd>−{euro(order.DiscountCents)}</dd></>}
            <dt>Versand</dt><dd>{euro(order.ShippingCents)}</dd>
            <dt>MwSt.</dt><dd>{euro(order.TaxCents)}</dd>
            <dt className="grand">Gesamt</dt><dd className="grand">{euro(order.TotalCents)}</dd>
            <dt>Zugeordnet</dt><dd>{euro(payment?.AppliedCents || 0)}</dd>
            <dt>Offen</dt><dd>{euro(payment?.OutstandingCents || 0)}</dd>
          </dl>
        </section>
      </div>
      <div className="billie-cards">
        <section className="enterprise-card activity-card">
          <h2>Mahnwesen</h2>
          <div className="billie-bank">
            <p>Status: <StatusPill label={label} /></p>
            {label === 'Überfällig'
              ? <p>Das Zahlungsziel ist überschritten. Erinnerungen und Mahnungen laufen über das Forderungsbuch.</p>
              : <p>Kein aktives Mahnverfahren. Mahnungen starten automatisch nach Fälligkeit.</p>}
          </div>
        </section>
        <section className="enterprise-card activity-card" style={{ gridColumn: 'span 2' }}>
          <h2>Zahlungshistorie</h2>
          <table>
            <thead><tr><th>Typ</th><th>Referenz</th><th>Details</th><th>Betrag</th><th>Datum</th></tr></thead>
            <tbody>
              {receipts.map(item => {
                const allocated = (item.Allocations || []).filter(row => row.OrderID === order.ID).reduce((sum, row) => sum + row.AmountCents, 0)
                return <tr key={item.ID} role="link" tabIndex="0" aria-label={`Zahlungseingang ${item.Reference} öffnen`} onClick={() => go(`/zahlungen/eingang/${item.ID}`)} onKeyDown={event => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); go(`/zahlungen/eingang/${item.ID}`) } }}><td>Eingang</td><td>{item.Reference}</td><td>{item.PayerName}<small>{item.Note}</small></td><td>{euro(allocated)}</td><td>{dateTime(item.ReceivedAt)}</td></tr>
              })}
              {(detail?.PaymentEvents || []).map((event, index) => <tr key={`${event.ProviderEventID}-${index}`}><td>{event.EventType}</td><td>{event.ProviderEventID}</td><td>—</td><td>—</td><td>{dateTime(event.ProcessedAt)}</td></tr>)}
              {!receipts.length && !(detail?.PaymentEvents || []).length && <tr><td colSpan="5">Noch keine Zahlungen dieser Bestellung zugeordnet.</td></tr>}
            </tbody>
          </table>
        </section>
      </div>
    </>
  )
}

function CustomersList({ go, customers, query, setQuery, loading }) {
  return (
    <>
      <header className="billie-toolbar">
        <div><p>Sales / Kunden</p><h1>Kunden</h1></div>
        <input className="billie-search" aria-label="Kunden nach Name, E-Mail oder Unternehmen suchen" value={query} onChange={e => setQuery(e.target.value)} placeholder="Name, E-Mail oder Unternehmen suchen" />
      </header>
      <div className="billie-table-wrap">
        <table className="billie-table">
          <thead><tr><th>Kunde</th><th>Unternehmen</th><th>E-Mail</th><th>Bestellungen</th><th className="right">Offen</th><th className="right">Gesamt</th></tr></thead>
          <tbody>
            {customers.map(item => (
              <tr key={item.key} role="link" tabIndex="0" aria-label={`Konto von ${item.name || item.email} öffnen`} onClick={() => go(`/zahlungen/kunden/${encodeURIComponent(item.key)}`)} onKeyDown={event => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); go(`/zahlungen/kunden/${encodeURIComponent(item.key)}`) } }}>
                <td><strong>{item.name || '—'}</strong><div className="muted">{item.type === 'b2b' ? 'Geschäftskunde' : 'Privatkunde'}</div></td>
                <td>{item.company || '—'}</td>
                <td>{item.email}</td>
                <td>{item.orders.length}</td>
                <td className="right">{euro(item.outstandingCents)}</td>
                <td className="right">{euro(item.totalCents)}</td>
              </tr>
            ))}
          </tbody>
        </table>
        {!customers.length && <p className="billie-empty">{loading ? 'Kunden werden geladen …' : query ? 'Keine Kunden für diese Suche gefunden.' : 'Noch keine Kunden vorhanden.'}</p>}
      </div>
    </>
  )
}

function CustomerDetail({ go, customer, paymentByOrder }) {
  if (!customer) return <p className="billie-empty">Kunde nicht gefunden.</p>
  const latest = customer.orders[customer.orders.length - 1]
  return (
    <>
      <button className="billie-back" onClick={() => go('/zahlungen/kunden')}>← Zurück zur Kundenübersicht</button>
      <header className="billie-toolbar">
        <div><p>Sales / Kunden / Kundendetails</p><h1>{customer.name || customer.email}</h1></div>
      </header>
      <section className="enterprise-customer">
        <h2>♟ Kunde</h2>
        <div>
          <article><small>Kunde</small><strong>{customer.name || '—'}</strong><a href={`mailto:${customer.email}`}>{customer.email}</a></article>
          <article><small>Unternehmen</small><strong>{customer.company || '—'}</strong><span>{customer.vat || 'Keine USt-IdNr.'}</span></article>
          <article><small>Rechnungsadresse</small><AddressBlock address={latest?.BillingAddress} /></article>
          <article><small>Lieferadresse</small><AddressBlock address={latest?.ShippingAddress} /></article>
          <article><small>Offen</small><strong>{euro(customer.outstandingCents)}</strong><span>{customer.orders.length} Bestellungen</span></article>
        </div>
      </section>
      <div className="billie-table-wrap" style={{ marginTop: 18 }}>
        <table className="billie-table">
          <thead><tr><th>Bestellung</th><th>Datum</th><th className="right">Betrag</th><th>Status</th></tr></thead>
          <tbody>
            {customer.orders.map(order => {
              const payment = paymentByOrder[order.ID]
              return (
                <tr key={order.ID} role="link" tabIndex="0" aria-label={`Bestellung ${order.ID} öffnen`} onClick={() => go(`/zahlungen/bestellung/${order.ID}`)} onKeyDown={event => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); go(`/zahlungen/bestellung/${order.ID}`) } }}>
                  <td><strong>{order.ID}</strong><div className="muted">{payment?.InvoiceNumber || payment?.Reference}</div></td>
                  <td>{dateOnly(order.CreatedAt)}</td>
                  <td className="right">{euro(order.TotalCents)}</td>
                  <td><StatusPill label={receivableLabel(order, payment)} /></td>
                </tr>
              )
            })}
          </tbody>
        </table>
      </div>
    </>
  )
}

function PaymentsList({ go, receipts, claims, orders, query, setQuery, filter, setFilter, newReceipt, setNewReceipt, createReceipt, busy, loading }) {
  const orderName = id => orders.find(item => item.ID === id)?.CustomerName || '—'
  return (
    <>
      <header className="billie-toolbar">
        <div><p>Sales / Zahlungen</p><h1>Zahlungen</h1></div>
        <input className="billie-search" aria-label="Zahlungen nach Kunde, Verwendungszweck oder Referenz suchen" value={query} onChange={e => setQuery(e.target.value)} placeholder="Kunde, Verwendungszweck oder Referenz suchen" />
      </header>
      <div className="billie-filters">{['all', 'Offen', 'Überfällig', 'Abgeschlossen'].map(value => <button key={value} className={filter === value ? 'active' : ''} onClick={() => setFilter(value)}>{value === 'all' ? 'Alle' : value}</button>)}</div>
      <section className="enterprise-card" style={{ marginBottom: 18 }}>
        <h2>Zahlungseingang erfassen</h2>
        <form className="billie-capture" onSubmit={createReceipt}>
          <label>Betrag in EUR<input required inputMode="decimal" value={newReceipt.amount} onChange={e => setNewReceipt({ ...newReceipt, amount: e.target.value })} placeholder="99,90" /></label>
          <label>Zahler<input required value={newReceipt.payerName} onChange={e => setNewReceipt({ ...newReceipt, payerName: e.target.value })} placeholder="Name laut Kontoauszug" /></label>
          <label>IBAN, letzte 4<input required inputMode="numeric" maxLength="4" value={newReceipt.ibanLast4} onChange={e => setNewReceipt({ ...newReceipt, ibanLast4: e.target.value.replace(/\D/g, '') })} placeholder="1234" /></label>
          <label>Verwendungszweck / Notiz<input value={newReceipt.note} onChange={e => setNewReceipt({ ...newReceipt, note: e.target.value })} placeholder="optional" /></label>
          <button disabled={busy === 'new'}>{busy === 'new' ? 'Wird erfasst …' : 'Erfassen'}</button>
        </form>
      </section>
      <h2 style={{ fontSize: 14, fontWeight: 400 }}>Bankeingänge</h2>
      <div className="billie-table-wrap" style={{ marginBottom: 18 }}>
        <table className="billie-table">
          <thead><tr><th>Referenz</th><th>Zahler</th><th>Verwendungszweck</th><th className="right">Betrag</th><th>Status</th><th>Datum</th></tr></thead>
          <tbody>
            {receipts.map(item => (
              <tr key={item.ID} role="link" tabIndex="0" aria-label={`Zahlungseingang ${item.Reference} öffnen`} onClick={() => go(`/zahlungen/eingang/${item.ID}`)} onKeyDown={event => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); go(`/zahlungen/eingang/${item.ID}`) } }}>
                <td><strong>{item.Reference}</strong></td>
                <td>{item.PayerName}<div className="muted">IBAN •••• {item.PayerIBANLast4}</div></td>
                <td>{item.Note || '—'}</td>
                <td className="right">{euro(item.AmountCents)}</td>
                <td><StatusPill label={receiptStatus(item)} /></td>
                <td>{dateTime(item.ReceivedAt)}</td>
              </tr>
            ))}
          </tbody>
        </table>
        {!receipts.length && <p className="billie-empty">{loading ? 'Zahlungseingänge werden geladen …' : query ? 'Keine Zahlungseingänge für diese Suche gefunden.' : 'Noch keine Zahlungseingänge erfasst.'}</p>}
      </div>
      <h2 style={{ fontSize: 14, fontWeight: 400 }}>Forderungen</h2>
      <div className="billie-table-wrap">
        <table className="billie-table">
          <thead><tr><th>Bestellung / Rechnung</th><th>Kunde</th><th>Verwendungszweck</th><th className="right">Offen</th><th>Status</th></tr></thead>
          <tbody>
            {claims.map(item => (
              <tr key={item.OrderID} role="link" tabIndex="0" aria-label={`Forderung ${item.OrderID} öffnen`} onClick={() => go(`/zahlungen/forderung/${item.OrderID}`)} onKeyDown={event => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); go(`/zahlungen/forderung/${item.OrderID}`) } }}>
                <td><strong>{item.InvoiceNumber || item.OrderID}</strong><div className="muted">{item.OrderID}</div></td>
                <td>{orderName(item.OrderID)}</td>
                <td>{item.Reference}</td>
                <td className="right">{euro(item.OutstandingCents)}</td>
                <td><StatusPill label={receivableLabel(orders.find(order => order.ID === item.OrderID), item)} /></td>
              </tr>
            ))}
          </tbody>
        </table>
        {!claims.length && <p className="billie-empty">{loading ? 'Forderungen werden geladen …' : query ? 'Keine Forderungen für diese Suche gefunden.' : 'Keine offenen Forderungen.'}</p>}
      </div>
    </>
  )
}

function ReceiptDetail({ go, receipt, invoices, rows, setRows, save, busy }) {
  const draftSum = rows.reduce((sum, row) => sum + (Number(row.amountCents) || 0), 0)
  return (
    <>
      <button className="billie-back" onClick={() => go('/zahlungen/eingang')}>← Zurück zu Zahlungen</button>
      <header className="billie-toolbar">
        <div><p>Sales / Zahlungen / Zahlungseingang</p><h1>{receipt.Reference}</h1></div>
        <StatusPill label={receiptStatus(receipt)} />
      </header>
      <div className="billie-cards">
        <section className="enterprise-card">
          <h2>Zahlung</h2>
          <dl className="billie-kv">
            <dt>Zahler</dt><dd>{receipt.PayerName}</dd>
            <dt>IBAN</dt><dd>•••• {receipt.PayerIBANLast4}</dd>
            <dt>Betrag</dt><dd>{euro(receipt.AmountCents)}</dd>
            <dt>Zugeordnet</dt><dd>{euro(receipt.AllocatedCents)}</dd>
            <dt>Frei</dt><dd>{euro(receipt.UnallocatedCents)}</dd>
            <dt>Eingegangen</dt><dd>{dateTime(receipt.ReceivedAt)}</dd>
            <dt>Erfasst von</dt><dd>{receipt.ReceivedBy || '—'}</dd>
            <dt>Notiz</dt><dd>{receipt.Note || '—'}</dd>
          </dl>
        </section>
        <section className="enterprise-card" style={{ gridColumn: 'span 2' }}>
          <h2>Auf Rechnungen buchen</h2>
          <div className="billie-bank">
            {rows.map((row, index) => (
              <div className="payment-allocation" key={`${receipt.ID}-${index}`}>
                <select value={row.orderId} onChange={e => setRows(rows.map((item, position) => position === index ? { ...item, orderId: e.target.value } : item))}>
                  <option value="">Rechnung wählen</option>
                  {invoices.map(invoice => <option key={invoice.OrderID} value={invoice.OrderID}>{invoice.InvoiceNumber || invoice.OrderID} · offen {euro(invoice.OutstandingCents)}</option>)}
                </select>
                <input inputMode="decimal" value={row.amountCents ? (row.amountCents / 100).toFixed(2).replace('.', ',') : ''} onChange={e => setRows(rows.map((item, position) => position === index ? { ...item, amountCents: Math.round(Number(e.target.value.replace(',', '.')) * 100) || 0 } : item))} placeholder="Betrag" />
                <button type="button" className="billie-action ghost" onClick={() => setRows(rows.filter((_, position) => position !== index))}>Entfernen</button>
              </div>
            ))}
            <button type="button" className="billie-action ghost" onClick={() => setRows([...rows, { orderId: '', amountCents: 0 }])}>+ Rechnung zuordnen</button>
            <p>Zuordnung: {euro(draftSum)} von {euro(receipt.AmountCents)}.</p>
            <button className="billie-action" disabled={busy === `receipt-${receipt.ID}` || draftSum > receipt.AmountCents} onClick={save}>{busy === `receipt-${receipt.ID}` ? 'Wird gespeichert …' : 'Zuordnung speichern'}</button>
          </div>
        </section>
      </div>
    </>
  )
}

function ClaimDetail({ go, claim, order, receipts }) {
  if (!claim) return <p className="billie-empty">Forderung nicht gefunden.</p>
  return (
    <>
      <button className="billie-back" onClick={() => go('/zahlungen/eingang')}>← Zurück zu Zahlungen</button>
      <header className="billie-toolbar">
        <div><p>Sales / Zahlungen / Forderung</p><h1>{claim.InvoiceNumber || claim.Reference}</h1></div>
        <StatusPill label={receivableLabel(order, claim)} />
      </header>
      <div className="billie-cards">
        <section className="enterprise-card">
          <h2>Forderung</h2>
          <dl className="billie-kv">
            <dt>Bestellung</dt><dd><button className="billie-back" onClick={() => go(`/zahlungen/bestellung/${claim.OrderID}`)}>{claim.OrderID}</button></dd>
            <dt>Kunde</dt><dd>{order?.CustomerName || claim.PayerName || '—'}</dd>
            <dt>Verwendungszweck</dt><dd>{claim.Reference}</dd>
            <dt>Betrag</dt><dd>{euro(claim.AmountCents)}</dd>
            <dt>Zugeordnet</dt><dd>{euro(claim.AppliedCents)}</dd>
            <dt>Offen</dt><dd>{euro(claim.OutstandingCents)}</dd>
            <dt>Fällig</dt><dd>{dateOnly(claim.DueAt)}</dd>
            <dt>Provider</dt><dd>{claim.Provider}</dd>
          </dl>
        </section>
        <section className="enterprise-card activity-card" style={{ gridColumn: 'span 2' }}>
          <h2>Zugeordnete Eingänge</h2>
          <table>
            <thead><tr><th>Referenz</th><th>Zahler</th><th>Betrag</th><th>Datum</th></tr></thead>
            <tbody>
              {receipts.map(item => {
                const allocated = (item.Allocations || []).filter(row => row.OrderID === claim.OrderID).reduce((sum, row) => sum + row.AmountCents, 0)
                return <tr key={item.ID} role="link" tabIndex="0" aria-label={`Zahlungseingang ${item.Reference} öffnen`} onClick={() => go(`/zahlungen/eingang/${item.ID}`)} onKeyDown={event => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); go(`/zahlungen/eingang/${item.ID}`) } }}><td>{item.Reference}</td><td>{item.PayerName}</td><td>{euro(allocated)}</td><td>{dateTime(item.ReceivedAt)}</td></tr>
              })}
              {!receipts.length && <tr><td colSpan="4">Noch keine Eingänge zugeordnet.</td></tr>}
            </tbody>
          </table>
        </section>
      </div>
    </>
  )
}
