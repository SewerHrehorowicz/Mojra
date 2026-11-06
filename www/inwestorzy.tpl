doctype html
html lang: pl
    head
        meta charset: UTF-8
        meta
            name: viewport
            content: "width=device-width, initial-scale=1.0"
        title "Mojra — Dla Inwestorów i Partnerów"
        link
            rel: icon
            type: image/png
            sizes: 256x256
            href: icon.png
        link
            rel: icon
            type: image/x-icon
            href: favicon.ico
        link
            rel: stylesheet
            href: style.css
    body.pitch-page data-theme: dark
        canvas.loom-canvas#loom-canvas aria-hidden: "true"

        header.pitch-topbar
            a.pitch-back href: index.html
                "← POWRÓT"
            svg.pitch-topbar-logo src: /icons/mojra-logo.svg
            a.pitch-cta-top href: "mailto:inwestorzy@mojra.pl?subject=Mojra%20%E2%80%94%20Rozmowa%20inwestorska%20%2F%20Pokaz%20na%20%C5%BCywo"
                span.corners
                "UMÓW POKAZ NA ŻYWO"

        main.pitch-container
            section.pitch-hero
                h1.pitch-title "Cyfrowe opowieści dotąd tylko naśladowały papier."
                p.pitch-subtitle "My daliśmy im własny język, głos i wybór."
                .hero-divider aria-hidden: "true"
                p.pitch-lead "Bez slajdów z obietnicami za dwa lata. Cały silnik działa już na macOS, Windows i Linux — właśnie otwieramy zamkniętą betę dla pierwszych polskich pisarzy, scenarzystów i studiów audio."

                .pitch-kpi-strip
                    .pitch-kpi-cell
                        span.pitch-kpi-val "600–700 mln"
                        span.pitch-kpi-lbl
                            "RYNEK KSIĄŻKI CYFROWEJ PL (ZŁ) "
                            a.cite-ref
                                href: "https://www.instytutksiazki.pl/wp-content/uploads/2026/10/the-polish-book-market-2026.pdf"
                                target: _blank
                                rel: "noopener noreferrer"
                                "[1]"
                            a.cite-ref
                                href: "https://wydawca.com.pl/2025/05/30/rynek-publikacji-cyfrowych-w-polsce/"
                                target: _blank
                                rel: "noopener noreferrer"
                                "[2]"
                    .pitch-kpi-cell
                        span.pitch-kpi-val "3,7 mln"
                        span.pitch-kpi-lbl
                            "SŁUCHACZY AUDIO (+20,9% R/R) "
                            a.cite-ref
                                href: "https://raportrynekaudiobookowy.pl/en"
                                target: _blank
                                rel: "noopener noreferrer"
                                "[3]"
                    .pitch-kpi-cell
                        span.pitch-kpi-val "~96%"
                        span.pitch-kpi-lbl "MARŻA BRUTTO STUDIO (LOCAL-FIRST)"
                    .pitch-kpi-cell
                        span.pitch-kpi-val "85 / 15%"
                        span.pitch-kpi-lbl "PODZIAŁ PRZYCHODU W KATALOGU"

            section.pitch-section
                .pitch-section-header
                    span.pitch-num "01 / RYNEK I LUKA TECHNOLOGICZNA"
                    h2.pitch-heading "Połowa konsumpcji w aplikacjach to już audio, ale formaty są ślepe na interakcję"
                    p.pitch-text
                        "Wartość polskiego rynku e-booków (350–400 mln zł) i audiobooków (228–300 mln zł) sięga łącznie 600–700 mln zł rocznie "
                        a.cite-ref
                            href: "https://www.instytutksiazki.pl/wp-content/uploads/2026/10/the-polish-book-market-2026.pdf"
                            target: _blank
                            rel: "noopener noreferrer"
                            "[1]"
                        a.cite-ref
                            href: "https://wydawca.com.pl/2025/05/30/rynek-publikacji-cyfrowych-w-polsce/"
                            target: _blank
                            rel: "noopener noreferrer"
                            "[2]"
                        ". Jednocześnie e-booki (.epub) i audiobooki (.mp3) wymuszają płaską, liniową lekturę, a twórcy słuchowisk, gier narracyjnych i scenariuszy muszą sklejać ze sobą Worda, Excela, Miro i programy z lat 90."

                .pitch-charts-grid
                    .pitch-chart-box
                        p.pitch-chart-title
                            "RYNEK AUDIOBOOKÓW W POLSCE — WARTOŚĆ I PROGNOZA (MLN ZŁ) "
                            a.cite-ref
                                href: "https://raportrynekaudiobookowy.pl/en"
                                target: _blank
                                rel: "noopener noreferrer"
                                "[3]"
                        .pitch-bars
                            .pitch-bar-row
                                span.pitch-bar-label "2024"
                                .pitch-bar-track
                                    .pitch-bar-fill.w-56
                                span.pitch-bar-value "198,7 mln"
                            .pitch-bar-row
                                span.pitch-bar-label "2025"
                                .pitch-bar-track
                                    .pitch-bar-fill.hi.w-65
                                span.pitch-bar-value "227,9 mln"
                            .pitch-bar-row
                                span.pitch-bar-label "2025 E-C"
                                .pitch-bar-track
                                    .pitch-bar-fill.hi.w-85
                                span.pitch-bar-value "~300 mln"
                            .pitch-bar-row
                                span.pitch-bar-label "2030P"
                                .pitch-bar-track
                                    .pitch-bar-fill.hi.w-100
                                span.pitch-bar-value "352,3 mln"

                    .pitch-chart-box
                        p.pitch-chart-title
                            "STRUKTURA KONSUMPCJI W APLIKACJACH (DANE EMPIK GO) "
                            a.cite-ref
                                href: "https://www.instytutksiazki.pl/wp-content/uploads/2026/10/the-polish-book-market-2026.pdf"
                                target: _blank
                                rel: "noopener noreferrer"
                                "[1]"
                            a.cite-ref
                                href: "https://wydawca.com.pl/2025/05/30/rynek-publikacji-cyfrowych-w-polsce/"
                                target: _blank
                                rel: "noopener noreferrer"
                                "[2]"
                        .pitch-split-bar
                            .pitch-split-seg.audio "51% AUDIOBOOKI & SŁUCHOWISKA"
                            .pitch-split-seg.ebook "49% E-BOOKI"
                        p.pitch-chart-note
                            "W 2025 r. liczba płacących subskrybentów Empik Go wzrosła o 60% r/r (Storytel PL +55% r/r) "
                            a.cite-ref
                                href: "https://www.instytutksiazki.pl/wp-content/uploads/2026/10/the-polish-book-market-2026.pdf"
                                target: _blank
                                rel: "noopener noreferrer"
                                "[1]"
                            ", a Audioteka wyprodukowała w rok 35 wieloobsadowych superprodukcji (+84% r/r) "
                            a.cite-ref
                                href: "https://raportrynekaudiobookowy.pl/en"
                                target: _blank
                                rel: "noopener noreferrer"
                                "[3]"
                            "."

            section.pitch-section
                .pitch-section-header
                    span.pitch-num "02 / ARCHITEKTURA PRODUKTU"
                    h2.pitch-heading "Jedno źródło. Cztery rynki zbytu"
                    p.pitch-text "W Playbook Studio twórca pracuje na jednym żywym grafie (WebGL 120fps) z wbudowanym Gitem dla wariantów fabuły i rentgenem produkcyjnym na żywo (1/8 strony, czas nagrań, dni zdjęciowe, kwestie per aktor)."

                .pitch-diagram-wrap
                    svg.pitch-diagram-svg src: /icons/pitch-diagram.svg

            section.pitch-section
                .pitch-section-header
                    span.pitch-num "03 / EKONOMIA JEDNOSTKOWA I INFRASTRUKTURA"
                    h2.pitch-heading "Architektura Local-First: ciężka praca na dysku autora, lekki CDN dla katalogu"
                    p.pitch-text "W klasycznym chmurowym SaaS-ie każdy użytkownik generuje stały koszt baz danych, serwerów aplikacyjnych i transferu wielogigabajtowych sesji audio. W Moirze całe środowisko Playbook Studio działa lokalnie na urządzeniu twórcy (0 zł kosztu chmury edycyjnej, jedyny koszt zmienny to ~2% operatora płatności). Z kolei katalog gotowych publikacji .playbook wymaga wyłącznie statycznego obiektowego CDN (np. Cloudflare R2 z zerową opłatą za transfer wychodzący) i bramki płatniczej."

                .pitch-charts-grid
                    .pitch-chart-box
                        p.pitch-chart-title "SZACOWANA MARŻA BRUTTO PO KOSZTACH INFRASTRUKTURY I BRAMEK"
                        .pitch-bars
                            .pitch-bar-row
                                span.pitch-bar-label-wide "Licencje Playbook Studio"
                                .pitch-bar-track
                                    .pitch-bar-fill.hi.w-96
                                span.pitch-bar-value "~96%"
                            .pitch-bar-row
                                span.pitch-bar-label-wide "Katalog .playbook (CDN)"
                                .pitch-bar-track
                                    .pitch-bar-fill.hi.w-88
                                span.pitch-bar-value "~88%"
                            .pitch-bar-row
                                span.pitch-bar-label-wide "Typowy Chmurowy SaaS"
                                .pitch-bar-track
                                    .pitch-bar-fill.w-68
                                span.pitch-bar-value "65–72%"

                    .pitch-chart-box
                        p.pitch-chart-title "UDZIAŁ TWÓRCY ZE SPRZEDAŻY DETALICZNEJ"
                        .pitch-bars
                            .pitch-bar-row
                                span.pitch-bar-label-wide "Katalog Mojra (.playbook)"
                                .pitch-bar-track
                                    .pitch-bar-fill.hi.w-85
                                span.pitch-bar-value "85%"
                            .pitch-bar-row
                                span.pitch-bar-label-wide "Tradycyjni pośrednicy"
                                .pitch-bar-track
                                    .pitch-bar-fill.w-35
                                span.pitch-bar-value "30–45%"

            section.pitch-footer-cta
                p.pitch-cta-headline "Chcesz zobaczyć Mojrę na żywo i przejrzeć pełne wyliczenia?"
                p.pitch-cta-sub "Napisz bezpośrednio — pokażemy Ci działające Studio w 15 minut i prześlemy deck finansowy."
                a.beta-submit href: "mailto:inwestorzy@mojra.pl?subject=Mojra%20%E2%80%94%20Pitch%20Deck%20i%20pokaz%20na%20%C5%BCywo"
                    span.corners
                    "NAPISZ DO TWÓRCÓW • INWESTORZY@MOJRA.PL"

            footer.pitch-sources
                p.pitch-sources-title "ŹRÓDŁA I RAPORTY RYNKOWE"
                .pitch-sources-list
                    a.pitch-source-item
                        href: "https://www.instytutksiazki.pl/wp-content/uploads/2026/10/the-polish-book-market-2026.pdf"
                        target: _blank
                        rel: "noopener noreferrer"
                        "[1] Instytut Książki — The Polish Book Market 2025 / 2026 (wartość rynku e-booków 350–400 mln zł, audiobooków 230–300 mln zł, dynamika Empik Go i Storytel)"
                    a.pitch-source-item
                        href: "https://wydawca.com.pl/2025/05/30/rynek-publikacji-cyfrowych-w-polsce/"
                        target: _blank
                        rel: "noopener noreferrer"
                        "[2] PMR Market Experts & Biblioteka Analiz — Rynek publikacji cyfrowych w Polsce 2024/2025 (łączna wartość rynku cyfrowego 600–700 mln zł, struktura konsumpcji 51% audio vs 49% e-book w Empik Go)"
                    a.pitch-source-item
                        href: "https://raportrynekaudiobookowy.pl/en"
                        target: _blank
                        rel: "noopener noreferrer"
                        "[3] Pick a Voice & Grand View Research — Raport Rynku Audiobookowego w Polsce 2025 (227,9 mln zł w 2025 r., ~300 mln zł e-commerce, prognoza 352,3 mln zł do 2030 r., 3,7 mln słuchaczy, 35 superprodukcji Audioteki)"
                    a.pitch-source-item
                        href: "https://pisf.pl/"
                        target: _blank
                        rel: "noopener noreferrer"
                        "[4] Polski Instytut Sztuki Filmowej (PISF) — Programy Operacyjne i standardy formatowania scenariuszy filmowych w Polsce"

        script src: /app.js
