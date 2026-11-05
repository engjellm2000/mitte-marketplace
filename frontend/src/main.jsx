import { useEffect, useState } from 'react'
import { createRoot } from 'react-dom/client'
import './styles.css'
import './enhancements.css'
import './oms.css'
import './enterprise.css'
import './storefront.css'
import './mitte.css'
import './marketing.css'
import { Payments } from './pages/payments/Payments.jsx'
import { InventoryConsole } from './pages/inventory/InventoryConsole.jsx'
import './operations.css'
import './warehouse-queue.css'
import './supplier-portal.css'
import { euro, csrf, api } from './lib/api.js'
import { dateTime } from './lib/labels.js'
import { Header } from './components/Header.jsx'
import { Footer } from './components/Footer.jsx'
import { CookieConsent } from './components/CookieConsent.jsx'
import { CartDrawer } from './components/CartDrawer.jsx'
import { ShoppingAssistant } from './components/ShoppingAssistant.jsx'
import { Home } from './pages/storefront/Home.jsx'
import { Shop } from './pages/storefront/Shop.jsx'
import { Product } from './pages/storefront/Product.jsx'
import { MarketingOffers } from './pages/storefront/MarketingOffers.jsx'
import { Saved } from './pages/storefront/Saved.jsx'
import { Cart } from './pages/storefront/Cart.jsx'
import { OrderSuccess } from './pages/storefront/OrderSuccess.jsx'
import { StaffLogin } from './pages/account/StaffLogin.jsx'
import { Account } from './pages/account/Account.jsx'
import { CatalogAdmin } from './pages/oms/CatalogAdmin.jsx'
import { MarketingAdmin } from './pages/oms/MarketingAdmin.jsx'
import { MarketingEmailAdmin } from './pages/oms/MarketingEmailAdmin.jsx'
import { EnterpriseOMS } from './pages/oms/EnterpriseOMS.jsx'
import { Operations } from './pages/oms/Operations.jsx'
import { WarehouseReturns } from './pages/oms/WarehouseReturns.jsx'
import { MittePay } from './pages/payments/MittePay.jsx'
import { MitteInvoice } from './pages/payments/MitteInvoice.jsx'
import { SupplierPortal } from './pages/supplier/SupplierPortal.jsx'
import { SupplierRequests } from './pages/oms/SupplierRequests.jsx'

function useRoute() {
  const [route, setRoute] = useState(location.pathname + location.search)
  useEffect(() => { const onPop = () => setRoute(location.pathname + location.search); addEventListener('popstate', onPop); return () => removeEventListener('popstate', onPop) }, [])
  const go = path => { history.pushState({}, '', path); setRoute(path); scrollTo({ top: 0, behavior: 'smooth' }) }
  return [route, go]
}

