aside.editor-context-panel#editor-context-panel .hidden?: {is_context_panel_hidden}
    {if has_selected_node}
        form.editor-context-form#editor-context-form .hidden?: {is_node_inspector_hidden}
            action: SaveScene
            data-context-scene-id: "{selected_node.id}"
            data-book-vars: "{book_vars_spec}"
            input
                type: hidden
                name: scene_id
                value: "{selected_node.id}"
            input
                type: hidden
                name: source_file
                value: "{selected_node.source_file}"
            input
                type: hidden
                name: choice_count
                value: "{selected_node.port_count}"

            .context-panel-section
                .custom-select.node-chapter-select
                    .node-chapter-input-wrap
                        input.node-inline-chapter.custom-select-input
                            type: text
                            name: chapter
                            value: "{selected_node.chapter}"
                            placeholder: "{t_chapter_placeholder}"
                            disabled?: {is_history_preview}
                            readonly?: {is_history_preview}
                        {if not is_history_preview}
                            button.node-chapter-caret-btn
                                type: button
                                data-select-toggle: "true"
                                svg.custom-select-caret src: /icons/chevron-down.svg
                    .custom-select-menu.hidden
                        {for ch_opt in selected_node.chapter_options}
                            button.custom-select-option .active?: {ch_opt.is_active}
                                type: button
                                data-select-value: "{ch_opt.value}"
                                data-select-label: "{ch_opt.label}"
                                {ch_opt.label}

            .context-panel-section
                .node-title-row
                    {if selected_node.is_start_scene}
                        span.node-start-indicator
                            data-hint: "{t_start_scene_badge_hint}"
                            svg.scene-star-icon src: /icons/star.svg
                    {elif selected_node.is_chapter_entry}
                        span.node-start-indicator.node-chapter-entry-indicator
                            data-hint: "{t_chapter_entry_hint}"
                            svg.scene-star-icon src: /icons/star.svg
                    input.node-inline-title
                        type: text
                        name: name
                        value: "{selected_node.name}"
                        placeholder: "{t_scene_name_opt_placeholder}"
                        disabled?: {is_history_preview}
                        readonly?: {is_history_preview}

            .context-panel-section
                .node-meta-fields
                    .node-meta-row
                        span.node-meta-key "scene:"
                        input.node-meta-input#context-scene-id-input
                            type: text
                            name: explicit_id
                            value: "{selected_node.explicit_id}"
                            placeholder: "{t_scene_id_placeholder}"
                            spellcheck: "false"
                            autocorrect: "off"
                            autocapitalize: "off"
                            autocomplete: "off"
                            disabled?: {is_history_preview}
                            readonly?: {is_history_preview}
                    {if has_book_variables}
                        .node-meta-row.node-meta-row-effects .is-empty-rule?: {!selected_node.has_effects}
                            span.node-meta-key "set:"
                            .syn-input-wrap .is-invalid?: {selected_node.is_effects_invalid}
                                data-hint?: {selected_node.effects_error_hint}
                                input.node-meta-input.syn-inner-input
                                    type: text
                                    name: scene_effects
                                    value: "{selected_node.effects}"
                                    placeholder: "odkryta_prawda = true; sumienie++"
                                    spellcheck: "false"
                                    autocorrect: "off"
                                    autocapitalize: "off"
                                    autocomplete: "off"
                                    data-hint?: {selected_node.effects_error_hint}
                                    disabled?: {is_history_preview}
                                    readonly?: {is_history_preview}
                    {if is_screenplay_mode}
                        .node-meta-row
                            span.node-meta-key {t_sp_scene_key}
                            input.node-meta-input.node-slugline-input
                                type: text
                                name: slugline
                                value: "{selected_node.slugline}"
                                placeholder: "{t_slugline_placeholder}"
                                disabled?: {is_history_preview}
                                readonly?: {is_history_preview}
                            {if selected_node.script_length_label}
                                span.node-script-length-badge {selected_node.script_length_label}
                        {if not is_history_preview}
                            .slugline-helper-pills
                                {for p in slugline_prefix_pills}
                                    button.slugline-pill-btn
                                        type: button
                                        action: ApplySluglineToken
                                        data-scene: "{selected_node.id}"
                                        data-prefix: "{p.token}"
                                        {p.token}
                                {for t in slugline_time_pills}
                                    button.slugline-pill-btn
                                        type: button
                                        action: ApplySluglineToken
                                        data-scene: "{selected_node.id}"
                                        data-time: "{t.token}"
                                        {t.token}
                        .node-meta-row.node-meta-row-textarea
                            span.node-meta-key {t_desc_key}
                            textarea.node-meta-input.node-meta-textarea
                                name: synopsis
                                rows: 1
                                placeholder: "{t_synopsis_placeholder}"
                                disabled?: {is_history_preview}
                                readonly?: {is_history_preview}
                                {selected_node.synopsis}
                    {if not is_screenplay_mode}
                        .node-meta-row
                            span.node-meta-key "image:"
                            .custom-select.node-media-select
                                input.custom-select-input
                                    type: hidden
                                    name: explicit_image
                                    value: "{selected_node.current_image}"
                                button.custom-select-trigger
                                    type: button
                                    data-select-toggle: "true"
                                    disabled?: {is_history_preview}
                                    span.custom-select-label {selected_node.current_image_label}
                                    svg.custom-select-caret src: /icons/chevron-down.svg
                                .custom-select-menu.hidden
                                    {for img_opt in selected_node.image_options}
                                        button.custom-select-option .active?: {img_opt.is_active}
                                            type: button
                                            data-select-value: "{img_opt.value}"
                                            data-select-label: "{img_opt.label}"
                                            {img_opt.label}
                            {if not is_history_preview}
                                button.node-icon-btn.node-media-add-btn
                                    type: button
                                    action: BrowseAddBookMedia
                                    data-kind: image
                                    data-scene: "{selected_node.id}"
                                    svg.media-plus-icon src: /icons/plus.svg
                    {if book_narrated}
                        {if not is_history_preview}
                            {if selected_node.has_voice_sections}
                                .voice-rec-sections.inspector-voice-rec-sections .hidden?: {!voice_recorder_open}
                                    .voice-rec-sections-tabs
                                        {for vsec in selected_node.voice_sections}
                                            button.voice-rec-section-btn .active?: {vsec.is_active} .is-recorded?: {vsec.has_audio}
                                                type: button
                                                action: SelectSceneVoiceSection
                                                data-scene: "{selected_node.id}"
                                                data-voice-section: "{vsec.key}"
                                                span.voice-rec-section-dot .is-recorded?: {vsec.has_audio}
                                                span.voice-rec-section-label {vsec.inspector_label}
                        .node-meta-row
                            span.node-meta-key "voice:"
                            .custom-select.node-media-select
                                input.custom-select-input
                                    type: hidden
                                    name: explicit_voice
                                    value: "{selected_node.current_voice}"
                                button.custom-select-trigger
                                    type: button
                                    data-select-toggle: "true"
                                    disabled?: {is_history_preview}
                                    span.custom-select-label {selected_node.current_voice_label}
                                    svg.custom-select-caret src: /icons/chevron-down.svg
                                .custom-select-menu.hidden
                                    {for voice_opt in selected_node.voice_options}
                                        button.custom-select-option .active?: {voice_opt.is_active}
                                            type: button
                                            data-select-value: "{voice_opt.value}"
                                            data-select-label: "{voice_opt.label}"
                                            {voice_opt.label}
                            {if not is_history_preview}
                                button.node-icon-btn.node-media-add-btn
                                    type: button
                                    action: BrowseAddBookMedia
                                    data-kind: audio
                                    data-scene: "{selected_node.id}"
                                    svg.media-plus-icon src: /icons/plus.svg
                                button.node-icon-btn.node-media-add-btn.icon-toggle-btn.voice-rec-toggle-btn#btn-inspector-voice-recorder .active?: {voice_recorder_open}
                                    type: button
                                    action: ToggleSceneVoiceRecorder
                                    data-scene: "{selected_node.id}"
                                    svg.media-plus-icon src: /icons/mic.svg
                        {if not is_history_preview}
                            .voice-rec-strip#inspector-voice-rec-strip .hidden?: {!voice_recorder_open}
                                data-scene: "{selected_node.id}"
                                data-voice-section: "{selected_node.active_voice_section_key}"
                                data-voice-url: "{selected_node.current_voice_url}"
                                data-voice-file: "{selected_node.current_voice}"
                                data-trim-start: "{selected_node.voice_trim_start}"
                                data-trim-end: "{selected_node.voice_trim_end}"
                                button.voice-rec-transport-btn
                                    type: button
                                    data-voice-transport: "true"
                                    svg.voice-rec-icon.voice-rec-icon-play .hidden?: {!selected_node.has_voice_file} src: /icons/voice-play.svg
                                    svg.voice-rec-icon.voice-rec-icon-mic .hidden?: {selected_node.has_voice_file} src: /icons/mic.svg
                                    svg.voice-rec-icon.voice-rec-icon-pause.hidden src: /icons/voice-pause.svg
                                    svg.voice-rec-icon.voice-rec-icon-stop.hidden src: /icons/voice-stop.svg
                                .voice-rec-waveform-wrap
                                    data-voice-waveform-wrap: "true"
                                    canvas.voice-rec-waveform-canvas
                                        width: "216"
                                        height: "28"
                                    span.voice-rec-playhead
                                        data-voice-playhead: "true"
                                    span.voice-rec-trim-handle.trim-start
                                        data-voice-trim: "start"
                                    span.voice-rec-trim-handle.trim-end
                                        data-voice-trim: "end"
                                span.voice-rec-timer
                                    data-voice-timer: "true"
                                    {selected_node.voice_duration_label}
                                button.voice-rec-action-btn.voice-rec-cut-btn .is-disabled?: {!selected_node.has_voice_trim}
                                    type: button
                                    data-voice-trim-destructive: "true"
                                    data-scene: "{selected_node.id}"
                                    data-voice-section: "{selected_node.active_voice_section_key}"
                                    data-hint: "{t_voice_trim_destructive_hint}"
                                    disabled?: {!selected_node.has_voice_trim}
                                    svg.voice-rec-btn-icon src: /icons/scissors.svg
                                button.voice-rec-action-btn.voice-rec-rerecord-btn .hidden?: {!selected_node.has_voice_file}
                                    type: button
                                    data-voice-record-start: "true"
                                    svg.voice-rec-btn-icon src: /icons/mic.svg
                                button.voice-rec-action-btn.voice-rec-delete-btn .hidden?: {!selected_node.has_voice_file}
                                    type: button
                                    action: DeleteSceneVoice
                                    data-scene: "{selected_node.id}"
                                    data-voice-section: "{selected_node.active_voice_section_key}"
                                    svg.voice-rec-btn-icon src: /icons/trash.svg

            .context-panel-section
                button.btn.btn-sm.node-edit-text-btn
                    type: button
                    action: OpenSceneTextModal
                    data-scene: "{selected_node.id}"
                    data-hint: "{t_open_scene_text_hint}"
                    {if is_history_preview}
                        "{t_preview_content_btn}"
                    {else}
                        "{t_edit_content_btn}"
                button.btn.btn-sm.node-edit-text-btn
                    type: button
                    action: PlayFromScene
                    data-scene: "{selected_node.id}"
                    data-hint: "{t_play_from_scene_hint}"
                    "{t_play_from_scene_btn}"

            {if selected_node.has_node_warnings}
                ul.node-warnings-list
                    {for nw in selected_node.node_warnings}
                        li {nw.text}

            .context-panel-section.context-choices-section
                .context-choices-list
                    {for pt in selected_node.inspector_ports}
                        .context-choice-item .is-rule-expanded?: {pt.is_rule_open} .has-active-rule?: {pt.has_rule}
                            {if not is_history_preview}
                                span.choice-drag-grip
                                    data-choice-grip: "true"
                                    data-choice-idx: "{pt.idx}"
                                    svg.choice-grip-icon src: /icons/grip.svg
                            .choice-triple-stack
                                .choice-triple-row.choice-triple-exit .is-invalid?: {pt.is_exit_invalid}
                                    data-hint?: {pt.exit_error_hint}
                                    span.choice-row-key "exit:"
                                    input.choice-exit-input
                                        type: text
                                        name: "choice_target_{pt.idx}"
                                        data-choice-idx: "{pt.idx}"
                                        value: "{pt.selected_value}"
                                        placeholder: "{t_exit_target_placeholder}"
                                        spellcheck: "false"
                                        autocorrect: "off"
                                        autocapitalize: "off"
                                        autocomplete: "off"
                                        data-hint?: {pt.exit_error_hint}
                                        disabled?: {is_history_preview}
                                        readonly?: {is_history_preview}
                                .choice-triple-row.choice-triple-label
                                    span.choice-row-key "text:"
                                    textarea.node-meta-input.node-meta-textarea.node-choice-label
                                        name: "choice_label_{pt.idx}"
                                        data-choice-idx: "{pt.idx}"
                                        rows: 1
                                        placeholder: "{t_choice_text_placeholder}"
                                        disabled?: {is_history_preview}
                                        readonly?: {is_history_preview}
                                        {pt.raw_label}
                                {if pt.is_rule_open}
                                    .choice-triple-row.choice-triple-cond .is-empty-rule?: {!pt.has_condition} .is-invalid?: {pt.is_cond_invalid}
                                        data-hint?: {pt.cond_error_hint}
                                        span.choice-row-key "if:"
                                        input.choice-rule-input
                                            type: text
                                            name: "choice_cond_{pt.idx}"
                                            data-choice-idx: "{pt.idx}"
                                            value: "{pt.condition}"
                                            placeholder: ""
                                            spellcheck: "false"
                                            autocorrect: "off"
                                            autocapitalize: "off"
                                            autocomplete: "off"
                                            data-hint?: {pt.cond_error_hint}
                                            disabled?: {is_history_preview}
                                            readonly?: {is_history_preview}
                                    .choice-triple-row.choice-triple-eff .is-empty-rule?: {!pt.has_effects} .is-invalid?: {pt.is_eff_invalid}
                                        data-hint?: {pt.eff_error_hint}
                                        span.choice-row-key "set:"
                                        input.choice-rule-input
                                            type: text
                                            name: "choice_effects_{pt.idx}"
                                            data-choice-idx: "{pt.idx}"
                                            value: "{pt.effects}"
                                            placeholder: ""
                                            spellcheck: "false"
                                            autocorrect: "off"
                                            autocapitalize: "off"
                                            autocomplete: "off"
                                            data-hint?: {pt.eff_error_hint}
                                            disabled?: {is_history_preview}
                                            readonly?: {is_history_preview}
                                {else}
                                    input
                                        type: hidden
                                        name: "choice_cond_{pt.idx}"
                                        value: "{pt.condition}"
                                    input
                                        type: hidden
                                        name: "choice_effects_{pt.idx}"
                                        value: "{pt.effects}"
                            {if not is_history_preview}
                                button.node-icon-btn.node-choice-btn.node-choice-remove
                                    type: button
                                    action: PromptRemoveChoice
                                    data-scene: "{selected_node.id}"
                                    data-choice-idx: "{pt.idx}"
                                    svg.choice-trash-icon src: /icons/trash.svg
                .syn-autocomplete-dropdown#inspector-syn-autocomplete-dropdown .hidden?: {!show_syntax_help_popover}
                        {if show_syntax_help_popover}
                            .syn-dropdown-help-box#syntax-help-popover
                                .syntax-help-section
                                    {for sh in syntax_help_cond_rows}
                                        .syntax-help-row
                                            span.syntax-help-code {sh.code}
                                            span.syntax-help-desc {sh.desc}
                        .syn-dropdown-list
                            {for vsug in book_var_suggestions}
                                button.syn-dropdown-item
                                    type: button
                                    data-syn-kind: "var"
                                    data-syn-var-name: "{vsug.name}"
                                    span.syn-dropdown-var-name {vsug.name}
                                    span.syn-dropdown-var-meta {vsug.kind}
                            {for esug in book_enum_suggestions}
                                button.syn-dropdown-item.hidden
                                    type: button
                                    data-syn-kind: "enum_val"
                                    data-syn-enum-var: "{esug.var_name}"
                                    data-syn-var-name: "{esug.value}"
                                    span.syn-dropdown-var-name {esug.value}
                                    span.syn-dropdown-var-meta {esug.meta}
                            {for tsug in book_target_suggestions}
                                button.syn-dropdown-item.hidden
                                    type: button
                                    data-syn-kind: "target"
                                    data-syn-var-name: "{tsug.value}"
                                    span.syn-dropdown-var-name {tsug.value}
                                    span.syn-dropdown-var-meta {tsug.meta}
                {if not is_history_preview}
                    button.btn.btn-sm.node-add-choice-btn
                        type: button
                        action: AddChoice
                        data-scene: "{selected_node.id}"
                        "{t_add_choice_btn}"

            {if not is_history_preview}
                .context-danger-zone
                    button.btn.btn-sm.context-delete-btn
                        type: button
                        action: PromptDeleteScene
                        data-scene: "{selected_node.id}"
                        svg.btn-icon src: /icons/trash.svg
                        span {t_delete_scene_btn}

    {if selected_chapter}
        form.editor-chapter-form#editor-chapter-form .hidden?: {!has_selected_chapter}
            action: SaveChapterSettings
            data-context-chapter: "{selected_chapter.name}"
            input
                type: hidden
                name: old_chapter
                value: "{selected_chapter.name}"
            input
                type: hidden
                name: chapter
                value: "{selected_chapter.name}"
            .history-panel-header
                {if is_screenplay_mode}
                    span.history-panel-title {t_act_settings_title}
                {else}
                    span.history-panel-title {t_chapter_settings_title}
            .history-panel-body
                .context-panel-section
                    input.node-inline-title.chapter-group-title-input
                        type: text
                        name: new_chapter
                        value: "{selected_chapter.name}"
                        data-old-chapter: "{selected_chapter.name}"
                        placeholder: "{t_chapter_name_placeholder}"
                        disabled?: {is_history_preview}
                        readonly?: {is_history_preview}
                .context-panel-section
                    .node-meta-fields
                        .node-meta-row
                            span.node-meta-key {t_entry_key}
                            .custom-select.node-media-select.chapter-start-scene-select
                                input.custom-select-input
                                    type: hidden
                                    name: start_scene
                                    value: "{selected_chapter.start_scene_id}"
                                button.custom-select-trigger
                                    type: button
                                    data-select-toggle: "true"
                                    disabled?: {is_history_preview}
                                    span.custom-select-label {selected_chapter.start_scene_label}
                                    svg.custom-select-caret src: /icons/chevron-down.svg
                                .custom-select-menu.hidden
                                    {for ch_st_opt in selected_chapter.start_scene_options}
                                        button.custom-select-option .active?: {ch_st_opt.is_active}
                                            type: button
                                            data-select-value: "{ch_st_opt.value}"
                                            data-select-label: "{ch_st_opt.label}"
                                            {ch_st_opt.label}
                .context-panel-section
                    .breakdown-metrics-grid
                        .breakdown-metric-cell
                            span.breakdown-metric-label {t_metric_scenes}
                            span.breakdown-metric-value {selected_chapter.scene_count}
                        .breakdown-metric-cell
                            span.breakdown-metric-label {t_metric_length}
                            span.breakdown-metric-value {selected_chapter.length_stat}
                        .breakdown-metric-cell
                            span.breakdown-metric-label {t_metric_branches}
                            span.breakdown-metric-value {selected_chapter.branch_count}
                        .breakdown-metric-cell
                            span.breakdown-metric-label {t_metric_exits}
                            span.breakdown-metric-value {selected_chapter.exit_count}

    {if has_editor_book}
        form.editor-book-form#editor-book-form .hidden?: {!book_settings_open}
            action: SaveBookSettings
            .history-panel-header
                span.history-panel-title {editor_settings_hint}
                button.node-close-btn
                    type: button
                    action: CloseBookSettings
                    svg.node-close-icon src: /icons/close.svg
            .history-panel-body
                .context-panel-section
                    input.node-inline-title#book-settings-title-input
                        type: text
                        name: title
                        value: "{editor_book_title}"
                        data-old-title: "{editor_book_title}"
                        placeholder: "{editor_title_placeholder}"
                        disabled?: {is_history_preview}
                        readonly?: {is_history_preview}
                .context-panel-section
                    .node-meta-fields
                        {if is_screenplay_mode}
                            .node-meta-row.node-meta-row-radio
                                span.node-meta-key {t_standard_key}
                                .form-radio-group
                                    {for std_opt in script_standard_options}
                                        label.form-radio-option .active?: {std_opt.is_active} .is-disabled?: {is_history_preview}
                                            data-hint: "{std_opt.label}"
                                            input.form-radio-input
                                                type: radio
                                                name: standard
                                                value: "{std_opt.value}"
                                                checked?: {std_opt.is_active}
                                                disabled?: {is_history_preview}
                                            span.form-radio-label {std_opt.label}
                                            span.form-radio-sq
                        .node-meta-row
                            {if is_screenplay_mode}
                                span.node-meta-key {t_act_key}
                            {else}
                                span.node-meta-key {t_chapter_key}
                            .custom-select.node-media-select.book-start-chapter-select#book-start-chapter-select
                                input.custom-select-input
                                    type: hidden
                                    name: start_chapter
                                    value: "{book_start_chapter_name}"
                                button.custom-select-trigger
                                    type: button
                                    data-select-toggle: "true"
                                    disabled?: {is_history_preview}
                                    span.custom-select-label {book_start_chapter_name}
                                    svg.custom-select-caret src: /icons/chevron-down.svg
                                .custom-select-menu.hidden
                                    {for bk_ch_opt in book_start_chapter_options}
                                        button.custom-select-option .active?: {bk_ch_opt.is_active}
                                            type: button
                                            data-select-value: "{bk_ch_opt.value}"
                                            data-select-label: "{bk_ch_opt.label}"
                                            {bk_ch_opt.label}
                        .node-meta-row
                            span.node-meta-key "start:"
                            .custom-select.node-media-select.book-start-scene-select#book-start-scene-select
                                input.custom-select-input
                                    type: hidden
                                    name: start_scene
                                    value: "{book_start_scene_id}"
                                button.custom-select-trigger
                                    type: button
                                    data-select-toggle: "true"
                                    disabled?: {is_history_preview}
                                    span.custom-select-label {book_start_scene_label}
                                    svg.custom-select-caret src: /icons/chevron-down.svg
                                .custom-select-menu.hidden
                                    {for bk_st_opt in book_start_scene_options}
                                        button.custom-select-option .active?: {bk_st_opt.is_active}
                                            type: button
                                            data-select-value: "{bk_st_opt.value}"
                                            data-select-label: "{bk_st_opt.label}"
                                            {bk_st_opt.label}
                        {if not is_screenplay_mode}
                            .node-meta-row
                                span.node-meta-key {t_narration_key}
                                button.book-setting-toggle-btn .active?: {book_narrated} .is-disabled?: {is_history_preview}
                                    type: button
                                    action: ToggleBookNarrated
                                    data-hint: "{book_narrated_label}"
                                    disabled?: {is_history_preview}
                                    span.toggle-right
                                        span.toggle-switch
                                            span.toggle-thumb
                                        {if book_narrated}
                                            svg.action-icon src: /icons/mic.svg
                                        {else}
                                            svg.action-icon src: /icons/mic-off.svg
                                    span.custom-select-label {book_narrated_short_label}
                            .node-meta-row
                                span.node-meta-key {t_images_key}
                                button.book-setting-toggle-btn .active?: {!book_imageless} .is-disabled?: {is_history_preview}
                                    type: button
                                    action: ToggleBookImageless
                                    data-hint: "{book_imageless_label}"
                                    disabled?: {is_history_preview}
                                    span.toggle-right
                                        span.toggle-switch
                                            span.toggle-thumb
                                        {if book_imageless}
                                            svg.action-icon src: /icons/image-off.svg
                                        {else}
                                            svg.action-icon src: /icons/image.svg
                                    span.custom-select-label {book_imageless_short_label}
                        .node-meta-row
                            span.node-meta-key {t_author_key}
                            input.node-meta-input
                                type: text
                                name: author
                                value: "{active_book_author}"
                                placeholder: "{editor_author_placeholder}"
                                disabled?: {is_history_preview}
                                readonly?: {is_history_preview}
                        .node-meta-row
                            span.node-meta-key {t_license_key}
                            input.node-meta-input#book-settings-license-input
                                type: text
                                name: license
                                value: "{active_book_license}"
                                placeholder: "{t_license_placeholder}"
                                disabled?: {is_history_preview}
                                readonly?: {is_history_preview}
                        .node-meta-row.node-meta-row-textarea
                            span.node-meta-key {t_desc_key}
                            textarea.node-meta-input.node-meta-textarea
                                name: description
                                rows: 1
                                placeholder: "{editor_description_placeholder}"
                                disabled?: {is_history_preview}
                                readonly?: {is_history_preview}
                                {active_book_description}
                        .node-meta-row
                            span.node-meta-key {t_folder_dir_key}
                            .node-media-picker-row
                                input.node-meta-input
                                    type: text
                                    name: book_save_dir
                                    value: "{active_book_save_dir}"
                                    placeholder: "{default_new_book_dir}"
                                    readonly: true
                                {if not is_history_preview}
                                    button.node-icon-btn.node-folder-btn
                                        type: button
                                        action: BrowseBookSaveFolder
                                        data-hint: "{t_change_project_dir_hint}"
                                        svg.folder-browse-icon src: /icons/folder.svg
                        .node-meta-row
                            span.node-meta-key {t_export_key}
                            .node-media-picker-row
                                input.node-meta-input
                                    type: text
                                    name: export_dir
                                    value: "{active_book_export_dir}"
                                    placeholder: "{default_export_dir}"
                                    disabled?: {is_history_preview}
                                    readonly?: {is_history_preview}
                                {if not is_history_preview}
                                    button.node-icon-btn.node-folder-btn
                                        type: button
                                        action: BrowseBookExportFolder
                                        data-hint: "{t_change_export_dir_hint}"
                                        svg.folder-browse-icon src: /icons/folder.svg

    .editor-multi-panel.hidden#editor-multi-panel
        .context-panel-section
            label.node-micro-label#multi-select-count-label {t_selected_scenes_label}
            .context-multi-scene-list#context-multi-scene-list
                {for ms in multi_scene_items}
                    .context-multi-scene-item.hidden
                        data-multi-scene-id: "{ms.id}"
                        button.context-multi-scene-select-btn
                            type: button
                            data-multi-select-scene: "{ms.id}"
                            span.context-multi-scene-title {ms.title}
                            span.context-multi-scene-meta {ms.chapter}
                        button.node-icon-btn.context-multi-scene-deselect-btn
                            type: button
                            data-multi-deselect-scene: "{ms.id}"
                            "✕"
        {if not is_history_preview}
            .context-danger-zone
                button.btn.btn-sm.context-delete-btn#btn-delete-selected-scenes
                    type: button
                    action: PromptDeleteScenes
                    data-scenes: ""
                    svg.btn-icon src: /icons/trash.svg
                    span#btn-delete-selected-scenes-label {t_delete_selected_scenes}
