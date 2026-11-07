aside.editor-breakdown-panel#editor-breakdown-panel .hidden?: {!show_editor_breakdown_panel}
    .history-panel-header
        span.history-panel-title {t_breakdown_panel_title}
        button.node-close-btn
            type: button
            action: CloseEditorBreakdownPanel
            svg.node-close-icon src: /icons/close.svg

    .history-panel-body.breakdown-panel-body
        .history-panel-section
            label.node-micro-label {t_bd_params_label}
            .breakdown-metrics-grid
                .breakdown-metric-cell
                    span.breakdown-metric-label {t_bd_volume}
                    span.breakdown-metric-value {breakdown_total_pages}
                .breakdown-metric-cell
                    span.breakdown-metric-label {t_bd_screen_time}
                    span.breakdown-metric-value {breakdown_runtime}
                .breakdown-metric-cell
                    span.breakdown-metric-label {t_bd_shoot_days}
                    span.breakdown-metric-value "~{breakdown_shoot_days} {t_bd_days_unit}"
                .breakdown-metric-cell
                    span.breakdown-metric-label {t_bd_scene_count}
                    span.breakdown-metric-value {breakdown_total_scenes}
                .breakdown-metric-cell
                    span.breakdown-metric-label {t_bd_int_ext}
                    span.breakdown-metric-value {breakdown_int_ext_label}
                .breakdown-metric-cell
                    span.breakdown-metric-label {t_bd_day_night}
                    span.breakdown-metric-value {breakdown_day_night_label}

        {if has_breakdown_acts}
            .history-panel-section
                label.node-micro-label {t_bd_act_balance}
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
                                span.breakdown-act-meta "{t_metric_scenes}: {act.scene_count} • {act.formatted_length}"
                            span.breakdown-act-pct "{act.share_pct}%"

        .history-panel-section
            label.node-micro-label {t_bd_cast_label}
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
                                    placeholder: "{t_char_name_placeholder}"
                                    disabled?: {is_history_preview}
                                    readonly?: {is_history_preview}
                                span.breakdown-entity-days "~{ch.estimated_call_days} {t_bd_days_unit}"
                            button.breakdown-jump-btn
                                type: button
                                action: SelectStudioScene
                                data-scene: "{ch.first_scene_id}"
                                span.history-version-time "{t_bd_cues}: {ch.dialogue_cues} • {t_bd_words}: {ch.word_count} • {t_metric_scenes}: {ch.total_scenes} ({t_bd_spoken}: {ch.speaking_scenes}, {t_bd_bg}: {ch.silent_scenes})"
                                span.breakdown-jump-arrow "→"
            {else}
                .history-empty-note {t_no_characters_empty}

        .history-panel-section
            label.node-micro-label {t_bd_locations_label}
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
                                    placeholder: "{t_loc_name_placeholder}"
                                    disabled?: {is_history_preview}
                                    readonly?: {is_history_preview}
                                span.breakdown-entity-days "~{loc.estimated_shoot_days} {t_bd_days_unit}"
                            button.breakdown-jump-btn
                                type: button
                                action: SelectStudioScene
                                data-scene: "{loc.first_scene_id}"
                                span.history-version-time "{t_metric_scenes}: {loc.scene_count} ({t_bd_day}: {loc.day_scenes} / {t_bd_night}: {loc.night_scenes}) • {loc.formatted_length}"
                                span.breakdown-jump-arrow "→"
            {else}
                .history-empty-note {t_no_locations_empty}
