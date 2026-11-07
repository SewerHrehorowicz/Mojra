header.topbar#topbar
    .topbar-left
        {if standalone_mode}
            span.brand-logo {active_book_title}
        {else}
            svg.brand-mark src: /icons/mojra.svg
            span.topbar-divider
            button.topbar-tab.topbar-icon-tab#tab-library .active?: {is_view_library}
                type: button
                action: SwitchView
                data-view: library
                data-hint: "{t_nav_library}"
                aria-label: "{t_nav_library}"
                svg.tab-icon src: /icons/library.svg

    {if !standalone_mode}
        .topbar-center
            {if has_active_book}
                button.topbar-tab.topbar-icon-tab#tab-player .active?: {is_view_player}
                    type: button
                    action: SwitchView
                    data-view: player
                    data-hint: "{active_book_title}"
                    aria-label: "{active_book_title}"
                    svg.tab-icon src: /icons/playbook.svg
            button.topbar-tab.topbar-icon-tab#tab-editor .active?: {is_view_editor}
                type: button
                action: SwitchView
                data-view: editor
                data-hint: "{t_nav_studio}"
                aria-label: "{t_nav_studio}"
                svg.tab-icon src: /icons/studio.svg

    .topbar-right#topbar-right
        button.top-action-btn.topbar-hamburger-btn#btn-topbar-menu
            type: button
            data-topbar-menu-toggle: "true"
            aria-label: "{t_menu}"
            svg.action-icon src: /icons/menu.svg
        .topbar-actions-menu#topbar-actions-menu
            {if show_reset_progress}
                button.top-action-btn
                    type: button
                    action: PromptResetProgress
                    data-hint: "{t_reset_story_hint}"
                    svg.action-icon src: /icons/reset.svg
            button.top-action-btn#btn-toggle-imageless .active?: {imageless_mode} .is-disabled?: {imageless_locked} .hidden?: {is_screenplay_mode}
                type: button
                action: ToggleImageless
                disabled?: {imageless_locked}
                data-hint: "{imageless_toggle_title}"
                svg.action-icon#icon-imageless-off .hidden?: {imageless_mode} src: /icons/image.svg
                svg.action-icon#icon-imageless-on .hidden?: {!imageless_mode} src: /icons/image-off.svg
            button.top-action-btn#btn-toggle-ui-scale .active?: {is_zen_ui}
                type: button
                action: ToggleUIScale
                data-hint: "{ui_scale_toggle_title}"
                svg.action-icon#icon-zoom-in .hidden?: {is_non_small_ui} src: /icons/zoom.svg
                svg.action-icon#icon-zoom-zen .hidden?: {!is_large_ui} src: /icons/zen.svg
                svg.action-icon#icon-zoom-out .hidden?: {!is_zen_ui} src: /icons/zoom-out.svg
            button.top-action-btn
                type: button
                action: ToggleTheme
                data-hint: "{t_toggle_theme_hint}"
                svg.action-icon#icon-theme-sun .hidden?: {theme == "light"} src: /icons/sun.svg
                svg.action-icon#icon-theme-moon .hidden?: {theme != "light"} src: /icons/theme.svg
            button.top-action-btn#btn-toggle-lang
                type: button
                action: ToggleLanguage
                data-hint: "{t_toggle_lang_hint}"
                {lang_upper}
            button.top-action-btn
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
        button.top-action-btn.btn-quit-app
            type: button
            action: QuitApp
            svg.action-icon src: /icons/close.svg
