const money = new Intl.NumberFormat('de-DE', { style: 'currency', currency: 'EUR' })

export const formatEuro = cents => money.format(cents / 100)
