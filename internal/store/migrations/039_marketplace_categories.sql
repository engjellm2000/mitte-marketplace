UPDATE categories
SET name = 'Wohnen & Möbel',
    description = 'Möbel, Licht, Dekoration und Textilien für jeden Raum.'
WHERE slug = 'wohnen';

INSERT INTO categories (slug, name, description)
VALUES
    ('technik', 'Technik', 'Elektronik, Computer und vernetzte Alltagsgeräte.'),
    ('beauty', 'Beauty & Pflege', 'Haut-, Haar- und Körperpflege sowie Make-up.'),
    ('mode', 'Mode', 'Kleidung, Schuhe und Accessoires.'),
    ('sport-outdoor', 'Sport & Outdoor', 'Ausrüstung für Training, Reise und draußen.'),
    ('kueche-haushalt', 'Küche & Haushalt', 'Kochen, Essen und praktische Haushaltsgeräte.'),
    ('garten-balkon', 'Garten & Balkon', 'Pflanzen, Werkzeuge und Ausstattung für draußen.'),
    ('baby-kinder', 'Baby & Kinder', 'Alltag, Spiel und Ausstattung für Familien.'),
    ('buero-schreibwaren', 'Büro & Schreibwaren', 'Arbeitsplatz, Papier und Büroausstattung.'),
    ('tierbedarf', 'Tierbedarf', 'Produkte für Hunde, Katzen und weitere Haustiere.'),
    ('lebensmittel', 'Lebensmittel & Genuss', 'Kaffee, Tee, Feinkost und Vorrat.'),
    ('auto-mobilitaet', 'Auto & Mobilität', 'Zubehör für unterwegs und den Alltag.'),
    ('werkzeug-handwerk', 'Werkzeug & Handwerk', 'Werkzeuge, Maschinen und Arbeitsschutz.'),
    ('reisen-freizeit', 'Reisen & Freizeit', 'Gepäck und Ausstattung für unterwegs.')
ON CONFLICT (slug) DO UPDATE
SET name = EXCLUDED.name, description = EXCLUDED.description;

UPDATE categories
SET parent_id = (SELECT id FROM categories WHERE slug = 'kueche-haushalt')
WHERE slug = 'tisch';

UPDATE categories
SET parent_id = (SELECT id FROM categories WHERE slug = 'wohnen'),
    name = 'Heimtextilien',
    description = 'Bettwäsche, Badtextilien und Textilien für Wohnräume.'
WHERE slug = 'textil';

