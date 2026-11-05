import { test } from 'node:test'
import assert from 'node:assert/strict'
import { unappliedDelayRows, validateDelayCSV } from './oms-csv.js'

test('parses BOM-prefixed CSV, CRLF, and quoted product names with commas', () => {
  const [record] = validateDelayCSV('\uFEFForder_id,product_name,delayed_delivery_date\r\nORD-1,"Chair, oak",2026-11-02\r\n')
  assert.equal(record.orderID, 'ORD-1')
  assert.equal(record.productName, 'Chair, oak')
  assert.equal(record.delayedDate, '2026-11-02')
  assert.equal(record.error, '')
})

test('reports missing headers and malformed quoted fields', () => {
  assert.throws(() => validateDelayCSV('order_id,product_name\nORD-1,Chair'), /Missing required CSV column/)
  assert.throws(() => validateDelayCSV('order_id,product_name,delayed_delivery_date\nORD-1,"Chair,2026-11-02'), /unclosed quoted field/)
})

test('marks missing fields, invalid dates, and duplicate order-item rows', () => {
  const records = validateDelayCSV([
    'order_id,product_name,delayed_delivery_date',
    'ORD-1,Chair,2026-02-30',
    'ORD-1,Chair,2026-11-02',
    'ORD-1,Chair,2026-12-02',
    ',Lamp,2026-11-02'
  ].join('\n'))
  assert.match(records[0].error, /real date/)
  assert.equal(records[1].error, '')
  assert.match(records[2].error, /more than once/)
  assert.match(records[3].error, /required/)
})

test('rejects files above the 200-row batch limit', () => {
  const rows = Array.from({ length: 201 }, (_, index) => `ORD-${index},Lamp,2026-11-02`)
  assert.throws(
    () => validateDelayCSV(['order_id,product_name,delayed_delivery_date', ...rows].join('\n')),
    /up to 200 records/
  )
})

test('retry selection excludes saved rows and includes failed or unattempted rows only', () => {
  const records = [{ orderID: 'ORD-1' }, { orderID: 'ORD-2' }, { orderID: 'ORD-3' }]
  const results = [
    { status: 'updated' },
    { status: 'failed' },
    { status: 'not_attempted' }
  ]
  assert.deepEqual(unappliedDelayRows(records, results), [
    { record: records[1], index: 1 },
    { record: records[2], index: 2 }
  ])
  assert.deepEqual(unappliedDelayRows(records, null), records.map((record, index) => ({ record, index })))
})
