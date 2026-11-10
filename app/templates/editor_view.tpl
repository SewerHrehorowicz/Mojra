section.view-panel#view-editor .is-history-preview?: {is_history_preview}
    data-history-preview: "{history_preview_attr}"
    {if has_editor_book}
        .studio-workspace-full
            .canvas-toolbar
                .canvas-toolbar-left
                    button.top-action-btn
                        type: button
                        action: OpenNewBookModal
                        data-hint: "{t_new_book_hint}"
                        svg.action-icon src: /icons/book-plus.svg
                    button.top-action-btn#btn-open-studio-project
                        type: button
                        action: OpenStudioProjectModal
                        data-hint: "{t_open_project_hint}"
                        svg.action-icon src: /icons/folder.svg
                    button.top-action-btn#btn-open-export-modal
                        type: button
                        action: OpenExportModal
                        data-hint: "{t_export_project_hint}"
                        svg.action-icon src: /icons/export.svg
                    span.canvas-selection-badge.hidden#canvas-selection-badge {t_zero_selected}
                .canvas-toolbar-center
                    .canvas-zoom-controls
                        button.top-action-btn
                            type: button
                            data-canvas-zoom: "out"
                            svg.action-icon src: /icons/minus.svg
                        button.top-action-btn.canvas-zoom-label#btn-zoom-reset
                            type: button
                            data-canvas-zoom: "reset"
                            "100%"
                        button.top-action-btn
                            type: button
                            data-canvas-zoom: "in"
                            svg.action-icon src: /icons/plus.svg
                        button.top-action-btn
                            type: button
                            data-canvas-zoom: "fit"
                            data-hint: "{t_fit_nodes_hint}"
                            svg.action-icon src: /icons/fit-nodes.svg
                        button.top-action-btn#btn-auto-layout-nodes
                            type: button
                            action: AutoLayoutNodes
                            data-hint: "{t_auto_layout_hint}"
                            svg.action-icon src: /icons/nodes.svg
                .canvas-toolbar-right
                    button.top-action-btn.icon-toggle-btn#btn-studio-search
                        type: button
                        data-studio-search-toggle: "true"
                        data-hint: "{t_studio_search_hint}"
                        svg.action-icon src: /icons/search.svg
                    button.top-action-btn.icon-toggle-btn#btn-book-settings .active?: {book_settings_open}
                        type: button
                        action: ToggleBookSettings
                        data-hint: "{editor_settings_hint}"
                        svg.action-icon src: /icons/settings.svg
                    button.top-action-btn.icon-toggle-btn#btn-editor-variables .active?: {show_editor_variables_panel}
                        type: button
                        action: ToggleEditorVariablesPanel
                        data-hint: "{t_variables_title}"
                        svg.action-icon src: /icons/braces.svg
                    {if is_screenplay_mode}
                        button.top-action-btn.icon-toggle-btn#btn-editor-breakdown .active?: {show_editor_breakdown_panel}
                            type: button
                            action: ToggleEditorBreakdownPanel
                            data-hint: "{t_breakdown_hint}"
                            svg.action-icon src: /icons/cast.svg
                    .canvas-history-controls
                        button.top-action-btn.icon-toggle-btn .active?: {show_editor_history_panel}
                            type: button
                            action: ToggleEditorHistoryPanel
                            data-hint: "{t_history_panel_hint}"
                            svg.action-icon src: /icons/history.svg
                    button.top-action-btn.icon-toggle-btn#btn-editor-help .active?: {show_editor_help_panel}
                        type: button
                        action: OpenEditorHelpModal
                        data-hint: "{t_editor_help_hint}"
                        svg.action-icon src: /icons/help.svg

            {if is_history_preview}
                .studio-preview-banner#studio-preview-banner
                    span.studio-preview-banner-text "{t_history_preview_banner}: {history_preview_label} — {t_editing_locked}"
                    .studio-preview-banner-actions
                        button.btn.btn-sm
                            type: button
                            action: PlayHistoryRevision
                            data-revision: "{history_preview_id}"
                            "{t_play_btn}"
                        button.btn.btn-sm
                            type: button
                            action: RestoreHistoryRevision
                            data-revision: "{history_preview_id}"
                            "{t_restore_for_editing}"
                        button.btn.btn-primary.btn-sm
                            type: button
                            action: ExitHistoryPreview
                            "{t_back_to_current_version}"

            .editor-workspace-body#editor-workspace-body
                @editor_context_panel
                @editor_variables_panel
                @editor_breakdown_panel
                @editor_history_panel
                aside.editor-help-panel#editor-help-panel .hidden?: {!show_editor_help_panel}
                    .history-panel-header
                        span.history-panel-title {t_help_panel_title}
                        button.node-close-btn
                            type: button
                            action: CloseEditorHelpModal
                            svg.node-close-icon src: /icons/close.svg
                    .history-panel-body
                        .help-shortcuts-list.help-panel-list
                            .help-shortcut-row
                                span.help-shortcut-key {t_help_k1}
                                span.help-shortcut-desc {t_help_d1}
                            .help-shortcut-row
                                span.help-shortcut-key {t_help_k2}
                                span.help-shortcut-desc {t_help_d2}
                            .help-shortcut-row
                                span.help-shortcut-key {t_help_k3}
                                span.help-shortcut-desc {t_help_d3}
                            .help-shortcut-row
                                span.help-shortcut-key {t_help_k4}
                                span.help-shortcut-desc {t_help_d4}
                            .help-shortcut-row
                                span.help-shortcut-key {t_help_k5}
                                span.help-shortcut-desc {t_help_d5}

                .timeline-scroll-viewport#timeline-scroll-viewport
                    button.scene-text-expand-btn#btn-expand-scene-text
                        type: button
                        action: OpenSceneTextModal
                        data-scene: "{selected_scene_id}"
                        svg.btn-icon src: /icons/chevrons-right.svg
                        span.scene-text-expand-title#scene-text-expand-title {selected_scene_title}
                    .studio-search-palette.hidden#studio-search-palette
                        .studio-search-header
                            svg.studio-search-icon src: /icons/search.svg
                            input.studio-search-input#studio-search-input
                                type: text
                                placeholder: "{t_studio_search_placeholder}"
                                autocomplete: "off"
                                spellcheck: "false"
                            span.studio-search-count#studio-search-count "{studio_search_count}"
                            button.node-close-btn#btn-close-studio-search
                                type: button
                                data-studio-search-close: "true"
                                svg.node-close-icon src: /icons/close.svg
                        .studio-search-results#studio-search-results
                            {for sr in studio_search_items}
                                .studio-search-row .is-active?: {sr.is_selected}
                                    data-search-scene: "{sr.id}"
                                    data-search-title: "{sr.title}"
                                    data-search-slug: "{sr.slugline}"
                                    data-search-chapter: "{sr.chapter}"
                                    data-search-synopsis: "{sr.synopsis}"
                                    data-search-b64: "{sr.text_b64}"
                                    data-search-choices: "{sr.choices}"
                                    data-default-badge: "{sr.default_badge}"
                                    data-default-snippet: "{sr.default_snippet}"
                                    .studio-search-row-top
                                        span.studio-search-row-title {sr.display_title}
                                        span.studio-search-row-id {sr.id}
                                        span.studio-search-row-badge {sr.default_badge}
                                        button.studio-search-edit-btn
                                            type: button
                                            data-search-open-text: "{sr.id}"
                                            data-hint: "{t_open_scene_text_hint}"
                                            svg.action-icon src: /icons/edit.svg
                                    .studio-search-row-chapter {sr.chapter_upper}
                                    .studio-search-row-snippet
                                        span.studio-search-snip-pre {sr.default_snippet}
                                        strong.studio-search-snip-hit ""
                                        span.studio-search-snip-post ""
                    canvas.studio-canvas#studio-canvas
                    .editor-status-bar#editor-status-bar
                        {if has_book_warnings}
                            .editor-status-warnings#editor-status-warnings
                                ul.editor-status-warnings-list.hidden#editor-status-warnings-list
                                    {for w in book_warnings}
                                        li {w.text}
                                {if has_multiple_book_warnings}
                                    button.editor-status-warnings-bar#btn-toggle-status-warnings
                                        type: button
                                        data-warnings-toggle: "true"
                                        span.editor-status-warnings-text "⚠ ({book_warning_count}) {first_book_warning}"
                                        svg.editor-status-warnings-icon#icon-warnings-expand src: /icons/plus.svg
                                        svg.editor-status-warnings-icon.hidden#icon-warnings-collapse src: /icons/minus.svg
                                {else}
                                    .editor-status-warnings-bar.is-single
                                        span.editor-status-warnings-text "⚠ {first_book_warning}"
                        {if dev_mode}
                            .editor-perf-overlay.hidden#editor-perf-overlay
                    pre.hidden#studio-graph-data {studio_graph_payload}
                    input.chapter-group-title-input.canvas-floating-chapter-input.hidden#canvas-chapter-input
                        type: text
                        placeholder: "{editor_chapter_title_placeholder}"
                    input.canvas-floating-node-input.hidden#canvas-node-title-input
                        type: text
                        placeholder: "{t_scene_title_placeholder}"
                        autocomplete: "off"
                    input.canvas-floating-choice-input.hidden#canvas-choice-input
                        type: text
                        placeholder: "{t_choice_text_placeholder}"
                        autocomplete: "off"
                    .canvas-context-menu.hidden#canvas-context-menu
                        {if not is_history_preview}
                            button.canvas-context-item#btn-context-add-node
                                type: button
                                "{t_ctx_add_node}"
                            button.canvas-context-item#btn-context-add-chapter
                                type: button
                                {if is_screenplay_mode}
                                    "{t_ctx_add_act}"
                                {else}
                                    "{t_ctx_add_chapter}"
                            button.canvas-context-item
                                type: button
                                action: AutoLayoutNodes
                                "{t_auto_layout_hint}"
                        button.canvas-context-item#btn-context-reset-view
                            type: button
                            "{t_ctx_fit_view}"
    {else}
        .studio-home-container#editor-empty-state
            .library-top-actions
                button.btn.btn-sm#btn-new-book
                    type: button
                    action: OpenNewBookModal
                    data-mode: book
                    "{t_new_book_short_btn}"
                button.btn.btn-sm#btn-new-screenplay
                    type: button
                    action: OpenNewBookModal
                    data-mode: screenplay
                    "{t_create_new_screenplay_btn}"
                button.btn.btn-sm#btn-open-studio-project-home
                    type: button
                    action: OpenStudioProjectModal
                    "{t_open_project_from_disk_btn}"
            {if not has_studio_home_projects}
                {if not has_studio_packed_books}
                    p.bookshelf-empty {t_no_open_project}
            .bookshelf-grid#studio-projects-grid
                {for p in studio_home_projects}
                    article.book-card.studio-project-card
                        button.book-cover-wrap.studio-project-cover-wrap
                            type: button
                            action: OpenStudioProject
                            data-book: "{p.id}"
                            span.book-cover-play-overlay.is-default-cover
                                svg.book-cover-play-icon.studio-project-cover-icon src: /icons/studio.svg
                        .book-card-body
                            .book-card-meta
                                span.book-card-author.studio-project-meta {p.meta_line}
                            h3.book-card-title {p.title}
                            p.book-card-desc.studio-project-path {p.path_line}
            {if has_studio_packed_books}
                .bookshelf-grid.studio-packed-grid#studio-packed-books-grid
                    {for p in studio_packed_books}
                        article.book-card.studio-project-card.studio-packed-card .is-unpacked?: {p.is_unpacked}
                            button.book-cover-wrap.studio-project-cover-wrap.studio-packed-cover-wrap
                                type: button
                                action: OpenStudioProject
                                data-book: "{p.id}"
                                disabled?: {p.is_unpacked}
                                span.book-cover-play-overlay.is-default-cover
                                    {if p.is_unpacked}
                                        svg.book-cover-play-icon.studio-project-cover-icon src: /icons/pack-open.svg
                                    {else}
                                        svg.book-cover-play-icon.studio-project-cover-icon src: /icons/pack.svg
                            .book-card-body
                                .book-card-meta
                                    span.book-card-author.studio-project-meta {p.meta_line}
                                h3.book-card-title {p.title}
                                p.book-card-desc.studio-project-path {p.path_line}
            .home-mode-footer
                button.home-mode-switch-btn#btn-switch-to-reader
                    type: button
                    action: SwitchView
                    data-view: library
                    svg.home-mode-switch-icon src: /icons/mojra.svg
                    span.home-mode-switch-label
                        "{t_home_switch_to_reader_prefix} "
                        strong.home-mode-switch-target "Mojra"
                        span.home-mode-switch-arrow "  →"
