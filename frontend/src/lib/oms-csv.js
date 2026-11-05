const MAX_ROWS = 200
const requiredHeaders = ['order_id', 'product_name', 'delayed_delivery_date']

function parseCSV(text) {
  const rows = []
  let row = []
  let value = ''
  let quoted = false

  for (let index = 0; index < text.length; index += 1) {
    const character = text[index]
    if (quoted) {
      if (character === '"' && text[index + 1] === '"') {
        value += '"'
        index += 1
      } else if (character === '"') {
        quoted = false
      } else {
        value += character
      }
    } else if (character === '"' && value.length === 0) {
      quoted = true
    } else if (character === ',') {
      row.push(value.trim())
      value = ''
    } else if (character === '\n' || character === '\r') {
      row.push(value.trim())
      value = ''
      if (row.some(cell => cell !== '')) rows.push(row)
      row = []
      if (character === '\r' && text[index + 1] === '\n') index += 1
    } else {
      value += character
    }
  }

  if (quoted) throw new Error('The CSV has an unclosed quoted field.')
  row.push(value.trim())
  if (row.some(cell => cell !== '')) rows.push(row)
  return rows
}

function isoDateIsValid(value) {
  if (!/^\d{4}-\d{2}-\d{2}$/.test(value)) return false
  const date = new Date(`${value}T00:00:00Z`)
  return !Number.isNaN(date.getTime()) && date.toISOString().slice(0, 10) === value
}

export function validateDelayCSV(text) {
  const source = parseCSV(text.replace(/^\uFEFF/, ''))
  if (source.length < 2) throw new Error('Add a header row and at least one delay record.')
  const headers = source[0].map(header => header.toLowerCase())
  const missing = requiredHeaders.filter(header => !headers.includes(header))
  if (missing.length) throw new Error(`Missing required CSV column${missing.length > 1 ? 's' : ''}: ${missing.join(', ')}.`)

  const positions = Object.fromEntries(requiredHeaders.map(header => [header, headers.indexOf(header)]))
  const dataRows = source.slice(1)
  if (dataRows.length > MAX_ROWS) throw new Error(`A file can contain up to ${MAX_ROWS} records.`)

  const seen = new Set()
  return dataRows.map((cells, index) => {
    const record = {
      line: index + 2,
      orderID: (cells[positions.order_id] || '').trim(),
      productName: (cells[positions.product_name] || '').trim(),
      delayedDate: (cells[positions.delayed_delivery_date] || '').trim(),
      error: ''
    }
    if (!record.orderID || !record.productName || !record.delayedDate) {
      record.error = 'Order ID, product name, and delivery date are required.'
    } else if (!isoDateIsValid(record.delayedDate)) {
      record.error = 'Use a real date in YYYY-MM-DD format.'
    } else {
      const key = `${record.orderID}\u0000${record.productName}`
      if (seen.has(key)) record.error = 'This order and product appear more than once.'
      seen.add(key)
    }
    return record
  })
}

export function unappliedDelayRows(records, results) {
  return records
    .map((record, index) => ({ record, index }))
    .filter(({ index }) => !results?.[index] || results[index].status !== 'updated')
}
