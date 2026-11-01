aside.editor-breakdown-panel#editor-breakdown-panel .hidden?: {!show_editor_breakdown_panel}
    .history-panel-header
        span.history-panel-title "Raport produkcyjny i obsada"
        button.node-close-btn
            type: button
            action: CloseEditorBreakdownPanel
            svg.node-close-icon src: /icons/close.svg

    .history-panel-body.breakdown-panel-body
        .history-panel-section
            label.node-micro-label "Parametry produkcyjne"
            .breakdown-metrics-grid
                .breakdown-metric-cell
                    span.breakdown-metric-label "Objętość"
                    span.breakdown-metric-value {breakdown_total_pages}
                .breakdown-metric-cell
                    span.breakdown-metric-label "Czas ekranowy"
                    span.breakdown-metric-value {breakdown_runtime}
                .breakdown-metric-cell
                    span.breakdown-metric-label "Dni zdjęciowe"
                    span.breakdown-metric-value "~{breakdown_shoot_days} dni"
                .breakdown-metric-cell
                    span.breakdown-metric-label "Liczba scen"
                    span.breakdown-metric-value {breakdown_total_scenes}
                .breakdown-metric-cell
                    span.breakdown-metric-label "WN. / PL."
                    span.breakdown-metric-value {breakdown_int_ext_label}
                .breakdown-metric-cell
                    span.breakdown-metric-label "Dzień / Noc"
                    span.breakdown-metric-value {breakdown_day_night_label}

        {if has_breakdown_acts}
            .history-panel-section
                label.node-micro-label "Balans aktów"
                .breakdown-acts-bar
                    {for act in breakdown_acts}
                        span.breakdown-act-segment .is-dominant?: {act.is_dominant}
                            data-share-pct: "{act.share_pct}"
                            data-hint: "{act.name}: {act.share_pct}% ({act.formatted_length})"
                .breakdown-acts-list
                    {for act in breakdown_acts}
                        .breakdown-act-row
                            .breakdown-act-left
                                span.breakdown-act-name {act.name}
                                span.breakdown-act-meta "Sceny: {act.scene_count} • {act.formatted_length}"
                            span.breakdown-act-pct "{act.share_pct}%"

        .history-panel-section
            label.node-micro-label "Obsada (Lista postaci)"
            {if has_breakdown_characters}
                .breakdown-entities-list
                    {for ch in breakdown_characters}
                        .breakdown-entity-item
                            form.breakdown-rename-form.breakdown-entity-line
                                action: RenameCharacter
                                input
                                    type: hidden
                                    name: old_name
                                    value: "{ch.name}"
                                input.breakdown-name-input
                                    type: text
                                    name: new_name
                                    value: "{ch.name}"
                                    data-old-value: "{ch.name}"
                                    placeholder: "Nazwa postaci..."
                                    disabled?: {is_history_preview}
                                    readonly?: {is_history_preview}
                                span.breakdown-entity-days "~{ch.estimated_call_days} dni"
                            button.breakdown-jump-btn
                                type: button
                                action: SelectStudioScene
                                data-scene: "{ch.first_scene_id}"
                                span.history-version-time "Kwestie: {ch.dialogue_cues} • Słowa: {ch.word_count} • Sceny: {ch.total_scenes} (mówione: {ch.speaking_scenes}, tło: {ch.silent_scenes})"
                                span.breakdown-jump-arrow "→"
            {else}
                .history-empty-note "Brak rozpoznanych postaci mówiących w scenariuszu."

        .history-panel-section
            label.node-micro-label "Zestawienie lokacji"
            {if has_breakdown_locations}
                .breakdown-entities-list
                    {for loc in breakdown_locations}
                        .breakdown-entity-item
                            form.breakdown-rename-form.breakdown-entity-line
                                action: RenameLocation
                                input
                                    type: hidden
                                    name: old_location
                                    value: "{loc.name}"
                                {if loc.int_ext}
                                    span.breakdown-loc-prefix {loc.int_ext}
                                input.breakdown-name-input
                                    type: text
                                    name: new_location
                                    value: "{loc.name}"
                                    data-old-value: "{loc.name}"
                                    placeholder: "Nazwa lokacji..."
                                    disabled?: {is_history_preview}
                                    readonly?: {is_history_preview}
                                span.breakdown-entity-days "~{loc.estimated_shoot_days} dni"
                            button.breakdown-jump-btn
                                type: button
                                action: SelectStudioScene
                                data-scene: "{loc.first_scene_id}"
                                span.history-version-time "Sceny: {loc.scene_count} (Dzień: {loc.day_scenes} / Noc: {loc.night_scenes}) • {loc.formatted_length}"
                                span.breakdown-jump-arrow "→"
            {else}
                .history-empty-note "Brak lokacji w nagłówkach scen."