function App() {
  const [route, go] = useRoute()
  const [cart, setCart] = useState({ Items: [], TotalCents: 0 })
  const [user, setUser] = useState(null)
	const [staffUser, setStaffUser] = useState(null)
	const [supplierUser, setSupplierUser] = useState(null)
	const [supplierChecked, setSupplierChecked] = useState(false)
  const [saved, setSaved] = useState(() => {
    try { return JSON.parse(localStorage.getItem('mitte:saved') || '[]') } catch { return [] }
  })
  const [drawer, setDrawer] = useState(false)
  const [cookieSettingsOpen, setCookieSettingsOpen] = useState(false)
  const [notice, setNotice] = useState('')
  const isSupplierRoute = route.startsWith('/lieferanten')
  const count = cart.Items?.reduce((sum, item) => sum + item.Quantity, 0) || 0
  useEffect(() => { api('/api/cart').then(data => { if (data.cart) setCart(data.cart) }).catch(() => {}) }, [])
  useEffect(() => { api('/api/auth/me').then(data => setUser(data.user)).catch(() => {}) }, [])
	useEffect(() => { api('/api/staff/me').then(data => setStaffUser(data.staff)).catch(() => {}) }, [])
  useEffect(() => {
    if (!isSupplierRoute) return
    api('/api/supplier/me').then(data => setSupplierUser(data.supplier)).catch(() => {}).finally(() => setSupplierChecked(true))
  }, [isSupplierRoute])
  useEffect(() => { localStorage.setItem('mitte:saved', JSON.stringify(saved)) }, [saved])
  const add = async (product, quantity = 1) => {
    try { const data = await api('/api/cart/items', { method: 'POST', headers: { 'X-CSRF-Token': csrf }, body: JSON.stringify({ productId: product.ID, quantity }) }); setCart(data.cart); setDrawer(true); setNotice(`${product.Name} liegt jetzt in deinem Warenkorb.`) }
    catch (error) { setNotice(error.message) }
  }
  const updateQuantity = async (productId, quantity) => {
    try { const data = await api('/api/cart/quantity', { method: 'POST', headers: { 'X-CSRF-Token': csrf }, body: JSON.stringify({ productId, quantity }) }); setCart(data.cart); if (quantity === 0) setNotice('Artikel aus deinem Warenkorb entfernt.') }
    catch (error) { setNotice(error.message) }
  }
  const toggleSaved = product => {
    setSaved(current => current.some(item => item.ID === product.ID) ? current.filter(item => item.ID !== product.ID) : [...current, product])
    setNotice(saved.some(item => item.ID === product.ID) ? `${product.Name} wurde von deiner Merkliste entfernt.` : `${product.Name} ist auf deiner Merkliste.`)
  }
  const navigate = path => { setDrawer(false); go(path) }
  if (isSupplierRoute) return <SupplierPortal user={supplierUser} checked={supplierChecked} onAuthenticated={supplier => { setSupplierUser(supplier); setSupplierChecked(true) }} go={go} />
  const staffArea = route.startsWith('/oms') || route.startsWith('/bestand') || route.startsWith('/verwaltung') || route.startsWith('/admin/catalog') || route.startsWith('/admin/promotions') || route.startsWith('/admin/email-marketing') || route.startsWith('/zahlungen')
	if (staffArea && !staffUser) return <StaffLogin onAuthenticated={setStaffUser} />
  const staffPermission = route.startsWith('/oms/categories') || route.startsWith('/oms/pricing') || route.startsWith('/oms/queues/sellable') ? 'catalog' : route.startsWith('/oms/queues/reorder') || route.startsWith('/bestand') ? 'inventory' : route.startsWith('/oms') ? 'orders' : route.startsWith('/verwaltung') ? 'warehouse' : route.startsWith('/admin/catalog') || route.startsWith('/admin/promotions') || route.startsWith('/admin/email-marketing') ? 'catalog' : route.startsWith('/zahlungen') ? 'payments' : ''
  if (staffArea && staffUser && !staffUser.permissions?.includes(staffPermission)) {
    const fallback = [
      ['orders', '/oms'],
      ['warehouse', '/verwaltung'],
      ['inventory', '/bestand'],
      ['catalog', '/admin/catalog'],
      ['payments', '/zahlungen']
    ].find(([permission]) => staffUser.permissions?.includes(permission))
    return <main className="auth-page" role="alert"><h1>Kein Zugriff auf diesen Bereich.</h1><p>Dein Mitarbeitendenkonto ist für diesen Arbeitsbereich nicht freigeschaltet.</p>{fallback && <button className="coral-button" onClick={() => go(fallback[1])}>Freigegebenen Bereich öffnen</button>}</main>
  }
  if (route.startsWith('/oms')) return <EnterpriseOMS user={staffUser} go={go} route={route} />
  if (route.startsWith('/bestand')) return <InventoryConsole user={staffUser} go={go} api={api} csrf={csrf} euro={euro} dateTime={dateTime} />
  if (route === '/verwaltung/retouren') return <WarehouseReturns user={staffUser} go={go} />
  if (route.startsWith('/verwaltung')) return <Operations user={staffUser} go={go} />
  if (route.startsWith('/admin/catalog')) return <CatalogAdmin user={staffUser} go={go} />
  if (route.startsWith('/admin/promotions')) return <MarketingAdmin user={staffUser} go={go} />
  if (route.startsWith('/admin/email-marketing')) return <MarketingEmailAdmin user={staffUser} go={go} />
  if (route.startsWith('/zahlungen')) return <Payments user={staffUser} go={go} route={route} />
  return <div className="storefront">
    <Header count={count} savedCount={saved.length} route={route} go={navigate} onCart={() => setDrawer(true)} user={user} />
    <main>{route.startsWith('/zahlung/mitte') ? <MittePay route={route} /> : route.startsWith('/zahlung/rechnung') ? <MitteInvoice route={route} /> : route.startsWith('/bestellung/erfolg') ? <OrderSuccess route={route} go={go} /> : route.startsWith('/konto') ? <Account user={user} setUser={setUser} /> : route.startsWith('/merkliste') ? <Saved products={saved} go={go} add={add} toggleSaved={toggleSaved} /> : route.startsWith('/produkt/') ? <Product slug={route.split('/')[2]} go={go} add={add} saved={saved} toggleSaved={toggleSaved} user={user} /> : route.startsWith('/warenkorb') ? <Cart cart={cart} setCart={setCart} updateQuantity={updateQuantity} user={user} go={navigate} /> : route.startsWith('/shop') ? <Shop go={go} add={add} route={route} saved={saved} toggleSaved={toggleSaved} /> : route.startsWith('/angebote') ? <MarketingOffers slug={decodeURIComponent(route.split('/')[2] || '')} go={navigate} /> : <Home go={go} add={add} saved={saved} toggleSaved={toggleSaved} user={user} />}</main>
    <Footer go={go} onCookieSettings={() => setCookieSettingsOpen(true)} />
    <CookieConsent
      forceOpen={cookieSettingsOpen}
      onClose={() => setCookieSettingsOpen(false)}
      go={navigate}
    />
    <CartDrawer open={drawer} cart={cart} go={navigate} close={() => setDrawer(false)} updateQuantity={updateQuantity} />
    <ShoppingAssistant go={navigate} add={add} />
    {notice && <div className="toast" role="status" onAnimationEnd={() => setNotice('')}>{notice}</div>}
  </div>
}


createRoot(document.getElementById('root')).render(<App />)
