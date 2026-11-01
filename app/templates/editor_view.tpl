section.view-panel#view-editor .is-history-preview?: {is_history_preview}
    data-history-preview: "{history_preview_attr}"
    .studio-workspace-full
        .canvas-toolbar
            .canvas-toolbar-left
                {if has_active_book}
                    input.canvas-legend.canvas-book-title-input#editor-book-title-input
                        type: text
                        name: title
                        value: "{active_book_title}"
                        data-old-title: "{active_book_title}"
                        placeholder: "{editor_title_placeholder}"
                        disabled?: {is_history_preview}
                        readonly?: {is_history_preview}
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

        {if has_book_warnings}
            .studio-warnings-banner
                {if is_screenplay_mode}
                    .book-warnings-title "⚠ Ostrzeżenia parsera i walidacji scenariusza ({book_warning_count}):"
                {else}
                    .book-warnings-title "⚠ Ostrzeżenia parsera i walidacji książki ({book_warning_count}):"
                ul.book-warnings-list
                    {for w in book_warnings}
                        li {w.text}

        .editor-workspace-body#editor-workspace-body
            @editor_context_panel
            @editor_history_panel
            @editor_breakdown_panel

            .timeline-scroll-viewport#timeline-scroll-viewport
                canvas.studio-canvas#studio-canvas
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
