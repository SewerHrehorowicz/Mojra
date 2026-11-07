section.view-panel#view-editor .is-history-preview?: {is_history_preview}
    data-history-preview: "{history_preview_attr}"
    .studio-workspace-full
        .canvas-toolbar
            .canvas-toolbar-left
                {if has_editor_book}
                    span.canvas-legend.canvas-book-title#editor-book-title {editor_book_title}
                {else}
                    span.canvas-legend {t_no_active_project}
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
                button.top-action-btn#btn-book-settings .active?: {book_settings_open}
                    type: button
                    action: ToggleBookSettings
                    data-hint: "{editor_settings_hint}"
                    svg.action-icon src: /icons/settings.svg
                button.top-action-btn#btn-editor-variables .active?: {show_editor_variables_panel}
                    type: button
                    action: ToggleEditorVariablesPanel
                    data-hint: "{t_variables_title}"
                    svg.action-icon src: /icons/braces.svg
                {if is_screenplay_mode}
                    button.top-action-btn#btn-editor-breakdown .active?: {show_editor_breakdown_panel}
                        type: button
                        action: ToggleEditorBreakdownPanel
                        data-hint: "{t_breakdown_hint}"
                        svg.action-icon src: /icons/cast.svg
                .canvas-history-controls
                    button.top-action-btn .active?: {show_editor_history_panel}
                        type: button
                        action: ToggleEditorHistoryPanel
                        data-hint: "{t_history_panel_hint}"
                        svg.action-icon src: /icons/history.svg
                button.top-action-btn#btn-editor-help .active?: {show_editor_help_panel}
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
                input.canvas-floating-choice-input.hidden#canvas-choice-input
                    type: text
                    placeholder: "{t_choice_text_placeholder}"
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
                {if not has_editor_book}
                    .editor-empty-state#editor-empty-state
                        .editor-empty-card
                            h3.editor-empty-title {t_no_open_project}
                            .editor-empty-actions
                                button.btn.btn-primary
                                    type: button
                                    action: OpenNewBookModal
                                    "{t_create_new_book_btn}"
                                button.btn
                                    type: button
                                    action: OpenStudioProjectModal
                                    "{t_open_project_btn}"
