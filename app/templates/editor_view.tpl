section.view-panel#view-editor .is-history-preview?: {is_history_preview}
    data-history-preview: "{history_preview_attr}"
    .studio-workspace-full
        .canvas-toolbar
            .canvas-toolbar-left
                {if has_active_book}
                    span.canvas-legend.canvas-book-title#editor-book-title {active_book_title}
                {else}
                    span.canvas-legend "Brak aktywnego projektu"
                button.top-action-btn
                    type: button
                    action: OpenNewBookModal
                    data-hint: "Utwórz nową książkę lub scenariusz"
                    svg.action-icon src: /icons/book-plus.svg
                button.top-action-btn
                    type: button
                    action: PackBook
                    data-hint: "{editor_pack_hint}"
                    svg.action-icon src: /icons/pack.svg
                {if is_screenplay_mode}
                    button.top-action-btn#btn-export-screenplay-pdf
                        type: button
                        action: ExportScreenplayPDF
                        data-hint: "Eksportuj scenariusz do druku (.pdf)"
                        "PDF"
                    button.top-action-btn#btn-export-screenplay-fountain
                        type: button
                        action: ExportScreenplayFountain
                        data-hint: "Eksportuj scenariusz (.fountain)"
                        ".fountain"
                    span.canvas-script-summary#canvas-script-summary {screenplay_toolbar_summary}
                span.canvas-selection-badge.hidden#canvas-selection-badge "0 zaznaczonych"
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
                        data-hint: "Dopasuj widok do węzłów (F)"
                        svg.action-icon src: /icons/fit-nodes.svg
            .canvas-toolbar-right
                {if is_screenplay_mode}
                    button.top-action-btn#btn-editor-breakdown .active?: {show_editor_breakdown_panel}
                        type: button
                        action: ToggleEditorBreakdownPanel
                        data-hint: "Raport produkcyjny, obsada i lokacje"
                        "Obsada / Lokacje"
                .canvas-history-controls
                    button.top-action-btn .active?: {show_editor_history_panel}
                        type: button
                        action: ToggleEditorHistoryPanel
                        data-hint: "Historia zmian, gałęzie i wersje"
                        svg.action-icon src: /icons/history.svg
                        span.history-unsaved-badge#history-unsaved-badge .hidden?: {!has_unnamed_edits} {unnamed_edits_count}
                button.top-action-btn#btn-book-settings .active?: {book_settings_open}
                    type: button
                    action: ToggleBookSettings
                    data-hint: "{editor_settings_hint}"
                    svg.action-icon src: /icons/settings.svg
                button.top-action-btn
                    type: button
                    action: OpenEditorHelpModal
                    data-hint: "Pomoc edytora węzłów"
                    svg.action-icon src: /icons/help.svg

        {if is_history_preview}
            .studio-preview-banner#studio-preview-banner
                span.studio-preview-banner-text "Podgląd wersji historycznej: {history_preview_label} — edycja zablokowana"
                .studio-preview-banner-actions
                    button.btn.btn-sm
                        type: button
                        action: PlayHistoryRevision
                        data-revision: "{history_preview_id}"
                        "Graj"
                    button.btn.btn-sm
                        type: button
                        action: RestoreHistoryRevision
                        data-revision: "{history_preview_id}"
                        "Przywróć do edycji"
                    button.btn.btn-primary.btn-sm
                        type: button
                        action: ExitHistoryPreview
                        "Wróć do aktualnej wersji"

        .editor-workspace-body#editor-workspace-body
            @editor_context_panel
            @editor_history_panel
            @editor_breakdown_panel

            .timeline-scroll-viewport#timeline-scroll-viewport
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
                    .editor-perf-overlay.hidden#editor-perf-overlay
                pre.hidden#studio-graph-data {studio_graph_payload}
                input.chapter-group-title-input.canvas-floating-chapter-input.hidden#canvas-chapter-input
                    type: text
                    placeholder: "{editor_chapter_title_placeholder}"
                .canvas-context-menu.hidden#canvas-context-menu
                    {if not is_history_preview}
                        button.canvas-context-item#btn-context-add-node
                            type: button
                            "+ Dodaj nową scenę tutaj"
                        button.canvas-context-item#btn-context-add-chapter
                            type: button
                            {if is_screenplay_mode}
                                "+ Utwórz nowy akt tutaj"
                            {else}
                                "+ Utwórz nowy rozdział tutaj"
                        button.canvas-context-item
                            type: button
                            action: AutoLayoutNodes
                            "Auto-układ węzłów na siatce"
                    button.canvas-context-item#btn-context-reset-view
                        type: button
                        "Dopasuj widok do węzłów (Fit)"
                {if not has_active_book}
                    .editor-empty-state#editor-empty-state
                        .editor-empty-card
                            h3.editor-empty-title "Brak otwartej książki"
                            p.editor-empty-desc "Utwórz nową opowieść w edytorze lub dodaj istniejącą książkę z dysku."
                            .editor-empty-actions
                                button.btn.btn-primary
                                    type: button
                                    action: OpenNewBookModal
                                    "+ Utwórz nową książkę"
                                button.btn
                                    type: button
                                    action: OpenAddBookModal
                                    "Dodaj książkę"
