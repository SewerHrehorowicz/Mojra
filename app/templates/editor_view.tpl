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
                        title: "Edytuj tytuł książki (zapisuje do data.txt)"
                -> else
                    span.canvas-legend "Brak aktywnej książki"
                span.canvas-selection-badge.hidden#canvas-selection-badge "0 zaznaczonych"
            .canvas-toolbar-right
                button.top-action-btn#btn-new-book
                    type: button
                    action: OpenNewBookModal
                    title: "Utwórz nową książkę w bibliotece"
                    svg.action-icon src: /icons/book-plus.svg
                button.top-action-btn#btn-add-canvas-node
                    type: button
                    data-canvas-add-node: "true"
                    title: "Dodaj nowy węzeł sceny na płótnie (lub kliknij 2× / PPM na tło)"
                    svg.action-icon src: /icons/node-plus.svg
                button.top-action-btn#btn-add-chapter-group
                    type: button
                    action: CreateChapterGroup
                    title: "Dodaj nowy rozdział (grupę węzłów) na płótnie"
                    svg.action-icon src: /icons/chapter-plus.svg
                button.top-action-btn#btn-auto-layout
                    type: button
                    action: AutoLayoutNodes
                    title: "Automatycznie rozmieść węzły i grupy rozdziałów na siatce"
                    svg.action-icon src: /icons/nodes.svg
                .canvas-history-controls
                    button.top-action-btn.{undo_disabled_class}#btn-editor-undo
                        type: button
                        action: UndoEditor
                        disabled: "{undo_disabled}"
                        title: "{undo_title}"
                        svg.action-icon src: /icons/reset.svg
                    button.top-action-btn.{redo_disabled_class}#btn-editor-redo
                        type: button
                        action: RedoEditor
                        disabled: "{redo_disabled}"
                        title: "{redo_title}"
                        svg.action-icon src: /icons/redo.svg
                    button.top-action-btn.{history_panel_active_class}#btn-editor-history
                        type: button
                        action: ToggleEditorHistoryPanel
                        title: "Historia zmian i zapisane wersje książki"
                        svg.action-icon src: /icons/history.svg
                        span.history-unsaved-badge.{history_badge_hidden_class}#history-unsaved-badge -> unnamed_edits_count
                .canvas-zoom-controls
                    button.top-action-btn#btn-zoom-out
                        type: button
                        data-canvas-zoom: "out"
                        title: "Oddal widok (-)"
                        svg.action-icon src: /icons/minus.svg
                    button.top-action-btn.canvas-zoom-label#btn-zoom-reset
                        type: button
                        data-canvas-zoom: "reset"
                        title: "Resetuj przybliżenie (100%)"
                        "100%"
                    button.top-action-btn#btn-zoom-in
                        type: button
                        data-canvas-zoom: "in"
                        title: "Przybliż widok (+)"
                        svg.action-icon src: /icons/plus.svg
                    button.top-action-btn#btn-zoom-fit
                        type: button
                        data-canvas-zoom: "fit"
                        title: "Dopasuj widok do wszystkich węzłów (F)"
                        svg.action-icon src: /icons/fullscreen.svg
                button.top-action-btn.active?=book_narrated#btn-toggle-narrated
                    type: button
                    action: ToggleBookNarrated
                    title: "{book_narrated_label}"
                    -> if book_narrated
                        svg.action-icon src: /icons/mic.svg
                    -> else
                        svg.action-icon src: /icons/mic-off.svg
                button.top-action-btn.active?=book_imageless#btn-toggle-book-imageless
                    type: button
                    action: ToggleBookImageless
                    title: "{book_imageless_label}"
                    -> if book_imageless
                        svg.action-icon src: /icons/image-off.svg
                    -> else
                        svg.action-icon src: /icons/image.svg
                button.top-action-btn#btn-pack-book
                    type: button
                    action: PackBook
                    title: "Eksportuj książkę do pakietu .stbook"
                    svg.action-icon src: /icons/pack.svg
                button.top-action-btn#btn-editor-help
                    type: button
                    action: OpenEditorHelpModal
                    title: "Pomoc i sterowanie edytorem węzłów"
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
                data-pan-x: "{editor_pan_x}"
                data-pan-y: "{editor_pan_y}"
                data-zoom: "{editor_zoom}"
                data-has-view: "{has_editor_view}"
                canvas.studio-canvas#studio-canvas
                pre.hidden#studio-graph-data -> studio_graph_payload
                input.chapter-group-title-input.canvas-floating-chapter-input.hidden#canvas-chapter-input
                    type: text
                    placeholder: "Nazwa rozdziału..."
                    title: "Edytuj nazwę rozdziału (zmienia we wszystkich scenach grupy)"
                .canvas-context-menu.hidden#canvas-context-menu
                    button.canvas-context-item#btn-context-add-node
                        type: button
                        "+ Dodaj nową scenę tutaj"
                    button.canvas-context-item#btn-context-add-chapter
                        type: button
                        "+ Utwórz nowy rozdział tutaj"
                    button.canvas-context-item#btn-context-auto-layout
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
                                button.btn.btn-primary#btn-empty-editor-new-book
                                    type: button
                                    action: OpenNewBookModal
                                    "+ Utwórz nową książkę"
                                button.btn#btn-empty-editor-add-book
                                    type: button
                                    action: OpenAddBookModal
                                    "Dodaj książkę"
