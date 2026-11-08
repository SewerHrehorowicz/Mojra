doctype html
html lang: pl
    head
        meta charset: UTF-8
        meta
            name: viewport
            content: "width=device-width, initial-scale=1.0"
        title "Mojra — Każda opowieść ma więcej niż jedną nić"
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
    body data-theme: dark
        main.stage
            canvas.loom-canvas#loom-canvas aria-hidden: "true"

            section.hero aria-label: "Mojra"
                .brand-lockup
                    svg.brand-logo-svg src: /icons/mojra-logo.svg

                p.brand-subtitle
                    "MOJRA APP"
                    span.plus "+"
                    "PLAYBOOK STUDIO"

                .hero-divider aria-hidden: "true"

                h2.tagline "Każda opowieść ma więcej niż jedną nić."

                p.description "Mojra to ekosystem żywej literatury stworzony przez twórców, dla twórców i czytelników."
                p.description "Mojra to ilustrowane, interaktywne e-booki i audiobooki na każdym urządzeniu."
                p.description.last "Mojra to środowisko, w którym stworzysz grę narracyjną, słuchowisko, książkę i scenariusz."

                button.playbook-trigger#btn-launch-wasm
                    type: button
                    aria-label: "Playbook"
                    svg.playbook-icon src: /icons/playbook.svg

                button.beta-trigger#btn-open-beta
                    type: button
                    span.corners
                    "APLIKUJ O DOSTĘP"
                    span.dot "•"
                    span.accent "ZAMKNIĘTA BETA"

            footer.status-footer
                span.corners
                "STORYTELLING REDEFINED"
                span.dot "•"
                span.soon "COMING SOON"
                span.dot "•"
                a.footer-investor-link
                    href: inwestorzy.html
                    "DLA INWESTORÓW"

        .wasm-overlay#wasm-overlay
            role: dialog
            aria-modal: "true"
            aria-label: "Mojra WASM"
            .wasm-window
                header.wasm-topbar
                    .wasm-topbar-left
                        svg.wasm-topbar-icon src: /icons/playbook.svg
                        span "MOJRA.PLAYBOOK"
                    button.wasm-close-btn#btn-close-wasm
                        type: button
                        aria-label: "Zamknij"
                        svg.wasm-close-icon src: /icons/close.svg
                .wasm-viewport#wasm-mount
                    data-wasm-src: mojra.wasm
                    data-book-id: nie-boj-sie-policji-rzecznej
                    .wasm-splash#wasm-splash
                        img.wasm-splash-bg
                            src: cover.jpg
                            alt: ""
                            aria-hidden: "true"
                        .wasm-splash-content
                            h1.wasm-splash-title "Nie bój się policji rzecznej"
                            p.wasm-splash-author "Tytus Czyżewski"
                            .wasm-splash-divider aria-hidden: "true"
                            p.wasm-splash-desc "Mroczna, rozgałęziona opowieść o prostodusznym Krzysiu, który wstępuje do Policji Rzecznej pod komendą kapitana Arvo i wyrusza w górę umierającej rzeki na swój krwawy chrzest bojowy."
                            button.wasm-splash-play-btn#btn-wasm-play
                                type: button
                                data-wasm-play: "true"
                                data-wasm-src: mojra.wasm
                                data-book-id: nie-boj-sie-policji-rzecznej
                                aria-label: "Odtwórz"
                                svg.wasm-splash-play-icon src: /icons/play.svg

        .beta-overlay#beta-overlay
            role: dialog
            aria-modal: "true"
            aria-label: "Zamknięta Beta"
            .beta-modal
                header.beta-header
                    .beta-header-left
                        svg.beta-header-icon src: /icons/mojra.svg
                        span "ZAMKNIĘTA PULA TESTERÓW"
                    button.wasm-close-btn#btn-close-beta
                        type: button
                        aria-label: "Zamknij"
                        svg.wasm-close-icon src: /icons/close.svg
                form.beta-form#beta-form
                    p.beta-intro "Dostęp przyznajemy ręcznie wybranym twórcom, studiom i uczelniom."
                    .beta-field
                        label.beta-label for: "beta-email" "E-MAIL"
                        input.beta-input#beta-email
                            type: email
                            name: email
                            required: required
                            autocomplete: email
                            placeholder: "twoj@adres.pl"
                    .beta-field
                        label.beta-label for: "beta-reason" "CO TWORZYSZ I DO CZEGO UŻYJESZ MOJRY?"
                        textarea.beta-textarea#beta-reason
                            name: reason
                            rows: 1
                            required: required
                            placeholder: "Scenariusz filmowy, słuchowisko, gra narracyjna, interaktywny audiobook..."
                    p.beta-rodo "Dane służą wyłącznie do rozpatrzenia zgłoszenia i wysłania zaproszenia do testów (art. 6 ust. 1 lit. b RODO)."
                    .beta-actions
                        button.beta-submit#btn-submit-beta
                            type: submit
                            span.corners
                            span#beta-submit-label "WYŚLIJ ZGŁOSZENIE"

        script src: /app.js
