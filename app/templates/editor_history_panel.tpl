aside.editor-history-panel.hidden?=!show_editor_history_panel
    .history-panel-header
        span.history-panel-title "Historia i wersje"
        button.node-icon-btn
            type: button
            action: CloseEditorHistoryPanel
            "✕"

    .history-unsaved-banner
        -> if has_unnamed_edits
            .history-unsaved-text -> "Niezapisane zmiany od ostatniej wersji: {unnamed_edits_count}"
        -> else
            .history-unsaved-text.is-clean "Brak niezapisanych zmian"

    form.history-version-form
        action: SaveHistoryVersion
        label.node-micro-label "Nazwij i zapisz bieżący stan"
        .history-version-input-row
            input.history-version-input
                type: text
                name: version_name
                placeholder: "{default_next_version_name}"
            button.btn.btn-primary.btn-sm.history-version-save-btn
                type: submit
                "Zapisz wersję"

    .history-panel-body
        .history-panel-section
            label.node-micro-label "Bieżące zmiany (od ostatniej wersji)"
            -> if has_history_unnamed_items
                .history-items-list -> history_unnamed_items.each() as item
                    button.history-step-item.history-item-current?=item.is_current.history-item-undone?=item.is_undone
                        type: button
                        action: RestoreHistoryRevision
                        data-revision: "{item.id}"
                        .history-step-top
                            span.history-step-label -> item.label
                            -> if item.status_label
                                span.history-step-badge -> item.status_label
                        .history-step-meta
                            span.history-step-time -> item.time_label
            -> else
                .history-empty-note "Brak pośrednich kroków od ostatniej zapisanej wersji."

        .history-panel-section
            label.node-micro-label "Zapisane wersje"
            -> if has_history_named_versions
                .history-items-list -> history_named_versions.each() as ver
                    .history-version-card.history-item-current?=ver.is_current
                        .history-version-info
                            span.history-version-name -> ver.label
                            span.history-version-time -> ver.time_label
                        -> if ver.is_current
                            span.history-step-badge "AKTYWNA"
                        -> else
                            button.btn.btn-sm.history-restore-btn
                                type: button
                                action: RestoreHistoryRevision
                                data-revision: "{ver.id}"
                                "Przywróć"
            -> else
                .history-empty-note "Brak zapisanych wersji."
