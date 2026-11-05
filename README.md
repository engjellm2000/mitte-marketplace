# MITTE

MITTE ist ein deutschsprachiger, mehrkategorischer Marktplatz für Technik, Wohnen, Beauty, Mode, Sport, Küche, Garten, Familie, Büro, Haustiere, Genuss, Mobilität, Werkzeug und Reisen. Das Backend setzt auf Go, PostgreSQL und parametrisierte SQL-Abfragen ohne ORM.

## Architektur

Mitte folgt einem **modularen Monolithen mit pragmatischen Ports-and-Adapters-Grenzen**. Ein gemeinsames Go-Modul und PostgreSQL halten transaktionale Abläufe wie Checkout, Bestand und Rechnungen zusammen; `cmd/` enthält getrennte Prozess-Einstiegspunkte für Web, OMS und Hintergrunddienste. `internal/` kapselt Domänenregeln, Adapter und Transportgrenzen. Neue fachliche Verträge sollen in ihrem eigenen Paket liegen statt von einem Transport- oder Persistenzpaket ausgeliehen zu werden; das gemeinsame Rechnungsprofil liegt deshalb in `internal/invoiceprofile/`. Die React-Storefront bleibt ein eigener Frontend-Build, der vom Go-Server ausgeliefert wird.

Die HTTP-Routen sind in `internal/web/routes.go` nach Storefront, Konten, Zahlungen, Betrieb und Katalog gruppiert. Handler bleiben in den zugehörigen Feature-Dateien, damit neue Endpunkte an einer klaren fachlichen Grenze ergänzt werden können. Im Frontend liegen Seiten nach Arbeitsbereich unter `frontend/src/pages/`, gemeinsame UI-Bausteine unter `frontend/src/components/` und geteilte Funktionen unter `frontend/src/lib/`.

Weitere Entkopplung sollte schrittweise an getesteten Anwendungsfällen beginnen: Der Checkout läuft über einen Anwendungsdienst mit Repository- und Zahlungs-Ports; `internal/commerce` enthält die transport- und persistenzunabhängigen Checkout-Verträge. Andere Web-Handler greifen weiterhin direkt auf `store.Store` zu. Dort können später kleine, fachlich zugeschnittene Interfaces die Persistenzadapter ersetzen; eine Aufteilung in zusätzliche Services wäre erst gerechtfertigt, wenn unabhängige Skalierung oder organisatorische Grenzen das erfordern.

## Repository-Status (aktueller Stand)

Das Repository ist aktuell kein Microservice-Ökosystem, sondern eine einzelne Commerce-App mit mehreren klar abgegrenzten Prozessen:

- `cmd/server` startet den HTTP-Server, die gRPC-Schnittstelle und die Datenbankmigrationen; die App lädt `.env` automatisch, wenn sie direkt mit `go run` gestartet wird.
- `cmd/order-relay`, `cmd/email-relay`, `cmd/payment-expiry`, `cmd/reconcile-stripe`, `cmd/migrate` und `cmd/seed-demo-catalog` sind dedizierte Hilfsprozesse für Outbox, E-Mails, Zahlungslaufzeiten und lokale Demo-Daten.
- `internal/store` enthält das PostgreSQL-Repository, Migrations und die Kernlogik von Bestellungen, Katalog, Lager und Zahlungsevents.
- `internal/web` übernimmt Storefront-, Account-, Zahlungs- und OMS-HTTP-Endpunkte; `internal/oms` kapselt Operations- und Fulfillment-Flows.
- `frontend` ist ein Vite/React-Projekt für die public-facing Storefront und die internen Betriebsoberflächen; das Go-Backend rendert die serverseitigen HTML-Templates und liefert die JSON-APIs.
- Die öffentliche Marke heißt MITTE, während Code, Umgebungsvariablen und Datenbanknamen überwiegend `mitte` verwenden; das ist eine beabsichtigte, einheitliche Projektbezeichnung und kein getrenntes Subsystem.

## Portfolio-Überblick

MITTE ist bewusst als End-to-End-Produkt statt als reine Shop-Oberfläche gebaut. Die Storefront verbindet ein responsives, such- und kategorieorientiertes Einkaufserlebnis mit den Abläufen, die bei einem realen Handelssystem entscheidend sind: persistentem Warenkorb, Merkliste, Konten, Gutscheinen, Bestandsprüfung, idempotentem Checkout und asynchroner Bestellverarbeitung.

| Bereich | Sichtbares Feature | Technische Umsetzung |
| --- | --- | --- |
| Entdecken | Suche, Kategorien, Sortierung, Produktdetailseiten | React Storefront + Go JSON API |
| Beratung | Gemini-Einkaufshilfe mit Live-Katalogsuche, Produktvergleich und Produktkarten | Server-seitige Gemini-API, begrenzter Funktionsaufruf, keine gespeicherten Chats |
| Kaufen | Warenkorb, Gutscheine, Versandschwelle, Stripe Checkout | PostgreSQL, CSRF, Idempotency-Key |
| Marketing | Öffentliche Angebotsseiten, geplante Kampagnen und automatisch angewendete Rabatte | Bestehende Promotions, CSRF-geschützte Mitarbeiterverwaltung, serverseitige Checkout-Quote |
| Wiederkommen | Konto, Session, Merkliste auf dem Gerät | Argon2id, widerrufbare Cookies, Local Storage |
| Betrieb | Webhooks, Outbox, Kafka-Relay, Health Checks, Metriken | Go, PostgreSQL, Kafka, Prometheus |

**Demo-Hinweis:** Das optionale MITTE-Demosortiment besteht aus synthetischen Produktdaten und Platzhalterbildern und ist ausschließlich für lokale Entwicklung gedacht. Zahlungs- und Webhook-Flows sind für Stripe Test Mode ausgelegt; vor einem echten Launch müssen rechtliche Texte, Lieferantendaten, Produktmedien und die im Abschnitt „Vor dem Launch“ beschriebenen Integrationen ersetzt beziehungsweise ergänzt werden.

**Markenhinweis:** MITTE ist aktuell der Arbeitsname des Shops. Marken-, Firmen- und Domainverfügbarkeit sowie die rechtliche Zuordnung des Verkäufers müssen vor dem Livebetrieb geprüft werden.

Die Einkaufshilfe ist aktiv, sobald der Shop mit Datenbank läuft: Ohne `GEMINI_API_KEY` sucht sie direkt im Live-Sortiment (Kategorien, Budgets, Vergleiche, Produktkarten). Mit gesetztem Schlüssel und funktionierendem Google-Gemini-Zugang (Billing erforderlich) antwortet zusätzlich das Sprachmodell mit Tool-Aufrufen gegen denselben Katalog. `GEMINI_MODEL` ist optional und steht standardmäßig auf `gemini-3.8-flash`. Der Schlüssel bleibt ausschließlich im Go-Server. Bei KI-Modus werden Chatnachrichten an Google Gemini übertragen; MITTE speichert den Gesprächsverlauf nicht. Bei Gemini-Ausfällen greift automatisch die katalogbasierte Suche. Vor dem Produktivstart müssen Anbieter-, Datenschutzangaben und Rechtsgrundlage geprüft werden.

## Was enthalten ist

- Deutsche, responsive Storefront mit Katalog, Suche, Kategorien und Produktdetailseiten
- Mehrstufige Marktplatz-Kategorien für die wichtigsten deutschen Einzelhandelsbereiche
- Bestseller-Rangfolge ausschließlich aus bestätigten bezahlten oder erfüllten Bestellpositionen
- Personalisierte Empfehlungen und Produktverlauf nur nach ausdrücklicher Konto-Zustimmung; Widerruf löscht den gespeicherten Verlauf
- Optionaler MITTE-Einkaufsassistent mit Google Gemini: sucht im aktiven Sortiment, nutzt aktuelle Preise und Bestände und zeigt passende Produktkarten; Warenkorbänderungen erfolgen nur durch den Kunden über diese Karten
- Merkliste mit persistiertem lokalen Speicher, direkt aus Katalog und Produktdetailseite bedienbar
- Server-Rendering mit Go `html/template` und kleine progressive JavaScript-Interaktionen
- Cookie-basierter Warenkorb, der in PostgreSQL persistiert wird
- Transaktionaler Checkout mit Bestellungen, Bestellpositionen und Bestandssperren gegen Ueberverkaeufe
- Cookie-basierter Warenkorb mit CSRF-Schutz, Request-Limits und sicheren Standard-Headern
- Transparente Cookie-Informationen im Shop; nur notwendige Warenkorb-, Sitzungs-, Sicherheits- und Anmelde-Cookies, keine eingebundenen Analyse- oder Werbe-Cookie-Skripte
- Kundenkonten mit Argon2id-Passwort-Hashes, serverseitig widerrufbaren Sitzungen und CSRF-geschützten Auth-Endpunkten
- Order-Events über eine transaktionale Outbox nach Kafka, mit mindestens-einmaliger Zustellung und Reihenfolge pro Bestellung
- HTTP Live- und Readiness-Checks unter `/healthz` und `/readyz`, plus standardisierter gRPC-Health-Check im privaten Netz
- Strukturierte JSON-Logs, Request-IDs, Timeouts und Graceful Shutdown
- Migrations-Ledger; Demo-Seeds werden nie im Produktionsmodus ausgefuehrt
- Gehaertetes Container-Image, CI mit Race- und Vulnerability-Checks

