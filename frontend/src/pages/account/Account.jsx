import { useEffect, useState } from 'react'
import { csrf, api } from '../../lib/api.js'
import { CustomerAccount } from './CustomerAccount.jsx'

export function Account({ user, setUser }) {
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [customerType, setCustomerType] = useState('b2c')
  const [showPassword, setShowPassword] = useState(false)
  const [mode, setMode] = useState('login')
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const [needsVerification, setNeedsVerification] = useState(false)
  const [flowToken, setFlowToken] = useState('')
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    const params = new URLSearchParams(location.search)
    const verificationToken = params.get('verify_email')
    const resetToken = params.get('reset_password')
    if (params.has('verify_email')) {
      setMode('verify')
      setFlowToken(verificationToken || '')
      history.replaceState({}, '', location.pathname)
    } else if (params.has('reset_password')) {
      setMode('reset')
      setFlowToken(resetToken || '')
      history.replaceState({}, '', location.pathname)
    }
  }, [])

  const submit = async event => {
    event.preventDefault()
    setBusy(true)
    setError('')
    setNotice('')
    try {
      if (mode === 'forgot') {
        const data = await api('/api/auth/password-reset/request', {
          method: 'POST', headers: { 'X-CSRF-Token': csrf }, body: JSON.stringify({ email }),
        })
        setNotice(data.message)
        setMode('login')
      } else if (mode === 'reset') {
        const data = await api('/api/auth/password-reset/complete', {
          method: 'POST', headers: { 'X-CSRF-Token': csrf }, body: JSON.stringify({ token: flowToken, newPassword: password }),
        })
        setNotice(data.message)
        setPassword('')
        setFlowToken('')
        setMode('login')
      } else if (mode === 'verify') {
        const data = await api('/api/auth/email/verify', {
          method: 'POST', headers: { 'X-CSRF-Token': csrf }, body: JSON.stringify({ token: flowToken }),
        })
        setNotice(data.message)
        setFlowToken('')
        setMode('login')
      } else {
        const details = { email, password }
        if (mode === 'register') details.customerType = customerType
        const data = await api(`/api/auth/${mode}`, {
          method: 'POST', headers: { 'X-CSRF-Token': csrf }, body: JSON.stringify(details),
        })
        if (mode === 'register') {
          setNotice(data.message)
          setMode('login')
        } else {
          setUser(data.user)
        }
        setPassword('')
        setNeedsVerification(false)
      }
    } catch (cause) {
      setError(cause.message)
      setNeedsVerification(cause.status === 403)
    } finally {
      setBusy(false)
    }
  }

  const resendVerification = async () => {
    setBusy(true)
    setError('')
    setNotice('')
    try {
      const data = await api('/api/auth/email/resend-verification', {
        method: 'POST', headers: { 'X-CSRF-Token': csrf }, body: JSON.stringify({ email }),
      })
      setNotice(data.message)
      setNeedsVerification(false)
    } catch (cause) {
      setError(cause.message)
    } finally {
      setBusy(false)
    }
  }

  const logout = async () => {
    setBusy(true)
    try {
      await api('/api/auth/logout', { method: 'POST', headers: { 'X-CSRF-Token': csrf } })
      setUser(null)
    } catch (cause) {
      setError(cause.message)
    } finally {
      setBusy(false)
    }
  }

  if (user) return <CustomerAccount user={user} setUser={setUser} logout={logout} logoutBusy={busy} />

  const registering = mode === 'register'
  const completingFlow = mode === 'verify' || mode === 'reset'
  const title = {
    verify: 'E-Mail-Adresse bestätigen.',
    reset: 'Neues Passwort festlegen.',
    forgot: 'Passwort zurücksetzen.',
    register: 'Ich bin Neukunde.',
    login: 'Willkommen zurück.',
  }[mode]
  const submitLabel = {
    verify: 'E-Mail-Adresse bestätigen →',
    reset: 'Passwort speichern →',
    forgot: 'Link anfordern →',
    register: 'Konto erstellen',
    login: 'Anmelden →',
  }[mode]

  return (
    <section className={`auth-page${registering ? ' auth-page--register' : ''}`}>
      {!registering && <p className="kicker">Mein Konto</p>}
      <h1>{title}</h1>
      {notice && <p className="account-message" role="status">{notice}</p>}
      {completingFlow && (
        <p>{mode === 'verify'
          ? 'Bestätige deine E-Mail-Adresse, um dein Kundenkonto freizuschalten.'
          : 'Wähle ein neues Passwort mit mindestens 12 Zeichen.'}</p>
      )}
      {completingFlow && !flowToken && <p className="auth-error" role="alert">Dieser Link ist ungültig oder abgelaufen. Fordere einen neuen Link an.</p>}
      <form className="auth-form" onSubmit={submit}>
        {mode !== 'verify' && mode !== 'reset' && (
          <>
            {registering && (
              <fieldset className="auth-type-picker">
                <legend>Ich bin</legend>
                <label>
                  <input
                    type="radio"
                    name="customerType"
                    value="b2c"
                    checked={customerType === 'b2c'}
                    onChange={event => setCustomerType(event.target.value)}
                  />
                  <span>Privatperson</span>
                </label>
                <label>
                  <input
                    type="radio"
                    name="customerType"
                    value="b2b"
                    checked={customerType === 'b2b'}
                    onChange={event => setCustomerType(event.target.value)}
                  />
                  <span>Geschäftskunde</span>
                </label>
              </fieldset>
            )}
            <label>
              E-Mail-Adresse
              <input
                required
                type="email"
                maxLength="254"
                autoComplete="email"
                value={email}
                onChange={event => setEmail(event.target.value)}
              />
            </label>
          </>
        )}
        {(registering || mode === 'login' || mode === 'reset') && (
          <label>
            Passwort
            <span className="auth-password-field">
              <input
                required
                type={showPassword ? 'text' : 'password'}
                minLength="12"
                maxLength="128"
                autoComplete={registering || mode === 'reset' ? 'new-password' : 'current-password'}
                value={password}
                onChange={event => setPassword(event.target.value)}
              />
              <button
                type="button"
                className="auth-password-toggle"
                aria-label={showPassword ? 'Passwort verbergen' : 'Passwort anzeigen'}
                aria-pressed={showPassword}
                onClick={() => setShowPassword(value => !value)}
              >
                {showPassword ? 'Verbergen' : 'Anzeigen'}
              </button>
            </span>
            {(registering || mode === 'reset') && <small>Mindestens 12 Zeichen.</small>}
          </label>
        )}
        {error && <p className="auth-error" role="alert">{error}</p>}
        <button disabled={busy || (completingFlow && !flowToken)} className="coral-button">
          {busy ? 'Bitte warten …' : submitLabel}
        </button>
      </form>
      {completingFlow && (!flowToken || error) && (
        <button
          type="button"
          className="text-button auth-mode-toggle"
          onClick={() => {
            setMode(mode === 'verify' ? 'login' : 'forgot')
            setError('')
            setPassword('')
            setFlowToken('')
          }}
        >
          {mode === 'verify' ? 'Zur Anmeldung und Verifizierung' : 'Neuen Passwort-Link anfordern'}
        </button>
      )}
      {needsVerification && mode === 'login' && (
        <button type="button" className="text-button auth-mode-toggle" disabled={busy} onClick={resendVerification}>
          Bestätigungslink erneut senden
        </button>
      )}
      {registering && (
        <div className="auth-benefits">
          <h2>Dein Kundenkonto</h2>
          <ul>
            <li>Bestellstatus jederzeit einsehen</li>
            <li>Persönliche Angaben und Adressen verwalten</li>
            <li>Ausgestellte Rechnungen als PDF abrufen</li>
          </ul>
        </div>
      )}
      {!completingFlow && (
        <>
          {mode === 'login' && <button type="button" className="text-button auth-mode-toggle" onClick={() => { setMode('forgot'); setError(''); setNotice('') }}>Passwort vergessen?</button>}
          <button
            type="button"
            className="text-button auth-mode-toggle"
            onClick={() => {
              setMode(registering || mode === 'forgot' ? 'login' : 'register')
              setError('')
              setNotice('')
              setNeedsVerification(false)
            }}
          >
            {registering ? 'Schon registriert? Anmelden' : mode === 'forgot' ? 'Zurück zur Anmeldung' : 'Neu hier? Kundenkonto erstellen'}
          </button>
        </>
      )}
    </section>
  )
}
