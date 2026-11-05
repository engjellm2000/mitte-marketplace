import { useEffect, useRef, useState } from 'react'
import { api, euro } from '../lib/api.js'

const greeting = { role: 'assistant', content: 'Hallo! Ich helfe dir, passende Produkte im MITTE-Sortiment zu finden und zu vergleichen. Frag nach einer Produktart, Kategorie oder einem Budget.', greeting: true }
const prompts = ['Finde Shampoo.', 'Was gibt es unter 20 Euro?', 'Welche Kategorien gibt es?', 'Vergleiche Haarpflege.']
const recommendedArticles = [
  ['Versandkosten', 'Ab 59 € Bestellwert liefern wir innerhalb Deutschlands versandkostenfrei. Darunter werden die Versandkosten vor dem Kaufabschluss transparent angezeigt.'],
  ['Barrierefreiheit / Accessibility', 'Wir arbeiten daran, unseren Shop für alle Menschen gut nutzbar zu machen. Wenn dir eine Barriere auffällt, sag uns bitte Bescheid.'],
  ['Lieferzeiten', 'Verfügbare Artikel werden in der Regel innerhalb von 2–4 Werktagen geliefert. Den aktuellen Status findest du nach dem Versand in deiner Bestellbestätigung.'],
  ['E-Rechnung (elektronische Rechnung)', 'Deine Rechnung erhältst du nach dem Kauf digital. Geschäftskund:innen können sie in ihrem Konto herunterladen.'],
  ['Zahlung per PayPal', 'Wähle im Checkout PayPal aus und schließe die Zahlung sicher bei PayPal ab. Danach kehrst du automatisch zu MITTE zurück.']
]
const topics = [
  ['Account', ['Konto erstellen und anmelden', 'Passwort zurücksetzen', 'Daten im Kundenkonto ändern']],
  ['Newsletter', ['Newsletter anmelden', 'Newsletter abbestellen']],
  ['Bestellung', ['Bestellstatus prüfen', 'Bestellung ändern', 'Bestellbestätigung nicht erhalten']],
  ['Zahlungsmethoden', ['Zahlung per PayPal', 'Kauf auf Rechnung', 'Zahlung fehlgeschlagen']],
  ['Gutscheine & Rabatte', ['Gutschein einlösen', 'Rabatt wurde nicht angewendet']],
  ['Lieferung', ['Versandkosten', 'Lieferzeiten', 'Sendung verfolgen']],
  ['Stornierungen & Rücksendungen', ['Bestellung stornieren', 'Artikel zurücksenden']],
  ['Reklamation', ['Artikel ist beschädigt', 'Falschen Artikel erhalten']],
  ['Rechnungen', ['E-Rechnung (elektronische Rechnung)', 'Rechnung erneut herunterladen']],
  ['Kontakt', ['Wie erreiche ich den Kundenservice?']]
]
const articleText = {
  'Konto erstellen und anmelden': 'Über „Kundenkonto“ kannst du dich registrieren oder anmelden. Für eine Bestellung kannst du den Checkout auch ohne Konto nutzen.',
  'Passwort zurücksetzen': 'Wähle auf der Anmeldeseite „Passwort vergessen“ und wir senden dir einen sicheren Link zum Zurücksetzen an deine E-Mail-Adresse.',
  'Daten im Kundenkonto ändern': 'Nach dem Anmelden kannst du deine Kontodaten im Bereich „Kundenkonto“ aktualisieren.',
  'Newsletter anmelden': 'Melde dich über das Formular im Shop für unseren Newsletter an. Du erhältst anschließend eine Bestätigungs-E-Mail.',
  'Newsletter abbestellen': 'Am Ende jedes Newsletters findest du einen Link zum Abbestellen. Die Änderung wird zeitnah übernommen.',
  'Bestellstatus prüfen': 'Sobald deine Bestellung versendet wurde, erhältst du eine Versandbestätigung. Dort findest du auch den aktuellen Status deiner Sendung.',
  'Bestellung ändern': 'Solange deine Bestellung noch nicht bearbeitet wird, helfen wir dir gerne weiter. Nenne dabei bitte deine Bestellnummer.',
  'Bestellbestätigung nicht erhalten': 'Bitte prüfe zuerst deinen Spam-Ordner. Ist dort nichts angekommen, frage uns mit der beim Kauf verwendeten E-Mail-Adresse.',
  'Kauf auf Rechnung': 'Wenn Kauf auf Rechnung für deine Bestellung verfügbar ist, wird die Zahlungsart im Checkout angezeigt.',
  'Zahlung fehlgeschlagen': 'Versuche es bitte erneut oder wähle eine andere Zahlungsart. Wenn das Problem bleibt, nenne uns die Fehlermeldung – aber keine Zahlungsdaten.',
  'Gutschein einlösen': 'Gib deinen Gutscheincode im Warenkorb oder Checkout in das dafür vorgesehene Feld ein. Der Rabatt wird direkt vor dem Kauf angezeigt.',
  'Rabatt wurde nicht angewendet': 'Bitte prüfe die Bedingungen und den Mindestbestellwert des Rabatts. Bei Fragen hilft dir unser Kundenservice gerne weiter.',
  'Sendung verfolgen': 'Mit der Versandbestätigung erhältst du einen Link zur Sendungsverfolgung, sobald dein Paket an den Versanddienstleister übergeben wurde.',
  'Bestellung stornieren': 'Kontaktiere uns möglichst schnell mit deiner Bestellnummer. Ob eine Stornierung möglich ist, hängt vom Bearbeitungsstand ab.',
  'Artikel zurücksenden': 'Melde deine Rücksendung beim Kundenservice an. Wir erklären dir anschließend die nächsten Schritte.',
  'Artikel ist beschädigt': 'Es tut uns leid. Schick uns bitte deine Bestellnummer und ein Foto des Schadens, damit wir eine Lösung finden können.',
  'Falschen Artikel erhalten': 'Bitte bewahre den Artikel und die Verpackung auf und melde dich mit deiner Bestellnummer bei uns.',
  'Rechnung erneut herunterladen': 'Wenn du ein Kundenkonto hast, kannst du deine Rechnung dort erneut herunterladen. Alternativ hilft dir unser Kundenservice weiter.',
  'Wie erreiche ich den Kundenservice?': 'Schreibe uns direkt hier im Chat. Für eine schnelle Zuordnung halte bitte – falls vorhanden – deine Bestellnummer bereit.'
}
const SearchIcon = () => <svg viewBox="0 0 24 24" aria-hidden="true"><circle cx="11" cy="11" r="6.5" /><path d="m16 16 4.5 4.5" /></svg>
const Arrow = () => <svg viewBox="0 0 24 24" aria-hidden="true"><path d="m9 6 6 6-6 6" /></svg>

