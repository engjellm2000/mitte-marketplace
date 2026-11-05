import { useEffect, useRef } from 'react'
import { euro } from '../lib/api.js'

export function CartDrawer({ open, cart, close, go, updateQuantity }) {
  const dialogRef = useRef(null)
  const closeButtonRef = useRef(null)
  const closeRef = useRef(close)
  closeRef.current = close
  const quote = cart.Quote || {}

  useEffect(() => {
    if (!open) return undefined

    const previousFocus = document.activeElement
    const previousOverflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    const focusTimer = setTimeout(() => closeButtonRef.current?.focus(), 0)

    const onKeyDown = event => {
      if (event.key === 'Escape') {
        event.preventDefault()
        closeRef.current()
        return
      }
      if (event.key !== 'Tab') return

      const items = [...dialogRef.current.querySelectorAll('button:not(:disabled), a[href], input:not(:disabled), select:not(:disabled), textarea:not(:disabled), [tabindex]:not([tabindex="-1"])')]
      if (!items.length) {
        event.preventDefault()
        dialogRef.current.focus()
      } else if (!dialogRef.current.contains(document.activeElement)) {
        event.preventDefault()
        items[0].focus()
      } else if (event.shiftKey && document.activeElement === items[0]) {
        event.preventDefault()
        items[items.length - 1].focus()
      } else if (!event.shiftKey && document.activeElement === items[items.length - 1]) {
        event.preventDefault()
        items[0].focus()
      }
    }

    document.addEventListener('keydown', onKeyDown)
    return () => {
      clearTimeout(focusTimer)
      document.body.style.overflow = previousOverflow
      document.removeEventListener('keydown', onKeyDown)
      if (previousFocus instanceof HTMLElement && previousFocus.isConnected) previousFocus.focus()
    }
  }, [open])

  return <aside className={`drawer ${open ? 'open' : ''}`} aria-hidden={!open}>
    <button className="scrim" onClick={close} aria-label="Warenkorb schließen"/>
    <div
      className="drawer-content"
      ref={dialogRef}
      role="dialog"
      aria-modal={open ? 'true' : undefined}
      aria-labelledby="cart-drawer-title"
      tabIndex={-1}
    >
      <button className="close" ref={closeButtonRef} onClick={close} aria-label="Warenkorb schließen">×</button>
      <p className="kicker">Deine Auswahl</p>
      <h2 id="cart-drawer-title">Warenkorb</h2>
      {cart.Items?.length ? <>
        <div className="drawer-items">{cart.Items.map(item => <div key={item.Product.ID}>
          <img src={item.Product.ImageURL} alt=""/>
          <span>{item.Product.Name}<small>{item.Quantity} × {euro(item.Product.PriceCents)}</small>
            <button className="drawer-remove" onClick={() => updateQuantity(item.Product.ID, 0)}>Entfernen</button>
          </span>
        </div>)}</div>
        <div className="drawer-total"><span>Gesamt</span><strong>{euro(quote.TotalCents ?? cart.TotalCents)}</strong></div>
        <button className="dark-button wide" onClick={() => go('/warenkorb')}>Zur Kasse <span>→</span></button>
      </> : <div className="drawer-empty">
        <p>Dein Warenkorb ist noch leer.</p>
        <button className="text-button" onClick={() => go('/shop')}>Objekte entdecken <span>↗</span></button>
      </div>}
    </div>
  </aside>
}
