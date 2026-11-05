const modules = [
  {
    label: 'Bestellungen',
    href: '/oms',
    permission: 'orders',
    items: [
      { label: 'Auftragsübersicht', href: '/oms' },
      { label: 'Rückstand', href: '/oms/backlog' },
      { label: 'Alle Aufträge', href: '/oms/orders' },
      { label: 'Auftragspositionen', href: '/oms/order-items', aliases: ['/oms/matrix/order-items'] },
      { label: 'Kund:innen', href: '/oms/customers' },
      { label: 'Zahlungen klären', href: '/oms/matrix/payment-clarify' },
      { label: 'Zahlungsübersicht', href: '/oms/matrix/payment' }
    ]
  },
  {
    label: 'Versand & Fulfillment',
    href: '/oms/matrix/fulfillment',
    permission: 'orders',
    items: [
      { label: 'Fulfillment-Queue', href: '/oms/matrix/fulfillment' },
      { label: 'Fulfillment klären', href: '/oms/matrix/fulfillment-clarify' },
      { label: 'Abgeschlossene Aufträge', href: '/oms/matrix/old-orders' },
      { label: 'Abschlussprüfung', href: '/oms/matrix/old-ops-clarify' }
    ]
  },
  {
    label: 'Service',
    href: '/oms/processes',
    permission: 'orders',
    items: [
      { label: 'Prozesscenter', href: '/oms/processes' },
      { label: 'Versandverzögerungen (CSV)', href: '/oms/processes/bulk-delays' },
      { label: 'Verzögerungsprotokoll', href: '/oms/processes/shipment-delays' },
      { label: 'Retouren & Ersatz', href: '/oms/processes/requests' },
      { label: 'Lieferantenanfragen', href: '/oms/supplier-requests' },
      { label: 'E-Mail-Zustellung', href: '/oms/queues/email-deliveries' }
    ]
  },
  {
    label: 'Katalog',
    href: '/oms/categories',
    permission: 'catalog',
    items: [
      { label: 'Kategorien', href: '/oms/categories' },
      { label: 'Verkaufspreise', href: '/oms/pricing' },
      { label: 'Verkaufbarkeit', href: '/oms/queues/sellable' },
      { label: 'Produkte', href: '/admin/catalog', permission: 'catalog' }
    ]
  },
  {
    label: 'Bestand',
    href: '/oms/queues/reorder',
    permission: 'inventory',
    items: [
      { label: 'Nachbestellungen', href: '/oms/queues/reorder' },
      { label: 'Bestandsverwaltung', href: '/bestand', permission: 'inventory' }
    ]
  }
]

function matchesPath(path, item) {
  if (item.aliases?.includes(path)) return true
  if (path === item.href) return true
  return ['/oms/orders', '/oms/pricing', '/oms/queues/sellable', '/oms/queues/reorder', '/admin/catalog', '/bestand']
    .some(prefix => item.href === prefix && path.startsWith(`${prefix}/`))
}

export function OMSNavigation({ user, go }) {
  const currentPath = window.location.pathname
  const permissions = user?.permissions || []
  const visibleModules = modules
    .filter(module => permissions.includes(module.permission))
    .map(module => ({
      ...module,
      items: module.items.filter(item => !item.permission || permissions.includes(item.permission))
    }))
  const activeModule = visibleModules.find(module => module.items.some(item => matchesPath(currentPath, item)))
    || visibleModules[0]

  return (
    <header className="oms-navigation">
      <div className="oms-nav-topline">
        <button className="oms-brand" onClick={() => go('/oms')} aria-label="MITTE Operations home">
          <span className="oms-brand-mark">M</span>
          <span><strong>MITTE</strong><small>OPERATIONS</small></span>
        </button>
        <nav className="oms-module-nav" aria-label="Operations modules">
          {visibleModules.map(module => (
            <button
              className="oms-module-link"
              key={module.label}
              onClick={() => go(module.href)}
              aria-current={activeModule?.label === module.label ? 'page' : undefined}
            >
              {module.label}
            </button>
          ))}
        </nav>
        <div className="oms-nav-identity">
          <span className="oms-nav-status">Staff workspace</span>
          <span className="oms-user-name">{user?.FullName || user?.Email || 'Signed-in staff'}</span>
        </div>
      </div>
      {activeModule && (
        <nav className="oms-subnav" aria-label={`${activeModule.label} workspace`}>
          {activeModule.items.map(item => (
            <button
              className="oms-nav-link"
              key={item.href}
              onClick={() => go(item.href)}
              aria-current={matchesPath(currentPath, item) ? 'page' : undefined}
            >
              {item.label}
            </button>
          ))}
        </nav>
      )}
    </header>
  )
}
