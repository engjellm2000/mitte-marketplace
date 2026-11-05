-- Product names, brands, barcodes, quantities, and pack-shot URLs were curated
-- from Open Beauty Facts. Product data is available under ODbL 1.0 and images
-- under CC BY-SA 3.0. Demo prices and stock are not verified retail offers.
WITH curated(
    category_slug, slug, name, short_description, description, price_cents,
    image_url, image_alt, inventory, sku, barcode, manufacturer
) AS (
    VALUES
        ('dufte', 'obf-6290360590189-lattafa-maahir-legacy', 'Lattafa Maahir Legacy', 'Duft · 100 ml', 'Original product listing: Lattafa Maahir Legacy, 100 ml.', 3999, 'https://images.openbeautyfacts.org/images/products/629/036/059/0189/front_en.4.400.jpg', 'Lattafa Maahir Legacy, 100 ml', 18, 'OBF-6290360590189', '6290360590189', 'Lattafa'),
        ('dufte', 'obf-6291108737194-lattafa-khamrah', 'Lattafa Khamrah Eau de Parfum', 'Eau de Parfum · 100 ml', 'Original product listing: Lattafa Khamrah, 100 ml.', 4499, 'https://images.openbeautyfacts.org/images/products/629/110/873/7194/front_fr.6.400.jpg', 'Lattafa Khamrah Eau de Parfum, 100 ml', 14, 'OBF-6291108737194', '6291108737194', 'Lattafa'),
        ('dufte', 'obf-6290360593661-lattafa-khamrah-qahwa', 'Lattafa Khamrah Qahwa', 'Duft · 100 ml', 'Original product listing: Lattafa Khamrah Qahwa, 100 ml.', 4699, 'https://images.openbeautyfacts.org/images/products/629/036/059/3661/front_en.3.400.jpg', 'Lattafa Khamrah Qahwa, 100 ml', 12, 'OBF-6290360593661', '6290360593661', 'Lattafa'),
        ('dufte', 'obf-6290362345749-lattafa-my-yara-collection', 'Lattafa My Yara Collection Perfume Set', 'Eau de Parfum · 4 × 25 ml', 'Original product listing: Lattafa My Yara Collection, four 25 ml fragrances.', 5299, 'https://images.openbeautyfacts.org/images/products/629/036/234/5749/front_es.3.400.jpg', 'Lattafa My Yara Collection perfume set', 9, 'OBF-6290362345749', '6290362345749', 'Lattafa'),
        ('dufte', 'obf-6085010044712-armaf-club-de-nuit-intense-man', 'Armaf Club De Nuit Intense Man Eau de Toilette', 'Eau de Toilette · 105 ml', 'Original product listing: Armaf Club De Nuit Intense Man EDT, 105 ml.', 4999, 'https://images.openbeautyfacts.org/images/products/608/501/004/4712/front_fr.8.400.jpg', 'Armaf Club De Nuit Intense Man Eau de Toilette, 105 ml', 11, 'OBF-6085010044712', '6085010044712', 'Armaf'),
        ('haarpflege', 'obf-5601059062534-head-shoulders-classic-clean-2-in-1', 'Head & Shoulders Classic Clean 2 in 1 Shampoo', 'Shampoo & Pflege · 200 ml', 'Original product listing: Head & Shoulders Classic Clean 2 in 1 Shampoo, 200 ml.', 499, 'https://images.openbeautyfacts.org/images/products/560/105/906/2534/front_tr.3.400.jpg', 'Head & Shoulders Classic Clean 2 in 1 Shampoo, 200 ml', 32, 'OBF-5601059062534', '5601059062534', 'Head & Shoulders'),
        ('haarpflege', 'obf-8001090047014-head-shoulders-classic-clean', 'Head & Shoulders Classic Clean Shampoo', 'Shampoo · 300 ml', 'Original product listing: Head & Shoulders Classic Clean, 300 ml.', 599, 'https://images.openbeautyfacts.org/images/products/800/109/004/7014/front_de.7.400.jpg', 'Head & Shoulders Classic Clean Shampoo, 300 ml', 26, 'OBF-8001090047014', '8001090047014', 'Head & Shoulders'),
        ('haarpflege', 'obf-4084500015012-head-shoulders-2-in-1-anti-dandruff', 'Head & Shoulders 2 in 1 Anti-Dandruff Shampoo & Conditioner', 'Shampoo & Conditioner', 'Original product listing: Head & Shoulders 2 in 1 Anti Dandruff Shampoo & Conditioner.', 799, 'https://images.openbeautyfacts.org/images/products/408/450/001/5012/front_en.8.400.jpg', 'Head & Shoulders 2 in 1 Anti-Dandruff Shampoo and Conditioner', 19, 'OBF-4084500015012', '4084500015012', 'Head & Shoulders'),
        ('haarpflege', 'obf-4987176279439-head-shoulders-bare-pure-clean', 'Head & Shoulders Bare Pure Clean Anti-Dandruff Shampoo', 'Shampoo · 400 ml', 'Original product listing: Head & Shoulders Bare Pure Clean Anti-Dandruff Shampoo, 400 ml.', 899, 'https://images.openbeautyfacts.org/images/products/498/717/627/9439/front_en.3.400.jpg', 'Head & Shoulders Bare Pure Clean Shampoo, 400 ml', 21, 'OBF-4987176279439', '4987176279439', 'Head & Shoulders'),
        ('haarpflege', 'obf-8001090580122-head-shoulders-supreme-repair', 'Head & Shoulders Supreme Repair Anti-Dandruff Shampoo', 'Shampoo · 400 ml', 'Original product listing: Head & Shoulders Supreme Repair Anti-Dandruff Shampoo, 400 ml.', 999, 'https://images.openbeautyfacts.org/images/products/800/109/058/0122/front_en.5.400.jpg', 'Head & Shoulders Supreme Repair Shampoo, 400 ml', 17, 'OBF-8001090580122', '8001090580122', 'Head & Shoulders'),
        ('haarpflege', 'obf-4015100716931-schauma-fresh-volume', 'Schauma Fresh Volume Shampoo', 'Shampoo', 'Original product listing: Schauma Fresh Volume Shampoo.', 399, 'https://images.openbeautyfacts.org/images/products/401/510/071/6931/front_en.3.400.jpg', 'Schauma Fresh Volume Shampoo', 24, 'OBF-4015100716931', '4015100716931', 'Schauma'),
        ('haarpflege', 'obf-4015100721843-schauma-kids-shampoo-balsam', 'Schauma Kids Shampoo & Balsam', 'Shampoo & Balsam · 250 ml', 'Original product listing: Schauma Kids Shampoo & Balsam, 250 ml.', 399, 'https://images.openbeautyfacts.org/images/products/401/510/072/1843/front_en.7.400.jpg', 'Schauma Kids Shampoo and Balsam, 250 ml', 20, 'OBF-4015100721843', '4015100721843', 'Schauma'),
        ('haarpflege', 'obf-4015001005677-schauma-frucht-vitamin', 'Schauma Frucht & Vitamin Shampoo', 'Shampoo · 400 ml', 'Original product listing: Schauma Frucht & Vitamin Shampoo, 400 ml.', 399, 'https://images.openbeautyfacts.org/images/products/401/500/100/5677/front_de.4.400.jpg', 'Schauma Frucht and Vitamin Shampoo, 400 ml', 18, 'OBF-4015001005677', '4015001005677', 'Schauma'),
        ('haarpflege', 'obf-4015001004656-schauma-frucht-vitamin-2-pack', 'Schauma Frucht & Vitamin Shampoo, 2er-Pack', 'Shampoo · 2 × 400 ml', 'Original product listing: Schauma Frucht & Vitamin Shampoo twin pack, 2 × 400 ml.', 699, 'https://images.openbeautyfacts.org/images/products/401/500/100/4656/front_de.4.400.jpg', 'Schauma Frucht and Vitamin Shampoo twin pack', 13, 'OBF-4015001004656', '4015001004656', 'Schauma'),
        ('haarpflege', 'obf-4015100188134-schauma-sonnengekuesstes-blond', 'Schauma Sonnengeküsstes Blond Shampoo', 'Shampoo · 400 ml', 'Original product listing: Schauma Sonnengeküsstes Blond Shampoo, 400 ml.', 399, 'https://images.openbeautyfacts.org/images/products/401/510/018/8134/front_de.4.400.jpg', 'Schauma Sonnengeküsstes Blond Shampoo, 400 ml', 12, 'OBF-4015100188134', '4015100188134', 'Schauma'),
        ('haarpflege', 'obf-4015001005462-schauma-pro-vitamin-b5', 'Schauma Pro-Vitamin B5 Shampoo', 'Shampoo · 400 ml', 'Original product listing: Schauma Pro-Vitamin B5 Shampoo, 400 ml.', 399, 'https://images.openbeautyfacts.org/images/products/401/500/100/5462/front_de.4.400.jpg', 'Schauma Pro-Vitamin B5 Shampoo, 400 ml', 16, 'OBF-4015001005462', '4015001005462', 'Schauma'),
        ('haarpflege', 'obf-3600542389204-garnier-fructis-watermelon', 'Garnier Fructis Watermelon Shampoo', 'Shampoo · 350 ml', 'Original product listing: Garnier Fructis Watermelon Shampoo, 350 ml.', 499, 'https://images.openbeautyfacts.org/images/products/360/054/238/9204/front_fr.10.400.jpg', 'Garnier Fructis Watermelon Shampoo, 350 ml', 18, 'OBF-3600542389204', '3600542389204', 'Garnier'),
        ('haarpflege', 'obf-3600541946613-garnier-fructis-anticaspa', 'Garnier Fructis Anticaspa Shampoo', 'Shampoo · 400 ml', 'Original product listing: Garnier Fructis Anticaspa Shampoo, 400 ml.', 499, 'https://images.openbeautyfacts.org/images/products/360/054/194/6613/front_pt.3.400.jpg', 'Garnier Fructis Anticaspa Shampoo, 400 ml', 14, 'OBF-3600541946613', '3600541946613', 'Garnier'),
        ('haarpflege', 'obf-3600541970533-garnier-fructis-coco-water', 'Garnier Fructis Coco Water Clarifying Shampoo', 'Shampoo · 400 ml', 'Original product listing: Garnier Fructis Coco Water, 400 ml.', 499, 'https://images.openbeautyfacts.org/images/products/360/054/197/0533/front_en.10.400.jpg', 'Garnier Fructis Coco Water Shampoo, 400 ml', 17, 'OBF-3600541970533', '3600541970533', 'Garnier'),
        ('haarpflege', 'obf-3600542543361-garnier-fructis-oil-repair-3', 'Garnier Fructis Oil Repair 3 Shampoo', 'Shampoo · 250 ml', 'Original product listing: Garnier Fructis Oil Repair 3, 250 ml.', 499, 'https://images.openbeautyfacts.org/images/products/360/054/254/3361/front_it.7.400.jpg', 'Garnier Fructis Oil Repair 3 Shampoo, 250 ml', 15, 'OBF-3600542543361', '3600542543361', 'Garnier'),
        ('haarpflege', 'obf-5410091712365-schwarzkopf-gliss-ultimate-repair', 'Schwarzkopf Gliss Kur Ultimate Repair Shampoo', 'Shampoo · 250 ml', 'Original product listing: Schwarzkopf Gliss Kur Ultimate Repair Shampoo, 250 ml.', 499, 'https://images.openbeautyfacts.org/images/products/541/009/171/2365/front_en.5.400.jpg', 'Schwarzkopf Gliss Kur Ultimate Repair Shampoo, 250 ml', 16, 'OBF-5410091712365', '5410091712365', 'Schwarzkopf'),
        ('hautpflege', 'obf-4005900009135-nivea-soft-cream', 'NIVEA Soft Moisturizing Cream', 'Feuchtigkeitscreme · 100 ml', 'Original product listing: NIVEA Soft Moisturizing Cream, 100 ml.', 499, 'https://images.openbeautyfacts.org/images/products/400/590/000/9135/front_en.3.400.jpg', 'NIVEA Soft Moisturizing Cream, 100 ml', 25, 'OBF-4005900009135', '4005900009135', 'NIVEA'),
        ('hautpflege', 'obf-4005900525017-nivea-anti-wrinkle-firming-day-cream', 'NIVEA Anti-Wrinkle + Firming Day Cream 45+', 'Tagescreme · 50 ml', 'Original product listing: NIVEA Anti-Wrinkle + Firming Day Cream 45+, 50 ml.', 799, 'https://images.openbeautyfacts.org/images/products/400/590/052/5017/front_en.8.400.jpg', 'NIVEA Anti-Wrinkle and Firming Day Cream, 50 ml', 15, 'OBF-4005900525017', '4005900525017', 'NIVEA'),
        ('hautpflege', 'obf-4005808357369-nivea-aloe-hand-cream', 'NIVEA Aloe Vera Hand Cream', 'Handcreme · 75 ml', 'Original product listing: NIVEA Aloe Vera Hand Cream, 75 ml.', 299, 'https://images.openbeautyfacts.org/images/products/400/580/835/7369/front_en.9.400.jpg', 'NIVEA Aloe Vera Hand Cream, 75 ml', 20, 'OBF-4005808357369', '4005808357369', 'NIVEA'),
        ('gesichtspflege', 'obf-3600541594968-garnier-micellar-water-sensitive', 'Garnier SkinActive Micellar Cleansing Water', 'Mizellenwasser · 400 ml', 'Original product listing: Garnier SkinActive Micellar Water for dry and sensitive skin, 400 ml.', 599, 'https://images.openbeautyfacts.org/images/products/360/054/159/4968/front_fr.4.400.jpg', 'Garnier SkinActive Micellar Cleansing Water, 400 ml', 18, 'OBF-3600541594968', '3600541594968', 'Garnier'),
        ('gesichtspflege', 'obf-3600541358485-garnier-micellar-water-all-in-one', 'Garnier SkinActive Micellar Water All-in-1', 'Mizellenwasser · 400 ml', 'Original product listing: Garnier SkinActive Micellar Water All-in-1, 400 ml.', 599, 'https://images.openbeautyfacts.org/images/products/360/054/135/8485/front_en.3.400.jpg', 'Garnier SkinActive Micellar Water All-in-1, 400 ml', 17, 'OBF-3600541358485', '3600541358485', 'Garnier'),
        ('gesichtspflege', 'obf-3600541358461-garnier-micellar-cleansing-water', 'Garnier Micellar Cleansing Water', 'Mizellenwasser · 400 ml', 'Original product listing: Garnier Micellar Cleansing Water, 400 ml.', 599, 'https://images.openbeautyfacts.org/images/products/360/054/135/8461/front_en.4.400.jpg', 'Garnier Micellar Cleansing Water, 400 ml', 15, 'OBF-3600541358461', '3600541358461', 'Garnier'),
        ('gesichtspflege', 'obf-3600541938489-garnier-micellar-all-in-one-700ml', 'Garnier Micellar Cleansing Water All-in-1', 'Mizellenwasser · 700 ml', 'Original product listing: Garnier Micellar Cleansing Water All-in-1, 700 ml.', 899, 'https://images.openbeautyfacts.org/images/products/360/054/193/8489/front_en.25.400.jpg', 'Garnier Micellar Cleansing Water All-in-1, 700 ml', 13, 'OBF-3600541938489', '3600541938489', 'Garnier'),
        ('gesichtspflege', 'obf-3600541897694-garnier-micellar-cleansing-water-in-oil', 'Garnier Micellar Cleansing Water in Oil', 'Mizellenwasser · 400 ml', 'Original product listing: Garnier Micellar Cleansing Water in Oil, 400 ml.', 699, 'https://images.openbeautyfacts.org/images/products/360/054/189/7694/front_ar.9.400.jpg', 'Garnier Micellar Cleansing Water in Oil, 400 ml', 14, 'OBF-3600541897694', '3600541897694', 'Garnier'),
        ('gesichtspflege', 'obf-3600542133425-garnier-micellar-cleansing-water-3-in-1', 'Garnier Micellar Cleansing Water 3-in-1', 'Mizellenwasser · 100 ml', 'Original product listing: Garnier Micellar Cleansing Water 3-in-1, 100 ml.', 299, 'https://images.openbeautyfacts.org/images/products/360/054/213/3425/front_en.4.400.jpg', 'Garnier Micellar Cleansing Water 3-in-1, 100 ml', 22, 'OBF-3600542133425', '3600542133425', 'Garnier'),
        ('gesichtspflege', 'obf-3600541729476-garnier-micellar-cleansing-water-125ml', 'Garnier Micellar Cleansing Water', 'Mizellenwasser · 125 ml', 'Original product listing: Garnier Micellar Cleansing Water, 125 ml.', 399, 'https://images.openbeautyfacts.org/images/products/360/054/172/9476/front_en.10.400.jpg', 'Garnier Micellar Cleansing Water, 125 ml', 19, 'OBF-3600541729476', '3600541729476', 'Garnier'),
        ('make-up', 'obf-30166967-maybelline-sky-high-mascara', 'Maybelline New York Lash Sensational Sky High Very Black Mascara', 'Mascara · 7,2 ml', 'Original product listing: Maybelline New York Lash Sensational Sky High Very Black Mascara, 7.2 ml.', 999, 'https://images.openbeautyfacts.org/images/products/000/003/016/6967/front_nl.10.400.jpg', 'Maybelline Lash Sensational Sky High Very Black Mascara', 17, 'OBF-30166967', '30166967', 'Maybelline'),
        ('make-up', 'obf-30166974-maybelline-colossal-36h-mascara', 'Maybelline New York The Colossal 36H Waterproof Mascara', 'Mascara · wasserfest', 'Original product listing: Maybelline New York The Colossal 36H Longwear Waterproof Mascara.', 899, 'https://images.openbeautyfacts.org/images/products/000/003/016/6974/front_nl.13.400.jpg', 'Maybelline The Colossal 36H Waterproof Mascara', 15, 'OBF-30166974', '30166974', 'Maybelline'),
        ('make-up', 'obf-3600531143459-maybelline-lash-sensational', 'Maybelline New York Lash Sensational Very Black Mascara', 'Mascara · 9,5 ml', 'Original product listing: Maybelline New York Lash Sensational Very Black Volume Mascara, 9.5 ml.', 899, 'https://images.openbeautyfacts.org/images/products/360/053/114/3459/front_en.5.400.jpg', 'Maybelline Lash Sensational Very Black Mascara, 9.5 ml', 18, 'OBF-3600531143459', '3600531143459', 'Maybelline'),
        ('make-up', 'obf-3600531584696-maybelline-falsies-lash-lift', 'Maybelline New York The Falsies Lash Lift Black Mascara', 'Mascara', 'Original product listing: Maybelline New York The Falsies Lash Lift Black Mascara.', 899, 'https://images.openbeautyfacts.org/images/products/360/053/158/4696/front_en.5.400.jpg', 'Maybelline The Falsies Lash Lift Black Mascara', 14, 'OBF-3600531584696', '3600531584696', 'Maybelline')
), upserted AS (
    INSERT INTO products (
        category_id, slug, name, short_description, description, price_cents,
        image_url, image_alt, inventory, featured, status, sku, barcode, manufacturer
    )
    SELECT
        categories.id,
        curated.slug,
        curated.name,
        curated.short_description,
        curated.description || E'\n\nProduktdaten: Open Beauty Facts, ODbL 1.0. Produktfoto: Open Beauty Facts, CC BY-SA 3.0. Community-Daten können unvollständig oder fehlerhaft sein; vor einem Verkauf verifizieren. Preis und Bestand sind lokale Demo-Platzhalter.\nQuelle: https://world.openbeautyfacts.org/product/' || curated.barcode,
        curated.price_cents,
        curated.image_url,
        curated.image_alt,
        curated.inventory,
        FALSE,
        'active',
        curated.sku,
        curated.barcode,
        curated.manufacturer
    FROM curated
    JOIN categories ON categories.slug = curated.category_slug
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
        barcode = EXCLUDED.barcode,
        manufacturer = EXCLUDED.manufacturer,
        updated_at = NOW()
    RETURNING id, inventory
), opening_balance AS (
    INSERT INTO product_inventory_movements (
        product_id, movement_type, quantity_delta, inventory_before, inventory_after,
        reason, reference, actor_email
    )
    SELECT id, 'opening_balance', inventory, 0, inventory,
        'Curated local demo catalog opening balance', 'Open Beauty Facts demo seed', 'system:mitte-demo-seed'
    FROM upserted
    WHERE inventory > 0
        AND NOT EXISTS (
            SELECT 1 FROM product_inventory_movements movement
            WHERE movement.product_id = upserted.id
                AND movement.movement_type = 'opening_balance'
                AND movement.reason = 'Curated local demo catalog opening balance'
        )
    RETURNING product_id
)
SELECT COUNT(*)::integer FROM upserted;
