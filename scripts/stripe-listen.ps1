# Starts Stripe CLI forwarding for local checkout testing. It intentionally
# never writes the returned webhook secret to disk; copy it into ignored .env.
$stripe = Get-Command stripe -ErrorAction SilentlyContinue
if (-not $stripe) {
    throw 'Stripe CLI is required. Install it from https://docs.stripe.com/stripe-cli, then run stripe login.'
}

Write-Host 'Forwarding Stripe test webhooks to http://localhost:8080/webhooks/stripe'
Write-Host 'Copy the displayed whsec_... value into the ignored .env file as STRIPE_WEBHOOK_SECRET.'
& $stripe.Source listen --forward-to http://localhost:8080/webhooks/stripe
