doctype html
html lang: "{lang}"
    head
        meta charset: UTF-8
        meta
            name: viewport
            content: "width=device-width, initial-scale=1.0, maximum-scale=1.0, user-scalable=no, viewport-fit=cover"
        {if StandaloneMode}
            title {ActiveBookTitle}
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
        data-theme: "{Theme}"
        data-lang: "{lang}"
        data-imageless: "{ImagelessAttr}"
        data-imageless-locked: "{ImagelessLockedAttr}"
        data-ui-scale: "{UIScale}"
        data-zen: "{ZenModeAttr}"
        data-scroll-sensitivity: "{ScrollSensitivity}"
        {if DevMode}
            .device-stage#device-stage data-preview: "{PreviewMode}"
                .device-preview-bar
                    button.preview-mode-btn .active?: {PreviewMode == "portrait"}
                        data-mode: portrait
                        aria-label: "{t_preview_phone}"
                        svg.preview-device-icon src: /icons/device-phone.svg
                    button.preview-mode-btn .active?: {PreviewMode == "tablet"}
                        data-mode: tablet
                        aria-label: "{t_preview_tablet}"
                        svg.preview-device-icon src: /icons/device-tablet.svg
                    button.preview-mode-btn .active?: {PreviewMode == "full"}
                        data-mode: full
                        aria-label: "{t_preview_desktop}"
                        svg.preview-device-icon src: /icons/device-desktop.svg

                .device-frame
                    #app.app-root
                        @app_shell
        {else}
            #app.app-root
                @app_shell

        script src: /app.js
