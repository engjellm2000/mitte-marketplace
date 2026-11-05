import { euro, api, isAuthorizationError } from '../../lib/api.js'
import { dateTime } from '../../lib/labels.js'
import { OMSDashboard } from './OMSDashboard.jsx'
import { OMSBacklog } from './OMSBacklog.jsx'
import { OMSOrderWorkspace } from './OMSOrderWorkspace.jsx'
import { OMSMatrix, OMSOverview } from './OMSWorkspace.jsx'
import { OMSProcesses } from './OMSProcesses.jsx'
import { OMSCustomers } from './OMSCustomers.jsx'
import { OMSCategoryTree } from './OMSCategoryTree.jsx'
import { OMSPriceWorkspace } from './OMSPriceWorkspace.jsx'
import { OMSReorderQueue } from './OMSReorderQueue.jsx'
import { OMSEmailDeliveryQueue } from './OMSEmailDeliveryQueue.jsx'
import { OMSOrderItems } from './OMSOrderItems.jsx'
import { OMSItemRequests } from './OMSItemRequests.jsx'
import { OMSShipmentDelays } from './OMSShipmentDelays.jsx'
import { SupplierRequests } from './SupplierRequests.jsx'

export function EnterpriseOMS({ user, go, route }) {
  const match = route.match(/^\/oms\/orders\/(.+)$/)
  if (match) {
    return <OMSOrderWorkspace user={user} go={go} id={decodeURIComponent(match[1])} />
  }

  const matrix = route.match(/^\/oms\/matrix\/([^/]+)$/)
  if (matrix) {
    if (matrix[1] === 'order-items') return <OMSOrderItems user={user} go={go} />
    return <OMSMatrix user={user} go={go} api={api} isAuthorizationError={isAuthorizationError} kind={matrix[1]} />
  }
  if (route === '/oms/order-items') {
    return <OMSOrderItems user={user} go={go} />
  }
  if (route === '/oms/processes') {
    return <OMSProcesses user={user} go={go} />
  }
  if (route === '/oms/processes/bulk-delays') {
    return <OMSProcesses user={user} go={go} />
  }
  if (route === '/oms/processes/requests') {
    return <OMSItemRequests user={user} go={go} />
  }
  if (route === '/oms/processes/shipment-delays') {
    return <OMSShipmentDelays user={user} go={go} />
  }
  if (route === '/oms/supplier-requests') {
    return <SupplierRequests user={user} go={go} />
  }
  if (route === '/oms/customers') {
    return <OMSCustomers user={user} go={go} />
  }
  if (route === '/oms/categories') {
    return <OMSCategoryTree user={user} go={go} api={api} isAuthorizationError={isAuthorizationError} />
  }
  if (route === '/oms/pricing') {
    return <OMSPriceWorkspace user={user} go={go} api={api} euro={euro} isAuthorizationError={isAuthorizationError} />
  }
  if (route === '/oms/queues/sellable') {
    return <OMSPriceWorkspace user={user} go={go} api={api} euro={euro} isAuthorizationError={isAuthorizationError} sellableOnly />
  }
  if (route === '/oms/queues/reorder') {
    return <OMSReorderQueue user={user} go={go} api={api} euro={euro} isAuthorizationError={isAuthorizationError} />
  }
  if (route === '/oms/queues/email-deliveries') {
    return <OMSEmailDeliveryQueue user={user} go={go} />
  }
  if (route === '/oms/orders') {
    return <OMSDashboard user={user} go={go} api={api} euro={euro} dateTime={dateTime} isAuthorizationError={isAuthorizationError} />
  }
  if (route === '/oms/backlog') {
    return <OMSBacklog user={user} go={go} api={api} euro={euro} dateTime={dateTime} isAuthorizationError={isAuthorizationError} />
  }

  return <OMSOverview user={user} go={go} api={api} euro={euro} isAuthorizationError={isAuthorizationError} />
}
