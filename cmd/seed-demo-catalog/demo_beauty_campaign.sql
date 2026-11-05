INSERT INTO promotions (
    code, name, kind, percent_bps, amount_cents, min_subtotal_cents, category_slug,
    active, starts_at, ends_at, per_email_limit, public_campaign, auto_apply,
    campaign_slug, headline, description, image_url, image_alt, cta_label,
    featured, demo_only
)
VALUES (
    'DEMO-BEAUTY-15', 'Demo: Beauty & Pflege', 'percent', 1500, 0, 0, 'beauty',
    TRUE, NOW() - INTERVAL '1 day', NOW() + INTERVAL '30 days', 0, TRUE, TRUE,
    'demo-beauty-days', 'Beauty & Pflege mit Demo-Rabatt',
    'Diese lokale Demo-Aktion nutzt synthetische Katalogpreise. Rabatt und Ersparnis sind simuliert und kein echtes Preisangebot.',
    'https://images.unsplash.com/photo-1556228720-195a672e8a03?auto=format&fit=crop&w=1100&q=80',
    'Beispielmotiv für Beauty und Pflege', 'Beauty entdecken', TRUE, TRUE
)
ON CONFLICT (code) DO UPDATE SET
    name = EXCLUDED.name, kind = EXCLUDED.kind, percent_bps = EXCLUDED.percent_bps,
    amount_cents = EXCLUDED.amount_cents, min_subtotal_cents = EXCLUDED.min_subtotal_cents,
    category_slug = EXCLUDED.category_slug, active = EXCLUDED.active,
    starts_at = EXCLUDED.starts_at, ends_at = EXCLUDED.ends_at,
    per_email_limit = EXCLUDED.per_email_limit, public_campaign = EXCLUDED.public_campaign,
    auto_apply = EXCLUDED.auto_apply, campaign_slug = EXCLUDED.campaign_slug,
    headline = EXCLUDED.headline, description = EXCLUDED.description,
    image_url = EXCLUDED.image_url, image_alt = EXCLUDED.image_alt,
    cta_label = EXCLUDED.cta_label, featured = EXCLUDED.featured,
    demo_only = EXCLUDED.demo_only
RETURNING 1;
