doctype html
html lang: pl
    head
        meta charset: UTF-8
        meta
            name: viewport
            content: "width=device-width, initial-scale=1.0, maximum-scale=1.0, user-scalable=no, viewport-fit=cover"
        title {Title}
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
    body
        data-theme: dark
        data-imageless: "{ImagelessAttr}"
        data-imageless-locked: "{ImagelessAttr}"
        data-ui-scale: small
        .app-root.wasm-player-root#app
            data-wasm-src: "{WasmSrc}"
            data-book-id: "{BookID}"
            .wasm-splash#wasm-splash
                {if CoverSrc}
                    img.wasm-splash-bg
                        src: "{CoverSrc}"
                        alt: ""
                        aria-hidden: "true"
                .wasm-splash-content
                    h1.wasm-splash-title {Title}
                    {if Author}
                        p.wasm-splash-author {Author}
                    {if Description}
                        .wasm-splash-divider aria-hidden: "true"
                        p.wasm-splash-desc {Description}
                    button.wasm-splash-play-btn#btn-wasm-play
                        data-wasm-src: "{WasmSrc}"
                        data-book-id: "{BookID}"
                        aria-label: "Odtwórz"
                        svg.wasm-splash-play-icon src: /icons/play.svg
        script src: app.js
