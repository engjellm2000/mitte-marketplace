import { useState } from 'react'
import { csrf, api } from '../../lib/api.js'

export function StaffLogin({ onAuthenticated }) {
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const submit = async event => {
    event.preventDefault(); setBusy(true); setError('')
    try {
      const data = await api('/api/staff/login', { method: 'POST', headers: { 'X-CSRF-Token': csrf }, body: JSON.stringify({ username, password }) })
      onAuthenticated(data.staff); setPassword('')
    } catch (cause) { setError(cause.message) } finally { setBusy(false) }
  }
  return <section className="auth-page"><p className="kicker">Interner Bereich</p><h1>Mitarbeitendenzugang.</h1><p>Bitte melde dich mit einem privaten Zugangscode an.</p><form className="auth-form" onSubmit={submit}><label>Benutzername<input required autoComplete="username" value={username} onChange={event => setUsername(event.target.value)} /></label><label>Passwort<input required type="password" autoComplete="current-password" value={password} onChange={event => setPassword(event.target.value)} /></label>{error && <p className="auth-error" role="alert">{error}</p>}<button disabled={busy} className="coral-button">{busy ? 'Bitte warten …' : 'Intern anmelden →'}</button></form></section>
}