WITH category_seed(slug, name, description, parent_slug) AS (
    VALUES
        ('smartphones', 'Smartphones', 'Mobiltelefone und Zubehör.', 'technik'),
        ('laptops-tablets', 'Laptops & Tablets', 'Mobile Computer und Tablets.', 'technik'),
        ('computer-zubehoer', 'Computerzubehör', 'Monitore, Eingabegeräte und Speicher.', 'technik'),
        ('audio', 'Audio', 'Kopfhörer, Lautsprecher und Audiogeräte.', 'technik'),
        ('smart-home', 'Smart Home', 'Vernetzte Geräte für Zuhause.', 'technik'),
        ('moebel', 'Möbel', 'Möbel für Wohnen, Schlafen und Arbeiten.', 'wohnen'),
        ('beleuchtung', 'Beleuchtung', 'Leuchten und Leuchtmittel.', 'wohnen'),
        ('deko', 'Dekoration', 'Dekorative Objekte für Innenräume.', 'wohnen'),
        ('hautpflege', 'Hautpflege', 'Gesichtspflege und Körperpflege.', 'beauty'),
        ('make-up', 'Make-up', 'Make-up und kosmetisches Zubehör.', 'beauty'),
        ('haarpflege', 'Haarpflege', 'Haarpflege und Styling.', 'beauty'),
        ('dufte', 'Düfte', 'Parfüm und Raumdüfte.', 'beauty'),
        ('damenmode', 'Damenmode', 'Kleidung und Accessoires für Damen.', 'mode'),
        ('herrenmode', 'Herrenmode', 'Kleidung und Accessoires für Herren.', 'mode'),
        ('schuhe', 'Schuhe', 'Schuhe für Alltag, Arbeit und Freizeit.', 'mode'),
        ('taschen', 'Taschen & Rucksäcke', 'Taschen und Rucksäcke für unterwegs.', 'mode'),
        ('fitness', 'Fitness', 'Trainingszubehör und Fitnessgeräte.', 'sport-outdoor'),
        ('camping', 'Camping', 'Ausrüstung für Camping und Outdoor.', 'sport-outdoor'),
        ('radfahren', 'Radfahren', 'Fahrräder und Fahrradzubehör.', 'sport-outdoor'),
        ('kochen-backen', 'Kochen & Backen', 'Kochgeschirr und Backzubehör.', 'kueche-haushalt'),
        ('kuechengeraete', 'Küchengeräte', 'Kleine und große Küchengeräte.', 'kueche-haushalt'),
        ('haushaltsgeraete', 'Haushaltsgeräte', 'Geräte für den Haushalt.', 'kueche-haushalt'),
        ('pflanzen', 'Pflanzen & Anzucht', 'Pflanzen, Saatgut und Anzuchtbedarf.', 'garten-balkon'),
        ('gartenmoebel', 'Gartenmöbel', 'Möbel für Garten, Terrasse und Balkon.', 'garten-balkon'),
        ('gartengeraete', 'Gartengeräte', 'Werkzeuge und Geräte für den Garten.', 'garten-balkon'),
        ('grillen', 'Grillen', 'Grills und Zubehör.', 'garten-balkon'),
        ('baby', 'Baby', 'Pflege und Ausstattung für Babys.', 'baby-kinder'),
        ('spielzeug', 'Spielzeug', 'Spielzeug und Spiele.', 'baby-kinder'),
        ('kindermode', 'Kindermode', 'Kleidung und Schuhe für Kinder.', 'baby-kinder'),
        ('schule', 'Schule', 'Schulbedarf und Lernmaterial.', 'baby-kinder'),
        ('arbeitsplatz', 'Arbeitsplatz', 'Schreibtische, Stühle und Ausstattung.', 'buero-schreibwaren'),
        ('schreibwaren', 'Schreibwaren', 'Papier, Stifte und Schreibzubehör.', 'buero-schreibwaren'),
        ('buerotechnik', 'Bürotechnik', 'Technik und Zubehör für Arbeitsplätze.', 'buero-schreibwaren'),
        ('hunde', 'Hunde', 'Zubehör und Pflege für Hunde.', 'tierbedarf'),
        ('katzen', 'Katzen', 'Zubehör und Pflege für Katzen.', 'tierbedarf'),
        ('tierfutter', 'Futter & Snacks', 'Futter und Snacks für Haustiere.', 'tierbedarf'),
        ('kaffee-tee', 'Kaffee & Tee', 'Kaffee, Tee und Zubereitung.', 'lebensmittel'),
        ('feinkost', 'Feinkost', 'Ausgewählte Zutaten und Spezialitäten.', 'lebensmittel'),
        ('snacks', 'Snacks', 'Snacks und kleine Genussmomente.', 'lebensmittel'),
        ('auto-zubehoer', 'Autozubehör', 'Praktisches Zubehör für Fahrzeuge.', 'auto-mobilitaet'),
        ('mobilitaet', 'Mobilität', 'Zubehör für tägliche Wege.', 'auto-mobilitaet'),
        ('handwerkzeuge', 'Handwerkzeuge', 'Handwerkzeuge für Werkstatt und Zuhause.', 'werkzeug-handwerk'),
        ('maschinen', 'Maschinen', 'Elektrowerkzeuge und Maschinen.', 'werkzeug-handwerk'),
        ('arbeitsschutz', 'Arbeitsschutz', 'Schutzkleidung und Sicherheitsausstattung.', 'werkzeug-handwerk'),
        ('messtechnik', 'Messgeräte', 'Mess- und Prüfgeräte.', 'werkzeug-handwerk'),
        ('reisegepaeck', 'Reisegepäck', 'Koffer und Taschen für Reisen.', 'reisen-freizeit'),
        ('reisebedarf', 'Reisebedarf', 'Praktisches Zubehör für unterwegs.', 'reisen-freizeit')
)
INSERT INTO categories (slug, name, description, parent_id)
SELECT seed.slug, seed.name, seed.description, parent.id
FROM category_seed seed
JOIN categories parent ON parent.slug = seed.parent_slug
ON CONFLICT (slug) DO UPDATE
SET name = EXCLUDED.name, description = EXCLUDED.description, parent_id = EXCLUDED.parent_id;

WITH category_seed(slug, name, description, parent_slug) AS (
    VALUES
        ('ladegeraete', 'Ladegeräte & Kabel', 'Ladegeräte und Verbindungskabel.', 'smartphones'),
        ('smartphone-schutz', 'Hüllen & Schutz', 'Schutzhüllen und Displayschutz.', 'smartphones'),
        ('monitore', 'Monitore', 'Monitore für Büro und Zuhause.', 'computer-zubehoer'),
        ('tastaturen-maeuse', 'Tastaturen & Mäuse', 'Eingabegeräte für Computer.', 'computer-zubehoer'),
        ('kopfhoerer', 'Kopfhörer', 'Kopfhörer und Headsets.', 'audio'),
        ('lautsprecher', 'Lautsprecher', 'Lautsprecher für Zuhause und unterwegs.', 'audio'),
        ('bettwaesche', 'Bettwäsche', 'Bettwäsche und Bettwaren.', 'textil'),
        ('badtextilien', 'Badtextilien', 'Handtücher und Badtextilien.', 'textil'),
        ('sofas-sessel', 'Sofas & Sessel', 'Sitzmöbel für Wohnräume.', 'moebel'),
        ('tische-stuehle', 'Tische & Stühle', 'Tische und Stühle für Zuhause.', 'moebel'),
        ('gesichtspflege', 'Gesichtspflege', 'Reinigung und Pflege für das Gesicht.', 'hautpflege'),
        ('koerperpflege', 'Körperpflege', 'Pflegeprodukte für den Körper.', 'hautpflege'),
        ('elektrowerkzeuge', 'Elektrowerkzeuge', 'Elektrische Werkzeuge für Werkstatt und Baustelle.', 'maschinen'),
        ('werkstattbedarf', 'Werkstattbedarf', 'Ausstattung und Zubehör für die Werkstatt.', 'werkzeug-handwerk')
)
INSERT INTO categories (slug, name, description, parent_id)
SELECT seed.slug, seed.name, seed.description, parent.id
FROM category_seed seed
JOIN categories parent ON parent.slug = seed.parent_slug
ON CONFLICT (slug) DO UPDATE
SET name = EXCLUDED.name, description = EXCLUDED.description, parent_id = EXCLUDED.parent_id;
