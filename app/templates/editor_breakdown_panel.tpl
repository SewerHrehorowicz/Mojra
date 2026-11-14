aside.editor-breakdown-panel#editor-breakdown-panel .hidden?: {!ShowEditorBreakdownPanel}
    .history-panel-header
        span.history-panel-title {t_breakdown_panel_title}
        button.node-close-btn.btn-close-breakdown
            svg.node-close-icon src: /icons/close.svg

    .history-panel-body.breakdown-panel-body
        .history-panel-section
            label.node-micro-label {t_bd_params_label}
            .breakdown-metrics-grid
                .breakdown-metric-cell
                    span.breakdown-metric-label {t_bd_volume}
                    span.breakdown-metric-value {BreakdownTotalPages}
                .breakdown-metric-cell
                    span.breakdown-metric-label {t_bd_screen_time}
                    span.breakdown-metric-value {BreakdownRuntime}
                .breakdown-metric-cell
                    span.breakdown-metric-label {t_bd_shoot_days}
                    span.breakdown-metric-value "~{BreakdownShootDays} {t_bd_days_unit}"
                .breakdown-metric-cell
                    span.breakdown-metric-label {t_bd_scene_count}
                    span.breakdown-metric-value {BreakdownTotalScenes}
                .breakdown-metric-cell
                    span.breakdown-metric-label {t_bd_int_ext}
                    span.breakdown-metric-value {BreakdownIntExtLabel}
                .breakdown-metric-cell
                    span.breakdown-metric-label {t_bd_day_night}
                    span.breakdown-metric-value {BreakdownDayNightLabel}

        {if HasBreakdownActs}
            .history-panel-section
                label.node-micro-label {t_bd_act_balance}
                .breakdown-acts-bar
                    {for act in BreakdownActs}
                        span.breakdown-act-segment .is-dominant?: {act.IsDominant}
                            data-share-pct: "{act.SharePct}"
                            data-hint: "{act.Name}: {act.SharePct}% ({act.FormattedLength})"
                .breakdown-acts-list
                    {for act in BreakdownActs}
                        .breakdown-act-row
                            .breakdown-act-left
                                span.breakdown-act-name {act.Name}
                                span.breakdown-act-meta "{t_metric_scenes}: {act.SceneCount} • {act.FormattedLength}"
                            span.breakdown-act-pct "{act.SharePct}%"

        .history-panel-section
            label.node-micro-label {t_bd_cast_label}
            {if HasBreakdownCharacters}
                .breakdown-entities-list
                    {for ch in BreakdownCharacters}
                        .breakdown-entity-item
                            form.breakdown-rename-form.breakdown-entity-line.breakdown-rename-char-form
                                input.breakdown-name-input
                                    type: text
                                    name: new_name
                                    value: "{ch.Name}"
                                    data-old-value: "{ch.Name}"
                                    placeholder: "{t_char_name_placeholder}"
                                    disabled?: {IsHistoryPreview}
                                    readonly?: {IsHistoryPreview}
                                span.breakdown-entity-days "~{ch.EstimatedCallDays} {t_bd_days_unit}"
                            button.breakdown-jump-btn
                                data-scene: "{ch.FirstSceneID}"
                                span.history-version-time "{t_bd_cues}: {ch.DialogueCues} • {t_bd_words}: {ch.WordCount} • {t_metric_scenes}: {ch.TotalScenes} ({t_bd_spoken}: {ch.SpeakingScenes}, {t_bd_bg}: {ch.SilentScenes})"
                                span.breakdown-jump-arrow "→"
            {else}
                .history-empty-note {t_no_characters_empty}

        .history-panel-section
            label.node-micro-label {t_bd_locations_label}
            {if HasBreakdownLocations}
                .breakdown-entities-list
                    {for loc in BreakdownLocations}
                        .breakdown-entity-item
                            form.breakdown-rename-form.breakdown-entity-line.breakdown-rename-loc-form
                                {if loc.IntExt}
                                    span.breakdown-loc-prefix {loc.IntExt}
                                input.breakdown-name-input
                                    type: text
                                    name: new_location
                                    value: "{loc.Name}"
                                    data-old-value: "{loc.Name}"
                                    placeholder: "{t_loc_name_placeholder}"
                                    disabled?: {IsHistoryPreview}
                                    readonly?: {IsHistoryPreview}
                                span.breakdown-entity-days "~{loc.EstimatedShootDays} {t_bd_days_unit}"
                            button.breakdown-jump-btn
                                data-scene: "{loc.FirstSceneID}"
                                span.history-version-time "{t_metric_scenes}: {loc.SceneCount} ({t_bd_day}: {loc.DayScenes} / {t_bd_night}: {loc.NightScenes}) • {loc.FormattedLength}"
                                span.breakdown-jump-arrow "→"
            {else}
                .history-empty-note {t_no_locations_empty}