## Weitere Dokumentation

- [Produktüberblick](PRODUCT.md): Zielgruppe, Funktionsumfang und Produktgrenzen.
- [Designsystem](DESIGN.md): visuelle Richtlinien für Storefront und interne Arbeitsbereiche.
- [DigitalOcean-Deployment](deploy/digitalocean/README.md) und [Kubernetes-Beispiel](deploy/kubernetes/production-example/README.md): deployment-spezifische Hinweise.

## Voraussetzungen und Entwicklung

Für den vollständigen lokalen Stack werden Docker mit Compose und eine Kopie von `.env.example` benötigt. Die erforderliche Go-Version steht in `go.mod`. Für Frontend-Entwicklung und -Build werden Node.js 22 und pnpm 10 verwendet, wie auch in der CI.

Die wichtigsten Prüfungen für Backend und Frontend:

```bash
make test
make test-race
make vet
make build
cd frontend
pnpm install --frozen-lockfile
pnpm run build
```

PostgreSQL-Integrationstests benötigen eine separate, wegwerfbare Testdatenbank; Details stehen unter [Betrieb: Release, Monitoring und Incident Response](#betrieb-release-monitoring-und-incident-response). Produktionswerte und echte Zugangsdaten gehören nicht in `.env.example` oder ins Repository.

## Lokal starten

Mit Docker:

Vor dem ersten Start kopiere `.env.example` nach `.env`, setze ein eigenes lokales Datenbankpasswort und ein zufälliges `MITTE_PAY_OTP_SECRET` mit mindestens 32 Zeichen (zum Beispiel `openssl rand -hex 32`). Wenn du interne Arbeitsbereiche nutzen möchtest, konfiguriere außerdem `STAFF_CREDENTIALS` und die passenden `STAFF_ROLES`; es gibt kein eingebautes Standard-Admin-Passwort mehr.

```bash
docker compose up --build
```

Danach ist der Shop unter [http://localhost:8080](http://localhost:8080) erreichbar. Neue Datenbanken enthalten absichtlich keine Produkte, Kunden, Bestellungen oder Demo-Rabatte. So erscheinen keine erfundenen Produkt-, Preis-, Lager- oder Herstellerangaben im Shop.

### Isoliertes lokales Staging

Für einen lokalen Release-Kandidaten kannst du denselben Entwicklungs-Compose-Stack mit eigenen Volumes und anderen Loopback-Ports starten. Erstelle `.env.local-staging` als lokale, ignorierte Kopie von `.env.example`, nicht aus einer möglicherweise vorhandenen `.env`. Setze darin ein eigenes `POSTGRES_PASSWORD`, ein `MITTE_PAY_OTP_SECRET` mit mindestens 32 zufälligen Zeichen, `OMS_API_KEY`, `OMS_USERNAME=operations`, `OMS_PASSWORD` und `SMTP_ADDRESS=mailpit:1025`. Verwende nur lokale Testwerte; setze keine Produktions- oder Live-Stripe-Zugangsdaten ein. Das Setup bleibt `APP_ENV=development`: Es eignet sich für lokale UI-, E-Mail-, Kafka- und Bestellabläufe, ist aber kein Ersatz für HTTPS/Proxy-, TLS/SASL-, verwaltete Datenbank-PITR-, Provider- oder Kapazitätstests.

Die abweichenden Ports verhindern, dass diese Instanz deinen normalen lokalen Compose-Stack unterbricht:

```powershell
docker compose --project-name mitte-local-staging --env-file .env.local-staging -f compose.yaml -f compose.local-staging.yaml config --quiet
docker compose --project-name mitte-local-staging --env-file .env.local-staging -f compose.yaml -f compose.local-staging.yaml up --build -d
docker compose --project-name mitte-local-staging --env-file .env.local-staging -f compose.yaml -f compose.local-staging.yaml ps
```

Der Shop ist unter `http://localhost:18080` erreichbar; die kanonische OMS-Oberfläche liegt unter `http://localhost:18080/oms`. Port `18081` stellt nur die separate Integrations-API bereit und keine zweite Oberfläche; Mailpit unter `http://localhost:18025`. Die Datenbank läuft isoliert auf `127.0.0.1:15432`. Die App führt Migrationen beim lokalen Start aus. Nutze ausschließlich synthetische Katalog- und Kundendaten. Für E-Mail-Verifizierung und Passwort-Reset kannst du empfangene Nachrichten in Mailpit öffnen; für Bestellungen steht die ausdrücklich lokale Zahlungs-Sandbox zur Verfügung. Ein echter Stripe-Testmodus braucht eigene Stripe-Testschlüssel und einen lokalen Webhook-Forwarder; echte Zahlungen werden in diesem Setup nicht getestet.

#### Lokales Monitoring

Die lokale Staging-Instanz kann optional mit Grafana, Prometheus, Alertmanager,
Loki, Tempo und dem OpenTelemetry Collector gestartet werden. Dieses Overlay ist nur für lokale Tests gedacht: Slack-Benachrichtigungen sind
deaktiviert, und Grafana, Prometheus sowie Alertmanager sind nur über Loopback
erreichbar. Das lokale Grafana-Passwort lautet standardmäßig
`local-monitoring-only`; du kannst vor dem Start in `.env.local-staging` mit
`GRAFANA_ADMIN_PASSWORD` ein eigenes lokales Passwort setzen.

```powershell
docker compose --project-name mitte-local-staging --env-file .env.local-staging -f compose.yaml -f compose.local-staging.yaml -f compose.local-monitoring.yaml up -d
```

Öffne Grafana unter `http://localhost:3000` (Benutzer `admin`), Prometheus
unter `http://localhost:9090` und Alertmanager unter `http://localhost:9093`.
Das Mitte-Dashboard wird automatisch bereitgestellt. Prometheus sammelt
Metriken von Shop, OMS und Workern sowie HTTP-Health-Probes; Logs und Traces
erscheinen, sobald die Dienste mit diesem Overlay Anfragen verarbeitet haben.
Zum Stoppen einschließlich Monitoring verwende dieselben Compose-Dateien mit
`down`.

Zum Stoppen ohne Löschen der Testdaten:

```powershell
docker compose --project-name mitte-local-staging --env-file .env.local-staging -f compose.yaml -f compose.local-staging.yaml down
```

Das Entfernen der benannten Daten-Volumes löscht die lokale Staging-Datenbank und Kafka-Daten. Führe `down -v` nur aus, wenn diese Testdaten ausdrücklich entbehrlich sind.

Für lokale UI- und Lasttests kannst du optional 1.000 synthetische MITTE-Demoprodukte pro Kategorie einspielen. Das ergibt insgesamt 1.000 Produkte je Eintrag der Kategoriehierarchie und kann bei vielen Kategorien entsprechend mehr als 50.000 Datensätze erzeugen. Diese Bulk-Artikel verwenden ausdrücklich synthetische Platzhaltertexte, Preise und Bestände sowie Picsum-Stockfotos, die nicht zwingend das Produkt zeigen. Zusätzlich legt das Kommando 578 Beauty-Produkte mit Community-Produktdaten und Packshots aus [Open Beauty Facts](https://world.openbeautyfacts.org/) an: Düfte, Shampoo und Conditioner, Gesichts- und Körperpflege, Mizellenwasser, Masken und Make-up. Die 543 zusätzlichen Artikel stammen aus produktbezogenen, nach Kategorien gefilterten API-Abfragen; Produktnamen, Marken, Barcodes, Größen und Fotos sind Original-Einträge, nicht generierte Varianten. Produktseiten nennen die jeweilige Quelle; Produktdaten stehen unter ODbL 1.0, Bilder unter CC BY-SA 3.0. Open-Beauty-Facts-Einträge können unvollständig oder falsch sein. Preise und Bestände aller Demoprodukte bleiben synthetisch. Nichts aus diesem Seed ist für Verkauf oder Veröffentlichung verifiziert oder für einen Produktionskatalog geeignet. Der lokale Seed legt außerdem die deutlich als Demo markierte Kampagne `DEMO-BEAUTY-15` an; sie demonstriert Angebotsseite, Aktionspreise und automatische Warenkorbrabatte, ist aber kein echtes Preisangebot. Das Kommando prüft `APP_ENV=development`, verlangt zusätzlich eine ausdrückliche Bestätigung und akzeptiert ausschließlich eine direkte Loopback-PostgreSQL-Verbindung:

```powershell
$env:APP_ENV = 'development'
$env:MITTE_ENABLE_DEMO_CATALOG_SEED = '1'
go run ./cmd/seed-demo-catalog
```

Für nur die 578 Beauty-Produkte ohne die synthetischen Bulk-Artikel kannst du stattdessen `go run ./cmd/seed-demo-catalog -curated-only` ausführen.

Das Seed-Kommando wendet zunächst ausstehende Datenbankmigrationen an und ist für wiederholte lokale Läufe idempotent. Der vollständige Lauf füllt alle Kategorien mit eindeutig gekennzeichneten Demoartikeln; beide Modi zeichnen synthetische Anfangsbestände auf. Vor Produktion müssen Lieferantenbeziehung, Produktdaten, Bilder, Preise und Bestände verifiziert werden; die Demo-Produkte werden nicht durch eine Migration angelegt.

Die Registrierung bietet Privat- und Geschäftskonten. Im Kundenkonto erscheinen Bestellungen und nach vollständigem Versand ausgestellte Rechnungen mit einem geschützten PDF-Link.

Persönliche Empfehlungen und der Verlauf angesehener Produkte sind standardmäßig deaktiviert. Kunden können die Nutzung von angesehenen und gekauften Produkten im Konto ausdrücklich aktivieren und jederzeit widerrufen; beim Widerruf werden gespeicherte Produktansichten gelöscht. Ohne Zustimmung werden keine Ansichten gespeichert. Die Bestsellerliste zeigt nur Produkte mit bestätigten bezahlten oder erfüllten Verkäufen.

### Cookies und Browser-Speicher

Der Shop dokumentiert seine tatsächlich verwendeten Cookies in den Cookie-Einstellungen im Seitenfuß. Warenkorb-, CSRF- und Anmelde-Cookies sind `HttpOnly`; Produktions-Cookies sind `Secure` und verwenden je nach Zweck `SameSite=Lax` oder `SameSite=Strict`. Der Gast-Warenkorb läuft nach 30 Tagen ab, der CSRF-Cookie nach einem Tag, die Kundenanmeldung nach sieben Tagen und Mitarbeitenden-Sitzungen nach acht beziehungsweise zwölf Stunden. Ein notwendiger Cookie speichert die Auswahl „nur notwendige Cookies“ für sechs Monate. Die Merkliste wird getrennt davon lokal im Browser gespeichert. Produktverlauf und Empfehlungen sind eine separate, widerrufbare Kontoeinstellung und werden nicht in einem Cookie gespeichert. Analyse- und Werbe-Skripte sind derzeit nicht eingebunden. Produktbilder können von externen Bilddiensten geladen werden; die Cookie-Auswahl verhindert diese Bildanfragen nicht.

Diese technische Bestandsaufnahme ersetzt keine vollständige Datenschutzerklärung oder rechtliche Prüfung. Vor dem Produktivbetrieb müssen insbesondere Verantwortliche, externe Bildanbieter, Verarbeitungszwecke und die dazugehörigen Datenschutzhinweise anhand der tatsächlichen Betriebsumgebung bestätigt werden.

Lokale Transaktions-E-Mails landen in Mailpit unter [http://localhost:8025](http://localhost:8025); sie werden nicht ins Internet zugestellt. Prüfe bei fehlenden Nachrichten zuerst `docker compose ps email-relay mailpit` und `docker compose logs --tail=100 email-relay`. Der Relay muss laufen, damit Nachrichten aus der Datenbank-Outbox zugestellt werden. Für einen lokalen Stripe-Test kopiere `.env.example` nach der ignorierten `.env`, setze ausschließlich **neu erzeugte Stripe-Testschlüssel** und starte in einem zweiten Terminal:

```powershell
./scripts/stripe-listen.ps1
```

Die Stripe CLI zeigt ein `whsec_...` an. Trage es nur in `.env` als `STRIPE_WEBHOOK_SECRET` ein. `.env`, `.env.*`, Schlüsseldateien und Stripe-CLI-Artefakte sind in `.gitignore` ausgeschlossen; `.env.example` bleibt bewusst als einzige Vorlage versioniert. Die Mitte-Pay-Sandbox sendet einen sechsstelligen, zehn Minuten gültigen Bestätigungscode an die beim Checkout gespeicherte E-Mail-Adresse. Für lokale Entwicklung ist das Mailpit unter [http://localhost:8025](http://localhost:8025). Für reale Zustellung setze `SMTP_ADDRESS`, `SMTP_USERNAME`, `SMTP_PASSWORD` und `SMTP_FROM` in `.env` auf einen SMTP-Anbieter mit STARTTLS (in der Regel Port 587); Compose verwendet diese Werte dann für den E-Mail-Relay.

Alternativ kannst du PostgreSQL separat starten und die App direkt ausfuehren:

```powershell
# Set DATABASE_URL to match POSTGRES_USER, POSTGRES_PASSWORD, and POSTGRES_DB from .env.
$env:PORT = '8080'
go run ./cmd/server
```

Starte in diesem Modus außerdem Mailpit (z. B. `docker compose up mailpit`) und in einem zweiten Terminal `./scripts/run-email-relay.ps1`, damit die E-Mail-Outbox abgearbeitet wird.

Die Datenbanktabellen und mehrstufigen Produktkategorien werden automatisch erstellt. Abgesehen vom optionalen lokalen Demokatalog müssen Produkte im freigeschalteten Katalogbereich mit verifizierten Herstellerangaben, SKUs/GTINs, Preisen, Beschreibungen und Lagerbeständen angelegt werden; ein Lieferantenfeed oder echter Produktdatenbestand wird nicht mitgeliefert. Beim lokalen Start lädt der Server die `.env` im Projektverzeichnis automatisch; bereits gesetzte Umgebungsvariablen haben Vorrang. Beispielwerte stehen in `.env.example`. Für den Start mit `server.exe` öffne ein Terminal in diesem Projektverzeichnis, damit die Datei gefunden wird.

### Bestellungen versenden

### Eigene Zahlungsabwicklung

Neben Stripe Checkout kann der Kunde an der Kasse **„Überweisung an Mitte"** wählen. Die App erzeugt dafür eine eindeutige Referenz und zeigt ausschließlich Empfänger, IBAN, Betrag und Verwendungszweck an. Sie verarbeitet keine Karten- oder Online-Banking-Zugangsdaten.

Für lokale Entwicklung gibt es zusätzlich **„Mitte Rechnung · Demo (30 Tage)"**. Diese eigene Rechnungs-Sandbox bildet den realistischen Ablauf nach: Freigabe → vollständiger Versand aller Positionen → Gesamtrechnung mit Bankdaten und Referenz → Eingang beim Zahlungsdienstleister → bezahlt. Erst nach vollständigem Versand wird die Rechnung samt Zahlungstransaktion angelegt. Auf der Rechnungsseite simuliert **„Überweisung simulieren"** den eingehenden Bank-Feed; der Eintrag erscheint im Zahlungsbereich und setzt die Bestellung auf bezahlt. Es werden keine Bonitätsdaten erhoben, keine echte Forderung erzeugt und keine externe Zahlung verarbeitet. Die Option ist über `MITTE_INVOICE_SANDBOX_ENABLED` steuerbar, in Entwicklung standardmäßig aktiv und in Produktion zwingend deaktiviert.

Die Zahlungseingänge liegen im separaten, staff-geschützten Bereich [http://localhost:8080/zahlungen](http://localhost:8080/zahlungen). Mitarbeitende melden sich dort mit einem privaten Zugang aus `STAFF_CREDENTIALS` an; es wird kein Kundenkonto und keine E-Mail-Adresse verwendet. In Produktion enthält diese Variable ausschließlich Argon2id-Hashes (`username:$argon2id$...`), nie Klartextpasswörter. Sie können offene Überweisungen anhand des Kontoauszugs abgleichen, Zahler sowie die letzten vier IBAN-Ziffern dokumentieren und den Eingang bestätigen. Erst dann wird die Bestellung bezahlt und für den Versand freigegeben. Die Gesamtrechnung wird nach dem vollständigen Versand aller Positionen ausgestellt. Die Datenbankmigration hält dabei Provider, Referenz, Betrag, Status und den Abgleich revisionssicher getrennt von Stripe-Ereignissen fest.

Setze für den Einsatz `MITTE_TRANSFER_ENABLED=true`, `MITTE_TRANSFER_RECIPIENT` und `MITTE_TRANSFER_IBAN` in der lokalen `.env` bzw. im Produktions-Secret-Manager. In Produktion ist der Transfer standardmäßig deaktiviert, bis die beiden Kontodaten konfiguriert sind. Für Kartenzahlungen bleibt Stripe der PCI-konforme Zahlungsdienst. Die Rechnungs-Demo ist ausdrücklich kein Ersatz für einen echten Kartenprozessor oder kauf-auf-Rechnung-Risikodienst wie Billie; dafür sind vor einem Livebetrieb eine vertragliche Provider-Integration, Bonitäts-/Betrugsprüfung, Webhooks und rechtlich geprüfte Rechnungsprozesse erforderlich.

Das interne OMS liegt unter [http://localhost:8080/oms](http://localhost:8080/oms). Die Versand- und Lagerzentrale ist unter [http://localhost:8080/verwaltung](http://localhost:8080/verwaltung) verfügbar; Bestandskontrolle und Wareneingänge laufen unter [http://localhost:8080/bestand](http://localhost:8080/bestand), die Produktpflege unter [http://localhost:8080/admin/catalog](http://localhost:8080/admin/catalog). Diese Bereiche sind nicht öffentlich. Lokal können `username:password`-Paare in `STAFF_CREDENTIALS` verwendet werden; in Produktion muss jeder Zugang als `username:$argon2id$...`-Hash hinterlegt und in `STAFF_ROLES` einer Rolle zugeordnet werden (`admin`, `cs`, `operations`, `orders`, `warehouse`, `catalog`, `inventory` oder `payments`). Die Rolle `cs` erlaubt den Zugriff auf Kunden- und Bestellvorgänge, aber nicht auf Lager, Zahlungen oder Fraud-Markierungen. Nur `admin` darf Fraud markieren oder die Markierung entfernen; diese Rolle hat systemweite Rechte. Weise jeder Person einen eigenen Zugang zu und wähle die kleinste passende Rolle. Produktionsstarts ohne vollständige Rollenzuordnung werden abgelehnt. Mitarbeitenden- und OMS-Sitzungen sind serverseitig widerrufbar. Für den Zahlcode-Sandbox-Flow muss `MITTE_PAY_OTP_SECRET` mindestens 32 zufällige Zeichen enthalten. Starte die App danach neu.

Das separate Lieferantenportal ist unter [http://localhost:8080/lieferanten](http://localhost:8080/lieferanten) erreichbar. Es verwendet eigene, je Lieferantenkonto getrennte Zugangsdaten aus `SUPPLIER_CREDENTIALS` im Format `benutzername:Lieferantenname:passwort`; in Produktion ausschließlich Argon2id-Passworthashes verwenden und Konten im Secret-Manager pflegen. Der Lieferantenname muss mit dem Feld „Lieferant“ des Produktstamms übereinstimmen. Neue Bestellpositionen übernehmen diesen Lieferanten beim Checkout als Snapshot; ältere Bestellungen werden aus Datenschutz- und Historiengründen nicht nachträglich zugeordnet. Lieferanten sehen nur bezahlte Aufträge mit eigenen Positionen und der für den Versand nötigen Lieferanschrift. Eine Lieferantenmeldung kann eine gemeinsame Sendung mit mehreren Positionen und Teilmengen enthalten; das OMS prüft je Position die noch offene Menge und speichert Carrier, Tracking, Herkunft und Auditverlauf transaktional. Der Kundenversandhinweis wird pro Sendung verschickt; eine einzige Gesamtrechnung wird erst mit dem Versand aller Bestellmengen erzeugt. Beim Upgrade werden identische historische Meldungen anhand der Trackingdaten zusammengeführt und importierte Mengen auf die bestellte Menge begrenzt; die ursprünglichen Lieferantenereignisse bleiben als Audit-Historie erhalten. Bestätigungen und Verzögerungen erscheinen ebenfalls in der bestehenden OMS-Positionshistorie. Eine Stornierungsanfrage storniert oder erstattet nichts automatisch; Mitarbeitende prüfen sie unter `/oms/supplier-requests` und schließen die Auftrags-/Zahlungsentscheidung im OMS ab. Der Lieferantenversand-Endpunkt lautet `POST /api/supplier/orders/{orderId}/shipments`.

In der Versandzentrale erscheinen nur Bestellungen mit eigenständig zu kommissionierenden Positionen. Nach Eingabe von Versanddienstleister, Sendungsnummer und optionalem HTTPS-Tracking-Link erfasst die Übergabe die verbleibenden Lagerpositionen als Sendung. Eine Bestellung bleibt offen, bis alle Lieferanten- und Lagerpositionen vollständig versendet wurden; erst dann werden Bestellstatus und Gesamtrechnung abgeschlossen. Für jede Sendung wird eine Kundenmail eingereiht, die finale Sendungsmail enthält zusätzlich die PDF-/XML-Rechnungslinks. Rechnung, Outbox und `invoice.issued`-Event werden atomar mit dem vollständigen Versand committed. Lokal lässt sich die E-Mail in Mailpit prüfen; in Produktion müssen `email-relay` und der echte SMTP-Anbieter laufen.

Kundenkonten können Marketing-E-Mails separat und freiwillig aktivieren; die Einstellung ist standardmäßig aus und steht nur mit bestätigter E-Mail-Adresse zur Verfügung. Mitarbeitende mit Katalogberechtigung verwalten Entwürfe unter `/admin/email-marketing`. Vor dem Einreihen verlangt die Oberfläche eine sichtbare Bestätigung der aktuellen Empfängerzahl; versendet wird ausschließlich an bestätigte Konten mit aktiver Einwilligung. Jede Kampagne enthält einen persönlichen Abmeldelink und RFC-8058-One-Click-Abmeldung; offene Versandaufträge werden beim Widerruf unterdrückt, eine bereits laufende SMTP-Zustellung kann aber noch abgeschlossen werden. Transaktionsmails werden zusätzlich als responsive HTML- und Textalternative mit MITTE-Absender gestaltet. Für den Produktivbetrieb SMTP-Absender, Domain-Authentifizierung und rechtlich erforderliche Unternehmens-/Datenschutzhinweise anhand der tatsächlichen Konfiguration prüfen; die technische Implementierung ersetzt keine rechtliche oder Zustellbarkeitsprüfung.

Der Link enthält ein zufälliges, nicht erratbares Zugriffstoken; in der Datenbank wird nur dessen SHA-256-Hash gespeichert. Damit können auch Gastkunden die Dokumente öffnen, ohne ein Kundenkonto oder eine aktive Sitzung zu benötigen. Das Token ist ein Bearer-Zugang und muss wie ein Passwort behandelt werden. Die Belege liegen unter `/api/invoices/{token}` als PDF (`?format=pdf`) sowie UBL-XML (`?format=xml`); beide Antworten sind privat und nicht cachebar. Dieselbe Transaktion schreibt `shipment.dispatched` und `invoice.issued` in die bestehende PostgreSQL-Outbox. Kafka verteilt diese Events nur an nachgelagerte Systeme und ist nicht Teil der kritischen Ausstellungskette. Bis eine länderspezifische Steuer- und Versandregel-Engine integriert ist, lässt das Backend nur Lieferadressen in Deutschland zu; die bisherige Preislogik berechnet den deutschen Regelsteuersatz von 19 % auf alle Produkte.

Für eine externe Verwaltungsoberfläche oder Fulfillment-Integration steht zusätzlich die staff-geschützte Detail-API bereit:

```text
GET  /api/operations/orders/{orderId}
POST /api/operations/orders/{orderId}/notes
POST /api/operations/orders/{orderId}/emails/summary
POST /api/operations/orders/{orderId}/items/action
POST /api/operations/orders/{orderId}/items/operation
POST /api/operations/orders/{orderId}/ship
POST /api/admin/customer-fraud/{customerId}
```

Die Bestellliste unterstützt `GET /api/operations/orders?q={query}&status={status}&page={page}&limit={limit}` mit Suche nach Bestellnummer, Kundenname/-E-Mail und Firmenname. `status` akzeptiert `all`, `open`, `paid`, `pending_payment`, `fulfilled` oder `cancelled`; Seiten enthalten standardmäßig 25 und höchstens 100 Einträge. Die paginierte Antwort enthält `orders`, `total`, `page`, `pageSize`, `pages` sowie unfiltrierte Gesamtkennzahlen unter `summary`, damit Kennzahlen nicht nur die aktuelle Seite abbilden. Aufrufe ohne diese Parameter behalten die bisherige vollständige Listenantwort.

Die Detailantwort enthält die vollständige Bestellsnapshot mit Kundendaten, Konto-/Beschaffungstyp, Positionen, Rechnungs- und Lieferadresse, Zahlung, Rechnung, allen Sendungen samt Positionen und Mengen sowie interne Notizen, E-Mail-Zustellungen (inklusive Wiederholungen/Fehlern), Statusverlauf und Zahlungsereignisse. Die API ist ausschließlich nach Anmeldung mit einem privaten `STAFF_CREDENTIALS`-Zugang zugänglich und alle schreibenden Aufrufe benötigen den bestehenden CSRF-Schutz.

Im OMS zeigt die Bestelldetailseite oben Bestell- und Kontaktdaten (einschließlich aktuellem Kundenkonto-Profil), getrennte Rechnungs- und Lieferadressen, alle Sendungen mit Lieferant/Lager, Tracking und versendeten Mengen, Zahlungsübersicht, interne Kommentare sowie den Zustellstatus der Bestell-E-Mails. Je Bestellposition wird der Fortschritt „versandt/Bestellmenge“ angezeigt. „Bestellübersicht senden“ reiht eine vollständige Zusammenfassung mit Positionen und Adressen in die transaktionale E-Mail-Outbox ein; sie geht an die verknüpfte Konto-E-Mail, bei Gastbestellungen an die Bestell-E-Mail. Die Positionstabelle zeigt einen stabilen Mitte-SKU, Brutto-/Nettopreis, Liefertermin und individuellen Bearbeitungsstatus.

Im Kundenbereich können ausschließlich Admins ein Konto mit einer verpflichtenden Begründung als Fraud markieren oder die Markierung mit dokumentierter Begründung aufheben. Die rote Warnung mit Grund, Admin und Zeitpunkt erscheint im Kundenprofil und oben in verknüpften Bestelldetails für CS. Jede Statusänderung wird mit Akteur und Begründung protokolliert. Eine aktive Markierung verhindert neue Checkouts des Kontos (auch bei Gast-Checkout mit derselben E-Mail-Adresse), blendet bestehende Aufträge aus der Lager- und Lieferantenwarteschlange aus und blockiert serverseitig weitere Fulfillment- und Positionsschritte. Der Endpunkt `POST /api/admin/customer-fraud/{customerId}` ist Admin-only, benötigt CSRF-Schutz und erwartet `flagged` sowie eine Begründung von 5 bis 500 Zeichen.

Positionen können gesammelt für eine Rückgabe oder Ersatzanfrage ausgewählt werden. Diese Aktionen werden mit Grund und Mitarbeitenden revisionsfähig protokolliert und lösen eine Kunden-E-Mail aus; ein Retourenlabel beziehungsweise eine neue Ersatzbestellung wird dadurch noch nicht erzeugt. Stornierung und Erstattung betreffen aus Sicherheits- und Abrechnungsgründen immer die ganze Bestellung: unbezahlte Bestellungen geben ihre Reservierung frei; bezahlte Bestellungen werden nur vor dem Versand vollständig über Stripe erstattet und anschließend storniert. Teil-Erstattungen, Erstattungen anderer Zahlungsanbieter und bereits versendete Aufträge werden nicht fälschlich als erledigt markiert.

### Lager und Versand

Der öffentliche Produktkatalog unterstützt `GET /api/products?page=1&limit=24&sort=featured&category=wohnen&search=...`. `limit` ist auf 100 und der Offset auf 100.000 Produkte begrenzt; die Antwort enthält `products`, `page`, `limit` und `hasMore`. Die Shop-Ansicht lädt 24 Treffer pro Seite und sortiert direkt in PostgreSQL. Kataloglisten enthalten nur Kartendaten; die vollständigen Produktdetails werden erst beim Öffnen eines einzelnen Produkts geladen. Öffentliche Kategorie- und Produktantworten dürfen 60 Sekunden im HTTP-Cache liegen; Verfügbarkeit wird beim Hinzufügen zum Warenkorb und beim Checkout erneut serverseitig geprüft.

Die Bestandszentrale unter `/bestand` zeigt verfügbaren, reservierten und physischen Bestand, Nachbestellpunkte, Lieferanten, Einstandswerte und niedrige beziehungsweise ausverkaufte Artikel. Wareneingänge und Inventurkorrekturen sind mit Grund, Referenz und Mitarbeiterkonto in einer Chronik nachvollziehbar. Beim Anlegen eines Produkts können SKU, Barcode, Lieferant, Einstandspreis, Nachbestellpunkt, Lieferzeit und Anfangsbestand gemeinsam gespeichert werden; der Anfangsbestand wird als Eröffnungsbewegung protokolliert.

Es gilt aktuell ein gemeinsamer Lagerbestand für einen Standort. Die Produktmenge ist der **verfügbare** Bestand: Checkout und nachträgliche Zahlungsbestätigung reservieren ihn transaktional und verhindern Überverkäufe; offene Reservierungen werden beim Ablauf oder bei einer Stornierung freigegeben. Die Übersicht leitet daraus reservierte und physisch noch vorhandene Mengen ab. Der revisionsfähige Bewegungsledger erfasst Eröffnungsbestand, Wareneingänge, manuelle Korrekturen, Reservierungen und Freigaben. Mehrere Lagerorte, Transfers und automatisierte Lieferantenbestellungen sind noch nicht abgebildet.

Die physische Kommissionierung, Label-Erfassung und Carrier-Übergabe laufen in der Versandzentrale unter `/verwaltung`. Das OMS unter `/oms` zeigt denselben Fulfillment-Status an, führt die Lager-Schritte aber nicht selbst aus. Shop-Produktdetails verwenden denselben verfügbaren Bestand wie Katalog-, Bestands- und Checkout-API.

Die Rückstandsansicht unter `/oms/backlog` bündelt offene Zahlungsfälle, bezahlte Aufträge ohne Fulfillment-Eintrag und aktive Fulfillment-Schritte. Die Queue ist serverseitig durchsuchbar und paginiert und sortiert nach Bestelleingang (älteste zuerst). Da keine verbindlichen SLA- oder Liefertermindaten hinterlegt sind, kennzeichnet sie keine Aufträge als überfällig; die Bearbeitung erfolgt in den bestehenden Auftrags- und Versanddetails.

Die staff-geschützte Bestands-API bietet paginierte Suche und Filter sowie eine getrennte Bewegungschronik:

```text
GET  /api/admin/inventory?search={query}&stock={low|out|available}&page={number}
GET  /api/admin/inventory/movements?productId={productId}
POST /api/admin/inventory/receive
POST /api/admin/catalog/products/{productId}/inventory
```

### Isolierter OMS-Service

Cookie-authentifizierte Schreibaktionen in der OMS-Konsole akzeptieren nur Anfragen mit passendem `Origin`-Host. Integrationen mit `Authorization: Bearer ...` bleiben davon unabhängig.

Die OMS-Integrations-API läuft als eigener Container; sie ist kein zweites Backoffice. Mitarbeitende nutzen ausschließlich die im Web-Container integrierte Oberfläche unter [http://localhost:8080/oms](http://localhost:8080/oms), die dieselben Bestell- und Fulfillment-Daten aus PostgreSQL anzeigt. Für technische Integrationen bleibt die API lokal unter `127.0.0.1:8081` verfügbar und wird mit `Authorization: Bearer <OMS_API_KEY>` aufgerufen. Sie bietet `/v1/orders`, `/v1/orders/{id}`, `/v1/fulfillments` und die Fulfillment-Aktionen; ihr Root-Pfad liefert bewusst keine zweite Konsole. Setze einen langen zufälligen `OMS_API_KEY` in `.env` bzw. im Secret Manager. Nutze für externe Integrationen einen Reverse Proxy/VPN oder API-Gateway und veröffentliche Port 8081 nicht direkt.

Nach dem Versand liefern `GET /v1/orders/{orderId}/invoice.pdf` und `GET /v1/orders/{orderId}/invoice.xml` (OMS-Authentifizierung erforderlich) die Dokumente aus dem unveränderlichen Bestellsnapshot und den beim Ausstellen gespeicherten Verkäuferdaten. Das XML ist ein UBL-2.1-Dokument mit EN-16931-Kernfeldern; es ist ausdrücklich **nicht** als XRechnung- oder ZUGFeRD-konform zertifiziert. Vor dem Live-Einsatz müssen Verkäufer- und Steuerdaten verifiziert, die XML-Ausgabe mit dem für den Geschäftsfall vorgeschriebenen offiziellen Validator geprüft und die Aufbewahrung/Unveränderbarkeit (GoBD) durch ein geeignetes Archiv- oder ERP-System sichergestellt werden. Das System erzeugt kein eingebettetes ZUGFeRD-PDF und unterstützt noch keine XRechnung-spezifischen Käufer-Endpunkt- und Routing-IDs. Im Entwicklungsmodus sind ausschließlich gekennzeichnete Demo-Verkäuferdaten zugelassen; der Produktionsstart verweigert unvollständige Verkäufer- und HTTPS-Basis-URL-Konfiguration.

Die staff-geschützte Lager-API bildet den physischen Ablauf ab, ohne sich an einen einzelnen Versanddienstleister zu binden:

```text
GET  /api/warehouse/fulfillments?status=ready_to_pick&q=&page=1&limit=50
GET  /api/warehouse/fulfillments/{orderId}
POST /api/warehouse/fulfillments/{orderId}/claim
POST /api/warehouse/fulfillments/{orderId}/pack
POST /api/warehouse/fulfillments/{orderId}/label
POST /api/warehouse/fulfillments/{orderId}/handover
```

Der Ablauf ist `ready_to_pick` → `picking` → `packed` → `label_ready` → `handed_over`. Ein Label-Aufruf speichert Carrier, Trackingnummer sowie optionale HTTPS-Links zum Tracking und Label. Der Handover-Aufruf erfasst die verbleibenden, lagerverwalteten Positionen als Sendung und reiht dafür eine Kundenmail ein. Lieferantenpositionen bleiben aus der Lagerkommissionierung ausgeschlossen; der Bestellabschluss und die Gesamtrechnung erfolgen erst nach der letzten Lager- oder Lieferantensendung. Wiederholte Handover-Aufrufe erzeugen keine zweite Übergabe. Die tatsächliche Label-Erstellung bleibt absichtlich bei einer späteren Adapter-Integration (zum Beispiel DHL, Sendcloud oder einem 3PL), damit keine Versanddienstleister-Credentials im Shop gespeichert werden.

Die staffseitige Fulfillment-Queue ist serverseitig durchsuchbar und paginiert (50 Einträge, maximal 100 pro Seite); Antworten enthalten `total`, `page`, `pages` und globale Queue-Kennzahlen. Die Suche umfasst Bestellnummer, Kundenname/-E-Mail, Carrier und Trackingnummer. Die Fulfillment-Aktivität liefert die neuesten 100 Ereignisse und zusätzlich `eventCount`, sodass die Oberfläche gekürzte Historien klar kennzeichnet. Auch `GET /v1/fulfillments?page=1&limit=50` ist paginiert; `orderIds` kann auf bis zu 100 konkrete IDs begrenzen, etwa für die Statusanzeige der aktuellen OMS-Bestellseite.

## Produktion

Produktionskonfigurationen werden absichtlich nicht aus `.env` geladen. Sie gehoeren in einen Secret Manager oder in die Secret-Verwaltung der Zielplattform.

Die Produktions-Compose-Datei bindet den Web-Port nur an `127.0.0.1:8080`. Stelle auf der VM einen TLS-Reverse-Proxy davor; veröffentliche weder OMS noch gRPC- oder Metrikports. Die Compose-Datei startet keine Datenbank und keinen Kafka-Broker: PostgreSQL und der für die Verbraucher benötigte Kafka-Cluster müssen separat betrieben und erreichbar sein. Diese Einzelhost-Variante ist nicht hochverfügbar.

Die ausgewählte DigitalOcean-Topologie, private Netzwerk- und Firewall-Grenzen sowie die noch offenen Infrastrukturentscheidungen stehen im [DigitalOcean-Rollout-Runbook](deploy/digitalocean/README.md).

Vor dem Rollout müssen die Image-Referenz und alle erforderlichen Variablen aus der folgenden Liste verfügbar sein. Prüfe zunächst die Compose-Auflösung, lade dann das Image, wende die Migrationen mit genau diesem Image an und starte zuletzt die Dienste:

```bash
docker compose -f compose.production.yaml config --quiet
docker compose -f compose.production.yaml pull
docker compose -f compose.production.yaml run --rm -e RUN_MIGRATIONS=true app /mitte-migrate
docker compose -f compose.production.yaml up -d
```

`/mitte-migrate` ist ein einmaliger Release-Schritt pro Image-Version; der normale App-Prozess bleibt bei `RUN_MIGRATIONS=false`. Warte auf `docker compose -f compose.production.yaml ps` und den HTTP-Healthcheck, bevor du den TLS-Proxy auf den Shop schaltest. Führe bei späteren Releases dieselbe Reihenfolge mit dem neuen unveränderlichen Digest aus.

Erforderliche Variablen:

- Image und Datenbank: `MITTE_IMAGE` (unveränderlicher Digest), `DATABASE_URL` (verwaltete PostgreSQL-Instanz mit `sslmode=verify-full`, damit Zertifikat und Hostname geprüft werden). Wenn die Provider-CA nicht im System-Vertrauensspeicher des Images liegt, binde das CA-Zertifikat sicher ein und verweise mit `sslrootcert` darauf.
- Web-Host und Proxy: `TRUSTED_HOSTS`, `PUBLIC_BASE_URL`, `TRUSTED_PROXY_CIDRS` und ein zufälliger `METRICS_BEARER_TOKEN` mit mindestens 32 Zeichen. `TRUSTED_PROXY_CIDRS` muss die tatsächliche Quelladresse des direkt vorgeschalteten Proxys enthalten, nicht die Besucher-IP-Bereiche. Ermittle diesen Wert erst nach Einrichtung des Reverse-Proxys und trage keinen geratenen oder unnötig breiten CIDR ein.
- Stripe: Live-`STRIPE_SECRET_KEY`, `STRIPE_WEBHOOK_SECRET`, `STRIPE_SUCCESS_URL` und `STRIPE_CANCEL_URL`; die URLs müssen HTTPS verwenden, und die Erfolgs-URL muss `session_id={CHECKOUT_SESSION_ID}` enthalten.
- Kafka-Relay: `KAFKA_BROKERS`, `KAFKA_TLS_ENABLED=true`, `KAFKA_SASL_MECHANISM`, `KAFKA_SASL_USERNAME` und `KAFKA_SASL_PASSWORD`. Das Relay verlangt in Produktion TLS und SASL; unterstützt werden `scram-sha-512`, `scram-sha-256` und `plain` über TLS. Verwende bevorzugt SCRAM-SHA-512, sofern es der Broker unterstützt. Für eine private CA kann `KAFKA_TLS_CA_CERT` deren PEM-Zertifikat enthalten; ansonsten werden die System-CA-Zertifikate verwendet. Broker-Listener, Topic-Berechtigungen, Firewall und DNS müssen die Relay-Verbindungen zulassen.
- Mitarbeitenden- und OMS-Zugang: `STAFF_CREDENTIALS` mit Argon2id-Hashes, `STAFF_ROLES`, `SUPPLIER_CREDENTIALS` mit Argon2id-Hashes, `OMS_API_KEY`, `OMS_USERNAME` und `OMS_PASSWORD`. Produktionsstarts akzeptieren keine Klartext-Mitarbeitenden- oder Lieferantenpasswörter.
- Rechnung und E-Mail: verifizierte Werte für `INVOICE_SELLER_NAME`, `INVOICE_SELLER_STREET`, `INVOICE_SELLER_POSTAL_CODE`, `INVOICE_SELLER_CITY`, `INVOICE_SELLER_COUNTRY_CODE`, `INVOICE_SELLER_VAT_ID`, `INVOICE_SELLER_EMAIL`, `SMTP_ADDRESS` und `SMTP_FROM`. SMTP-Anmeldedaten setzt du bei Bedarf über `SMTP_USERNAME` und `SMTP_PASSWORD`.
- Kundenkonten: Die Registrierung setzt eine E-Mail-Bestätigung voraus; Passwort-Zurücksetzen und Verifizierungsnachrichten werden über den dauerhaften E-Mail-Outbox-Relay versendet. Stelle vor dem Launch sicher, dass `PUBLIC_BASE_URL`, `SMTP_ADDRESS`, `SMTP_FROM` und der SMTP-Zugang korrekt sind und die Domain E-Mails zuverlässig zustellt. Bestätigungslinks gelten 24 Stunden, Passwort-Reset-Links 30 Minuten und sind einmalig verwendbar. Die Migration widerruft bestehende Kunden-Sitzungen; bestehende Konten müssen ihre E-Mail-Adresse bestätigen oder das Passwort über den verifizierten Posteingang zurücksetzen.
- `GEMINI_API_KEY` ist optional; ohne ihn (oder bei Gemini-Ausfall) bleibt die katalogbasierte Einkaufshilfe aktiv. Für den KI-Modus brauchst du einen bezahlten Gemini-API-Zugang.

Mit dem integrierten Monitoring-Overlay:

```bash
docker compose -f compose.production.yaml -f compose.monitoring.yaml up -d
```

## Betrieb: Release, Monitoring und Incident Response

Pull Requests durchlaufen Backend-Tests inklusive Race Detector, Format- und Vet-Pruefung, Dependency-Vulnerability-Check, Frontend-Build, Container-Schwachstellenscan sowie Render- und Konfigurationsprüfungen für Kubernetes und Produktions-Compose. Der Container baut das React-Frontend selbst und bettet genau dieses Bundle in die Go-Binaries ein. Ein erfolgreicher Push auf `main` (oder ein `v*`-Tag) veröffentlicht ein SBOM- und Provenance-attestiertes Image in GitHub Container Registry. Verwende für Produktionsdeployments ausschliesslich den vom Release ausgegebenen Digest, nie `latest`.

Die CI enthält zusätzlich PostgreSQL-Integrationstests mit vielen gleichzeitig eintreffenden Checkouts und gleichzeitigen Wiederholungen desselben Idempotency Keys. Sie beweisen, dass Bestand nicht unter null sinkt und ein wiederholter Checkout genau eine Bestellung reserviert. Ein Integrationstest deckt außerdem die atomare Rechnungsausstellung, E-Mail-Queue, Event-Outbox und den Gastzugriffstoken beim Versand ab. Lokal können sie gegen eine **wegwerfbare** PostgreSQL-Datenbank über `INTEGRATION_DATABASE_URL=... go test -race -tags=integration ./internal/store` ausgeführt werden.

Wenn ein bestätigter Stripe-Zahlungseingang nach einer Webhook-Störung nicht im Auftrag erscheint, kann ein berechtigter Operator den Checkout-Session-Status gezielt bei Stripe verifizieren und den Auftrag wiederherstellen. Das Produktionsimage enthält dafür `/mitte-reconcile-stripe`; vor dem Aufruf muss das Release-Migrations-Job erfolgreich abgeschlossen sein. Beispiel für Docker Compose:

```bash
docker compose -f compose.production.yaml run --rm --no-deps app \
  /mitte-reconcile-stripe cs_live_REPLACE_WITH_CHECKOUT_SESSION_ID
```

Der Befehl prüft den Status und Betrag bei Stripe und ist für dieselbe Checkout-Session idempotent. Er führt selbst keine Datenbankmigration aus. Verwende eine geschützte Operator-Umgebung, protokolliere die Incident-Referenz und gib weder Stripe-Zugangsdaten noch Session-IDs in öffentlichen Logs oder Tickets aus.

Ein Release ist absichtlich vom Deployment getrennt: Lege in der geschuetzten Produktionsumgebung eine `MITTE_IMAGE`-Referenz mit diesem Digest sowie die Produktionssecrets im Secret Manager ab. Führe die Migration einmalig als Release-Job mit `RUN_MIGRATIONS=true` aus. Aktualisiere danach die App- und Relay-Prozesse mit `RUN_MIGRATIONS=false`. Warte jeweils auf `GET /readyz` und prüfe nach der Auslieferung den Checkout und einen Stripe-Webhook im Testmodus. Bei einem Fehler kann die vorherige Digest-Referenz zurückgesetzt werden; rückwärtskompatible Datenbankmigrationen sind deshalb verpflichtend.

Die Web- und OMS-Prozesse exportieren Prometheus-Metriken unter `/metrics`. Wenn `OTEL_EXPORTER_OTLP_ENDPOINT` gesetzt ist, werden HTTP-Traces mit begrenzten Routennamen an den OpenTelemetry Collector gesendet; Request-Logs enthalten die korrelierenden Trace- und Span-IDs. Sampling ist standardmäßig 10 % und lässt sich über `OTEL_TRACES_SAMPLE_RATIO` auf einen Wert von 0 bis 1 setzen. Rohpfade mit Bestell-/Kunden-IDs werden weder als Trace-Route noch als Request-Log-Pfad ausgegeben. Checkout- und Stripe-Webhook-Routen haben eigene, feste Metriklabels, um Checkout- und Webhook-Fehler gesondert zu alarmieren.

Das Monitoring-Overlay stellt Prometheus (30 Tage Metrik-Retention), Alertmanager, Grafana, OpenTelemetry Collector, Tempo (48 Stunden Trace-Retention) und Loki (14 Tage Log-Retention) bereit. Web-, OMS- und Worker-Prozesse schreiben weiterhin JSON nach stderr und exportieren zusätzlich strukturierte Logs via OTLP an den Collector; personenbezogene Felder wie E-Mail, Empfänger, Bestell-/Session-IDs, rohe Pfade und Fehlertexte werden aus dem zentralen Logexport entfernt, bleiben jedoch unverändert im lokalen JSON-Log. Loki ist in Grafana als Log-Datenquelle konfiguriert. Der Collector scrapt zusätzlich seine eigenen Betriebsmetriken. Order-, E-Mail- und Payment-Expiry-Worker exportieren begrenzte Erfolgs-/Fehlerzähler über bearer-geschützte interne `/metrics`-Endpunkte auf Port 9464. Prometheus scrapt auch das eigenständige OMS aus dem Compose-Stack und alarmiert bei nicht verfügbaren Workern/OMS sowie wiederholten Worker-Fehlern. Ein interner Blackbox-Exporter prüft die Liveness- und Readiness-Endpunkte von Storefront und OMS; Prometheus scrapt zusätzlich die Monitoring-Dienste selbst und alarmiert bei deren Ausfällen. Diese HTTP-Probes prüfen keine vollständige Bestellung oder Zahlung und ersetzen keine unabhängigen synthetischen Transaktionsprüfungen. Grafana, Prometheus und Alertmanager sind ausschließlich an Loopback-Ports 3000, 9090 und 9093 gebunden; für Remote-Zugriff nutze einen abgesicherten Tunnel oder Reverse Proxy mit Authentifizierung. In Kubernetes werden OMS und Worker über die Prometheus-Pod-Annotations entdeckt; die Worker benötigen `METRICS_BEARER_TOKEN` als Key im Secret `mitte-secrets`, erhalten TCP-Probes auf dem internen Metrikport und PodDisruptionBudgets, die bei freiwilligen Unterbrechungen jeweils mindestens einen Worker-Pod verfügbar halten.

Wenn der OTLP-Exporter nicht initialisiert werden kann, laufen die App-Prozesse mit JSON-Logs auf stderr und ohne zentrale Logs beziehungsweise Traces weiter; die Warnung muss im lokalen Container-Log alarmiert werden. Ein erfolgreicher Prozessstart ist daher kein Nachweis, dass Telemetrie exportiert wird.

Vor dem Start müssen `METRICS_BEARER_TOKEN` (mindestens 32 zufällige Zeichen), `SLACK_WEBHOOK_URL_FILE`, `GRAFANA_ADMIN_PASSWORD_FILE`, `OMS_API_KEY`, `OMS_USERNAME` und `OMS_PASSWORD` gesetzt sein. Die beiden `*_FILE`-Variablen müssen auf geschützte Dateien zeigen, die jeweils den Slack-Incoming-Webhook und ein eigenes Grafana-Adminpasswort enthalten; diese Dateien gehören in den Secret Manager und niemals ins Repository. `PROMETHEUS_APP_TARGET` ist standardmäßig `app:8080`. Der öffentliche Reverse Proxy darf `/metrics` nicht ins Internet weiterleiten; OMS bleibt intern, sofern es nicht bewusst über einen abgesicherten VPN/API-Gateway zugänglich gemacht wird.

Diese Compose-Installation ist ein funktionsfähiger Einzelhost-Baustein, kein hochverfügbarer Monitoring-Dienst: Hostverlust kann die lokalen Prometheus-, Grafana-, Alertmanager-, Tempo- und Loki-Daten unzugänglich machen. Ein Ausfall von Alertmanager kann nicht zuverlässig durch einen Alert erkannt werden, der genau diesen Alertmanager benötigt. Für eine wirksame Produktionsüberwachung müssen daher mindestens Alertmanager-Erreichbarkeit und Hostverfügbarkeit zusätzlich von einem unabhängigen Dienst geprüft und benachrichtigt werden. Für Produktion müssen Retention, Volumes, Backups und Restore getestet werden; Monitoring und Alert-Routing sollten auf einer unabhängigen Plattform bzw. einem separaten Host betrieben und über ein abgesichertes privates Netz verbunden werden. Prüfe außerdem Datenschutz, Datenminimierung und Aufbewahrungsfristen für Logs; vermeide das Protokollieren unnötiger personenbezogener Daten und Zahlungsdaten. Für OMS- und Worker-Scrapes müssen Netzwerkrichtlinien den Zugriff auf interne Metrikports auf Prometheus beschränken.

Für einen echten Launch gehören außerdem in die Plattformkonfiguration: mindestens zwei App-Replikate hinter einem TLS-Proxy, managed PostgreSQL mit PITR und regelmäßig getesteten Restore-Übungen, Kafka mit Authentifizierung/TLS und Retention, ein Secret Manager mit Rotation, WAF/DDoS-Schutz, synthetische Checkout-Prüfungen sowie eine dokumentierte On-Call-Eskalation. Metriken, Logs und Alerts müssen außerhalb des einzelnen Hosts gespeichert werden.

Für Last nutze mindestens zwei API- und zwei Relay-Replikate. Relays dürfen horizontal skaliert werden: sie verarbeiten unterschiedliche Bestellungen parallel, bewahren aber die Reihenfolge innerhalb einer Bestellung. Lege `DB_MAX_OPEN_CONNS` und `DB_MAX_IDLE_CONNS` pro Prozess fest; die Summe aller API-, Relay- und Betriebsverbindungen muss unter dem Connection-Limit der Datenbank bleiben. Mit den Basiswerten in den Kubernetes-Manifesten ergeben sich bei zehn API-Replikaten und den definierten Worker-/OMS-Replikaten bis zu 74 gleichzeitige Anwendungspool-Verbindungen im Normalbetrieb und bis zu etwa 91 während paralleler Rolling Updates. Rechne zusätzlich Migrationen, administrative Zugriffe und Provider-/Failover-Reserven ein oder senke Pool-/Replica-Limits. Der Alert `MitteDatabasePoolSaturated` zeigt, wenn die Verbindungspools ausgelastet sind. Ausgebliebene Stripe-Webhooks werden durch den Relay-Prozess spätestens nach 30 Minuten bereinigt und geben Bestand automatisch frei.

Der Prozess stellt zusätzlich den standardisierten `grpc.health.v1.Health/Check`-Dienst unter `GRPC_LISTEN_ADDR` (standardmäßig `:9090`) bereit. Er prüft die Datenbank vor der Antwort und eignet sich damit für private Service-Discovery-, Load-Balancer- und Worker-Checks. Der Port ist im Produktions-Compose nur im internen Container-Netz sichtbar und darf nicht über den öffentlichen Reverse Proxy veröffentlicht werden. Für clusterübergreifende oder nicht vertrauenswürdige Netzwerke ist mTLS am Service-Mesh oder am gRPC-Ingress verpflichtend. Der aktuelle gRPC-Endpunkt ist absichtlich ein Control Plane: Geschäfts-APIs bleiben HTTP/JSON, bis ein unabhängiger Consumer eine versionierte `.proto`-Schnittstelle rechtfertigt.

Schreibtransaktionen für Warenkorb, Checkout, Zahlungsstatus und Reservierungsablauf folgen einer festen Sperrreihenfolge (zuerst die eigene Entität, dann Produkte nach Produkt-ID) und begrenzen Sperr-Wartezeiten auf eine Sekunde. Bei PostgreSQL-Deadlocks (`40P01`), Serialisierungsfehlern (`40001`) oder konfigurierten Lock-Timeouts (`55P03`) werden sie maximal dreimal mit kurzem Jitter erneut ausgeführt. Externe Side Effects werden nie innerhalb dieses Retry-Mechanismus wiederholt: Stripe ist durch den Checkout-Idempotency-Key geschützt, und die Kafka-Outbox bleibt bewusst mindestens-einmalig mit Consumer-Deduplizierung.

Setze beim ersten Rollout `RUN_MIGRATIONS=true` nur fuer einen einzelnen Release- oder Job-Prozess. Die normale App-Auslieferung bleibt bei `RUN_MIGRATIONS=false`. Der Reverse Proxy terminiert TLS und leitet nur HTTPS an die App weiter; in Produktion sind Session-Cookies deshalb automatisch `Secure`.

## Vor dem Launch

- Wenn Produktionszugangsdaten jemals in einer lokalen `.env`, einem Commit, Build-Artefakt oder Chat offengelegt wurden, behandle sie als kompromittiert: widerrufe und rotiere die betroffenen Zugangsdaten beim jeweiligen Anbieter und ersetze sie im Secret Manager. `.gitignore` verhindert keine Offenlegung aus bereits erstellten Commits oder externen Kopien.
- Für Docker Compose muss `TRUSTED_PROXY_CIDRS` die tatsächliche, eng begrenzte Quelladresse des vorgeschalteten Proxys enthalten. Für Kubernetes gilt dasselbe in `deploy/kubernetes/base/configmap.yaml`; dort werden `STAFF_CREDENTIALS`, `STAFF_ROLES`, `KAFKA_SASL_USERNAME` und `KAFKA_SASL_PASSWORD` als Secret-Keys in `mitte-secrets` benötigt. Keine Beispiel-CIDRs oder gemeinsamen Standardpasswörter in Produktion verwenden.
- Kubernetes-Worker benötigen zusätzlich `METRICS_BEARER_TOKEN` als Secret-Key in `mitte-secrets`; dieser muss mit dem Token der Prometheus-Scrape-Konfiguration übereinstimmen. Die Worker-Metrikports dürfen ausschließlich aus dem Monitoring-Netz erreichbar sein.
- Die anwendungsinternen Rate-Limits gelten pro Prozess und sind nicht zwischen Replikaten geteilt. Das Kubernetes-Beispiel begrenzt deshalb `/api/` zusätzlich am NGINX-Ingress auf 10 Anfragen pro Sekunde und Client-IP mit einem Burst-Faktor von 3. `/api/auth/` und `POST /api/staff/login` haben zusätzlich ein Limit von 1 Anfrage pro Sekunde und Client-IP mit Burst 3. Überschreitungen erhalten HTTP 429. Prüfe die Grenzwerte anhand echter Staging- und Lasttestdaten. NGINX-Ingress-Zähler sind nicht clusterweit zwischen mehreren Controller-Pods geteilt; für eine harte globale Grenze verwende einen Gateway-/WAF-Dienst mit gemeinsamem Rate-Limit-Speicher oder einen geteilten Rate-Limiter. Besonders Anmeldung, Verifizierung, Passwort-Reset und Checkout müssen bei mehreren Replikaten getestet werden.
- Produktionsimages werden erst veröffentlicht, wenn der wiederverwendete CI-Workflow erfolgreich war. Schütze zusätzlich `main` und Release-Tags mit den passenden Repository-Regeln und prüfe, dass die eingesetzte unveränderliche Image-Digest-Referenz aus genau diesem erfolgreichen Lauf stammt.
- Stripe Checkout konfigurieren: Die Anwendung erwartet `checkout.session.completed` und `checkout.session.expired` am Endpoint `/webhooks/stripe`. Die Webhook-Signatur wird verifiziert; nur eine als `paid` gemeldete EUR-Session mit passender Bestellsumme und gespeicherter Session-ID kann eine Bestellung als bezahlt markieren. Setze die Webhook-URL in Stripe auf die öffentliche HTTPS-Adresse und hinterlege das Signing Secret als `STRIPE_WEBHOOK_SECRET`.
- Die Rücksprung-URLs müssen in Produktion HTTPS sein. `STRIPE_SUCCESS_URL` muss `session_id={CHECKOUT_SESSION_ID}` enthalten; Beispielwerte stehen in `.env.example`.
- Transaktions-E-Mails, Retourenprozess, Versanddienstleister und Rechnungslogik mit den jeweiligen Zugangsdaten integrieren.
- Den SMTP-Relay mit dem Produktionsanbieter konfigurieren und Verifizierung, erneute Verifizierung, Passwort-Zurücksetzen sowie Zustellfehler in einer Staging-Umgebung end-to-end testen. Ohne funktionierende Zustellung können neue Kundenkonten nicht aktiviert und vergessene Passwörter nicht wiederhergestellt werden.
- Datenschutz, Impressum, Widerruf, AGB, Steuer- und Preisangaben mit fachlicher beziehungsweise rechtlicher Pruefung bereitstellen.
- Backups, Point-in-Time-Recovery, Monitoring/Alerting und eine zentrale Log-Ablage fuer die Produktionsdatenbank aktivieren.
- Produktbilder zu einem eigenen CDN oder Object Storage umziehen; die jetzigen Unsplash-Bilder sind nur Platzhalter fuer das Portfolio.

## Architektur

```text
Browser -> Checkout API -> Checkout use case -> PostgreSQL order + checkout saga
                                       |                    |
                                       v                    v
                               Payment provider      Transactional outbox
                                                            |
                                                            v
                                                  Order relay -> Kafka

Private infrastructure -> gRPC health check -> API + PostgreSQL readiness
```

Die SQL-Abfragen liegen im `internal/store`-Paket nach Verantwortung getrennt (`store.go`, `cart.go`, `orders.go`) und verwenden ausschliesslich parametrisierte Platzhalter. Versionierte Schemaaenderungen liegen unter `internal/store/migrations` und werden mit `go:embed` in das Binary eingebunden. Es gibt keinen Produkt-Seed; Migration 036 entfernt bekannte Entwicklungs-Fixtures aus bereits vorhandenen Datenbanken und bewahrt Bestellhistorie, indem betroffene Artikel archiviert und auf null Bestand gesetzt werden. Der Migrations-Ledger und eine transaktionale PostgreSQL-Advisory-Lock verhindern doppelte oder parallele Schemaausfuehrungen.

Bestellereignisse werden in derselben Datenbanktransaktion wie die Bestellung bzw. der Statuswechsel in `order_outbox_events` geschrieben. Der getrennt deployte Prozess `cmd/order-relay` überträgt sie nach Kafka auf `KAFKA_ORDER_TOPIC` und verwendet die Bestell-ID als Kafka-Key. Jede Nachricht ist ein versionierter Envelope (`schema_version`, `event_id`, `event_type`, `order_id`, `occurred_at`, `data`). Konsumenten müssen `event_id` deduplizieren: Die Zustellung ist bewusst mindestens-einmalig, damit ein Prozessabbruch nach einem Kafka-Ack kein Ereignis verlieren kann. Die API benötigt keine Kafka-Verbindung; nur der Relay-Service setzt `ORDER_RELAY_ENABLED=true` und `KAFKA_BROKERS` voraus. Der separate Prozess `cmd/payment-expiry` gibt abgelaufene Zahlungsreservierungen frei und ist absichtlich nicht von Kafka abhängig.

Der Checkout-Saga-Datensatz wird gemeinsam mit der Bestellung angelegt. Er protokolliert Zahlungsart und Fortschritt (`started`, `retryable`, `awaiting_payment`, `completed`, `compensated`). Providerfehler lassen die Bestandssperre bestehen, damit ein erneuter Request mit demselben `Idempotency-Key` dieselbe Bestellung und – bei Stripe – denselben Provider-Idempotency-Key fortsetzen kann. Der Ablauf zur Zahlungsfrist storniert abgelaufene offene Bestellungen und gibt Bestand sowie Gutscheinreservierung in der bestehenden Datenbanktransaktion frei; eine verifizierte verspätete Stripe-Zahlung kann über den vorhandenen Reconciliation-Pfad sicher wiederhergestellt werden.

Im lokalen Compose-Stack erstellt `kafka-init` das konfigurierte Topic nach dem erfolgreichen Kafka-Healthcheck; der Order-Relay wartet, bis diese Einrichtung abgeschlossen ist. Produktionsumgebungen müssen das Topic separat mit geeigneter Partitionierung, Replikation und Retention provisionieren. Compose-Healthchecks sind für HTTP-Dienste auf deren eigenen Endpunkt ausgerichtet; Hintergrund-Worker haben keinen HTTP-Healthcheck.

Für die nächste Entkopplungsstufe sollten Kundenidentität, Katalog, Warenkorb, Zahlung und Fulfillment jeweils eigene Datenhoheit erhalten und ausschließlich versionierte Events bzw. klar versionierte APIs verwenden. Solange sie dieselbe PostgreSQL-Datenbank besitzen, sind sie keine echten unabhängigen Microservices.
