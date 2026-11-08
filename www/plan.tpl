doctype html
html lang: pl
    head
        meta charset: UTF-8
        meta
            name: viewport
            content: "width=device-width, initial-scale=1.0"
        title "Mojra — Plan Działania Pilot-First (12 Miesięcy)"
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
    body.pitch-page.deck-page data-theme: light
        header.pitch-topbar
            svg.deck-header-loom src: /icons/deck-loom-header.svg
            a.pitch-back href: inwestorzy.html
                "← DLA INWESTORÓW"
            .deck-brand-hero
                .brand-lockup
                    svg.brand-logo-svg src: /icons/mojra-logo.svg
                p.brand-subtitle
                    "MOJRA APP"
                    span.plus "+"
                    "PLAYBOOK STUDIO"
                .hero-divider aria-hidden: "true"
            a.pitch-cta-top href: mojra-plan.pdf
                span.corners
                "POBIERZ PDF"

        main.pitch-container
            .deck-sheet.sheet-1
                .deck-sheet-body
                    section.pitch-hero
                        span.pitch-num "PLAN DZIAŁANIA (PO STRESS-TEŚCIE) • STRATEGIA PILOT-FIRST I DYSTRYBUCJA WIELOKANAŁOWA"
                        h1.pitch-title "Najpierw 1-godzinny Pilot za 8 000 zł, potem przedsprzedaż i wydanie na Steam, iOS i Android."
                        p.pitch-lead "Nie zamrażamy 50 tys. zł w ciemno na pełne nagranie książki i nie czekamy, aż użytkownicy zainstalują nowy czytnik. Każdy tytuł testujemy tanim 1-godzinnym Pilotem, finansujemy z przedsprzedaży i wydajemy równocześnie na Steam, App Store, Google Play oraz w darmowym czytniku Mojra."

                        .plan-gantt
                            .plan-gantt-head
                                span.plan-gantt-task-hdr "ZADANIE / ŚCIEŻKA REALIZACJI"
                                span.plan-gantt-q "KWARTAŁ I (M1–M3)"
                                span.plan-gantt-q "KWARTAŁ II (M4–M6)"
                                span.plan-gantt-q "KWARTAŁ III (M7–M9)"
                                span.plan-gantt-q "KWARTAŁ IV (M10–M12)"
                            .plan-gantt-row
                                span.plan-gantt-label "1. Audyt bezpieczeństwa kodu + certyfikaty Apple/MS"
                                .plan-gantt-track
                                    .plan-gantt-bar.g-m1-1
                            .plan-gantt-row
                                span.plan-gantt-label "2. Nagranie dwóch 1-godzinnych Pilotów (2 × 8 000 zł)"
                                .plan-gantt-track
                                    .plan-gantt-bar.g-m1-2
                            .plan-gantt-row
                                span.plan-gantt-label "3. Przedsprzedaż na Wspieram.to ({zrodlo_crowdfunding_tys} tys. zł) + 2 aniołów ({zrodlo_inwestorzy_tys} tys.)"
                                .plan-gantt-track
                                    .plan-gantt-bar.g-m2-4
                            .plan-gantt-row
                                span.plan-gantt-label "4. Wnioski dotacyjne CRPK / MKiDN ({zrodlo_dotacje_tys} tys.) + uczelnie B2B"
                                .plan-gantt-track
                                    .plan-gantt-bar.g-m3-4
                            .plan-gantt-row
                                span.plan-gantt-label "5. Pełne nagrania 4 zweryfikowanych premier ({budzet_premiery_tys} tys. zł)"
                                .plan-gantt-track
                                    .plan-gantt-bar.g-m5-9
                            .plan-gantt-row
                                span.plan-gantt-label "6. Powłoka mobilna iOS / Android + eksporter Steam ({budzet_it_tys} tys.)"
                                .plan-gantt-track
                                    .plan-gantt-bar.g-m5-10
                            .plan-gantt-row
                                span.plan-gantt-label "7. Premiery na Steam, App Store i Google Play (próg BEP: {bep_mies_szt} szt./mies.)"
                                .plan-gantt-track
                                    .plan-gantt-bar.g-m8-12

                    section.pitch-section
                        .pitch-section-header
                            span.pitch-num "OŚ CZASU • ETAPY I I II (MIESIĄCE 1–3: AUDYT, 1-GODZINNE PILOTY I WALIDACJA POPYTU)"
                            h2.pitch-heading "Zamiast zgadywać, co się sprzeda — sprawdzamy popyt darmowym 1-godzinnym Pilotem"

                        .plan-timeline
                            .plan-tl-item
                                .plan-tl-left
                                    span.plan-tl-dot.solid
                                    span.plan-tl-period "TYDZIEŃ 1–3"
                                    span.plan-tl-tag "AUDYT I PILOT 1H"
                                .plan-tl-right
                                    h3.plan-tl-title "Zewnętrzny audyt kodu ({koszt_audyt_cert_zl} zł) oraz nagranie 1-godzinnego Pilota (8 000 zł)"
                                    p.plan-tl-desc "Zlecamy zewnętrzny pentest bezpieczeństwa gotowego silnika Go/WASM oraz nagrywamy 1. rozdział pierwszej superprodukcji (1h audio z rozpoznawalnym aktorem). Ryzykujemy maksymalnie 8 000 zł zamiast 52 500 zł."
                            .plan-tl-item
                                .plan-tl-left
                                    span.plan-tl-dot.hollow
                                    span.plan-tl-period "MIESIĄC 2"
                                    span.plan-tl-tag "WSPIERAM.TO + STEAM"
                                .plan-tl-right
                                    h3.plan-tl-title "Darmowy Pilot w przeglądarce i na YouTube → przedsprzedaż ({zrodlo_crowdfunding_zl} zł)"
                                    p.plan-tl-desc "Słuchacze przechodzą darmowy 1. rozdział na mojra.pl i YouTube, a w kulminacyjnym momencie fabuły trafiają na stronę zbiórki Wspieram.to oraz kartę Steam Wishlist. Pełne nagranie studyjne uruchamiamy dopiero po zebraniu wpłat od fanów."
                            .plan-tl-item
                                .plan-tl-left
                                    span.plan-tl-dot.solid
                                    span.plan-tl-period "MIESIĄC 2–3"
                                    span.plan-tl-tag "30 TWÓRCÓW + UCZELNIE"
                                .plan-tl-right
                                    h3.plan-tl-title "Klucze założycielskie dla 30 twórców oraz pierwsze 4 licencje B2B ({zrodlo_wlasne_zl} zł)"
                                    p.plan-tl-desc "Zapraszamy 30 autorów do lokalnych testów na ich własnych komputerach (100% poszanowania praw autorskich) i prowadzimy warsztaty w szkołach filmowych (Łódź, WSF, Wajda, Katowice, PJATK), pozyskując pierwsze 4 umowy kampusowe po {cena_uczelnia_zl} zł."

                footer.deck-page-footer
                    span.deck-footer-left "MOJRA • PLAN DZIAŁANIA (12 MIESIĘCY)"
                    span.deck-footer-center "INWESTORZY@MOJRA.PL • MOJRA.PL"
                    span.deck-footer-right "STRONA 01 / 02"

            .deck-sheet.sheet-2
                .deck-sheet-body
                    section.pitch-section.first-on-sheet
                        .pitch-section-header
                            span.pitch-num "OŚ CZASU • ETAPY III I IV (MIESIĄCE 3–12: KAPITAŁ, PREMIERY NA STEAM/MOBILE I RENTOWNOŚĆ)"
                            h2.pitch-heading "Wejście na Steam, App Store i Google Play oraz wzrost EBITDA z {ebitda_r1_zl} do {ebitda_r3_zl} zł/rok"

                        .plan-timeline
                            .plan-tl-item
                                .plan-tl-left
                                    span.plan-tl-dot.solid
                                    span.plan-tl-period "MIESIĄC 3–4"
                                    span.plan-tl-tag "2 BILETY + DOTACJA"
                                .plan-tl-right
                                    h3.plan-tl-title "2 bilety inwestorskie po {bilet_inwestora_min_zl} zł ({kwota_inwestycji_zl} zł za {udzial_inwestora_proc}%) + granty ({zrodlo_dotacje_zl} zł)"
                                    p.plan-tl-desc "Mając twardy dowód popytu z Pilota i zbiórki Wspieram.to, zamykamy 2 bilety po {bilet_inwestora_min_zl} zł (po {bilet_inwestora_proc}% udziałów) z inwestorami z branży audio/gier oraz składamy wnioski do CRPK i MKiDN/PARP (atut: gotowy prototyp TRL 7 + tryb bezwzrokowy dla niewidomych)."
                            .plan-tl-item
                                .plan-tl-left
                                    span.plan-tl-dot.hollow
                                    span.plan-tl-period "MIESIĄC 4–8"
                                    span.plan-tl-tag "APP STORE + STEAM"
                                .plan-tl-right
                                    h3.plan-tl-title "Wydanie powłoki iOS / Android ({koszt_mobile_dev_zl} zł) i eksportera gier na Steam"
                                    p.plan-tl-desc "Każda z 4 wyprodukowanych superprodukcji Mojra Originals trafia do sprzedaży jednocześnie jako samodzielna aplikacja w App Store, Google Play i na Steamie (w cenie {cena_originals_zl} zł, z czego {marza_originals_szt_zl} zł zostaje w firmie) oraz do wspólnego czytnika Mojra."
                            .plan-tl-item
                                .plan-tl-left
                                    span.plan-tl-dot.solid
                                    span.plan-tl-period "MIESIĄC 6–12"
                                    span.plan-tl-tag "{bep_dzien_szt} SZT. DZIENNIE"
                                .plan-tl-right
                                    h3.plan-tl-title "Przekroczenie progu rentowności operacyjnej ({bep_mies_szt} sprzedanych egzemplarzy miesięcznie)"
                                    p.plan-tl-desc "Sprzedaż zaledwie {bep_dzien_szt} egzemplarzy dziennie łącznie na wszystkich 4 tytułach ({bep_mies_szt} szt./mies. × {marza_originals_szt_zl} zł) w pełni pokrywa bieżący budżet R&D architektury silnika, działania Business Development i księgowość spółki z czystej marży rynkowej."
                            .plan-tl-item
                                .plan-tl-left
                                    span.plan-tl-dot.hollow
                                    span.plan-tl-period "ROK 2–3"
                                    span.plan-tl-tag "SKALA I ANGIELSKI"
                                .plan-tl-right
                                    h3.plan-tl-title "Tłumaczenie najlepszych tytułów na angielski (Steam Global) i wzrost EBITDA do {ebitda_r3_zl} zł/rok"
                                    p.plan-tl-desc "W latach 2–3 powiększamy katalog własnych hitów do 8–12 tytułów i nagrywamy anglojęzyczne wersje lektorskie na globalnego Steama. Przychód spółki rośnie do {przychod_s_r3_zl} zł w 3. roku, a inwestorzy osiągają {zwrot_inwestora_x} zwrotu."

                    footer.pitch-sources
                        p.pitch-sources-title "ZASADY BEZPIECZEŃSTWA KAPITAŁU I PRAW AUTORSKICH (DOCS/FINANSE.CFG)"
                        .pitch-sources-list
                            p.pitch-source-item "1. Reguła Pilot-First: żadna pełna superprodukcja nie wchodzi do studia przed potwierdzeniem popytu 1-godzinnym Pilotem (8 000 zł) i przedsprzedażą."
                            p.pitch-source-item "2. Wszystkie liczby są wyliczane automatycznie z pliku docs/finanse.cfg (pełna tabela: docs/kalkulator-finansowy.csv)."

                footer.deck-page-footer
                    span.deck-footer-left "MOJRA • PLAN DZIAŁANIA (12 MIESIĘCY)"
                    span.deck-footer-center "INWESTORZY@MOJRA.PL • MOJRA.PL"
                    span.deck-footer-right "STRONA 02 / 02"
