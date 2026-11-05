---
name: MITTE Storefront
description: A confident, search-first German marketplace that makes a broad assortment easy to explore.
colors:
  ink: "#142e50"
  navy: "#193d68"
  navy-wash: "#e9eff5"
  orange: "#ee5b32"
  yellow: "#ffcf55"
  paper: "#ffffff"
  ground: "#f5f6f7"
  line: "#e1e5e9"
  muted: "#5f6b7b"
typography:
  display:
    fontFamily: "DM Serif Display, Georgia, serif"
    fontSize: "clamp(43px, 5.2vw, 72px)"
    fontWeight: 400
    lineHeight: 0.99
    letterSpacing: "-0.035em"
  body:
    fontFamily: "Manrope, Arial, sans-serif"
    fontSize: "14px"
    fontWeight: 400
    lineHeight: 1.6
  label:
    fontFamily: "Manrope, Arial, sans-serif"
    fontSize: "10px"
    fontWeight: 800
    letterSpacing: "0.09em"
    lineHeight: 1.4
  operations-display:
    fontFamily: "Manrope, Arial, sans-serif"
    fontSize: "clamp(27px, 3vw, 38px)"
    fontWeight: 750
    lineHeight: 1.08
    letterSpacing: "-0.04em"
  operations-section:
    fontFamily: "Manrope, Arial, sans-serif"
    fontSize: "19px"
    fontWeight: 750
    lineHeight: 1.2
  operations-body:
    fontFamily: "Manrope, Arial, sans-serif"
    fontSize: "12px"
    fontWeight: 400
    lineHeight: 1.6
  operations-control:
    fontFamily: "Manrope, Arial, sans-serif"
    fontSize: "12px"
    fontWeight: 750
    lineHeight: 1.4
  operations-mark:
    fontFamily: "Manrope, Arial, sans-serif"
    fontSize: "17px"
    fontWeight: 900
rounded:
  sm: "7px"
  md: "11px"
  lg: "15px"
spacing:
  sm: "8px"
  md: "16px"
  lg: "24px"
  xl: "40px"
components:
  button-primary:
    backgroundColor: "{colors.orange}"
    textColor: "{colors.paper}"
    rounded: "{rounded.sm}"
    padding: "13px 17px"
  button-primary-hover:
    backgroundColor: "#c74425"
---

# Design System: MITTE Storefront

## Overview

**Creative North Star: "The Useful Find"**

MITTE is a German multi-category marketplace. It should feel familiar enough to shop quickly, but have a clear visual identity: confident navy, crisp white product surfaces, warm orange actions, and a search-led entry point. Help shoppers find products across technology, home, beauty, fashion, tools, food, family, pets, and travel without implying that demo products, prices, stock, popularity, or recommendations are real.

The storefront visual system applies to public shopping routes. Staff operations retain their purpose-built interfaces. Existing checkout, payment, inventory, seller, and legal constraints remain authoritative.

### Staff operations
OMS is a separate task surface within the MITTE identity: ink/navy navigation, a neutral canvas, paper work areas, orange active actions, and compact Manrope typography. Operations screens prioritize real order state, fast lookup, and keyboard-accessible tables; they do not inherit the storefront's editorial display treatment.

**Key Characteristics:**
- Search and category discovery are visible before editorial content.
- Orange marks the primary action; navy anchors navigation and trust cues.
- Product cards are structured and scannable, with real catalog price and availability.
- Personal recommendations and viewed-product history are shown only after account consent.

## Colors

### Primary
- **Navy** (#193d68): Main navigation, primary dark surfaces, and dependable emphasis.
- **Orange** (#ee5b32): Primary actions, active discovery cues, and small highlights.
- **Warm Yellow** (#ffcf55): Compact counters and occasional directional accents.

### Neutral
- **Ink** (#142e50): Main text and high-contrast headings.
- **Paper** (#ffffff): Storefront and product surfaces.
- **Ground** (#f5f6f7): Secondary panels and quiet image areas.
- **Line** (#e1e5e9): Card borders and separators.
- **Muted** (#5f6b7b): Supporting copy and metadata.

### Named Rule
**The Useful Accent Rule.** Use orange for the next useful action, navy for stable navigation, and yellow sparingly. Do not use color as a substitute for state text or accessible contrast.

## Typography

**Display:** DM Serif Display, with Georgia and serif fallbacks. **Body and labels:** Manrope, with Arial and sans-serif fallbacks.

### Hierarchy
- **Display:** 400 weight, responsive 43–72px, compact line height; reserve for the home proposition.
- **Headline:** 750–800 weight, responsive 27–38px; section and catalog headings.
- **Title:** 700–800 weight, 13–19px; product names and card prices.
- **Body:** 400–500 weight, 12–15px, 1.45–1.7 line height; descriptions and guidance.
- **Label:** 700–850 weight, 9–11px; categories, state labels, and wayfinding.

**The Two-Family Rule.** Use DM Serif Display only for the home proposition; keep navigation, product information, controls, and supporting copy in Manrope.

## Layout

The home page opens with a search-first hero and direct entry to the broad assortment. Prominent category entry points prioritize technology, home, beauty, fashion, sports, tools, kitchen, and garden. The category grid and catalog navigation expose the full nested taxonomy. Product feeds use four columns on wide screens and two on compact screens. At 760px, the hero and account layouts stack and navigation becomes a touch-friendly menu.

The catalog keeps search, sorting, category state, and product results legible together. Child and grandchild categories are revealed in context. Product detail places product media beside product and purchase information on desktop, then stacks them on mobile.

## Elevation and Shapes

Use a white canvas, subtle 1px borders, and small hover movement; avoid persistent floating shadows. Product cards and images have a modest 11–12px radius, controls use compact 7–8px radii, and larger editorial panels may use 15px. Keep card geometry consistent and ensure focus indicators remain visible.

## Components

### Buttons and Inputs
- Primary actions use orange with white text and explicit verbs.
- Secondary actions use white or a navy wash with a visible border.
- Search and form inputs have clear labels, visible focus, and adequate touch targets.
- Loading, empty, and failed states are distinct; a failed API request must not appear as a successful empty assortment.

### Navigation
- The search field remains prominent on desktop and mobile.
- Show the most useful category roots first, then a clear all-categories route.
- Mobile navigation uses full-width, keyboard-accessible rows; category selection does not depend on hover.

### Product Card
Use an image, category, readable name, price, availability, save control, and add-to-cart action. Disable purchase actions when unavailable. Only badge products as bestsellers when confirmed paid or fulfilled order data supports it. Use demo labels and disclaimers for synthetic development fixtures.

### Personalization
Personalized recommendations and recent views are available to signed-in customers only after explicit opt-in. Explain the purpose and the effect of withdrawal; disabling consent deletes stored view history. Without consent, do not record product views or render personal history.

## Do's and Don'ts

### Do
- Make search, categories, price, availability, save, cart, account, and retry actions easy to find.
- Keep filters and nested categories usable on small screens and with a keyboard.
- Use real catalog information for production storefront claims.
- Distinguish real purchase-ranked bestsellers from neutral featured products.

### Don't
- Invent supplier names, product claims, delivery promises, popularity, prices, or stock for production.
- Store viewed-product history before consent or after consent is withdrawn.
- Treat a failed catalog request as a successful empty result.
- Ship synthetic demo catalog data to production.
