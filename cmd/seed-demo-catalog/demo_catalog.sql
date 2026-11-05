-- Synthetic local-development catalog only. Never add this file to migrations.
WITH RECURSIVE seed(
    category_slug, slug, name, short_description, description, price_cents,
    image_url, image_alt, inventory, featured, sku
) AS (
    VALUES
        ('smartphones', 'demo-usb-c-ladegeraet-65w', 'USB-C Ladegerät 65 W', 'Kompaktes Ladegerät mit zwei USB-C-Anschlüssen.', 'Lädt kompatible Mobilgeräte und Laptops. Mit klappbarem Stecker und Überlastschutz.', 3999, 'https://images.unsplash.com/photo-1583863788434-e58a36330cf0?auto=format&fit=crop&w=900&q=80', 'Kompaktes USB-C-Ladegerät', 32, TRUE, 'MITTE-DEMO-0001'),
        ('smartphones', 'demo-schutzhuelle-flex', 'Flexible Schutzhülle', 'Griffige Hülle mit erhöhtem Rand für das Display.', 'Flexible Schutzhülle mit präzisen Aussparungen für Anschlüsse und Tasten. Passform vor realem Verkauf prüfen.', 1499, 'https://images.unsplash.com/photo-1601593346740-925612772716?auto=format&fit=crop&w=900&q=80', 'Schutzhülle für ein Smartphone', 48, FALSE, 'MITTE-DEMO-0002'),
        ('laptops-tablets', 'demo-notebook-14', 'Notebook 14 Zoll', 'Mobiler Arbeitsbegleiter mit entspiegeltem Display.', 'Beispielkonfiguration: 14-Zoll-Display, 16 GB Arbeitsspeicher und 512 GB SSD. Technische Daten sind Demo-Platzhalter.', 79900, 'https://images.unsplash.com/photo-1496181133206-80ce9b88a853?auto=format&fit=crop&w=900&q=80', 'Notebook auf einem Tisch', 9, TRUE, 'MITTE-DEMO-0003'),
        ('monitore', 'demo-monitor-27-qhd', 'Monitor 27 Zoll QHD', 'Großzügige Arbeitsfläche mit höhenverstellbarem Standfuß.', 'Beispielmonitor mit 27 Zoll, QHD-Auflösung und höhenverstellbarem Standfuß. Anschluss- und Leistungsangaben vor Verkauf verifizieren.', 24900, 'https://images.unsplash.com/photo-1527443224154-c4a3942d3acf?auto=format&fit=crop&w=900&q=80', 'Monitor auf einem Schreibtisch', 12, TRUE, 'MITTE-DEMO-0004'),
        ('audio', 'demo-kopfhoerer-komfort', 'Over-Ear-Kopfhörer Komfort', 'Kabelloser Kopfhörer für Alltag und Homeoffice.', 'Weiche Ohrpolster, integriertes Mikrofon und faltbare Bauweise. Akkulaufzeit und Funkstandards sind nicht verifiziert.', 6999, 'https://images.unsplash.com/photo-1505740420928-5e560c06d30e?auto=format&fit=crop&w=900&q=80', 'Kabellose Over-Ear-Kopfhörer', 18, TRUE, 'MITTE-DEMO-0005'),
        ('smart-home', 'demo-led-lichtset', 'LED-Lichtset für Zuhause', 'Dimmbares Licht für eine angenehme Raumstimmung.', 'Drei Leuchtmittel mit warm- bis kaltweißer Lichtfarbe. Kompatibilität und Energiekennzeichnung vor realem Verkauf prüfen.', 2999, 'https://images.unsplash.com/photo-1507473885765-e6ed057f782c?auto=format&fit=crop&w=900&q=80', 'Moderne Leuchte in einem Wohnraum', 20, FALSE, 'MITTE-DEMO-0006'),
        ('moebel', 'demo-sessel-lina', 'Sessel Lina', 'Kompakter Polstersessel für Leseecke und Wohnzimmer.', 'Beispielmöbel mit stabilem Holzgestell und pflegeleichtem Stoffbezug. Maße und Materialangaben sind Platzhalter.', 22900, 'https://images.unsplash.com/photo-1567538096630-e0c55bd6374c?auto=format&fit=crop&w=900&q=80', 'Polstersessel in einem hellen Wohnraum', 5, TRUE, 'MITTE-DEMO-0007'),
        ('moebel', 'demo-regal-modul', 'Modulares Wandregal', 'Flexibles Regal für Bücher und Lieblingsstücke.', 'Modulares Regal mit mehreren Ablageflächen. Belastbarkeit, Wandbefestigung und Maße vor echtem Verkauf verifizieren.', 8999, 'https://images.unsplash.com/photo-1594620302200-9a762244a156?auto=format&fit=crop&w=900&q=80', 'Holzregal mit dekorativen Objekten', 14, FALSE, 'MITTE-DEMO-0008'),
        ('beleuchtung', 'demo-tischleuchte-nord', 'Tischleuchte Nord', 'Schlichtes Design und angenehmes Licht am Abend.', 'Tischleuchte mit textilem Kabel und austauschbarem Leuchtmittel. Elektrische Spezifikationen und Konformität müssen vor Livebetrieb geprüft werden.', 4599, 'https://images.unsplash.com/photo-1507473885765-e6ed057f782c?auto=format&fit=crop&w=900&q=80', 'Tischleuchte mit warmem Licht', 21, FALSE, 'MITTE-DEMO-0009'),
        ('deko', 'demo-keramik-vase', 'Keramikvase Forma', 'Handwerklich inspirierte Form mit matter Oberfläche.', 'Dekorative Vase aus Keramik. Farbe und Oberfläche können bei echten Lieferantenprodukten variieren.', 2499, 'https://images.unsplash.com/photo-1578500494198-246f612d3b3d?auto=format&fit=crop&w=900&q=80', 'Keramikvase als Wohnaccessoire', 26, FALSE, 'MITTE-DEMO-0010'),
        ('bettwaesche', 'demo-bettwaesche-baumwolle', 'Bettwäsche Baumwolle', 'Angenehm weicher Bezug für erholsame Nächte.', 'Bettwäsche-Set aus Baumwollgewebe. Faseranteil, Pflegekennzeichnung und Maße vor Veröffentlichung mit Lieferantendaten abgleichen.', 3999, 'https://images.unsplash.com/photo-1631049307264-da0ec9d70304?auto=format&fit=crop&w=900&q=80', 'Bett mit heller Bettwäsche', 17, TRUE, 'MITTE-DEMO-0011'),
        ('tische-stuehle', 'demo-esstisch-eiche', 'Esstisch Eiche', 'Zeitloser Mittelpunkt für gemeinsame Mahlzeiten.', 'Beispielhafter Esstisch mit Holzoptik und Platz für vier bis sechs Personen. Material, Herkunft und Maße sind nicht verifiziert.', 34900, 'https://images.unsplash.com/photo-1617806118233-18e1de247200?auto=format&fit=crop&w=900&q=80', 'Esstisch aus Holz mit Stühlen', 4, FALSE, 'MITTE-DEMO-0012'),
        ('hautpflege', 'demo-gesichtscreme-sanft', 'Gesichtscreme Sanft', 'Leichte Pflege für die tägliche Routine.', 'Beispielhafte Feuchtigkeitspflege ohne bestätigte Wirkversprechen. Inhaltsstoffe, Sicherheitsbewertung und Kennzeichnung müssen vor Verkauf vorliegen.', 1299, 'https://images.unsplash.com/photo-1556228720-195a672e8a03?auto=format&fit=crop&w=900&q=80', 'Hautpflegeprodukte in neutraler Umgebung', 36, TRUE, 'MITTE-DEMO-0013'),
        ('make-up', 'demo-pinselset-essentials', 'Make-up Pinselset', 'Vielseitiges Set für präzises Auftragen.', 'Set mit Pinseln unterschiedlicher Formen für eine einfache Make-up-Routine. Material- und Hygieneangaben sind Platzhalter.', 1999, 'https://images.unsplash.com/photo-1522335789203-aabd1fc54bc9?auto=format&fit=crop&w=900&q=80', 'Make-up und Pinsel auf einem Tisch', 23, FALSE, 'MITTE-DEMO-0014'),
        ('gesichtspflege', 'demo-reinigungsbalsam', 'Reinigungsbalsam', 'Sanfte Reinigung für Gesicht und Hals.', 'Balsam zur kosmetischen Reinigung. Zusammensetzung, Verträglichkeit, verantwortliche Person und Kennzeichnung vor echtem Verkauf prüfen.', 999, 'https://images.unsplash.com/photo-1608248543803-ba4f8c70ae0b?auto=format&fit=crop&w=900&q=80', 'Tiegel mit Gesichtspflege', 30, FALSE, 'MITTE-DEMO-0015'),
        ('damenmode', 'demo-pullover-atelier', 'Pullover Atelier', 'Weicher Strickpullover für entspannte Tage.', 'Beispielpullover mit klassischem Rundhalsausschnitt. Zusammensetzung, Größenlauf und Pflegehinweise müssen lieferantenseitig bestätigt werden.', 4999, 'https://images.unsplash.com/photo-1434389677669-e08b4cac3105?auto=format&fit=crop&w=900&q=80', 'Strickpullover auf neutralem Hintergrund', 15, TRUE, 'MITTE-DEMO-0016'),
        ('herrenmode', 'demo-hemd-alltag', 'Hemd Alltag', 'Vielseitiges Hemd mit geradem Schnitt.', 'Hemd für Büro und Freizeit. Stoff, Passform, Größen und Herkunft sind Demo-Angaben und nicht als Verkaufsinformation geeignet.', 3999, 'https://images.unsplash.com/photo-1598033129183-c4f50c736f10?auto=format&fit=crop&w=900&q=80', 'Hemd auf einem Kleiderbügel', 19, FALSE, 'MITTE-DEMO-0017'),
        ('schuhe', 'demo-sneaker-stadt', 'Sneaker Stadt', 'Leichter Schuh für tägliche Wege.', 'Beispielhafter Sneaker mit herausnehmbarer Innensohle. Größen, Materialien und Konformitätsangaben vor Verkauf verifizieren.', 5999, 'https://images.unsplash.com/photo-1542291026-7eec264c27ff?auto=format&fit=crop&w=900&q=80', 'Sneaker in kräftiger Farbe', 12, FALSE, 'MITTE-DEMO-0018'),
        ('taschen', 'demo-rucksack-kompakt', 'Rucksack Kompakt', 'Praktischer Stauraum für Pendelweg und Wochenende.', 'Kompakter Rucksack mit Laptopfach und verstellbaren Trägern. Volumen und Materialangaben sind Platzhalter.', 4499, 'https://images.unsplash.com/photo-1553062407-98eeb64c6a62?auto=format&fit=crop&w=900&q=80', 'Rucksack für den Alltag', 16, TRUE, 'MITTE-DEMO-0019'),
        ('fitness', 'demo-yogamatte-balance', 'Yogamatte Balance', 'Rutschhemmende Unterlage für dein Training.', 'Trainingsmatte für Yoga und Gymnastik. Maße, Material und Rutschhemmung müssen für echte Produkte nachgewiesen werden.', 2499, 'https://images.unsplash.com/photo-1592432678016-e910b452f9a2?auto=format&fit=crop&w=900&q=80', 'Yogamatte für ein Training', 25, FALSE, 'MITTE-DEMO-0020'),
        ('camping', 'demo-laterne-camp', 'Campinglaterne Camp', 'Kompaktes Licht für Zelt und Terrasse.', 'Tragbare Laterne mit mehreren Helligkeitsstufen. Laufzeit, Schutzklasse und Batterietyp sind nicht bestätigt.', 2199, 'https://images.unsplash.com/photo-1478131143081-80f7f84ca84d?auto=format&fit=crop&w=900&q=80', 'Campingzelt im Freien', 13, FALSE, 'MITTE-DEMO-0021'),
        ('radfahren', 'demo-fahrradhelm-city', 'Fahrradhelm City', 'Alltagshelm mit verstellbarem Sitzsystem.', 'Beispielhelm für Stadtfahrten. Sicherheitsnorm, Größen, Material und Prüfberichte müssen vor realem Verkauf nachgewiesen werden.', 4999, 'https://images.unsplash.com/photo-1507035895480-2b3156c31fc8?auto=format&fit=crop&w=900&q=80', 'Fahrrad im urbanen Umfeld', 11, FALSE, 'MITTE-DEMO-0022'),
        ('kochen-backen', 'demo-pfanne-28', 'Bratpfanne 28 cm', 'Vielseitige Pfanne für den täglichen Einsatz.', 'Pfanne für verschiedene Gerichte mit hitzebeständigem Griff. Beschichtung, Herdarten und Pflegehinweise vor Veröffentlichung prüfen.', 2999, 'https://images.unsplash.com/photo-1556911220-bff31c812dba?auto=format&fit=crop&w=900&q=80', 'Kochgeschirr in einer Küche', 22, TRUE, 'MITTE-DEMO-0023'),
        ('kuechengeraete', 'demo-wasserkocher-pur', 'Wasserkocher Pur', 'Schnell heißes Wasser mit automatischer Abschaltung.', 'Beispielgerät mit 1,5 Liter Volumen und herausnehmbarem Filter. Energie-, Material- und Sicherheitsangaben sind Platzhalter.', 3499, 'https://images.unsplash.com/photo-1594213114663-d94db9b171ba?auto=format&fit=crop&w=900&q=80', 'Wasserkocher auf einer Küchenarbeitsfläche', 18, FALSE, 'MITTE-DEMO-0024'),
        ('haushaltsgeraete', 'demo-handstaubsauger', 'Akku-Handstaubsauger', 'Kompakte Hilfe für Krümel und kleine Flächen.', 'Handstaubsauger mit abnehmbarem Behälter. Laufzeit, Filterleistung und Ladezeit müssen durch echte Produktunterlagen belegt werden.', 4999, 'https://images.unsplash.com/photo-1558317374-067fb5f30001?auto=format&fit=crop&w=900&q=80', 'Kompaktes Reinigungsgerät im Haushalt', 10, FALSE, 'MITTE-DEMO-0025'),
        ('tisch', 'demo-servierteller-steinzeug', 'Servierteller Steinzeug', 'Schlichte Form für Frühstück und Abendessen.', 'Servierteller aus Steinzeug für den gedeckten Tisch. Maße, Glasur und Spülmaschineneignung vor realem Verkauf bestätigen.', 1799, 'https://images.unsplash.com/photo-1490312278390-ab64016e0aa9?auto=format&fit=crop&w=900&q=80', 'Gedeckter Tisch mit Geschirr', 28, FALSE, 'MITTE-DEMO-0026'),
        ('pflanzen', 'demo-pflanzset-kraeuter', 'Kräuter-Anzuchtset', 'Starte eine kleine Ernte auf Fensterbank oder Balkon.', 'Set mit Pflanzgefäßen und Saatgut als Demo. Saatgutart, Keimfähigkeit, Herkunft und Kennzeichnung vor Verkauf verifizieren.', 1599, 'https://images.unsplash.com/photo-1416879595882-3373a0480b5b?auto=format&fit=crop&w=900&q=80', 'Kräuter und Pflanzen auf einem Balkon', 34, TRUE, 'MITTE-DEMO-0027'),
        ('gartenmoebel', 'demo-balkonstuhl-klappbar', 'Klappstuhl Balkon', 'Platzsparende Sitzgelegenheit für kleine Außenbereiche.', 'Beispielhafter Klappstuhl für Balkon und Terrasse. Belastbarkeit, Maße und Witterungsbeständigkeit müssen geprüft werden.', 3999, 'https://images.unsplash.com/photo-1600210492486-724fe5c67fb0?auto=format&fit=crop&w=900&q=80', 'Sitzmöbel in einem Außenbereich', 8, FALSE, 'MITTE-DEMO-0028'),
        ('gartengeraete', 'demo-gartenschere-kompakt', 'Gartenschere Kompakt', 'Handliches Werkzeug für Pflegeschnitte.', 'Gartenschere mit ergonomischem Griff und Sicherheitsverschluss. Schnittstärke und Materialangaben sind Platzhalter.', 1499, 'https://images.unsplash.com/photo-1416879595882-3373a0480b5b?auto=format&fit=crop&w=900&q=80', 'Gartenarbeit mit Handwerkzeug', 20, FALSE, 'MITTE-DEMO-0029'),
        ('grillen', 'demo-grillbesteck-set', 'Grillbesteck Set', 'Drei praktische Werkzeuge für den Grillabend.', 'Beispielset mit Zange, Wender und Gabel. Material, Länge und Hitzebeständigkeit sind nicht zertifiziert.', 1999, 'https://images.unsplash.com/photo-1529193591184-b1d58069ecdd?auto=format&fit=crop&w=900&q=80', 'Grillabend im Freien', 17, FALSE, 'MITTE-DEMO-0030'),
        ('baby', 'demo-wickelauflage-soft', 'Wickelauflage Soft', 'Abwischbare Unterlage für den Wickelplatz.', 'Beispielauflage für den Wickeltisch. Maße, Inhaltsstoffe, Sicherheits- und Pflegeangaben müssen vor Verkauf geprüft werden.', 2499, 'https://images.unsplash.com/photo-1519689680058-324335c77eba?auto=format&fit=crop&w=900&q=80', 'Babyzimmer mit Wickelplatz', 7, FALSE, 'MITTE-DEMO-0031'),
        ('spielzeug', 'demo-holzbausteine', 'Holzbausteine 40-teilig', 'Offenes Spiel für kleine Bauideen.', 'Holzbausteine als Demo-Spielzeug. Altersangabe, Warnhinweise, Material und CE-Konformität müssen vor Verkauf geklärt werden.', 1999, 'https://images.unsplash.com/photo-1596461404969-9ae70f2830c1?auto=format&fit=crop&w=900&q=80', 'Bunte Holzbausteine für Kinder', 16, TRUE, 'MITTE-DEMO-0032'),
        ('kindermode', 'demo-kinderregenjacke', 'Kinderregenjacke', 'Leichte Jacke für wechselhaftes Wetter.', 'Beispieljacke mit Kapuze und reflektierenden Details. Größen, Material und Pflegekennzeichnung sind nicht verifiziert.', 2999, 'https://images.unsplash.com/photo-1519238263530-99bdd11df2ea?auto=format&fit=crop&w=900&q=80', 'Kinderkleidung für draußen', 10, FALSE, 'MITTE-DEMO-0033'),
        ('arbeitsplatz', 'demo-schreibtischstuhl', 'Arbeitsstuhl Balance', 'Bequemer Sitz für Homeoffice und Lernplatz.', 'Beispielstuhl mit höhenverstellbarem Sitz. Ergonomie, Belastbarkeit und Montageangaben sind nicht geprüft.', 11900, 'https://images.unsplash.com/photo-1497366754035-f200968a6e72?auto=format&fit=crop&w=900&q=80', 'Arbeitsstuhl am Schreibtisch', 6, TRUE, 'MITTE-DEMO-0034'),
        ('schreibwaren', 'demo-notizbuch-punkt', 'Notizbuch Punkt', 'Punktkariertes Papier für Listen und Skizzen.', 'Notizbuch mit festem Einband und Leseband. Seitenzahl und Papierqualität sind unverbindliche Demo-Angaben.', 899, 'https://images.unsplash.com/photo-1531346878377-a5be20888e57?auto=format&fit=crop&w=900&q=80', 'Notizbuch und Schreibwaren', 40, FALSE, 'MITTE-DEMO-0035'),
        ('buerotechnik', 'demo-tastatur-kompakt', 'Tastatur Kompakt', 'Kabelgebundene Tastatur mit leisem Anschlag.', 'Kompakte Tastatur im deutschen QWERTZ-Layout. Anschlüsse und Kompatibilität vor echtem Verkauf überprüfen.', 2499, 'https://images.unsplash.com/photo-1587829741301-dc798b83add3?auto=format&fit=crop&w=900&q=80', 'Tastatur auf einem Schreibtisch', 17, FALSE, 'MITTE-DEMO-0036'),
        ('hunde', 'demo-hundeleine-alltag', 'Hundeleine Alltag', 'Verstellbare Leine für tägliche Spaziergänge.', 'Leine aus textilem Gurtband mit Karabiner. Zugfestigkeit, Material und passende Tiergrößen vor Verkauf bestätigen.', 1799, 'https://images.unsplash.com/photo-1543466835-00a7907e9de1?auto=format&fit=crop&w=900&q=80', 'Hund an der Leine im Freien', 22, FALSE, 'MITTE-DEMO-0037'),
        ('katzen', 'demo-kratzbrett-wand', 'Kratzbrett Wand', 'Wandmontiertes Kratzbrett für aktive Katzen.', 'Kratzbrett mit austauschbarer Sisalfläche. Befestigung, Maße und Materialangaben sind Demo-Platzhalter.', 2199, 'https://images.unsplash.com/photo-1514888286974-6c03e2ca1dba?auto=format&fit=crop&w=900&q=80', 'Katze auf einem Kratzmöbel', 12, FALSE, 'MITTE-DEMO-0038'),
        ('tierfutter', 'demo-hundesnack-training', 'Trainingssnack Hund', 'Kleine Belohnung für gemeinsame Übungsmomente.', 'Demo-Futtersnack ohne bestätigte Inhaltsstoffe oder Nährwerte. Zusammensetzung, Kennzeichnung und Rückverfolgbarkeit sind vor Verkauf zwingend zu prüfen.', 599, 'https://images.unsplash.com/photo-1582798358481-d199fb7347bb?auto=format&fit=crop&w=900&q=80', 'Hundesnacks für das Training', 25, FALSE, 'MITTE-DEMO-0039'),
        ('kaffee-tee', 'demo-filterkaffee-mild', 'Filterkaffee Mild', 'Ausgewogene Röstung für den täglichen Kaffee.', 'Beispielhafte Kaffeebohnen als Sortimentsplatzhalter. Herkunft, Röstung, Gewicht und Lebensmittelkennzeichnung müssen verifiziert werden.', 1099, 'https://images.unsplash.com/photo-1442512595331-e89e73853f31?auto=format&fit=crop&w=900&q=80', 'Frisch gebrühter Filterkaffee', 30, TRUE, 'MITTE-DEMO-0040'),
        ('feinkost', 'demo-olivenoel-nativ', 'Olivenöl Nativ', 'Fruchtiges Öl für Küche und Tisch.', 'Demo-Lebensmittel mit vollständig unverifizierter Herkunft und Zusammensetzung. Nicht zum Verkauf geeignet, bevor Lieferant, Chargen und Pflichtangaben geklärt sind.', 1299, 'https://images.unsplash.com/photo-1474979266404-7eaacbcd87c5?auto=format&fit=crop&w=900&q=80', 'Olivenöl in einer Küche', 14, FALSE, 'MITTE-DEMO-0041'),
        ('snacks', 'demo-nussmix-geroestet', 'Nussmix Geröstet', 'Herzhafter Snack für kleine Pausen.', 'Demo-Lebensmittel ohne bestätigte Rezeptur, Allergendeklaration oder Nährwerte. Kennzeichnung und Rückverfolgbarkeit vor Verkauf verifizieren.', 499, 'https://images.unsplash.com/photo-1505576633757-0ac1084af824?auto=format&fit=crop&w=900&q=80', 'Nüsse und Snacks in einer Schale', 27, FALSE, 'MITTE-DEMO-0042'),
        ('auto-zubehoer', 'demo-kofferraum-organizer', 'Kofferraum-Organizer', 'Mehr Ordnung für Einkäufe und Zubehör.', 'Faltbarer Organizer mit mehreren Fächern. Material, Maße und Befestigung vor Livebetrieb prüfen.', 2299, 'https://images.unsplash.com/photo-1503376780353-7e6692767b70?auto=format&fit=crop&w=900&q=80', 'Fahrzeug im Alltag', 15, FALSE, 'MITTE-DEMO-0043'),
        ('mobilitaet', 'demo-fahrradschloss-city', 'Fahrradschloss City', 'Kompaktes Schloss für kurze Stopps unterwegs.', 'Beispielhaftes Fahrradschloss mit Schlüssel. Sicherheitsstufe und Widerstandswerte müssen durch einen echten Hersteller belegt werden.', 2499, 'https://images.unsplash.com/photo-1485965120184-e220f721d03e?auto=format&fit=crop&w=900&q=80', 'Fahrrad in der Stadt', 19, FALSE, 'MITTE-DEMO-0044'),
        ('handwerkzeuge', 'demo-schraubendreher-set', 'Schraubendreher Set', 'Acht gängige Profile für Werkstatt und Haushalt.', 'Demo-Werkzeugset aus acht Schraubendrehern. Material, Normen und Qualitätsangaben sind nicht bestätigt.', 1899, 'https://images.unsplash.com/photo-1504148455328-c376907d081c?auto=format&fit=crop&w=900&q=80', 'Handwerkzeuge auf einer Werkbank', 23, TRUE, 'MITTE-DEMO-0045'),
        ('elektrowerkzeuge', 'demo-akku-schrauber', 'Akku-Bohrschrauber 18 V', 'Flexibler Helfer für Montage und Reparatur.', 'Beispielgerät mit Akku und Ladegerät. Spannung, Drehmoment, Akkusystem und Sicherheitsunterlagen vor Verkauf verifizieren.', 8999, 'https://images.unsplash.com/photo-1581147036324-c1c89c2c8b5c?auto=format&fit=crop&w=900&q=80', 'Akku-Bohrschrauber als Werkzeug', 8, FALSE, 'MITTE-DEMO-0046'),
        ('messtechnik', 'demo-laser-entfernungsmesser', 'Laser-Entfernungsmesser', 'Schnelles Messen für Projekte rund ums Zuhause.', 'Demo-Messgerät mit digitaler Anzeige. Messbereich, Genauigkeit und rechtliche Produktangaben sind nicht verifiziert.', 4999, 'https://images.unsplash.com/photo-1504148455328-c376907d081c?auto=format&fit=crop&w=900&q=80', 'Messwerkzeug für Handwerksarbeiten', 9, FALSE, 'MITTE-DEMO-0047'),
        ('arbeitsschutz', 'demo-arbeitshandschuhe', 'Arbeitshandschuhe Grip', 'Griffiger Schutz für leichte Arbeiten.', 'Beispielhandschuhe für Montagearbeiten. Schutzklassen, Zertifizierungen und Größen sind vor realem Verkauf zu belegen.', 999, 'https://images.unsplash.com/photo-1504148455328-c376907d081c?auto=format&fit=crop&w=900&q=80', 'Arbeitshandschuhe für die Werkstatt', 31, FALSE, 'MITTE-DEMO-0048'),
        ('reisegepaeck', 'demo-kabinenkoffer-kompakt', 'Kabinenkoffer Kompakt', 'Leichtes Gepäck für kurze Reisen.', 'Koffer mit Rollen und Teleskopgriff als Demo. Außenmaße, Volumen und Airline-Kompatibilität müssen geprüft werden.', 7999, 'https://images.unsplash.com/photo-1553062407-98eeb64c6a62?auto=format&fit=crop&w=900&q=80', 'Kompakter Reisekoffer', 7, TRUE, 'MITTE-DEMO-0049'),
        ('reisebedarf', 'demo-packbeutel-set', 'Packbeutel Reise Set', 'Praktische Ordnung für Koffer und Rucksack.', 'Set aus Packbeuteln in verschiedenen Größen. Material und genaue Abmessungen sind Demo-Angaben.', 1699, 'https://images.unsplash.com/photo-1476514525535-07fb3b4ae5f1?auto=format&fit=crop&w=900&q=80', 'Reisegepäck für einen Urlaub', 18, FALSE, 'MITTE-DEMO-0050')
), category_ancestors(category_id, ancestor_id, depth) AS (
    SELECT id, id, 0 FROM categories
    UNION ALL
    SELECT ancestry.category_id, current.parent_id, ancestry.depth + 1
    FROM category_ancestors ancestry
    JOIN categories current ON current.id = ancestry.ancestor_id
    WHERE current.parent_id IS NOT NULL
), category_roots AS (
    SELECT DISTINCT ON (ancestry.category_id)
        ancestry.category_id, ancestry.ancestor_id AS root_id
    FROM category_ancestors ancestry
    JOIN categories root ON root.id = ancestry.ancestor_id
    WHERE root.parent_id IS NULL
    ORDER BY ancestry.category_id, ancestry.depth DESC
), category_templates AS (
    SELECT
        category.id,
        category.slug,
        category.name,
        template.name AS template_name,
        template.short_description AS template_short_description,
        template.description AS template_description,
        template.price_cents,
        template.image_url
    FROM categories category
    JOIN category_roots target_root ON target_root.category_id = category.id
    JOIN LATERAL (
        SELECT
            seed.name,
            seed.short_description,
            seed.description,
            seed.price_cents,
            seed.image_url,
            CASE WHEN ancestry.depth IS NULL THEN 1 ELSE 0 END AS fallback_order,
            ancestry.depth,
            seed.category_slug
        FROM seed
        JOIN categories source_category ON source_category.slug = seed.category_slug
        JOIN category_roots source_root ON source_root.category_id = source_category.id
        LEFT JOIN category_ancestors ancestry
            ON ancestry.category_id = category.id
            AND ancestry.ancestor_id = source_category.id
        WHERE source_root.root_id = target_root.root_id
        ORDER BY fallback_order, ancestry.depth NULLS LAST, seed.category_slug
        LIMIT 1
    ) template ON TRUE
), expanded_seed AS (
    SELECT
        category.slug AS category_slug,
        'demo-bulk-' || category.slug || '-' || lpad(variant.number::text, 4, '0') AS slug,
        category.template_name || ' · ' || category.name || ' Modell ' || lpad(variant.number::text, 4, '0') AS name,
        category.name || ': ' || category.template_short_description || ' Synthetische Demo-Variante ' || lpad(variant.number::text, 4, '0') || '.' AS short_description,
        category.template_description || E'\n\nMITTE-Demoprodukt: synthetische Platzhalterdaten, nicht für Verkauf oder Veröffentlichung bestimmt. Vor einem echten Angebot vollständig durch verifizierte Lieferantendaten ersetzen.' AS description,
        GREATEST(199, category.price_cents + ((variant.number * 71 + category.id * 131) % 4001) - 2000) AS price_cents,
        'https://picsum.photos/seed/mitte-' || category.slug || '-' || lpad(variant.number::text, 4, '0') || '/640/640' AS image_url,
        'Zufälliges Stockfoto als Platzhalter für ein synthetisches Produkt in der Kategorie ' || category.name || '; nicht das tatsächliche Produkt.' AS image_alt,
        ((variant.number * 17 + category.id * 13) % 51) AS inventory,
        (variant.number % 100 = 1) AS featured,
        'MITTE-DEMO-BULK-' || lpad(category.id::text, 5, '0') || '-' || lpad(variant.number::text, 4, '0') AS sku
    FROM category_templates category
    CROSS JOIN LATERAL generate_series(1, $1) AS variant(number)
), upserted AS (
INSERT INTO products (
    category_id, slug, name, short_description, description, price_cents,
    image_url, image_alt, inventory, featured, status, sku
)
SELECT
    categories.id,
    seed.slug,
    seed.name,
    seed.short_description,
    seed.description || E'\n\nMITTE-Demoprodukt: Preis, Bestand, Beschreibung und Bild sind synthetische Platzhalter. Vor einem Verkauf durch verifizierte Lieferantendaten ersetzen.',
    seed.price_cents,
    seed.image_url,
    seed.image_alt,
    seed.inventory,
    seed.featured,
    'active',
    seed.sku
FROM expanded_seed seed
JOIN categories ON categories.slug = seed.category_slug
ON CONFLICT (slug) DO UPDATE SET
    category_id = EXCLUDED.category_id,
    name = EXCLUDED.name,
    short_description = EXCLUDED.short_description,
    description = EXCLUDED.description,
    price_cents = EXCLUDED.price_cents,
    image_url = EXCLUDED.image_url,
    image_alt = EXCLUDED.image_alt,
    featured = EXCLUDED.featured,
    status = EXCLUDED.status,
    sku = EXCLUDED.sku,
    updated_at = NOW()
RETURNING id, inventory
), opening_balance AS (
    INSERT INTO product_inventory_movements (
        product_id, movement_type, quantity_delta, inventory_before, inventory_after,
        reason, reference, actor_email
    )
    SELECT id, 'opening_balance', inventory, 0, inventory,
        'Synthetic local development catalog opening balance', 'MITTE demo seed', 'system:mitte-demo-seed'
    FROM upserted
    WHERE inventory > 0
        AND NOT EXISTS (
            SELECT 1 FROM product_inventory_movements movement
            WHERE movement.product_id = upserted.id
                AND movement.movement_type = 'opening_balance'
                AND movement.reason = 'Synthetic local development catalog opening balance'
        )
    RETURNING product_id
)
SELECT COUNT(*)::integer FROM upserted;
