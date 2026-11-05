import { formatEuro } from './format.js'

export const euro = formatEuro
export const csrf = document.querySelector('meta[name="csrf-token"]')?.content
const readCache = new Map()
const publicReadCacheTTL = 30_000
const isPublicRead = path => path === '/api/categories' || path === '/api/offers' || path.startsWith('/api/offers/') || path === '/api/products' || path.startsWith('/api/products/')
export const api = async (path, options = {}) => {
  const cacheRead = !options.method && options.cache !== 'no-store' && isPublicRead(path)
  if (cacheRead) {
    const cached = readCache.get(path)
    if (cached && cached.expiresAt > Date.now()) return cached.payload
    if (cached) readCache.delete(path)
  }
  const response = await fetch(path, { ...options, headers: { 'Content-Type': 'application/json', ...(options.headers || {}) } })
  // Erfolgreiche Antworten ohne Inhalt (z. B. 204 bei Statuswechsel oder Logout) haben keinen JSON-Body.
  if (response.ok && (response.status === 204 || response.headers.get('content-length') === '0')) return null
  if (!response.headers.get('content-type')?.includes('application/json')) {
    throw new Error('Der Server hat keine gültige JSON-Antwort gesendet.')
  }
  const payload = await response.json()
  if (!response.ok) {
    const error = new Error(payload.error || 'Etwas ist schiefgelaufen.')
    error.status = response.status
    throw error
  }
  if (cacheRead) {
    readCache.set(path, { payload, expiresAt: Date.now() + publicReadCacheTTL })
    if (readCache.size > 100) {
      const oldest = readCache.keys().next().value
      if (oldest) readCache.delete(oldest)
    }
  }
  return payload
}
export const isAuthorizationError = error => /unauthori[sz]ed|forbidden|permission|access denied|staff.{0,16}(access|permission)|zugriff|403|401/i.test(error || '')
