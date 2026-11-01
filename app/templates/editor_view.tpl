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
                button.top-action-btn#btn-open-export-modal
                    type: button
                    action: OpenExportModal
                    data-hint: "Eksportuj projekt"
                    svg.action-icon src: /icons/export.svg
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
                    button.top-action-btn#btn-auto-layout-nodes
                        type: button
                        action: AutoLayoutNodes
                        data-hint: "Auto-układ węzłów na siatce"
                        svg.action-icon src: /icons/nodes.svg
            .canvas-toolbar-right
                button.top-action-btn#btn-book-settings .active?: {book_settings_open}
                    type: button
                    action: ToggleBookSettings
                    data-hint: "{editor_settings_hint}"
                    svg.action-icon src: /icons/settings.svg
                {if is_screenplay_mode}
                    button.top-action-btn#btn-editor-breakdown .active?: {show_editor_breakdown_panel}
                        type: button
                        action: ToggleEditorBreakdownPanel
                        data-hint: "Raport produkcyjny, obsada i lokacje"
                        svg.action-icon src: /icons/cast.svg
                .canvas-history-controls
                    button.top-action-btn .active?: {show_editor_history_panel}
                        type: button
                        action: ToggleEditorHistoryPanel
                        data-hint: "Historia zmian, gałęzie i wersje"
                        svg.action-icon src: /icons/history.svg
                button.top-action-btn#btn-editor-help .active?: {show_editor_help_panel}
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
            @editor_breakdown_panel
            @editor_history_panel
            aside.editor-help-panel#editor-help-panel .hidden?: {!show_editor_help_panel}
                .history-panel-header
                    span.history-panel-title "Pomoc — Node Studio"
                    button.node-close-btn
                        type: button
                        action: CloseEditorHelpModal
                        svg.node-close-icon src: /icons/close.svg
                .history-panel-body
                    .help-shortcuts-list.help-panel-list
                        .help-shortcut-row
                            span.help-shortcut-key "Chwyć uchwyt (⋮⋮)"
                            span.help-shortcut-desc "Przesuwa węzeł sceny na płótnie po siatce (24 px)."
                        .help-shortcut-row
                            span.help-shortcut-key "Tytuł / Wybór na węźle"
                            span.help-shortcut-desc "Kliknij tytuł sceny lub wiersz wyboru na węźle, aby zmienić jego nazwę bezpośrednio na płótnie."
                        .help-shortcut-row
                            span.help-shortcut-key "Stopka węzła (Rozgałęzienie / Rysik)"
                            span.help-shortcut-desc "Ikona po lewej dodaje nowy wybór i port wyjściowy; ikona rysika po prawej (lub 2× klik) otwiera edytor treści sceny."
                        .help-shortcut-row
                            span.help-shortcut-key "Grupy rozdziałów / aktów"
                            span.help-shortcut-desc "Kliknij nagłówek grupy, aby otworzyć jej ustawienia i statystyki w panelu bocznym; 2× klik zmienia nazwę inline."
                        .help-shortcut-row
                            span.help-shortcut-key "Spacja + przeciągnięcie / PPM"
                            span.help-shortcut-desc "Przesuwa widok płótna (Pan). Rolka myszy przybliża i oddala widok (Zoom)."

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
                input.canvas-floating-node-input.hidden#canvas-node-title-input
                    type: text
                    placeholder: "Tytuł sceny..."
                input.canvas-floating-choice-input.hidden#canvas-choice-input
                    type: text
                    placeholder: "Tekst wyboru..."
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
