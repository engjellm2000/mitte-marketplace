import React, { useEffect, useState } from 'react'
import '../../inventory.css'
import { OperationsSidebar } from '../../components/operations/OperationsSidebar.jsx'

export function InventoryConsole({ user, go, api, csrf, euro, dateTime, onSwitchAccount }) {
  const [data, setData] = useState(null)
  const [movements, setMovements] = useState([])
  const [loading, setLoading] = useState(true)
  const [selectedId, setSelectedId] = useState(null)
  const [search, setSearch] = useState('')
  const [stock, setStock] = useState('')
  const [page, setPage] = useState(1)
  const [receipt, setReceipt] = useState({ quantity: '', reason: 'Wareneingang', reference: '' })
  const [adjustment, setAdjustment] = useState({ delta: '', reason: '', reference: '' })
  const [notice, setNotice] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState('')

  const refresh = async () => {
    setLoading(true)
    try {
      const result = await api(`/api/admin/inventory?search=${encodeURIComponent(search)}&stock=${encodeURIComponent(stock)}&page=${page}`, { cache: 'no-store' })
      setData(result)
      setSelectedId(current => result.products.some(product => product.ID === current) ? current : result.products[0]?.ID || null)
      setError('')
    } catch (cause) {
      setError(cause.message)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { refresh() }, [search, stock, page])
  useEffect(() => {
    if (!selectedId) { setMovements([]); return }
    api(`/api/admin/inventory/movements?productId=${selectedId}`, { cache: 'no-store' })
      .then(result => setMovements(result.movements || []))
      .catch(cause => setError(cause.message))
  }, [selectedId])

  const selected = data?.products?.find(product => product.ID === selectedId)
  const submitMovement = async (event, type) => {
    event.preventDefault()
    if (!selected) return
    const form = type === 'receipt' ? receipt : adjustment
    setBusy(type); setError(''); setNotice('')
    try {
      if (type === 'receipt') {
        await api('/api/admin/inventory/receive', {
          method: 'POST', headers: { 'X-CSRF-Token': csrf },
          body: JSON.stringify({ productId: selected.ID, quantity: Number(form.quantity), reason: form.reason, reference: form.reference })
        })
        setReceipt({ quantity: '', reason: 'Wareneingang', reference: '' })
        setNotice(`${Number(form.quantity)} Stück für ${selected.Name} eingebucht.`)
      } else {
        await api(`/api/admin/catalog/products/${selected.ID}/inventory`, {
          method: 'POST', headers: { 'X-CSRF-Token': csrf },
          body: JSON.stringify({ delta: Number(form.delta), reason: form.reason, reference: form.reference })
        })
        setAdjustment({ delta: '', reason: '', reference: '' })
        setNotice(`Bestandskorrektur für ${selected.Name} gebucht.`)
      }
      await refresh()
      const result = await api(`/api/admin/inventory/movements?productId=${selected.ID}`, { cache: 'no-store' })
      setMovements(result.movements || [])
    } catch (cause) {
      setError(cause.message)
    } finally {
      setBusy('')
    }
  }

  if (error && !data) {
    const denied = /unauthori[sz]ed|forbidden|permission|access denied|staff.{0,16}(access|permission)|zugriff|403|401/i.test(error)
    return <section className="oms-access"><div><b>M</b><span>Mitte INVENTORY</span></div><h1>{denied && user ? 'Zugriff nicht freigeschaltet.' : 'Zugriff erforderlich.'}</h1><p>{denied && user ? `Angemeldet als Konto ${user.ID}. Wenn dies Ihr verifiziertes Mitarbeiterkonto ist, setzen Sie STAFF_USER_IDS=${user.ID} in der lokalen .env (oder im Secret Manager) und starten Sie den Server neu. Eine E-Mail-Adresse allein erteilt keinen Zugriff.` : error}</p>{denied && user && onSwitchAccount ? <button onClick={onSwitchAccount}>Zur Anmeldung wechseln</button> : <button onClick={() => go('/konto')}>Anmelden</button>}{denied && user && <button className="text-button" onClick={() => go('/')}>Zum Shop</button>}</section>
  }

  const summary = data?.summary
  return <div className="operations-shell inventory-shell">
    <OperationsSidebar active="inventory" user={user} go={go}/>
    <main className="inventory-main operations-main">
      <header className="inventory-heading">
        <div><p className="oms-eyebrow">LAGERBESTAND · EIN LAGER</p><h1>Verfügbarkeit im Blick.</h1><p>Wareneingänge, Reservierungen und Korrekturen laufen in einer nachvollziehbaren Bestandschronik zusammen.</p></div>
        <div className="inventory-heading-actions"><button onClick={() => go('/admin/catalog')}>Produkt anlegen</button><button onClick={refresh}>↻ Aktualisieren</button></div>
      </header>

      {(error || notice) && <div className={`inventory-message ${error ? 'error' : 'success'}`} role={error ? 'alert' : 'status'}>{error || notice}<button onClick={() => { setError(''); setNotice('') }}>×</button></div>}

      <section className="inventory-kpis" aria-label="Bestandsübersicht">
        <article><span>Produkte geführt</span><strong>{summary?.productCount ?? '—'}</strong></article>
        <article><span>Physisch auf Lager</span><strong>{summary?.onHandUnits ?? '—'}</strong></article>
        <article><span>Davon reserviert</span><strong>{summary?.reservedUnits ?? '—'}</strong></article>
        <article><span>Verfügbar</span><strong>{summary?.availableUnits ?? '—'}</strong></article>
        <article className={summary?.lowStockProducts ? 'warning' : ''}><span>Niedrig / ausverkauft</span><strong>{summary ? `${summary.lowStockProducts} / ${summary.outOfStockProducts}` : '—'}</strong></article>
        <article><span>Lagerwert zum Einstand</span><strong>{summary ? euro(summary.stockValueCents) : '—'}</strong></article>
      </section>

      <section className="inventory-layout">
        <div className="inventory-list">
          <div className="inventory-list-head"><div><h2>Artikelbestand</h2><span>{data?.total ?? 0} Artikel · Seite {page} von {Math.max(data?.pages || 1, 1)}</span></div>
            <div className="inventory-tools">
              <input type="search" value={search} onChange={event => { setPage(1); setSearch(event.target.value) }} placeholder="Name, SKU oder Barcode" aria-label="Bestand durchsuchen"/>
              <select value={stock} onChange={event => { setPage(1); setStock(event.target.value) }} aria-label="Bestand filtern"><option value="">Alle Artikel</option><option value="low">Nachbestellen</option><option value="out">Ausverkauft</option><option value="available">Ausreichend verfügbar</option></select>
            </div>
          </div>
          <div className="inventory-table-scroll"><table className="inventory-table">
            <thead><tr><th>Artikel / SKU</th><th>Status</th><th>Verfügbar</th><th>Reserviert</th><th>Nachbestellen ab</th><th>Lieferant</th></tr></thead>
            <tbody>{data?.products?.map(product => {
              const low = product.Inventory === 0 || (product.reorderPoint > 0 && product.Inventory <= product.reorderPoint)
              return <tr key={product.ID} className={selectedId === product.ID ? 'selected' : ''} onClick={() => setSelectedId(product.ID)} tabIndex="0" onKeyDown={event => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); setSelectedId(product.ID) } }}>
                <td><strong>{product.Name}</strong><small>{product.SKU}{product.barcode ? ` · ${product.barcode}` : ''}</small></td>
                <td><span className={`inventory-state ${product.Inventory === 0 ? 'out' : low ? 'low' : 'ok'}`}>{product.Inventory === 0 ? 'Ausverkauft' : low ? 'Nachbestellen' : 'Verfügbar'}</span></td>
                <td><b>{product.Inventory}</b><small>von {product.onHand} physisch</small></td>
                <td>{product.reservedStock}</td><td>{product.reorderPoint || '—'}</td><td>{product.supplierName || '—'}</td>
              </tr>
            })}</tbody>
          </table></div>
          {!data?.products?.length && <p className="inventory-empty">{loading ? 'Bestand wird geladen …' : error ? 'Bestand konnte nicht geladen werden.' : 'Keine passenden Artikel. Filter anpassen oder ein Produkt anlegen.'}</p>}
          <footer className="inventory-pagination"><button disabled={page <= 1} onClick={() => setPage(value => value - 1)}>← Zurück</button><span>{data?.total ?? 0} Treffer</span><button disabled={page >= (data?.pages || 1)} onClick={() => setPage(value => value + 1)}>Weiter →</button></footer>
        </div>

        <aside className="inventory-detail">
          {!selected ? <div className="inventory-empty"><h2>Artikel auswählen</h2><p>Wähle einen Artikel aus, um Bestand einzubuchen oder die letzten Bewegungen zu prüfen.</p></div> : <>
            <header><p className="oms-eyebrow">ARTIKELDETAIL</p><h2>{selected.Name}</h2><span>{selected.SKU} · {selected.Category?.Name}</span></header>
            <dl className="inventory-facts">
              <div><dt>Auf Lager</dt><dd>{selected.onHand}</dd></div><div><dt>Reserviert</dt><dd>{selected.reservedStock}</dd></div>
              <div><dt>Verfügbar</dt><dd>{selected.Inventory}</dd></div><div><dt>Nachbestellpunkt</dt><dd>{selected.reorderPoint} Stück</dd></div>
              <div><dt>Einstandspreis</dt><dd>{euro(selected.unitCostCents)}</dd></div><div><dt>Lieferzeit</dt><dd>{selected.leadTimeDays ? `${selected.leadTimeDays} Tage` : 'Nicht hinterlegt'}</dd></div>
              <div><dt>Lieferant</dt><dd>{selected.supplierName || 'Nicht hinterlegt'}</dd></div><div><dt>Bestandswert</dt><dd>{euro(selected.unitCostCents * selected.onHand)}</dd></div>
            </dl>
            <form className="inventory-action-form" onSubmit={event => submitMovement(event, 'receipt')}>
              <h3>Wareneingang buchen</h3><p>Erhöht den verfügbaren Bestand und protokolliert Menge, Grund und Referenz.</p>
              <label>Menge<input required type="number" min="1" max="1000000" value={receipt.quantity} onChange={event => setReceipt({ ...receipt, quantity: event.target.value })}/></label>
              <label>Grund<input required minLength="3" maxLength="240" value={receipt.reason} onChange={event => setReceipt({ ...receipt, reason: event.target.value })}/></label>
              <label>Lieferschein / Referenz <span>optional</span><input maxLength="120" value={receipt.reference} onChange={event => setReceipt({ ...receipt, reference: event.target.value })}/></label>
              <button disabled={Boolean(busy) || !receipt.quantity}>{busy === 'receipt' ? 'Wird gebucht …' : 'Wareneingang einbuchen'}</button>
            </form>
            <form className="inventory-action-form adjustment" onSubmit={event => submitMovement(event, 'adjustment')}>
              <h3>Inventurkorrektur</h3><p>Abweichung mit Vorzeichen erfassen. Der Bestand kann nicht unter null fallen.</p>
              <label>Änderung<input required type="number" min="-1000000" max="1000000" value={adjustment.delta} onChange={event => setAdjustment({ ...adjustment, delta: event.target.value })} placeholder="z. B. −2 oder +5"/></label>
              <label>Grund<input required minLength="3" maxLength="240" value={adjustment.reason} onChange={event => setAdjustment({ ...adjustment, reason: event.target.value })}/></label>
              <label>Inventur / Referenz <span>optional</span><input maxLength="120" value={adjustment.reference} onChange={event => setAdjustment({ ...adjustment, reference: event.target.value })}/></label>
              <button disabled={Boolean(busy) || !adjustment.delta}>{busy === 'adjustment' ? 'Wird gebucht …' : 'Korrektur speichern'}</button>
            </form>
            <section className="inventory-ledger"><header><h3>Bestandschronik</h3><span>Neueste 50</span></header>
              {!movements.length ? <p className="inventory-empty">Noch keine Bewegungen protokolliert.</p> : movements.map(movement => <article key={movement.id}>
                <div><strong>{movement.type === 'opening_balance' ? 'Anfangsbestand' : movement.type === 'receipt' ? 'Wareneingang' : movement.type === 'return_restock' ? 'Rücksendung eingelagert' : movement.type === 'adjustment' ? 'Inventurkorrektur' : movement.type === 'reservation' ? 'Reserviert' : 'Reservierung freigegeben'}</strong><small>{dateTime(movement.createdAt)} · {movement.actorEmail}</small></div>
                <b className={movement.quantityDelta < 0 ? 'negative' : 'positive'}>{movement.quantityDelta > 0 ? '+' : ''}{movement.quantityDelta}</b>
                <p>{movement.reason}{movement.reference ? ` · ${movement.reference}` : ''}</p>
                <small>{movement.inventoryBefore} → {movement.inventoryAfter} verfügbar</small>
              </article>)}
            </section>
          </>}
        </aside>
      </section>
    </main>
  </div>
}
