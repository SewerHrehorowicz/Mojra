header.topbar#topbar
    .topbar-left
        {if IsViewEditor}
            {if HasEditorBook}
                button.topbar-brand-btn#tab-studio-home
                    data-view: studio
                    data-hint: "Playbook Studio"
                    aria-label: "Playbook Studio"
                    svg.brand-mark src: /icons/studio.svg
                span.topbar-divider
                span.brand-logo.topbar-book-title#editor-book-title {EditorBook.Title}
            {else}
                svg.brand-mark src: /icons/studio.svg
                span.brand-logo "Playbook Studio"
        {elif IsViewPlayer}
            {if StandaloneMode}
                svg.brand-mark src: /icons/mojra.svg
                span.topbar-divider
                span.brand-logo {ActiveBookTitle}
            {else}
                button.topbar-brand-btn#tab-library
                    data-view: library
                    data-hint: "{t_nav_library}"
                    aria-label: "{t_nav_library}"
                    svg.brand-mark src: /icons/mojra.svg
                span.topbar-divider
                span.brand-logo.topbar-book-title#topbar-book-title
                    data-hint: "{ActiveBookTitle}"
                    {ActiveBookTitle}
        {else}
            svg.brand-mark src: /icons/mojra.svg
            span.brand-logo "Mojra"

    .topbar-right#topbar-right
        .topbar-actions-menu#topbar-actions-menu
            {if IsViewPlayer}
                {if ShowZenMode}
                    button.top-action-btn.icon-toggle-btn#btn-toggle-zen .active?: {ZenMode}
                        data-hint: "{ZenToggleTitle}"
                        svg.action-icon#icon-zen src: /icons/zen.svg
            {else}
                button.top-action-btn#btn-open-settings
                    data-hint: "{t_app_settings_hint}"
                    svg.action-icon src: /icons/settings.svg
            button.top-action-btn.desktop-only-btn#btn-toggle-fullscreen
                data-hint: "{t_fullscreen_hint}"
                svg.action-icon#icon-fullscreen-enter src: /icons/fullscreen.svg
                svg.action-icon.hidden#icon-fullscreen-exit src: /icons/windowed.svg
        button.top-action-btn.btn-quit-app.desktop-only-btn
            svg.action-icon src: /icons/close.svg
