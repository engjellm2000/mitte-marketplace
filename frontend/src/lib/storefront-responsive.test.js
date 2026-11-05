import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

const storefrontStyles = readFileSync(new URL('../mitte.css', import.meta.url), 'utf8')
const storefrontBaseStyles = readFileSync(new URL('../storefront.css', import.meta.url), 'utf8')
const cartPage = readFileSync(new URL('../pages/storefront/Cart.jsx', import.meta.url), 'utf8')
const documentMarkup = readFileSync(new URL('../../index.html', import.meta.url), 'utf8')

test('mobile checkout uses page scrolling instead of a viewport-height nested scroller', () => {
  assert.match(storefrontStyles, /\.storefront \.checkout-details\{max-height:none;overflow:visible\}/)
  assert.match(storefrontStyles, /\.storefront \.checkout-summary\.checkout\{position:static;grid-row:2\}/)
})

test('checkout keeps the order summary beside the complete one-page form on desktop', () => {
  assert.match(storefrontStyles, /\.storefront \.checkout-summary\.checkout\{[\s\S]*?position:sticky/)
  assert.match(cartPage, /<form className="checkout-flow checkout-details"[\s\S]*<aside className="checkout checkout-summary"/)
  assert.match(cartPage, /aria-labelledby="checkout-summary-title"/)
})

test('mobile category load errors occupy layout space instead of covering page content', () => {
  assert.match(storefrontStyles, /\.storefront \.header-category-error\{position:static;grid-column:1\/-1;grid-row:3;/)
})

test('mobile viewport supports safe-area insets and checkout inputs retain a readable size', () => {
  assert.match(documentMarkup, /name="viewport" content="[^"]*viewport-fit=cover/)
  assert.match(storefrontStyles, /\.storefront \.header-search input,[\s\S]*?\.storefront \.auth-form input\{font-size:16px\}/)
})

test('mobile product, navigation, and quantity controls expose 44px touch targets', () => {
  assert.match(storefrontStyles, /\.storefront \.menu-toggle,[\s\S]*?\.storefront \.bag\{min-width:44px;min-height:44px\}/)
  assert.match(storefrontStyles, /\.storefront \.product \.save-product,[\s\S]*?\.storefront \.save-detail\{min-width:44px;min-height:44px\}/)
})

test('checkout failures render as a readable inline alert', () => {
  assert.match(storefrontBaseStyles, /\.storefront \.checkout-error\{[^}]*background:#fff0ed;[^}]*color:#7c201a/)
})