function ProductMatches({ products, go, add }) {
  if (!products?.length) return null
  return <div className="assistant-products" aria-label="Passende Produkte">{products.map(product => {
    const available = product.Inventory > 0
    return <article className="assistant-product" key={product.Slug}><button type="button" className="assistant-product-image" onClick={() => go(`/produkt/${product.Slug}`)} aria-label={`${product.Name} ansehen`}>{product.ImageURL && <img src={product.ImageURL} alt={product.ImageAlt || ''} loading="lazy" />}</button><div className="assistant-product-copy"><span>{product.Category?.Name || 'MITTE Sortiment'}</span><button type="button" className="assistant-product-name" onClick={() => go(`/produkt/${product.Slug}`)}>{product.Name}</button><strong>{euro(product.PriceCents)}</strong><small className={available ? 'available' : 'unavailable'}>{available ? 'Verfügbar' : 'Derzeit nicht verfügbar'}</small><div className="assistant-product-actions"><button type="button" onClick={() => go(`/produkt/${product.Slug}`)}>Details</button><button type="button" disabled={!available} onClick={() => add(product)}>{available ? 'In den Warenkorb' : 'Nicht verfügbar'}</button></div></div></article>
  })}</div>
}

export function ShoppingAssistant({ go, add }) {
  const [open, setOpen] = useState(false), [view, setView] = useState('help'), [status, setStatus] = useState(null), [statusError, setStatusError] = useState(false)
  const [messages, setMessages] = useState([greeting]), [draft, setDraft] = useState(''), [search, setSearch] = useState(''), [expanded, setExpanded] = useState(null), [article, setArticle] = useState(null), [busy, setBusy] = useState(false)
  const panelRef = useRef(null), inputRef = useRef(null), searchRef = useRef(null), launcherRef = useRef(null), transcriptRef = useRef(null), previousFocus = useRef(null)
  const enabled = status?.enabled === true, aiEnabled = status?.aiEnabled === true
  const refreshStatus = () => api('/api/assistant/status', { cache: 'no-store' }).then(value => { setStatus(value); setStatusError(false); return value }).catch(() => { setStatus({ enabled: false, aiEnabled: false }); setStatusError(true); return null })
  useEffect(() => { refreshStatus() }, [])
  useEffect(() => { if (!open) return; previousFocus.current = document.activeElement; setTimeout(() => (view === 'chat' ? inputRef : searchRef).current?.focus(), 0); return () => previousFocus.current instanceof HTMLElement && previousFocus.current.isConnected && previousFocus.current.focus() }, [open])
  useEffect(() => { if (transcriptRef.current) transcriptRef.current.scrollTop = transcriptRef.current.scrollHeight }, [messages, busy])
  useEffect(() => { if (!open) return; const onKey = event => { if (event.key === 'Escape') { event.preventDefault(); article ? setArticle(null) : setOpen(false); return } if (event.key !== 'Tab') return; const items = [...panelRef.current.querySelectorAll('button:not(:disabled), textarea:not(:disabled), input:not(:disabled)')], first = items[0], last = items.at(-1); if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last.focus() } else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus() } }; document.addEventListener('keydown', onKey); return () => document.removeEventListener('keydown', onKey) }, [open, article])
  const openChat = () => { setArticle(null); setView('chat'); setTimeout(() => inputRef.current?.focus(), 0) }
  const ask = async content => { const text = content.trim(); if (!text || busy || !enabled) return; const next = [...messages, { role: 'user', content: text }]; setMessages(next); setDraft(''); setBusy(true); try { const response = await api('/api/assistant/chat', { method: 'POST', body: JSON.stringify({ messages: next.filter(message => !message.greeting).slice(-12) }) }); setMessages(current => [...current, { role: 'assistant', content: response.reply, products: response.products || [] }]) } catch (error) { setMessages(current => [...current, { role: 'assistant', content: error.message || 'Deine Nachricht konnte gerade nicht gesendet werden. Bitte versuche es erneut.' }]) } finally { setBusy(false); refreshStatus() } }
  const openArticle = title => setArticle({ title, body: recommendedArticles.find(([name]) => name === title)?.[1] || articleText[title] || 'Zu diesem Thema helfen wir dir gerne persönlich weiter.' })
  const resultTitles = search.trim() ? [...new Set([...recommendedArticles.map(([title]) => title), ...topics.flatMap(([, titles]) => titles)])].filter(title => title.toLowerCase().includes(search.toLowerCase())).slice(0, 6) : []
  const statusLabel = status === null ? 'Verbindung wird geprüft …' : enabled ? (aiEnabled ? 'KI-Assistenz verfügbar' : 'Direkt mit dem Sortiment verbunden') : 'Chat gerade nicht verfügbar'
  return <div className="shopping-assistant">
    {open && <section id="assistant-dialog" className="assistant-panel assistant-help-panel" role="dialog" aria-modal="true" aria-labelledby="assistant-title" ref={panelRef}><header className="assistant-header"><span className="assistant-mark" aria-hidden="true">M</span><div><h2 id="assistant-title">MITTE Hilfecenter</h2><p>{statusLabel}</p></div>{view === 'chat' && <><button type="button" className="assistant-back" onClick={() => setView('help')}>Hilfe</button><button type="button" className="assistant-reset" onClick={() => { setMessages([greeting]); setDraft(''); inputRef.current?.focus() }} disabled={messages.length === 1}>Neu</button></>}<button type="button" className="assistant-close" onClick={() => setOpen(false)} aria-label="Hilfecenter schließen">×</button></header>
      {view === 'help' ? <div className="assistant-help-content"><section className="assistant-help-hero"><p className="assistant-eyebrow">Hallo, schön, dass du da bist</p><h3>Wie können wir dir helfen?</h3><p>Finde schnell Antworten oder schreib direkt mit unserem KI-Assistenten.</p><button type="button" onClick={openChat} disabled={!enabled}>Stelle hier deine Frage <span aria-hidden="true">→</span></button></section><div className="assistant-help-search"><SearchIcon /><label className="visually-hidden" htmlFor="help-search">Im Hilfecenter suchen</label><input id="help-search" ref={searchRef} value={search} onChange={event => setSearch(event.target.value)} placeholder="Keine Zeit? Gib ein Stichwort ein." /></div>{resultTitles.length > 0 && <div className="assistant-search-results" aria-label="Suchergebnisse">{resultTitles.map(title => <button type="button" key={title} onClick={() => openArticle(title)}>{title}<Arrow /></button>)}</div>}<section className="help-section"><h3>Empfohlene Artikel</h3><div className="help-article-list">{recommendedArticles.map(([title]) => <button type="button" key={title} onClick={() => openArticle(title)}><span>{title}</span><Arrow /></button>)}</div></section><section className="help-section help-topics"><h3>Beliebte Themen</h3><div className="help-topic-list">{topics.map(([title, articles]) => <div className="help-topic" key={title}><button type="button" className="help-topic-toggle" onClick={() => setExpanded(current => current === title ? null : title)} aria-expanded={expanded === title}><span>{title}</span><span className="help-topic-icon" aria-hidden="true">{expanded === title ? '−' : '+'}</span></button>{expanded === title && <div className="help-topic-articles">{articles.map(item => <button type="button" key={item} onClick={() => openArticle(item)}>{item}<Arrow /></button>)}</div>}</div>)}</div></section><p className="assistant-help-footnote">Dein Anliegen ist nicht dabei? <button type="button" onClick={openChat} disabled={!enabled}>Frag unseren Assistenten.</button></p></div> : <><div className="assistant-privacy">{aiEnabled ? 'Deine Nachrichten werden zur Antwort an Google Gemini gesendet. Teile keine Zahlungsdaten oder vertraulichen Angaben.' : 'Deine Produktsuche wird direkt mit dem MITTE-Sortiment abgeglichen. MITTE speichert diesen Chat nicht.'}</div><div className="assistant-transcript" ref={transcriptRef} aria-live="polite">{messages.map((message, index) => <div className={`assistant-message ${message.role}`} key={`${index}-${message.role}`}><p>{message.content}</p>{message.products?.length > 0 && <ProductMatches products={message.products} go={go} add={add} />}</div>)}{messages.length === 1 && enabled && <div className="assistant-suggestions">{prompts.map(prompt => <button type="button" key={prompt} disabled={busy} onClick={() => ask(prompt)}>{prompt}</button>)}</div>}{status && !enabled && <div className="assistant-unavailable" role="status"><p>{statusError ? 'Der Assistent ist gerade nicht erreichbar. Prüfe deine Verbindung und versuche es erneut.' : 'Der Assistent ist gerade nicht eingerichtet. Bitte versuche es später erneut.'}</p><button type="button" className="assistant-retry" onClick={refreshStatus}>Erneut prüfen</button></div>}{busy && <p className="assistant-thinking" role="status">Ich suche im Sortiment …</p>}</div><form className="assistant-composer" onSubmit={event => { event.preventDefault(); ask(draft) }}><label className="visually-hidden" htmlFor="assistant-message">Deine Frage an MITTE</label><textarea id="assistant-message" ref={inputRef} value={draft} maxLength={1200} rows={2} placeholder="Stelle deine Frage …" onChange={event => setDraft(event.target.value)} onKeyDown={event => { if (event.key === 'Enter' && !event.shiftKey) { event.preventDefault(); ask(draft) } }} disabled={!enabled || busy} /><button type="submit" disabled={!enabled || busy || !draft.trim()}>Senden <span aria-hidden="true">↑</span></button></form></>}
      {article && <div className="assistant-article-backdrop"><article className="assistant-article"><button type="button" className="assistant-article-close" onClick={() => setArticle(null)} aria-label="Artikel schließen">×</button><p className="assistant-eyebrow">Hilfecenter</p><h3>{article.title}</h3><p>{article.body}</p><button type="button" className="assistant-article-chat" onClick={openChat} disabled={!enabled}>Noch Fragen? Chat starten</button></article></div>}
    </section>}
    <button type="button" className="assistant-launcher" ref={launcherRef} onClick={() => setOpen(value => !value)} aria-expanded={open} aria-controls="assistant-dialog" aria-label={open ? 'Hilfecenter schließen' : 'MITTE Hilfecenter öffnen'}><span className="assistant-launcher-mark" aria-hidden="true">M</span><span>Hilfe &amp; Chat</span></button>
  </div>
}
