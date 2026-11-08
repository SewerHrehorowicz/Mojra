doctype html
html lang: pl
    head
        meta charset: UTF-8
        meta
            name: viewport
            content: "width=device-width, initial-scale=1.0"
        title "Mojra — Deck Finansowy i Model Pilot-First (Po Stress-Teście)"
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
            a.pitch-cta-top href: mojra-deck-finansowy.pdf
                span.corners
                "POBIERZ PDF"

        main.pitch-container
            .deck-sheet.sheet-1
                .deck-sheet-body
                    section.pitch-hero
                        span.pitch-num "PROPOZYCJA INWESTYCYJNA (PO STRESS-TEŚCIE) • MODEL PILOT-FIRST I DYSTRYBUCJA STEAM / MOBILE"
                        h1.pitch-title "Szczupły budżet {budzet_caly_zl} zł z 4 źródeł: tylko {kwota_inwestycji_zl} zł za {udzial_inwestora_proc}% udziałów i zero zamrażania kapitału w ciemno."
                        p.pitch-lead "Wyeliminowaliśmy trzy największe ryzyka startu: (1) nie zmuszamy ludzi do instalowania nowego czytnika w ciemno — wydajemy nasze tytuły od razu jako gotowe gry/aplikacje na Steam, App Store i Google Play; (2) nie ryzykujemy 50 tys. zł na produkcję nietrafionej książki — najpierw nagrywamy 1-godzinny Pilot za 8 000 zł i dogrywamy całość dopiero po zebraniu przedsprzedaży; (3) bieżący rozwój R&D silnika i sprzedaż B2B są finansowane w 100% z wypracowanej marży (próg rentowności operacyjnej to zaledwie {bep_dzien_szt} sprzedanych sztuk dziennie po {marza_originals_szt_zl} zł marży), a nie z przepalania kapitału inwestora."

                        .pitch-kpi-strip
                            .pitch-kpi-cell
                                span.pitch-kpi-val "{budzet_caly_zl} zł"
                                span.pitch-kpi-lbl "BUDŻET STARTOWY (4 ŹRÓDŁA)"
                            .pitch-kpi-cell
                                span.pitch-kpi-val "{kwota_inwestycji_zl} zł"
                                span.pitch-kpi-lbl "PULA INWESTORSKA (ZA {udzial_inwestora_proc}% • 2 × {bilet_inwestora_min_zl} ZŁ)"
                            .pitch-kpi-cell
                                span.pitch-kpi-val "8 000 zł"
                                span.pitch-kpi-lbl "KOSZT 1H PILOTA PRZED ZBIÓRKĄ"
                            .pitch-kpi-cell
                                span.pitch-kpi-val "{zwrot_inwestora_x}"
                                span.pitch-kpi-lbl "ZWROT DLA INWESTORA W 3 LATA"

                    section.pitch-section
                        .pitch-section-header
                            span.pitch-num "01 / STRESS-TEST RYNKOWY I KOMPLEMENTARNY ZESPÓŁ (ARCHITEKTURA TECH + BIZNES B2B)"
                            h2.pitch-heading "Dlaczego ten model spina się w polskich realiach bez huraoptymizmu"

                        .deck-mech-list
                            .deck-mech-row
                                .deck-mech-left
                                    span.deck-mech-source "1. Ryzyko „Zimnego Czytnika”"
                                    span.deck-mech-tag "ROZWIĄZANIE: STEAM + APP STORE"
                                p.deck-mech-how "Pułapka: nikt nie pobiera nowej, pustej aplikacji czytnika dla 4 książek. Rozwiązanie: nasz silnik posiada eksporter pakujący każdą produkcję Mojra Originals jako samodzielną grę/aplikację na Steam, iOS (App Store) i Android (Google Play) oraz w przeglądarce. Wchodzimy tam, gdzie miliony odbiorców mają już podpięte karty płatnicze."
                                span.deck-mech-amt "Steam + Mobile"

                            .deck-mech-row
                                .deck-mech-left
                                    span.deck-mech-source "2. Ryzyko klapy premier"
                                    span.deck-mech-tag "ROZWIĄZANIE: PILOT 1H ZA 8 TYS. ZŁ"
                                p.deck-mech-how "Pułapka: wydanie 52 500 zł z góry na pełne nagranie książki, która może się nie sprzedać. Rozwiązanie: najpierw nagrywamy tylko 1. rozdział (1h audio z głównym aktorem za 8 000 zł), udostępniamy go za darmo w przeglądarce i na YouTube/Spotify, i otwieramy przedsprzedaż na Wspieram.to. Pełne studio zamawiamy dopiero, gdy fani wpłacą min. 40–50 tys. zł z góry."
                                span.deck-mech-amt "Pilot: 8 000 zł"

                            .deck-mech-row
                                .deck-mech-left
                                    span.deck-mech-source "3. Komplementarny Zespół"
                                    span.deck-mech-tag "ARCHITEKT ({udzial_architekta_proc}%) + BIZNES ({udzial_biznes_proc}%)"
                                p.deck-mech-how "Spółka łączy dwa filary: Główny Architekt Oprogramowania ({udzial_architekta_proc}% udziałów — twórca natywnego silnika Go/WebGL 120fps, eksportera Steam/Mobile i formatu .playbook) oraz Co-Founder Business Development ({udzial_biznes_proc}% udziałów z 3-letnim vestingiem — relacje z uczelniami filmowymi, wydawcami, studiami audio i grantodawcami). Kapitał inwestora ({kwota_inwestycji_zl} zł za {udzial_inwestora_proc}%) idzie w 100% na produkt i dystrybucję."
                                span.deck-mech-amt "Zespół {udzial_zespolu_proc}% / Inw. {udzial_inwestora_proc}%"

                            .deck-mech-row
                                .deck-mech-left
                                    span.deck-mech-source "4. Skąd {budzet_caly_zl} zł na start?"
                                    span.deck-mech-tag "4 NIEZALEŻNE ŹRÓDŁA"
                                p.deck-mech-how "(1) Dotacje CRPK / MKiDN / PARP Dostępność (tryb głosowy dla niewidomych i rozwój R&D): {zrodlo_dotacje_zl} zł (0% udziałów); (2) 2 aniołów biznesu po {bilet_inwestora_min_zl} zł za {bilet_inwestora_proc}% każdy (lub ECF): {zrodlo_inwestorzy_zl} zł ({udzial_inwestora_proc}% udziałów); (3) Przedsprzedaż po darmowych Pilotach na Wspieram.to: {zrodlo_crowdfunding_zl} zł (0%); (4) Pierwsze 4 licencje uczelniane/studyjne B2B: {zrodlo_wlasne_zl} zł (0%)."
                                span.deck-mech-amt "{budzet_caly_zl} zł"

                footer.deck-page-footer
                    span.deck-footer-left "MOJRA • PROPOZYCJA INWESTYCYJNA"
                    span.deck-footer-center "INWESTORZY@MOJRA.PL • MOJRA.PL"
                    span.deck-footer-right "STRONA 01 / 03"

            .deck-sheet.sheet-2
                .deck-sheet-body
                    section.pitch-section.first-on-sheet
                        .pitch-section-header
                            span.pitch-num "02 / KOSZTORYS CO DO ZŁOTÓWKI ({budzet_caly_zl} ZŁ) — PILOT-FIRST, AUDYT KODU I DYSTRYBUCJA"
                            h2.pitch-heading "{budzet_premiery_zl} zł na produkcję 4 premier (etapowanych Pilotami), {budzet_it_zl} zł na audyt i iOS/Android, {budzet_sprzedaz_zl} zł na promocję"
                            p.pitch-text "Ponieważ silnik aplikacji (WebGL 120fps, WASM, eksport PDF PISF) już istnieje i powstał w czystym języku Go (zero zewnętrznych bibliotek), nie przepisujemy kodu i nie zatrudniamy sztabu programistów. Pieniądze technologiczne idą na zewnętrzny audyt bezpieczeństwa (pentest), certyfikaty Apple/Microsoft/Steam i powłokę mobilną."

                        .deck-cost-grid
                            .deck-cost-col
                                p.pitch-chart-title "A. PRODUKCJA 4 PREMIER MOJRA ORIGINALS ({budzet_premiery_zl} ZŁ = {budzet_premiery_proc}%)"
                                .deck-cost-list
                                    .deck-cost-item
                                        span.deck-cost-name "1. Gaże rozpoznawalnych aktorów (najpierw 4 Piloty 1h, potem pełne nagrania po zbiórce)"
                                        span.deck-cost-val "{koszt_aktorzy_zl} zł"
                                    .deck-cost-item
                                        span.deck-cost-name "2. Studio nagraniowe, reżyseria, montaż i mastering dźwięku (4 tytuły)"
                                        span.deck-cost-val "{koszt_studio_dzwiek_zl} zł"
                                    .deck-cost-item
                                        span.deck-cost-name "3. Zaliczki autorskie (wypłacane transzami: 1. rozdział Pilot → całość po walidacji)"
                                        span.deck-cost-val "{koszt_autorzy_zl} zł"
                                    .deck-cost-item
                                        span.deck-cost-name "4. Ilustracje scen, oprawa muzyczna i efekty dźwiękowe SFX"
                                        span.deck-cost-val "{koszt_ilustracje_sfx_zl} zł"
                                    .deck-cost-item.total-row
                                        span.deck-cost-name "RAZEM KOSZYK A (KAPITAŁ OBROTOWY NA 4 PRODUKCJE)"
                                        span.deck-cost-val "{budzet_premiery_zl} zł"

                            .deck-cost-col
                                p.pitch-chart-title "B. AUDYT KODU I MOBILE ({budzet_it_tys} TYS.) + C. PROMOCJA ({budzet_sprzedaz_tys} TYS.)"
                                .deck-cost-list
                                    .deck-cost-item
                                        span.deck-cost-name "5. Zewnętrzny pentest bezpieczeństwa + certyfikaty Apple, Microsoft i opłata Steamworks"
                                        span.deck-cost-val "{koszt_audyt_cert_zl} zł"
                                    .deck-cost-item
                                        span.deck-cost-name "6. Powłoka mobilna iOS (App Store) + Android (Google Play) + CarPlay wokół silnika"
                                        span.deck-cost-val "{koszt_mobile_dev_zl} zł"
                                    .deck-cost-item
                                        span.deck-cost-name "7. Płatności Stripe / Apple Pay / Google Pay ({koszt_sklep_infra_zl} zł) oraz wzory umów ({koszt_prawo_umowy_zl} zł)"
                                        span.deck-cost-val "18 000 zł"
                                    .deck-cost-item
                                        span.deck-cost-name "8. Wideo i nagłośnienie 4 darmowych Pilotów na Wspieram.to ({koszt_kampania_cf_zl} zł) + promocja premier na Steam/Mobile ({koszt_promocja_premier_zl} zł)"
                                        span.deck-cost-val "42 000 zł"
                                    .deck-cost-item
                                        span.deck-cost-name "9. Warsztaty w szkołach filmowych ({koszt_warsztaty_b2b_zl} zł) oraz targi książki/gier i Steam Next Fest ({koszt_targi_konkursy_zl} zł)"
                                        span.deck-cost-val "18 000 zł"
                                    .deck-cost-item.total-row
                                        span.deck-cost-name "RAZEM KOSZYK B + C (AUDYT, MOBILE I DYSTRYBUCJA)"
                                        span.deck-cost-val "120 000 zł"

                    section.pitch-section
                        .pitch-section-header
                            span.pitch-num "EKONOMIA JEDNOSTKOWA I PRÓG RENTOWNOŚCI OPERACYJNEJ SPÓŁKI (BREAK-EVEN POINT)"
                            h2.pitch-heading "Wystarczy sprzedać {bep_mies_szt} egzemplarzy miesięcznie ({bep_dzien_szt} sztuk dziennie), aby pokryć pełne koszty R&D silnika i operacji B2B"

                        .deck-mech-list
                            .deck-mech-row
                                .deck-mech-left
                                    span.deck-mech-source "Matematyka 1 egzemplarza"
                                    span.deck-mech-tag "{marza_originals_szt_zl} ZŁ CZYSTEJ MARŻY / SZT."
                                p.deck-mech-how "Jako wydawca naszych własnych tytułów Mojra Originals (cena detaliczna {cena_originals_zl} zł na Wspieram.to, mojra.pl, Steam i mobile) po odjęciu prowizji płatniczych i tantiem zatrzymujemy średnio {marza_originals_szt_zl} zł na każdym egzemplarzu. Aby w pełni sfinansować stały rozwój architektury R&D, działania Business Development i księgowość spółki ({bep_mies_opex_zl} zł/mies.), wystarcza sprzedaż {bep_mies_szt} sztuk miesięcznie (~196 sztuk na każdy z 4 tytułów)."
                                span.deck-mech-amt "{bep_dzien_szt} szt. / dzień"

                            .deck-mech-row
                                .deck-mech-left
                                    span.deck-mech-source "Licencje B2B i katalog 15%"
                                    span.deck-mech-tag "DŹWIGNIA MARŻY EBITDA"
                                p.deck-mech-how "Przychody z licencji uczelnianych ({cena_uczelnia_zl} zł/rok), subskrypcji autorskich ({cena_indie_zl} / {cena_pro_zl} zł/rok) oraz {prowizja_katalog_proc}% prowizji ({prowizja_szt_zl} zł/szt.) z książek zewnętrznych autorów charakteryzują się ~94% marżą kontrybucyjną (architektura Local-First = 0 zł za chmurę edycyjną), budując bezpośrednio zysk netto spółki do podziału dywidendowego ({udzial_zespolu_proc}% Zespół Założycielski / {udzial_inwestora_proc}% Inwestorzy)."
                                span.deck-mech-amt "EBITDA 30% → 66%"

                footer.deck-page-footer
                    span.deck-footer-left "MOJRA • PROPOZYCJA INWESTYCYJNA"
                    span.deck-footer-center "INWESTORZY@MOJRA.PL • MOJRA.PL"
                    span.deck-footer-right "STRONA 02 / 03"

            .deck-sheet.sheet-3
                .deck-sheet-body
                    section.pitch-section.first-on-sheet
                        .pitch-section-header
                            span.pitch-num "03 / RACHUNEK ZYSKÓW I STRAT SPÓŁKI (P&L ROK 1–3) — KONSERWATYWNE WOLUMENY OD DOŁU"
                            h2.pitch-heading "Wzrost przychodów z {przychod_s_r1_zl} zł w 1. roku do {przychod_s_r3_zl} zł w 3. roku • EBITDA 3-letnia: {ebitda_3lata} zł"
                            p.pitch-text "W poniższym modelu P&L przyjęliśmy ostrożne, urealnione wolumeny: tylko 6 uczelni i 60 licencji Indie w 1. roku oraz średni nakład 2 800 sprzedanych sztuk na własną premierę rocznie (łącznie na Wspieram.to, Steam, iOS i Android). Koszty operacyjne (OPEX) obejmują pełny kontrakt B2B Głównego Architekta (R&D silnika i eksporterów), działania Business Development (prowizje B2B i relacje) oraz obsługę prawno-księgową sp. z o.o."

                        .deck-math-table
                            .deck-math-hdr
                                span "POZYCJA RACHUNKU ZYSKÓW I STRAT (P&L)"
                                span "ROK 1 (4 PREMIERY ORIGINALS)"
                                span "ROK 2 (8 ORIGINALS + STEAM/MOBILE)"
                                span "ROK 3 (12 ORIGINALS PL/EN + KATALOG)"
                            .deck-math-row
                                span.deck-math-label "1. Własne produkcje Mojra Originals ({marza_originals_szt_zl} zł marży/szt.)"
                                .deck-math-cell
                                    span.deck-math-eq "{s_r1_orig_tyt} tyt. × {s_r1_orig_szt} szt. ({s_r1_orig_suma_szt} egz.)"
                                    span.deck-math-val "{przychod_orig_r1_zl} zł"
                                .deck-math-cell
                                    span.deck-math-eq "{s_r2_orig_tyt} tyt. × {s_r2_orig_szt} szt. ({s_r2_orig_suma_szt} egz.)"
                                    span.deck-math-val "{przychod_orig_r2_zl} zł"
                                .deck-math-cell
                                    span.deck-math-eq "{s_r3_orig_tyt} tyt. × {s_r3_orig_szt} szt. ({s_r3_orig_suma_szt} egz.)"
                                    span.deck-math-val "{przychod_orig_r3_zl} zł"
                            .deck-math-row
                                span.deck-math-label "2. Katalog zewnętrzny ({prowizja_szt_zl} zł prowizji/szt.)"
                                .deck-math-cell
                                    span.deck-math-eq "{s_r1_kat_tyt} tyt. ({s_r1_kat_suma_szt} egz. × {prowizja_szt_zl} zł)"
                                    span.deck-math-val "{przychod_kat_r1_zl} zł"
                                .deck-math-cell
                                    span.deck-math-eq "{s_r2_kat_tyt} tyt. ({s_r2_kat_suma_szt} egz. × {prowizja_szt_zl} zł)"
                                    span.deck-math-val "{przychod_kat_r2_zl} zł"
                                .deck-math-cell
                                    span.deck-math-eq "{s_r3_kat_tyt} tyt. ({s_r3_kat_suma_szt} egz. × {prowizja_szt_zl} zł)"
                                    span.deck-math-val "{przychod_kat_r3_zl} zł"
                            .deck-math-row
                                span.deck-math-label "3. Licencje (Uczelnie {cena_uczelnia_zl} zł + Indie/Pro)"
                                .deck-math-cell
                                    span.deck-math-eq "{s_r1_uczelnie} uczelni + {s_r1_indie} Indie + {s_r1_pro} Pro"
                                    span.deck-math-val "{przychod_lic_r1_zl} zł"
                                .deck-math-cell
                                    span.deck-math-eq "{s_r2_uczelnie} uczelni + {s_r2_indie} Indie + {s_r2_pro} Pro"
                                    span.deck-math-val "{przychod_lic_r2_zl} zł"
                                .deck-math-cell
                                    span.deck-math-eq "{s_r3_uczelnie} uczelni + {s_r3_indie} Indie + {s_r3_pro} Pro"
                                    span.deck-math-val "{przychod_lic_r3_zl} zł"
                            .deck-math-row.summary
                                span.deck-math-label "PRZYCHÓD SPÓŁKI OGÓŁEM"
                                span "{przychod_s_r1_zl} zł"
                                span "{przychod_s_r2_zl} zł"
                                span "{przychod_s_r3_zl} zł"
                            .deck-math-row
                                span.deck-math-label "4. OPEX: Rozwój R&D silnika i mobile (Główny Architekt B2B)"
                                .deck-math-cell
                                    span.deck-math-eq "Architektura Go/WebGL/Mobile ({rd_arch_r1_mies_zl} zł/m)"
                                    span.deck-math-val "−{rd_arch_r1_rok_zl} zł"
                                .deck-math-cell
                                    span.deck-math-eq "Silnik, eksportery i DRM ({rd_arch_r2_mies_zl} zł/m)"
                                    span.deck-math-val "−{rd_arch_r2_rok_zl} zł"
                                .deck-math-cell
                                    span.deck-math-eq "Skalowanie multi-platform ({rd_arch_r3_mies_zl} zł/m)"
                                    span.deck-math-val "−{rd_arch_r3_rok_zl} zł"
                            .deck-math-row
                                span.deck-math-label "5. OPEX: Sprzedaż B2B (BD + 15% prowizji) i księgowość"
                                .deck-math-cell
                                    span.deck-math-eq "BD ({bd_ops_r1_rok_zl} zł) + biuro (24 000 zł)"
                                    span.deck-math-val "−{bd_admin_r1_rok_zl} zł"
                                .deck-math-cell
                                    span.deck-math-eq "BD ({bd_ops_r2_rok_zl} zł) + biuro (24 000 zł)"
                                    span.deck-math-val "−{bd_admin_r2_rok_zl} zł"
                                .deck-math-cell
                                    span.deck-math-eq "BD ({bd_ops_r3_rok_zl} zł) + biuro (24 000 zł)"
                                    span.deck-math-val "−{bd_admin_r3_rok_zl} zł"
                            .deck-math-row.summary
                                span.deck-math-label "ZYSK OPERACYJNY EBITDA / ZYSK NETTO PO CIT"
                                span "{ebitda_r1_zl} zł (netto: {nadwyzka_r1_zl} zł)"
                                span "{ebitda_r2_zl} zł (netto: {nadwyzka_r2_zl} zł)"
                                span "{ebitda_r3_zl} zł (netto: {nadwyzka_r3_zl} zł)"

                    section.pitch-section
                        .pitch-section-header
                            span.pitch-num "04 / RENTOWNOŚĆ SPÓŁKI ORAZ ZWROT DLA INWESTORÓW ({udzial_inwestora_proc}% ZA {kwota_inwestycji_zl} ZŁ)"
                            h2.pitch-heading "Zysk EBITDA spółki: {ebitda_r1_zl} → {ebitda_r2_zl} → {ebitda_r3_zl} zł/rok • Inwestor: {zwrot_inwestora_x} zwrotu"

                        .pitch-charts-grid
                            .pitch-chart-box
                                p.pitch-chart-title "ZYSK OPERACYJNY SPÓŁKI (EBITDA PO KOSZTACH R&D I SPRZEDAŻY B2B)"
                                .pitch-bars
                                    .pitch-bar-row
                                        span.pitch-bar-label-wide "Rok 1 — EBITDA spółki (marża 29,6%)"
                                        .pitch-bar-track
                                            .pitch-bar-fill.hi.w-16
                                        span.pitch-bar-value "{ebitda_r1_zl} zł"
                                    .pitch-bar-row
                                        span.pitch-bar-label-wide "Rok 2 — EBITDA spółki (marża 52,7%)"
                                        .pitch-bar-track
                                            .pitch-bar-fill.hi.w-45
                                        span.pitch-bar-value "{ebitda_r2_zl} zł"
                                    .pitch-bar-row
                                        span.pitch-bar-label-wide "Rok 3 — EBITDA spółki (marża 66,1%)"
                                        .pitch-bar-track
                                            .pitch-bar-fill.hi.w-100
                                        span.pitch-bar-value "{ebitda_r3_zl} zł"

                            .pitch-chart-box
                                p.pitch-chart-title "ZWROT DLA PULI INWESTORSKIEJ ({kwota_inwestycji_zl} ZŁ ZA {udzial_inwestora_proc}% UDZIAŁÓW)"
                                .pitch-bars
                                    .pitch-bar-row
                                        span.pitch-bar-label-wide "Wkład inwestorów (2 bilety po {bilet_inwestora_min_zl} zł = 5%)"
                                        .pitch-bar-track
                                            .pitch-bar-fill.w-16
                                        span.pitch-bar-value "{kwota_inwestycji_zl} zł"
                                    .pitch-bar-row
                                        span.pitch-bar-label-wide "Wypłacona dywidenda gotówkowa (10% z 3 lat)"
                                        .pitch-bar-track
                                            .pitch-bar-fill.hi.w-30
                                        span.pitch-bar-value "{zysk_inwestora_3lata_zl} zł"
                                    .pitch-bar-row
                                        span.pitch-bar-label-wide "Wartość 10% udziałów w 3. roku ({zwrot_inwestora_x})"
                                        .pitch-bar-track
                                            .pitch-bar-fill.hi.w-100
                                        span.pitch-bar-value "{wartosc_udzialow_r3_tys}"

                    footer.pitch-sources
                        p.pitch-sources-title "STRUKTURA UDZIAŁOWA (CAP TABLE) I MODEL FINANSOWY (DOCS/FINANSE.CFG)"
                        .pitch-sources-list
                            p.pitch-source-item "1. Struktura udziałowa: {udzial_architekta_proc}% Główny Architekt / Założyciel Tech, {udzial_biznes_proc}% Co-Founder Business Development (3-letni vesting z 12M cliffem), {udzial_inwestora_proc}% Inwestorzy Seed. Łączny zysk netto spółki po CIT w 3 lata wynosi {nadwyzka_3lata} zł, a szacowana wycena spółki w 3. roku (4,5×–6,0× przychodu): {wycena_spolki_r3_mln}."

                footer.deck-page-footer
                    span.deck-footer-left "MOJRA • PROPOZYCJA INWESTYCYJNA"
                    span.deck-footer-center "INWESTORZY@MOJRA.PL • MOJRA.PL"
                    span.deck-footer-right "STRONA 03 / 03"
