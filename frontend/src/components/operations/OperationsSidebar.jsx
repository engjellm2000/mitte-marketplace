import React from 'react'

const destinations = [
  { id: 'orders', permission: 'orders', label: 'Bestellungen', path: '/oms', mark: '▤' },
  { id: 'fulfillment', permission: 'warehouse', label: 'Versand', path: '/verwaltung', mark: '↗' },
  { id: 'returns', permission: 'warehouse', label: 'Retoureneingang', path: '/verwaltung/retouren', mark: '↩' },
  { id: 'inventory', permission: 'inventory', label: 'Lagerbestand', path: '/bestand', mark: '▦' },
  { id: 'catalog', permission: 'catalog', label: 'Produkte', path: '/admin/catalog', mark: '◇' },
  { id: 'offers', permission: 'catalog', label: 'Angebote & Marketing', path: '/admin/promotions', mark: '↗' },
  { id: 'email', permission: 'catalog', label: 'E-Mail-Kampagnen', path: '/admin/email-marketing', mark: '✉' },
  { id: 'payments', permission: 'payments', label: 'Zahlungen', path: '/zahlungen', mark: '€' }
]

export function OperationsSidebar({ active, user, go }) {
  return (
    <aside className="operations-sidebar">
      <button className="operations-brand" onClick={() => go('/')} aria-label="Mitte-Shop öffnen">
        <b>M</b><span>Mitte<small>BETRIEB &amp; SERVICE</small></span>
      </button>
      <div className="operations-nav-label">ARBEITSBEREICHE</div>
      <nav aria-label="Betriebsbereiche">
        {destinations.filter(item => user?.permissions?.includes(item.permission)).map(item => (
          <button key={item.id} className={active === item.id ? 'active' : ''} aria-current={active === item.id ? 'page' : undefined} onClick={() => go(item.path)}>
            <span className="operations-nav-mark" aria-hidden="true">{item.mark}</span>{item.label}
          </button>
        ))}
      </nav>
      <div className="operations-identity">
        <span className="operations-avatar">{(user?.username || user?.Email || 'M').slice(0, 1).toUpperCase()}</span>
        <span>{user?.username || user?.Email || 'Mitarbeitende:r'}<small>Interner Zugang</small></span>
      </div>
    </aside>
  )
}
