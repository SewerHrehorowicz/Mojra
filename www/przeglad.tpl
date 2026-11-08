doctype html
html lang: pl
    head
        meta charset: UTF-8
        meta
            name: viewport
            content: "width=device-width, initial-scale=1.0"
        title "Mojra — Przegląd Założyciela: Podział Tortu, Symulacja 36M i Realność (Poufne)"
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
            a.pitch-back href: deck.html
                "← DECK INWESTORSKI"
            .deck-brand-hero
                .brand-lockup
                    svg.brand-logo-svg src: /icons/mojra-logo.svg
                p.brand-subtitle
                    "DOKUMENT WEWNĘTRZNY ZAŁOŻYCIELA"
                    span.plus "•"
                    "SYMULACJA 36M & PODZIAŁ TORTU"
                .hero-divider aria-hidden: "true"
            a.pitch-cta-top href: mojra-przeglad-zalozyciela.pdf
                span.corners
                "POBIERZ PDF"

        main.pitch-container
            .deck-sheet.sheet-1
                .deck-sheet-body
                    section.pitch-hero
                        span.pitch-num "DOKUMENT POUFNY DLA ZAŁOŻYCIELA • TE SAME DANE CO W DECKU INWESTORSKIM (DOCS/FINANSE.CFG)"
                        h1.pitch-title "Jak wyjmujesz {arch_suma_3lata_zl} zł w 3 lata (B2B R&D + {udzial_architekta_proc}% dywidendy), jak rozliczasz Przyjaciela ({udzial_biznes_proc}%) i Inwestorów ({udzial_inwestora_proc}%)."
                        p.pitch-lead "W Decku Inwestorskim nie ma ani słowa o „pensji założyciela na rękę”. Zamiast tego Twoje wynagrodzenie jest legalnie i rynkowo zabudżetowane w kosztach operacyjnych spółki jako Kontrakt B2B Głównego Architekta Oprogramowania (R&D silnika, powłoki iOS/Android i eksportera Steam — stawka {stawka_architekta_godz_zl} zł/h netto, opodatkowana 5% IP Box), a udział Twojego przyjaciela od biznesu ({udzial_biznes_proc}% z 3-letnim vestingiem) łączy niski retainer z 15% prowizji od przyprowadzonych umów B2B."

                        .pitch-kpi-strip
                            .pitch-kpi-cell
                                span.pitch-kpi-val "{arch_suma_3lata_zl} zł"
                                span.pitch-kpi-lbl "DLA CIEBIE W 3 LATA (R&D + {udzial_architekta_proc}% ZYSKU)"
                            .pitch-kpi-cell
                                span.pitch-kpi-val "{bd_suma_3lata_zl} zł"
                                span.pitch-kpi-lbl "DLA PRZYJACIELA W 3 LATA (BD + {udzial_biznes_proc}% ZYSKU)"
                            .pitch-kpi-cell
                                span.pitch-kpi-val "{zysk_inwestora_3lata_zl} zł"
                                span.pitch-kpi-lbl "DYWIDENDA INWESTORÓW ({udzial_inwestora_proc}% ZYSKU)"
                            .pitch-kpi-cell
                                span.pitch-kpi-val "{arch_wycena_r3_mln}"
                                span.pitch-kpi-lbl "WARTOŚĆ TWOICH {udzial_architekta_proc}% UDZIAŁÓW W R3"

                    section.pitch-section
                        .pitch-section-header
                            span.pitch-num "01 / KONSTRUKCJA KONTRAKTÓW: TY (ARCHITEKT R&D) ORAZ PRZYJACIEL (CO-FOUNDER BIZNES / BD)"
                            h2.pitch-heading "Dlaczego ujęcie Twojej pracy jako R&D B2B i hybrydowy model dla Przyjaciela są kuloodporne"

                        .deck-mech-list
                            .deck-mech-row
                                .deck-mech-left
                                    span.deck-mech-source "1. Ty: Główny Architekt"
                                    span.deck-mech-tag "{udzial_architekta_proc}% UDZIAŁÓW + B2B IP BOX 5%"
                                p.deck-mech-how "Fakturujesz spółkę z JDG (B2B, PKWiU 62.01.1) za rozwój architektury silnika Go/WebGL, powłoki iOS/Android, integracji Steamworks i kryptografii .playbook. Stawka {stawka_architekta_godz_zl} zł/h netto to dla inwestora okazja rynkowa (Principal Go/Systems Architect kosztuje na rynku 220–280 zł/h). W R1 bierzesz z faktury R&D {rd_arch_r1_rok_zl} zł (12 tys./m w M1–M4, 24 tys./m od M5), w R2: {rd_arch_r2_rok_zl} zł (25 tys./m), w R3: {rd_arch_r3_rok_zl} zł (30 tys./m) + {udzial_architekta_proc}% dywidendy."
                                span.deck-mech-amt "{arch_razem_r1_mies_zl} → {arch_razem_r3_mies_zl} zł/m"

                            .deck-mech-row
                                .deck-mech-left
                                    span.deck-mech-source "2. Przyjaciel: Biznes & BD"
                                    span.deck-mech-tag "{udzial_biznes_proc}% UDZIAŁÓW (VESTING 36M)"
                                p.deck-mech-how "Odpowiada za spotkania, szkoły filmowe/uczelnie, wydawców, studia lektorskie, wnioski dotacyjne i inwestorów. Otrzymuje: (a) {udzial_biznes_proc}% udziałów objętych 3-letnim Reverse Vestingiem z 12-miesięcznym Cliffem (jeśli odpuści przed końcem 1. roku, udziały wracają do Ciebie); (b) {prowizja_bd_b2b_proc}% prowizji (Success Fee) od każdej sprzedanej licencji B2B/Studio; (c) rosnący retainer BD ({bd_ret_r1_mies_zl} → {bd_ret_r2_mies_zl} → {bd_ret_r3_mies_zl} zł/m) + {udzial_biznes_proc}% dywidendy."
                                span.deck-mech-amt "{bd_razem_r1_mies_zl} → {bd_razem_r3_mies_zl} zł/m"

                    section.pitch-section
                        .pitch-section-header
                            span.pitch-num "02 / PODZIAŁ TORTU W LATACH 1–3 — KTO ILE DOKŁADNIE DOSTAJE Z WYPRACOWANYCH PIENIĘDZY"
                            h2.pitch-heading "Pełne zestawienie roczne i 3-letnie (spójne co do złotówki z Deckiem Inwestorskim)"

                        .deck-math-table
                            .deck-math-hdr
                                span "UCZESTNIK TORTU / STRUMIEŃ WYPŁAT"
                                span "ROK 1 (PRZYCHÓD {przychod_s_r1_zl} ZŁ)"
                                span "ROK 2 (PRZYCHÓD {przychod_s_r2_zl} ZŁ)"
                                span "ROK 3 (PRZYCHÓD {przychod_s_r3_zl} ZŁ)"
                            .deck-math-row
                                span.deck-math-label "1. TY — Faktura B2B R&D (Architekt Oprogramowania)"
                                .deck-math-cell
                                    span.deck-math-eq "M1–M4: 12 tys./m, M5–M12: 24 tys./m"
                                    span.deck-math-val "{rd_arch_r1_rok_zl} zł"
                                .deck-math-cell
                                    span.deck-math-eq "12 mies. × {rd_arch_r2_mies_zl} zł/m"
                                    span.deck-math-val "{rd_arch_r2_rok_zl} zł"
                                .deck-math-cell
                                    span.deck-math-eq "12 mies. × {rd_arch_r3_mies_zl} zł/m (3 lata: {rd_arch_3lata_zl} zł)"
                                    span.deck-math-val "{rd_arch_r3_rok_zl} zł"
                            .deck-math-row
                                span.deck-math-label "2. TY — Dywidenda z zysku netto ({udzial_architekta_proc}% udziałów)"
                                .deck-math-cell
                                    span.deck-math-eq "{udzial_architekta_proc}% z {nadwyzka_r1_zl} zł zysku netto"
                                    span.deck-math-val "{arch_dyw_r1_zl} zł"
                                .deck-math-cell
                                    span.deck-math-eq "{udzial_architekta_proc}% z {nadwyzka_r2_zl} zł zysku netto"
                                    span.deck-math-val "{arch_dyw_r2_zl} zł"
                                .deck-math-cell
                                    span.deck-math-eq "{udzial_architekta_proc}% z {nadwyzka_r3_zl} zł (3 lata: {arch_dyw_3lata_zl} zł)"
                                    span.deck-math-val "{arch_dyw_r3_zl} zł"
                            .deck-math-row.summary
                                span.deck-math-label "RAZEM DLA CIEBIE (B2B R&D + {udzial_architekta_proc}% DYWIDENDY)"
                                span "{arch_razem_r1_rok_zl} zł ({arch_razem_r1_mies_zl} zł/m)"
                                span "{arch_razem_r2_rok_zl} zł ({arch_razem_r2_mies_zl} zł/m)"
                                span "{arch_razem_r3_rok_zl} zł ({arch_razem_r3_mies_zl} zł/m)"
                            .deck-math-row
                                span.deck-math-label "3. PRZYJACIEL (BD) — Faktura BD + {prowizja_bd_b2b_proc}% prowizji + {udzial_biznes_proc}% dyw."
                                .deck-math-cell
                                    span.deck-math-eq "BD {bd_ops_r1_rok_zl} zł + dyw. {bd_dyw_r1_zl} zł"
                                    span.deck-math-val "{bd_razem_r1_rok_zl} zł ({bd_razem_r1_mies_zl} zł/m)"
                                .deck-math-cell
                                    span.deck-math-eq "BD {bd_ops_r2_rok_zl} zł + dyw. {bd_dyw_r2_zl} zł"
                                    span.deck-math-val "{bd_razem_r2_rok_zl} zł ({bd_razem_r2_mies_zl} zł/m)"
                                .deck-math-cell
                                    span.deck-math-eq "BD {bd_ops_r3_rok_zl} zł + dyw. {bd_dyw_r3_zl} zł (3L: {bd_suma_3lata_zl} zł)"
                                    span.deck-math-val "{bd_razem_r3_rok_zl} zł ({bd_razem_r3_mies_zl} zł/m)"
                            .deck-math-row
                                span.deck-math-label "4. INWESTORZY ({udzial_inwestora_proc}% za {kwota_inwestycji_zl} zł) — Dywidenda gotówkowa"
                                .deck-math-cell
                                    span.deck-math-eq "10% zysku netto R1"
                                    span.deck-math-val "{inv_dyw_r1_zl} zł"
                                .deck-math-cell
                                    span.deck-math-eq "10% zysku netto R2"
                                    span.deck-math-val "{inv_dyw_r2_zl} zł"
                                .deck-math-cell
                                    span.deck-math-eq "10% zysku netto R3 (3 lata: {zysk_inwestora_3lata_zl} zł)"
                                    span.deck-math-val "{inv_dyw_r3_zl} zł"

                footer.deck-page-footer
                    span.deck-footer-left "MOJRA • PRZEGLĄD ZAŁOŻYCIELA (POUFNE)"
                    span.deck-footer-center "SYMULACJA 36M & PODZIAŁ TORTU"
                    span.deck-footer-right "STRONA 01 / 03"

            .deck-sheet.sheet-2
                .deck-sheet-body
                    section.pitch-section.first-on-sheet
                        .pitch-section-header
                            span.pitch-num "03 / SYMULACJA MIESIĄC PO MIESIĄCU — ROK 1 (M01–M12) ORAZ ROK 2 (M13–M24)"
                            h2.pitch-heading "Rozbicie źródeł przychodu i wypłat dla Ciebie, Przyjaciela i Inwestorów w pierwszych 24 miesiącach"

                        p.pitch-chart-title "ROK 1 (MIESIĄCE M01–M12) — OD PILOTA #1 DO 4 PREMIER NA STEAM / MOBILE (WARTOŚCI W ZŁ)"
                        .sim-month-table
                            .sim-month-hdr
                                span "MIES."
                                span "KAMIEŃ MILOWY / ZDARZENIE"
                                span.sim-m-num "ORIGINALS"
                                span.sim-m-num "LICENCJE B2B"
                                span.sim-m-num "KATALOG"
                                span.sim-m-num "PRZYCHÓD"
                                span.sim-m-num "TY (R&D+78%)"
                                span.sim-m-num "PRZYJACIEL (BD+12%)"
                                span.sim-m-num "INWEST. (10%)"
                            for: m in months_y1
                                .sim-month-row
                                    span.sim-m-code "{m.code}"
                                    span "{m.milestone}"
                                    span.sim-m-num "{m.rev_orig} ({m.orig_szt})"
                                    span.sim-m-num "{m.rev_lic}"
                                    span.sim-m-num "{m.rev_cat}"
                                    span.sim-m-hi "{m.rev_total}"
                                    span.sim-m-hi "{m.arch_total}"
                                    span.sim-m-num "{m.bd_total}"
                                    span.sim-m-num "{m.inv_div}"
                            .sim-month-row.summary
                                span.sim-m-code "ROK 1"
                                span "SUMA ROKU 1 (M01–M12 • EBITDA: {ebitda_r1_zl} zł)"
                                span.sim-m-num "{przychod_orig_r1_zl} zł"
                                span.sim-m-num "{przychod_lic_r1_zl} zł"
                                span.sim-m-num "{przychod_kat_r1_zl} zł"
                                span.sim-m-hi "{przychod_s_r1_zl} zł"
                                span.sim-m-hi "{arch_razem_r1_rok_zl} zł"
                                span.sim-m-num "{bd_razem_r1_rok_zl} zł"
                                span.sim-m-num "{inv_dyw_r1_zl} zł"

                        p.pitch-chart-title "ROK 2 (MIESIĄCE M13–M24) — 8 TYTUŁÓW ORIGINALS W SPRZEDAŻY NA STEAM / MOBILE + 12 UCZELNI (WARTOŚCI W ZŁ)"
                        .sim-month-table
                            .sim-month-hdr
                                span "MIES."
                                span "KAMIEŃ MILOWY / ZDARZENIE"
                                span.sim-m-num "ORIGINALS"
                                span.sim-m-num "LICENCJE B2B"
                                span.sim-m-num "KATALOG"
                                span.sim-m-num "PRZYCHÓD"
                                span.sim-m-num "TY (R&D+78%)"
                                span.sim-m-num "PRZYJACIEL (BD+12%)"
                                span.sim-m-num "INWEST. (10%)"
                            for: m in months_y2
                                .sim-month-row
                                    span.sim-m-code "{m.code}"
                                    span "{m.milestone}"
                                    span.sim-m-num "{m.rev_orig} ({m.orig_szt})"
                                    span.sim-m-num "{m.rev_lic}"
                                    span.sim-m-num "{m.rev_cat}"
                                    span.sim-m-hi "{m.rev_total}"
                                    span.sim-m-hi "{m.arch_total}"
                                    span.sim-m-num "{m.bd_total}"
                                    span.sim-m-num "{m.inv_div}"
                            .sim-month-row.summary
                                span.sim-m-code "ROK 2"
                                span "SUMA ROKU 2 (M13–M24 • EBITDA: {ebitda_r2_zl} zł)"
                                span.sim-m-num "{przychod_orig_r2_zl} zł"
                                span.sim-m-num "{przychod_lic_r2_zl} zł"
                                span.sim-m-num "{przychod_kat_r2_zl} zł"
                                span.sim-m-hi "{przychod_s_r2_zl} zł"
                                span.sim-m-hi "{arch_razem_r2_rok_zl} zł"
                                span.sim-m-num "{bd_razem_r2_rok_zl} zł"
                                span.sim-m-num "{inv_dyw_r2_zl} zł"

                footer.deck-page-footer
                    span.deck-footer-left "MOJRA • PRZEGLĄD ZAŁOŻYCIELA (POUFNE)"
                    span.deck-footer-center "PEŁNY ARKUSZ CSV: DOCS/SYMULACJA-36-MIESIECY.CSV"
                    span.deck-footer-right "STRONA 02 / 03"

            .deck-sheet.sheet-3
                .deck-sheet-body
                    section.pitch-section.first-on-sheet
                        .pitch-section-header
                            span.pitch-num "04 / SYMULACJA ROKU 3 (M25–M36) ORAZ BRUTALNIE SZCZERA OCENA: CZY MILIONY W 3. ROKU SĄ REALNE?"
                            h2.pitch-heading "Rok 3 ({przychod_s_r3_zl} zł przychodu, {arch_razem_r3_rok_zl} zł dla Ciebie) + weryfikacja 3 scenariuszy rynkowych"

                        p.pitch-chart-title "ROK 3 (MIESIĄCE M25–M36) — 12 TYTUŁÓW ORIGINALS (PL + STEAM GLOBAL EN) + 22 UCZELNIE (WARTOŚCI W ZŁ)"
                        .sim-month-table
                            .sim-month-hdr
                                span "MIES."
                                span "KAMIEŃ MILOWY / ZDARZENIE"
                                span.sim-m-num "ORIGINALS"
                                span.sim-m-num "LICENCJE B2B"
                                span.sim-m-num "KATALOG"
                                span.sim-m-num "PRZYCHÓD"
                                span.sim-m-num "TY (R&D+78%)"
                                span.sim-m-num "PRZYJACIEL (BD+12%)"
                                span.sim-m-num "INWEST. (10%)"
                            for: m in months_y3
                                .sim-month-row
                                    span.sim-m-code "{m.code}"
                                    span "{m.milestone}"
                                    span.sim-m-num "{m.rev_orig} ({m.orig_szt})"
                                    span.sim-m-num "{m.rev_lic}"
                                    span.sim-m-num "{m.rev_cat}"
                                    span.sim-m-hi "{m.rev_total}"
                                    span.sim-m-hi "{m.arch_total}"
                                    span.sim-m-num "{m.bd_total}"
                                    span.sim-m-num "{m.inv_div}"
                            .sim-month-row.summary
                                span.sim-m-code "ROK 3"
                                span "SUMA ROKU 3 (M25–M36 • EBITDA: {ebitda_r3_zl} zł)"
                                span.sim-m-num "{przychod_orig_r3_zl} zł"
                                span.sim-m-num "{przychod_lic_r3_zl} zł"
                                span.sim-m-num "{przychod_kat_r3_zl} zł"
                                span.sim-m-hi "{przychod_s_r3_zl} zł"
                                span.sim-m-hi "{arch_razem_r3_rok_zl} zł"
                                span.sim-m-num "{bd_razem_r3_rok_zl} zł"
                                span.sim-m-num "{inv_dyw_r3_zl} zł"

                    section.pitch-section
                        .pitch-section-header
                            span.pitch-num "05 / CHŁODNY RACHUNEK SUMIENIA: NA ILE REALNE SĄ TE MILIONY W 3. ROKU?"
                            h2.pitch-heading "Czy musisz stać się światowym standardem, żeby zarobić 1,12 mln zł w 3. roku? Nie — i oto dlaczego"

                        .deck-mech-list
                            .deck-mech-row
                                .deck-mech-left
                                    span.deck-mech-source "Scenariusz A: Pesymistyczny"
                                    span.deck-mech-tag "35% PLANU • MAŁA NISZA PL"
                                p.deck-mech-how "Załóżmy, że format .playbook NIE staje się standardem, uczelnie wchodzą opornie (tylko 6 szkół), a nasze gry audio sprzedają tylko po ~1 000 szt. rocznie w Polsce. Przychód spółki w R3 wynosi wtedy ~420 tys. zł/rok. Efekt: nie ma milionowej dywidendy, ale spółka bez długów pokrywa Twoją fakturę Architekta R&D (~22–25 tys. zł/mies.) i prowizje Przyjaciela. Masz bezpieczną, zyskowną firmę produktową."
                                span.deck-mech-amt "Ty: ~24 tys. zł/m"

                            .deck-mech-row
                                .deck-mech-left
                                    span.deck-mech-source "Scenariusz B: Bazowy z Decku"
                                    span.deck-mech-tag "100% PLANU • {przychod_s_r3_zl} ZŁ W R3"
                                p.deck-mech-how "Skąd bierze się {przychod_s_r3_zl} zł w 3. roku? Aż {przychod_orig_r3_zl} zł (66% przychodu) to sprzedaż 12 własnych tytułów Mojra Originals w nakładzie zaledwie 3 000 sztuk rocznie na tytuł (czyli 250 sztuk miesięcznie łącznie na Steam PL/EN, iOS, Android i Wspieram.to!). Na Steamie 3 000 sprzedanych kopii gry narracyjnej za $10.99 (45 zł) to wynik zwykłego średniaka indie (~90 recenzji na całym świecie), a nie globalnego hitu. Dlatego ten scenariusz jest w pełni osiągalny bez bycia „nowym PDF-em”."
                                span.deck-mech-amt "Ty: {arch_razem_r3_mies_zl} zł/m"

                            .deck-mech-row
                                .deck-mech-left
                                    span.deck-mech-source "Scenariusz C: Standard Rynku"
                                    span.deck-mech-tag "PRZEŁOM • 4,8–7,5 MLN ZŁ / ROK"
                                p.deck-mech-how "A co jeśli Wasze narzędzie (WebGL 120fps + Git fabularny + PDF PISF + Auto-Choice w aucie) rzeczywiście stanie się standardem zamiast przestarzałego Celtxa/Twine, a choć 1 z 12 tytułów wypali na angielskim Steamie jak „Slay the Princess” czy „Wayhaven” (30 000+ kopii)? Wtedy przychód w R3 przebija 5 mln zł/rok, Twoje 78% zysku daje >250 tys. zł/mies., a wycena spółki przy przejęciu przez gracza typu Storytel/Empik/Audioteka sięga 25–40 mln zł."
                                span.deck-mech-amt "Ty: >250 tys. zł/m"

                footer.deck-page-footer
                    span.deck-footer-left "MOJRA • PRZEGLĄD ZAŁOŻYCIELA (POUFNE)"
                    span.deck-footer-center "DANE SPÓJNE Z DOCS/MOJRA-DECK-FINANSOWY.PDF"
                    span.deck-footer-right "STRONA 03 / 03"
