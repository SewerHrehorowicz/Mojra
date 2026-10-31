html lang: pl
    head
        meta charset: UTF-8
        meta
            name: viewport
            content: "width=device-width, initial-scale=1.0, maximum-scale=1.0, user-scalable=no, viewport-fit=cover"
        title "StoryTeller — Interaktywne Audiobooki & Studio"
        link
            rel: icon
            type: image/png
            sizes: 256x256
            href: /icon.png
        link
            rel: icon
            type: image/x-icon
            href: /favicon.ico
        link
            rel: stylesheet
            href: /style.css
    body
        data-theme: "{theme}"
        data-imageless: "{imageless_attr}"
        data-imageless-locked: "{imageless_locked_attr}"
        data-ui-scale: "{ui_scale}"
        -> if dev_mode
            .device-stage#device-stage data-preview: "{preview_mode}"
                .device-preview-bar
                    span.preview-bar-label "Widok ekranu:"
                    button.preview-mode-btn.active?=(preview_mode == "full")
                        type: button
                        action: SetDeviceMode
                        data-mode: full
                        "Pełny ekran"
                    button.preview-mode-btn.active?=(preview_mode == "portrait")
                        type: button
                        action: SetDeviceMode
                        data-mode: portrait
                        "Telefon (Pionowo)"
                    button.preview-mode-btn.active?=(preview_mode == "landscape")
                        type: button
                        action: SetDeviceMode
                        data-mode: landscape
                        "Telefon (Poziomo)"
                    button.preview-mode-btn
                        type: button
                        action: ExtractEmbeddedBook
                        "Wypakuj wbudowaną książkę"

                .device-frame
                    .device-notch
                    #app.app-root
                        @app_shell
                    .device-home-bar
        -> else
            #app.app-root
                @app_shell

        script src: /app.js
