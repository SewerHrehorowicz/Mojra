section.view-panel#view-editor .is-history-preview?: {IsHistoryPreview}
    data-history-preview: "{HistoryPreviewAttr}"
    {if HasEditorBook}
        .studio-workspace-full
            .canvas-toolbar
                .canvas-toolbar-left
                    button.top-action-btn.btn-open-new-book
                        data-hint: "{t_new_book_hint}"
                        svg.action-icon src: /icons/book-plus.svg
                    button.top-action-btn#btn-open-studio-project
                        data-hint: "{t_open_project_hint}"
                        svg.action-icon src: /icons/folder.svg
                    button.top-action-btn#btn-open-export-modal
                        data-hint: "{t_export_project_hint}"
                        svg.action-icon src: /icons/export.svg
                    span.canvas-selection-badge.hidden#canvas-selection-badge {t_zero_selected}
                .canvas-toolbar-center
                    .canvas-zoom-controls
                        button.top-action-btn
                            data-canvas-zoom: "out"
                            svg.action-icon src: /icons/minus.svg
                        button.top-action-btn.canvas-zoom-label#btn-zoom-reset
                            data-canvas-zoom: "reset"
                            "100%"
                        button.top-action-btn
                            data-canvas-zoom: "in"
                            svg.action-icon src: /icons/plus.svg
                        button.top-action-btn
                            data-canvas-zoom: "fit"
                            data-hint: "{t_fit_nodes_hint}"
                            svg.action-icon src: /icons/fit-nodes.svg
                        button.top-action-btn#btn-auto-layout-nodes
                            data-hint: "{t_auto_layout_hint}"
                            svg.action-icon src: /icons/nodes.svg
                .canvas-toolbar-right
                    button.top-action-btn.icon-toggle-btn#btn-studio-search .active?: {StudioSearchOpen}
                        data-studio-search-toggle: "true"
                        data-hint: "{t_studio_search_hint}"
                        svg.action-icon src: /icons/search.svg
                    button.top-action-btn.icon-toggle-btn#btn-book-settings .active?: {BookSettingsOpen}
                        data-hint: "{EditorSettingsHint}"
                        svg.action-icon src: /icons/settings.svg
                    button.top-action-btn.icon-toggle-btn#btn-editor-variables .active?: {ShowEditorVariablesPanel}
                        data-hint: "{t_variables_title}"
                        svg.action-icon src: /icons/braces.svg
                    {if IsScreenplayMode}
                        button.top-action-btn.icon-toggle-btn#btn-editor-breakdown .active?: {ShowEditorBreakdownPanel}
                            data-hint: "{t_breakdown_hint}"
                            svg.action-icon src: /icons/cast.svg
                    .canvas-history-controls
                        button.top-action-btn.icon-toggle-btn.btn-toggle-history .active?: {ShowEditorHistoryPanel}
                            data-hint: "{t_history_panel_hint}"
                            svg.action-icon src: /icons/history.svg
                    button.top-action-btn.icon-toggle-btn#btn-editor-help .active?: {ShowEditorHelpPanel}
                        data-hint: "{t_editor_help_hint}"
                        svg.action-icon src: /icons/help.svg

            {if IsHistoryPreview}
                .studio-preview-banner#studio-preview-banner
                    span.studio-preview-banner-text "{t_history_preview_banner}: {HistoryPreviewLabel} — {t_editing_locked}"
                    .studio-preview-banner-actions
                        button.btn.btn-sm.btn-play-history-revision
                            data-revision: "{HistoryPreviewID}"
                            "{t_play_btn}"
                        button.btn.btn-sm.btn-restore-history-revision
                            data-revision: "{HistoryPreviewID}"
                            "{t_restore_for_editing}"
                        button.btn.btn-primary.btn-sm.btn-exit-history-preview
                            "{t_back_to_current_version}"

            .editor-workspace-body#editor-workspace-body
                @ContextPanel
                @VariablesPanel
                @BreakdownPanel
                @HistoryPanel
                aside.editor-help-panel#editor-help-panel .hidden?: {!ShowEditorHelpPanel}
                    .history-panel-header
                        span.history-panel-title {t_help_panel_title}
                        button.node-close-btn.btn-close-editor-help
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
                        data-scene: "{SelectedSceneID}"
                        svg.btn-icon src: /icons/chevrons-right.svg
                        span.scene-text-expand-title#scene-text-expand-title {SelectedSceneTitle}
                    .studio-search-palette#studio-search-palette .hidden?: {!StudioSearchOpen}
                        .studio-search-header
                            svg.studio-search-icon src: /icons/search.svg
                            input.studio-search-input#studio-search-input
                                type: text
                                name: q
                                value: "{StudioSearchQuery}"
                                placeholder: "{t_studio_search_placeholder}"
                                autocomplete: "off"
                                spellcheck: "false"
                            span.studio-search-count#studio-search-count "{StudioSearchCount}"
                            button.node-close-btn#btn-close-studio-search
                                data-studio-search-close: "true"
                                svg.node-close-icon src: /icons/close.svg
                        .studio-search-results#studio-search-results
                            {for sr in StudioSearchItems}
                                .studio-search-row .is-active?: {sr.IsSelected}
                                    data-search-scene: "{sr.ID}"
                                    data-search-title: "{sr.Title}"
                                    data-search-slug: "{sr.Slugline}"
                                    data-search-chapter: "{sr.Chapter}"
                                    data-search-synopsis: "{sr.Synopsis}"
                                    data-search-b64: "{sr.TextB64}"
                                    data-search-choices: "{sr.Choices}"
                                    data-default-badge: "{sr.DefaultBadge}"
                                    data-default-snippet: "{sr.DefaultSnippet}"
                                    .studio-search-row-top
                                        span.studio-search-row-title {sr.DisplayTitle}
                                        span.studio-search-row-id {sr.ID}
                                        span.studio-search-row-badge {sr.DefaultBadge}
                                        button.studio-search-edit-btn
                                            data-search-open-text: "{sr.ID}"
                                            data-hint: "{t_open_scene_text_hint}"
                                            svg.action-icon src: /icons/edit.svg
                                    .studio-search-row-chapter {sr.ChapterUpper}
                                    .studio-search-row-snippet
                                        span.studio-search-snip-pre {sr.DefaultSnippet}
                                        strong.studio-search-snip-hit ""
                                        span.studio-search-snip-post ""
                    canvas.studio-canvas#studio-canvas
                    .editor-status-bar#editor-status-bar
                        {if HasBookWarnings}
                            .editor-status-warnings#editor-status-warnings
                                ul.editor-status-warnings-list#editor-status-warnings-list .hidden?: {!BookWarningsOpen}
                                    {for w in BookWarnings}
                                        li {w}
                                {if HasMultipleBookWarnings}
                                    button.editor-status-warnings-bar#btn-toggle-status-warnings
                                        data-warnings-toggle: "true"
                                        span.editor-status-warnings-text "⚠ ({BookWarningCount}) {FirstBookWarning}"
                                        svg.editor-status-warnings-icon#icon-warnings-expand .hidden?: {BookWarningsOpen} src: /icons/plus.svg
                                        svg.editor-status-warnings-icon#icon-warnings-collapse .hidden?: {!BookWarningsOpen} src: /icons/minus.svg
                                {else}
                                    .editor-status-warnings-bar.is-single
                                        span.editor-status-warnings-text "⚠ {FirstBookWarning}"
                    pre.hidden#studio-graph-data {StudioGraphPayload}
                    input.chapter-group-title-input.canvas-floating-chapter-input.hidden#canvas-chapter-input
                        type: text
                        placeholder: "{EditorChapterTitlePlaceholder}"
                    input.canvas-floating-node-input.hidden#canvas-node-title-input
                        type: text
                        placeholder: "{t_scene_title_placeholder}"
                        autocomplete: "off"
                    input.canvas-floating-choice-input.hidden#canvas-choice-input
                        type: text
                        placeholder: "{t_choice_text_placeholder}"
                        autocomplete: "off"
                    .canvas-context-menu.hidden#canvas-context-menu
                        {if not IsHistoryPreview}
                            button.canvas-context-item#btn-context-add-node
                                "{t_ctx_add_node}"
                            button.canvas-context-item#btn-context-add-chapter
                                {if IsScreenplayMode}
                                    "{t_ctx_add_act}"
                                {else}
                                    "{t_ctx_add_chapter}"
                            button.canvas-context-item.btn-auto-layout-nodes
                                "{t_auto_layout_hint}"
                        button.canvas-context-item#btn-context-reset-view
                            "{t_ctx_fit_view}"
    {else}
        .studio-home-container#editor-empty-state
            .library-top-actions
                button.btn.btn-sm#btn-new-book
                    data-mode: book
                    "{t_new_book_short_btn}"
                button.btn.btn-sm#btn-new-screenplay
                    data-mode: screenplay
                    "{t_create_new_screenplay_btn}"
                button.btn.btn-sm#btn-open-studio-project-home
                    "{t_open_project_from_disk_btn}"
            {if not HasStudioHomeProjects}
                {if not HasStudioPackedBooks}
                    p.bookshelf-empty {t_no_open_project}
            .bookshelf-grid#studio-projects-grid
                {for p in StudioHomeProjects}
                    article.book-card.studio-project-card
                        button.book-cover-wrap.studio-project-cover-wrap
                            data-book: "{p.ID}"
                            span.book-cover-play-overlay.is-default-cover
                                svg.book-cover-play-icon.studio-project-cover-icon src: /icons/studio.svg
                        .book-card-body
                            .book-card-meta
                                span.book-card-author.studio-project-meta {p.MetaLine}
                            h3.book-card-title {p.Title}
                            p.book-card-desc.studio-project-path {p.PathLine}
            {if HasStudioPackedBooks}
                .bookshelf-grid.studio-packed-grid#studio-packed-books-grid
                    {for p in StudioPackedBooks}
                        article.book-card.studio-project-card.studio-packed-card .is-unpacked?: {p.IsUnpacked}
                            button.book-cover-wrap.studio-project-cover-wrap.studio-packed-cover-wrap
                                data-book: "{p.ID}"
                                disabled?: {p.IsUnpacked}
                                span.book-cover-play-overlay.is-default-cover
                                    {if p.IsUnpacked}
                                        svg.book-cover-play-icon.studio-project-cover-icon src: /icons/pack-open.svg
                                    {else}
                                        svg.book-cover-play-icon.studio-project-cover-icon src: /icons/pack.svg
                            .book-card-body
                                .book-card-meta
                                    span.book-card-author.studio-project-meta {p.MetaLine}
                                h3.book-card-title {p.Title}
                                p.book-card-desc.studio-project-path {p.PathLine}
            .home-mode-footer
                button.home-mode-switch-btn#btn-switch-to-reader
                    data-view: library
                    svg.home-mode-switch-icon src: /icons/mojra.svg
                    span.home-mode-switch-label
                        "{t_home_switch_to_reader_prefix} "
                        strong.home-mode-switch-target "Mojra"
                        span.home-mode-switch-arrow "  →"
