doctype html
html lang: pl
    head
        meta charset: UTF-8
        meta
            name: viewport
            content: "width=device-width, initial-scale=1.0, maximum-scale=1.0, user-scalable=no, viewport-fit=cover"
        {if standalone_mode}
            title {active_book_title}
        {else}
            title "Mojra"
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
        {if dev_mode}
            .device-stage#device-stage data-preview: "{preview_mode}"
                .device-preview-bar
                    button.preview-mode-btn .active?: {preview_mode == "portrait"}
                        type: button
                        action: SetDeviceMode
                        data-mode: portrait
                        aria-label: "Telefon"
                        svg.preview-device-icon src: /icons/device-phone.svg
                    button.preview-mode-btn .active?: {preview_mode == "tablet"}
                        type: button
                        action: SetDeviceMode
                        data-mode: tablet
                        aria-label: "Tablet"
                        svg.preview-device-icon src: /icons/device-tablet.svg
                    button.preview-mode-btn .active?: {preview_mode == "full"}
                        type: button
                        action: SetDeviceMode
                        data-mode: full
                        aria-label: "Desktop"
                        svg.preview-device-icon src: /icons/device-desktop.svg

                .device-frame
                    #app.app-root
                        @app_shell
        {else}
            #app.app-root
                @app_shell

        script src: /app.js
