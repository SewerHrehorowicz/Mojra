section.view-panel#view-editor
    .studio-workspace-full
        .canvas-toolbar
            .canvas-toolbar-left
                -> if has_active_book
                    input.canvas-legend.canvas-book-title-input#editor-book-title-input
                        type: text
                        name: title
                        value: "{active_book_title}"
                        data-old-title: "{active_book_title}"
                        placeholder: "Tytuł książki..."
                -> else
                    span.canvas-legend "Brak aktywnej książki"
                span.canvas-selection-badge.hidden#canvas-selection-badge "0 zaznaczonych"
            .canvas-toolbar-right
                button.top-action-btn
                    type: button
                    action: OpenNewBookModal
                    data-hint: "Utwórz nową książkę w bibliotece"
                    svg.action-icon src: /icons/book-plus.svg
                button.top-action-btn
                    type: button
                    data-canvas-add-node: "true"
                    data-hint: "Dodaj nową scenę na płótnie"
                    svg.action-icon src: /icons/node-plus.svg
                button.top-action-btn
                    type: button
                    action: CreateChapterGroup
                    data-hint: "Dodaj nowy rozdział na płótnie"
                    svg.action-icon src: /icons/chapter-plus.svg
                button.top-action-btn
                    type: button
                    action: AutoLayoutNodes
                    data-hint: "Automatycznie rozmieść węzły na siatce"
                    svg.action-icon src: /icons/nodes.svg
                .canvas-history-controls
                    button.top-action-btn.is-disabled?=!can_undo#btn-editor-undo
                        type: button
                        action: UndoEditor
                        disabled?=!can_undo
                        data-hint: "{undo_title}"
                        svg.action-icon src: /icons/reset.svg
                    button.top-action-btn.is-disabled?=!can_redo#btn-editor-redo
                        type: button
                        action: RedoEditor
                        disabled?=!can_redo
                        data-hint: "{redo_title}"
                        svg.action-icon src: /icons/redo.svg
                    button.top-action-btn.active?=show_editor_history_panel
                        type: button
                        action: ToggleEditorHistoryPanel
                        data-hint: "Historia zmian i wersje"
                        svg.action-icon src: /icons/history.svg
                        span.history-unsaved-badge.hidden?=!has_unnamed_edits#history-unsaved-badge -> unnamed_edits_count
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
                        svg.action-icon src: /icons/fullscreen.svg
                button.top-action-btn.active?=book_settings_open#btn-book-settings
                    type: button
                    action: ToggleBookSettings
                    data-hint: "Ustawienia książki (tytuł, autor, scena początkowa)"
                    svg.action-icon src: /icons/settings.svg
                button.top-action-btn.active?=book_narrated
                    type: button
                    action: ToggleBookNarrated
                    data-hint: "{book_narrated_label}"
                    -> if book_narrated
                        svg.action-icon src: /icons/mic.svg
                    -> else
                        svg.action-icon src: /icons/mic-off.svg
                button.top-action-btn.active?=book_imageless
                    type: button
                    action: ToggleBookImageless
                    data-hint: "{book_imageless_label}"
                    -> if book_imageless
                        svg.action-icon src: /icons/image-off.svg
                    -> else
                        svg.action-icon src: /icons/image.svg
                button.top-action-btn
                    type: button
                    action: PackBook
                    data-hint: "Eksportuj książkę (.stbook)"
                    svg.action-icon src: /icons/pack.svg
                button.top-action-btn
                    type: button
                    action: OpenEditorHelpModal
                    data-hint: "Pomoc edytora węzłów"
                    svg.action-icon src: /icons/help.svg

        -> if has_book_warnings
            .studio-warnings-banner
                .book-warnings-title -> "⚠ Ostrzeżenia parsera i walidacji książki ({book_warning_count}):"
                ul.book-warnings-list -> book_warnings.each() as w
                    li -> w.text

        .editor-workspace-body#editor-workspace-body
            @editor_context_panel
            @editor_history_panel

            .timeline-scroll-viewport#timeline-scroll-viewport
                canvas.studio-canvas#studio-canvas
                pre.hidden#studio-graph-data -> studio_graph_payload
                input.chapter-group-title-input.canvas-floating-chapter-input.hidden#canvas-chapter-input
                    type: text
                    placeholder: "Nazwa rozdziału..."
                .canvas-context-menu.hidden#canvas-context-menu
                    button.canvas-context-item#btn-context-add-node
                        type: button
                        "+ Dodaj nową scenę tutaj"
                    button.canvas-context-item#btn-context-add-chapter
                        type: button
                        "+ Utwórz nowy rozdział tutaj"
                    button.canvas-context-item
                        type: button
                        action: AutoLayoutNodes
                        "Auto-układ węzłów na siatce"
                    button.canvas-context-item#btn-context-reset-view
                        type: button
                        "Dopasuj widok do węzłów (Fit)"
                -> if not has_active_book
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
