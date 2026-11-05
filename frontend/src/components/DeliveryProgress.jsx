import { euro } from '../lib/api.js'

export function DeliveryProgress({ quote }) { const remaining = quote.RemainingForFreeShippingCents || 0; const unlocked = quote.FreeShippingUnlocked; const progress = unlocked ? 100 : Math.min(100, Math.round((1 - remaining / 7500) * 100)); return <div className="delivery-progress"><span>{unlocked ? 'Versand ist kostenlos für deine Auswahl.' : <>Noch <strong>{euro(remaining)}</strong> bis zum kostenlosen Versand.</>}</span><div aria-label={`${progress}% bis kostenloser Versand`}><i style={{ width: `${progress}%` }} /></div></div> }
