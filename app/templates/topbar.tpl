header.topbar#topbar
    .topbar-left
        -> if standalone_mode
            span.brand-logo -> active_book_title
        -> else
            span.brand-logo "StoryTeller"
            span.brand-logo.brand-logo-short "ST"
            button.topbar-tab.active?=is_view_library
                type: button
                action: SwitchView
                data-view: library
                svg.tab-icon src: /icons/library.svg
                span.tab-label "Biblioteka"
            -> if has_active_book
                button.topbar-tab.active?=is_view_player#tab-player
                    type: button
                    action: SwitchView
                    data-view: player
                    svg.tab-icon src: /icons/book.svg
                    span.tab-label -> active_book_title
            button.topbar-tab.active?=is_view_editor
                type: button
                action: SwitchView
                data-view: editor
                svg.tab-icon src: /icons/nodes.svg
                span.tab-label "Node Studio"

    .topbar-right#topbar-right
        button.top-action-btn.topbar-hamburger-btn#btn-topbar-menu
            type: button
            data-topbar-menu-toggle: "true"
            aria-label: "Menu"
            svg.action-icon src: /icons/menu.svg
        .topbar-actions-menu#topbar-actions-menu
            -> if show_reset_progress
                button.top-action-btn
                    type: button
                    action: PromptResetProgress
                    data-hint: "Wymaż postęp i zacznij od początku"
                    svg.action-icon src: /icons/reset.svg
            button.top-action-btn.active?=imageless_mode.is-disabled?=imageless_locked#btn-toggle-imageless
                type: button
                action: ToggleImageless
                disabled?=imageless_locked
                data-hint: "{imageless_toggle_title}"
                svg.action-icon.hidden?=imageless_mode#icon-imageless-off src: /icons/image.svg
                svg.action-icon.hidden?=!imageless_mode#icon-imageless-on src: /icons/image-off.svg
            button.top-action-btn#btn-toggle-ui-scale
                type: button
                action: ToggleUIScale
                data-hint: "{ui_scale_toggle_title}"
                svg.action-icon.hidden?=is_large_ui#icon-zoom-in src: /icons/zoom.svg
                svg.action-icon.hidden?=!is_large_ui#icon-zoom-out src: /icons/zoom-out.svg
            button.top-action-btn
                type: button
                action: ToggleTheme
                data-hint: "Przełącz motyw Jasny / Ciemny"
                svg.action-icon.hidden?=(theme == "light")#icon-theme-sun src: /icons/sun.svg
                svg.action-icon.hidden?=(theme != "light")#icon-theme-moon src: /icons/theme.svg
            -> if dev_mode
                button.top-action-btn
                    type: button
                    action: OpenQRModal
                    data-hint: "Ustawienia / Kod QR LAN"
                    svg.action-icon src: /icons/settings.svg
            button.top-action-btn.desktop-only-btn#btn-toggle-fullscreen
                type: button
                action: ToggleFullscreen
                data-hint: "Przełącz pełny ekran"
                svg.action-icon#icon-fullscreen-enter src: /icons/fullscreen.svg
                svg.action-icon.hidden#icon-fullscreen-exit src: /icons/windowed.svg
        button.top-action-btn.btn-quit-app
            type: button
            action: QuitApp
            svg.action-icon src: /icons/close.svg
