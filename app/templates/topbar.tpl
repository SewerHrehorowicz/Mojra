header.topbar#topbar
    .topbar-left
        {if is_view_editor}
            {if has_editor_book}
                button.topbar-brand-btn#tab-studio-home
                    type: button
                    action: SwitchView
                    data-view: studio
                    data-hint: "Playbook Studio"
                    aria-label: "Playbook Studio"
                    svg.brand-mark src: /icons/studio.svg
                span.topbar-divider
                span.brand-logo.topbar-book-title#editor-book-title {editor_book_title}
            {else}
                svg.brand-mark src: /icons/studio.svg
                span.brand-logo "Playbook Studio"
        {elif is_view_player}
            {if standalone_mode}
                svg.brand-mark src: /icons/mojra.svg
                span.topbar-divider
                span.brand-logo {active_book_title}
            {else}
                button.topbar-brand-btn#tab-library
                    type: button
                    action: SwitchView
                    data-view: library
                    data-hint: "{t_nav_library}"
                    aria-label: "{t_nav_library}"
                    svg.brand-mark src: /icons/mojra.svg
                span.topbar-divider
                span.brand-logo.topbar-book-title#topbar-book-title
                    data-hint: "{active_book_title}"
                    {active_book_title}
        {else}
            svg.brand-mark src: /icons/mojra.svg
            span.brand-logo "Mojra"

    .topbar-right#topbar-right
        .topbar-actions-menu#topbar-actions-menu
            {if is_view_player}
                {if show_zen_mode}
                    button.top-action-btn.icon-toggle-btn#btn-toggle-zen .active?: {zen_mode}
                        type: button
                        action: ToggleZenMode
                        data-hint: "{zen_toggle_title}"
                        svg.action-icon#icon-zen src: /icons/zen.svg
            {else}
                button.top-action-btn#btn-open-settings
                    type: button
                    action: OpenQRModal
                    data-hint: "{t_app_settings_hint}"
                    svg.action-icon src: /icons/settings.svg
            button.top-action-btn.desktop-only-btn#btn-toggle-fullscreen
                type: button
                action: ToggleFullscreen
                data-hint: "{t_fullscreen_hint}"
                svg.action-icon#icon-fullscreen-enter src: /icons/fullscreen.svg
                svg.action-icon.hidden#icon-fullscreen-exit src: /icons/windowed.svg
        button.top-action-btn.btn-quit-app.desktop-only-btn
            type: button
            action: QuitApp
            svg.action-icon src: /icons/close.svg
