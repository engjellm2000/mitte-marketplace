export const dateTime = value => value ? new Intl.DateTimeFormat('de-DE', { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(value)) : '—'
export const paymentLabel = status => status === 'paid' || status === 'fulfilled' ? 'Bezahlt' : status === 'cancelled' ? 'Storniert' : 'Zahlung wird geprüft'
export const deliveryLabel = status => status === 'fulfilled' ? 'Versendet / erfüllt' : status === 'paid' ? 'In Vorbereitung' : status === 'cancelled' ? 'Storniert' : 'Warten auf Zahlung'
export const addressText = address => [address?.RecipientName, address?.CompanyName, address?.AddressLine1, address?.AddressLine2, [address?.PostalCode, address?.City].filter(Boolean).join(' '), address?.CountryCode].filter(Boolean)

export const itemStatusLabels = {
  ready_to_process: 'Bereit zur Bearbeitung',
  stock_confirmed: 'Bestand bestätigt',
  payment_received: 'Zahlung eingegangen',
  exported: 'Exportiert',
  fulfillment: 'In Fulfillment',
  fulfillment_completed: 'Erfüllt',
  return_open: 'Rückgabe angefragt',
  replacement_requested: 'Ersatz angefragt',
  supplier_order_accepted: 'Lieferant hat Auftrag bestätigt',
  supplier_delay_reported: 'Lieferant meldet Verzögerung',
  supplier_shipment_reported: 'Lieferant hat Versand gemeldet',
  supplier_cancellation_requested: 'Lieferant bittet um Stornierung',
  supplier_cancellation_handled: 'Stornierungsanfrage bearbeitet',
  supplier_cancellation_rejected: 'Stornierungsanfrage abgelehnt'
}
export const deliveryDateLabel = value => value ? new Intl.DateTimeFormat('de-DE', { dateStyle: 'medium' }).format(new Date(`${value}T12:00:00`)) : 'Noch nicht zugesagt'
export const addressLines = value => [
  value?.recipientName || value?.RecipientName, value?.companyName || value?.CompanyName,
  value?.addressLine1 || value?.AddressLine1, value?.addressLine2 || value?.AddressLine2,
  [value?.postalCode || value?.PostalCode, value?.city || value?.City].filter(Boolean).join(' '),
  value?.countryCode || value?.CountryCode
].filter(Boolean)


export const positionLabels = { ready_to_process: 'Bereit zur Bearbeitung', stock_confirmed: 'Bestand bestätigt', payment_received: 'Zahlung eingegangen', exported: 'Exportiert', fulfillment: 'In Fulfillment', return_open: 'Rückgabe offen', fulfillment_completed: 'Erfüllt', cancelled: 'Storniert', refunded: 'Erstattet', replacement_created: 'Ersatzauftrag erstellt' }

export const fulfillmentSteps = ['ready_to_pick', 'picking', 'packed', 'label_ready', 'handed_over']
