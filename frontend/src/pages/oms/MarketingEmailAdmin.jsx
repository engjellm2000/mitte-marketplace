import { useEffect, useState } from 'react'
import { api, csrf } from '../../lib/api.js'
import { dateTime } from '../../lib/labels.js'
import { OperationsSidebar } from '../../components/operations/OperationsSidebar.jsx'

const emptyCampaign = {
  name: '',
  subject: '',
  preheader: '',
  body: '',
}

const statusLabels = {
  draft: 'Entwurf',
  queued: 'Wird versendet',
  sent: 'Abgeschlossen',
}

export function MarketingEmailAdmin({ user, go }) {
  const [campaigns, setCampaigns] = useState([])
  const [audienceSize, setAudienceSize] = useState(0)
  const [selected, setSelected] = useState(null)
  const [form, setForm] = useState({ ...emptyCampaign })
  const [loading, setLoading] = useState(true)
  const [busy, setBusy] = useState(false)
  const [confirmSend, setConfirmSend] = useState(false)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')

  const refresh = async () => {
    setLoading(true)
    try {
      const data = await api('/api/admin/marketing-emails', { cache: 'no-store' })
      setCampaigns(data.campaigns || [])
      setAudienceSize(data.audienceSize || 0)
      setError('')
    } catch (cause) {
      setError(cause.message)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { refresh() }, [])

  const edit = campaign => {
    setSelected(campaign)
    setForm({
      name: campaign.name,
      subject: campaign.subject,
      preheader: campaign.preheader,
      body: campaign.body,
    })
    setConfirmSend(false)
    setError('')
    setNotice('')
  }

  const reset = () => {
    setSelected(null)
    setForm({ ...emptyCampaign })
    setConfirmSend(false)
    setError('')
    setNotice('')
  }

  const update = (field, value) => {
    setForm(current => ({ ...current, [field]: value }))
    setConfirmSend(false)
  }

  const save = async event => {
    event.preventDefault()
    setBusy(true)
    setError('')
    setNotice('')
    try {
      const response = await api(selected
        ? `/api/admin/marketing-emails/${selected.id}`
        : '/api/admin/marketing-emails', {
        method: selected ? 'PATCH' : 'POST',
        headers: { 'X-CSRF-Token': csrf },
        body: JSON.stringify(form),
      })
      await refresh()
      edit(response.campaign)
      setNotice('Entwurf gespeichert.')
    } catch (cause) {
      setError(cause.message)
    } finally {
      setBusy(false)
    }
  }

  const send = async () => {
    if (!selected || selected.status !== 'draft' || !confirmSend || audienceSize < 1) return
    setBusy(true)
    setError('')
    setNotice('')
    try {
      const response = await api(`/api/admin/marketing-emails/${selected.id}/send`, {
        method: 'POST',
        headers: { 'X-CSRF-Token': csrf },
        body: JSON.stringify({ expectedAudienceSize: audienceSize }),
      })
      setConfirmSend(false)
      await refresh()
      const queued = (await api('/api/admin/marketing-emails', { cache: 'no-store' }))
        .campaigns?.find(campaign => campaign.id === selected.id)
      if (queued) edit(queued)
      setNotice(`${response.queued} E-Mails wurden in die Zustellwarteschlange gestellt.`)
    } catch (cause) {
      setConfirmSend(false)
      await refresh()
      setError(cause.message)
    } finally {
      setBusy(false)
    }
  }

  const editable = !selected || selected.status === 'draft'
  const draftIsSaved = selected &&
    selected.name === form.name &&
    selected.subject === form.subject &&
    selected.preheader === form.preheader &&
    selected.body === form.body

  return <div className="operations-shell">
    <OperationsSidebar active="email" user={user} go={go} />
    <main className="operations-main">
      <section className="marketing-email-admin">
        <header className="marketing-email-head">
          <div>
            <h1>E-Mail-Kampagnen</h1>
            <p>Schreibe Angebote für Kundinnen und Kunden, die Marketing-E-Mails in ihrem verifizierten Konto ausdrücklich aktiviert haben.</p>
          </div>
          <div className="marketing-email-head-actions">
            <button type="button" onClick={() => go('/admin/promotions')}>Angebote verwalten</button>
            <button type="button" className="dark-button" onClick={reset}>+ Entwurf erstellen</button>
          </div>
        </header>

        <div className="marketing-email-truth" role="note">
          <strong>{audienceSize} aktive Empfänger:innen</strong>
          <span>Jede Kampagne enthält einen persönlichen Abmeldelink. Offene Versandaufträge werden bei Abmeldung unterdrückt; eine bereits laufende Zustellung kann noch eintreffen. Bestell- und Kontonachrichten bleiben davon unberührt.</span>
        </div>
        {(error || notice) && <div className={`oms-alert ${error ? 'error' : ''}`} role={error ? 'alert' : 'status'}>{error || notice}</div>}

        <div className="marketing-email-layout">
          <section className="marketing-email-list" aria-label="E-Mail-Kampagnen">
            <div className="marketing-email-list-head">
              <h2>Kampagnen</h2>
              <button type="button" disabled={loading} onClick={refresh}>Aktualisieren</button>
            </div>
            {loading && <p className="oms-empty-state" role="status">Kampagnen werden geladen …</p>}
            {!loading && campaigns.map(campaign => (
              <button
                type="button"
                key={campaign.id}
                className={`marketing-email-row${selected?.id === campaign.id ? ' selected' : ''}`}
                onClick={() => edit(campaign)}
              >
                <span>
                  <strong>{campaign.name}</strong>
                  <small>{campaign.subject} · {dateTime(campaign.createdAt)}</small>
                  {campaign.status !== 'draft' && <small>{campaign.deliveredCount} zugestellt · {campaign.pendingCount} ausstehend · {campaign.suppressedCount} abgemeldet</small>}
                </span>
                <b className={`marketing-email-status ${campaign.status}`}>{statusLabels[campaign.status] || campaign.status}</b>
              </button>
            ))}
            {!loading && !campaigns.length && !error && <p className="oms-empty-state">Noch keine Kampagnen. Erstelle einen Entwurf, um zu beginnen.</p>}
          </section>

          <div className="marketing-email-workbench">
            <form className="marketing-email-editor" onSubmit={save}>
              <div className="marketing-email-editor-head">
                <div>
                  <h2>{selected ? (editable ? 'Kampagne bearbeiten' : 'Kampagne im Versand') : 'Neuer Entwurf'}</h2>
                  <p>{selected ? `Kampagne ${selected.id} · ${statusLabels[selected.status] || selected.status}` : 'Nur Entwürfe können bearbeitet werden.'}</p>
                </div>
                {selected && <button type="button" onClick={reset}>Neuer Entwurf</button>}
              </div>
              <fieldset disabled={!editable || busy}>
                <label>Kampagnenname
                  <input required minLength="3" maxLength="120" value={form.name} onChange={event => update('name', event.target.value)} placeholder="Zum Beispiel: Herbstneuheiten" />
                </label>
                <label>Betreff
                  <input maxLength="120" value={form.subject} onChange={event => update('subject', event.target.value)} placeholder="Was gibt es Neues bei MITTE?" />
                </label>
                <label>Vorschautext <small>Optional · erscheint neben dem Betreff im Posteingang</small>
                  <input maxLength="180" value={form.preheader} onChange={event => update('preheader', event.target.value)} placeholder="Ein kurzer, hilfreicher Einblick in diese E-Mail" />
                </label>
                <label>Nachricht
                  <textarea maxLength="12000" rows="12" value={form.body} onChange={event => update('body', event.target.value)} placeholder={'Schreibe eine klare, hilfreiche Nachricht.\n\nLinks mit https:// werden in der E-Mail anklickbar.'} />
                </label>
              </fieldset>
              {editable && <button className="coral-button marketing-email-save" disabled={busy}>
                {busy ? 'Wird gespeichert …' : 'Entwurf speichern'}
              </button>}
              {selected?.status === 'draft' && (
                <div className="marketing-email-send">
                  <p><strong>Vor dem Versand</strong><span>Die Empfängerliste umfasst nur verifizierte Kundenkonten mit aktiver Marketing-Einwilligung.</span></p>
                  {!draftIsSaved && <p className="marketing-email-save-note" role="status">Speichere deine Änderungen, bevor du den Versand bestätigst.</p>}
                  <label>
                    <input type="checkbox" checked={confirmSend} onChange={event => setConfirmSend(event.target.checked)} disabled={busy || audienceSize === 0 || !draftIsSaved} />
                    <span>Ich habe den Entwurf geprüft und bestätige den Versand an {audienceSize} aktuell berechtigte Empfänger:innen.</span>
                  </label>
                  <button type="button" className="dark-button" onClick={send} disabled={busy || !confirmSend || audienceSize === 0 || !draftIsSaved}>
                    {busy ? 'Wird eingereiht …' : `Kampagne einreihen · ${audienceSize}`}
                  </button>
                </div>
              )}
            </form>

            <section className="marketing-email-preview" aria-label="E-Mail-Vorschau">
              <div className="marketing-email-preview-head">
                <span>VORSCHAU</span>
                <small>MITTE · E-Mail</small>
              </div>
              <article>
                <header><strong>MITTE</strong><small>ALLES FÜR JEDEN TAG</small></header>
                <div className="marketing-email-preview-meta">
                  <b>{form.subject || 'Betreff deiner E-Mail'}</b>
                  <span>{form.preheader || 'Vorschautext wird hier angezeigt.'}</span>
                </div>
                <div className="marketing-email-preview-body">
                  <pre>{form.body || 'Deine Nachricht erscheint hier in einer gut lesbaren, responsiven E-Mail-Vorlage.'}</pre>
                </div>
                <footer>Du erhältst diese Nachricht, weil du Marketing-E-Mails in deinem MITTE-Kundenkonto aktiviert hast.<br />Abmeldelink in jeder Kampagne enthalten.</footer>
              </article>
            </section>
          </div>
        </div>
      </section>
    </main>
  </div>
}
