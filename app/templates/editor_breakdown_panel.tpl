aside.editor-breakdown-panel#editor-breakdown-panel .hidden?: {!show_editor_breakdown_panel}
    .history-panel-header
        span.history-panel-title "Raport produkcyjny i obsada"
        button.node-close-btn
            type: button
            action: CloseEditorBreakdownPanel
            svg.node-close-icon src: /icons/close.svg

    .breakdown-export-bar
        button.btn.btn-sm
            type: button
            action: ExportScreenplayPDF
            data-hint: "Zapisz scenariusz PDF do katalogu dist/"
            "Eksport PDF"
        button.btn.btn-sm
            type: button
            action: ExportScreenplayFountain
            data-hint: "Zapisz plik .fountain do katalogu dist/"
            "Eksport .fountain"
        a.btn.btn-sm
            href: /api/export/screenplay.pdf
            download: ""
            "Pobierz PDF"
        a.btn.btn-sm
            href: /api/export/screenplay.fountain
            download: ""
            "Pobierz .fountain"

    .history-panel-body.breakdown-panel-body
        .history-panel-section
            label.node-micro-label "Parametry produkcyjne"
            .breakdown-metrics-grid
                .breakdown-metric-card
                    span.breakdown-metric-label "Objętość"
                    span.breakdown-metric-value {breakdown_total_pages}
                .breakdown-metric-card
                    span.breakdown-metric-label "Czas ekranowy"
                    span.breakdown-metric-value {breakdown_runtime}
                .breakdown-metric-card
                    span.breakdown-metric-label "Dni zdjęciowe"
                    span.breakdown-metric-value "~{breakdown_shoot_days} dni"
                .breakdown-metric-card
                    span.breakdown-metric-label "Liczba scen"
                    span.breakdown-metric-value {breakdown_total_scenes}
                .breakdown-metric-card
                    span.breakdown-metric-label "WN. / PL."
                    span.breakdown-metric-value {breakdown_int_ext_label}
                .breakdown-metric-card
                    span.breakdown-metric-label "Dzień / Noc"
                    span.breakdown-metric-value {breakdown_day_night_label}

        {if has_breakdown_acts}
            .history-panel-section
                label.node-micro-label "Balans aktów"
                .history-items-list
                    {for act in breakdown_acts}
                        .history-version-card
                            .history-version-info
                                span.history-version-name {act.name}
                                span.history-version-time "Sceny: {act.scene_count} • {act.formatted_length}"
                            span.history-step-badge "{act.share_pct}%"

        .history-panel-section
            label.node-micro-label "Obsada (Lista postaci)"
            {if has_breakdown_characters}
                .history-items-list
                    {for ch in breakdown_characters}
                        .breakdown-item-card
                            .breakdown-item-top
                                button.breakdown-jump-btn
                                    type: button
                                    action: SelectStudioScene
                                    data-scene: "{ch.first_scene_id}"
                                    span.history-version-name {ch.name}
                                span.history-step-badge "~{ch.estimated_call_days} dni"
                            .history-version-time "Kwestie: {ch.dialogue_cues} • Słowa: {ch.word_count} • Sceny: {ch.total_scenes} (mówione: {ch.speaking_scenes}, tło: {ch.silent_scenes})"
                            {if not is_history_preview}
                                form.breakdown-rename-form
                                    action: RenameCharacter
                                    input
                                        type: hidden
                                        name: old_name
                                        value: "{ch.name}"
                                    input.history-version-input
                                        type: text
                                        name: new_name
                                        value: "{ch.name}"
                                        placeholder: "Zmień postać globalnie..."
                                    button.btn.btn-sm.history-restore-btn
                                        type: submit
                                        "Zmień"
            {else}
                .history-empty-note "Brak rozpoznanych postaci mówiących w scenariuszu."

        .history-panel-section
            label.node-micro-label "Zestawienie lokacji"
            {if has_breakdown_locations}
                .history-items-list
                    {for loc in breakdown_locations}
                        .breakdown-item-card
                            .breakdown-item-top
                                button.breakdown-jump-btn
                                    type: button
                                    action: SelectStudioScene
                                    data-scene: "{loc.first_scene_id}"
                                    span.history-version-name "{loc.int_ext} {loc.name}"
                                span.history-step-badge "~{loc.estimated_shoot_days} dni"
                            .history-version-time "Sceny: {loc.scene_count} (Dzień: {loc.day_scenes} / Noc: {loc.night_scenes}) • {loc.formatted_length}"
                            {if not is_history_preview}
                                form.breakdown-rename-form
                                    action: RenameLocation
                                    input
                                        type: hidden
                                        name: old_location
                                        value: "{loc.name}"
                                    input.history-version-input
                                        type: text
                                        name: new_location
                                        value: "{loc.name}"
                                        placeholder: "Nowa nazwa lokacji..."
                                    button.btn.btn-sm
                                        type: submit
                                        "Zmień"
            {else}
                .history-empty-note "Brak lokacji w nagłówkach scen."
