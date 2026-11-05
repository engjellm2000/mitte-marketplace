import { useState, useEffect } from 'react'
import { euro, api } from '../../lib/api.js'

export function OrderSuccess({ route, go }) {
  const sessionID = new URLSearchParams(route.split('?')[1]).get('session_id')
  const [order, setOrder] = useState(null); const [error, setError] = useState('')
  useEffect(() => {
    if (!sessionID) { setError('Die Bestellreferenz fehlt.'); return }
    let active = true; let timer
    const refresh = async () => {
      try {
        const data = await api(`/api/orders/status?session_id=${encodeURIComponent(sessionID)}`)
        if (!active) return
        setOrder(data)
        if (data.status === 'pending_payment') timer = setTimeout(refresh, 1800)
      } catch (cause) { if (active) setError(cause.message) }
    }
    refresh()
    return () => { active = false; clearTimeout(timer) }
  }, [sessionID])
  const status = order?.status
  const state = status === 'paid' ? 'paid' : status === 'cancelled' ? 'cancelled' : 'pending'
  const heading = state === 'paid' ? 'Zahlung bestätigt.' : state === 'cancelled' ? 'Bestellung storniert.' : 'Zahlung wird bestätigt …'
  const copy = state === 'paid' ? 'Deine Bestellung ist bei uns eingegangen. Wir bereiten alles Weitere mit Sorgfalt vor.' : state === 'cancelled' ? 'Die Zahlung wurde nicht abgeschlossen. Deine Auswahl wartet weiterhin im Shop auf dich.' : 'Stripe meldet die Zahlung gerade an unseren Shop. Das dauert normalerweise nur einen Moment.'
  return <section className="order-success"><div className={`order-orb ${state}`} aria-hidden="true">{state === 'paid' ? '✓' : state === 'cancelled' ? '×' : '·'}</div><p className="kicker">MITTE / BESTELLUNG</p><h1>{heading}</h1><p className="order-lead">{error || copy}</p>{order && <div className="order-receipt"><span>Bestellnummer</span><strong>{order.orderId}</strong><span>Gesamt</span><strong>{euro(order.totalCents)}</strong><span>Status</span><strong className={`status-${state}`}>{state === 'paid' ? 'Bezahlt' : state === 'cancelled' ? 'Storniert' : 'Wird geprüft'}</strong></div>}<div className="order-actions"><button className="dark-button" onClick={() => go('/shop')}>Weiter entdecken <span>→</span></button>{state !== 'paid' && <button className="text-button" onClick={() => go('/warenkorb')}>Zurück zum Warenkorb <span>↗</span></button>}</div><small>Eine Bestätigung folgt, sobald die Zahlung vollständig verarbeitet ist.</small></section>
}
