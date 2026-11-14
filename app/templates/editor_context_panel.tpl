aside.editor-context-panel#editor-context-panel .hidden?: {IsContextPanelHidden}
    {if HasSelectedNode}
        form.editor-context-form#editor-context-form .hidden?: {IsNodeInspectorHidden}
            data-context-scene-id: "{SelectedNode.ID}"
            data-book-vars: "{BookVarsSpec}"
            input
                type: hidden
                name: scene_id
                value: "{SelectedNode.ID}"
            input
                type: hidden
                name: source_file
                value: "{SelectedNode.SourceFile}"
            input
                type: hidden
                name: choice_count
                value: "{SelectedNode.PortCount}"

            .context-panel-section
                .custom-select.node-chapter-select
                    data-select-root: "true"
                    .node-chapter-input-wrap
                        input.node-inline-chapter
                            type: text
                            name: chapter
                            value: "{SelectedNode.Chapter}"
                            placeholder: "{t_chapter_placeholder}"
                            disabled?: {IsHistoryPreview}
                            readonly?: {IsHistoryPreview}
                        {if not IsHistoryPreview}
                            button.node-chapter-caret-btn
                                data-select-toggle: "true"
                                svg.custom-select-caret src: /icons/chevron-down.svg
                    .custom-select-menu.hidden
                        data-select-menu: "true"
                        {for ch_opt in SelectedNode.ChapterOptions}
                            button.custom-select-option .active?: {ch_opt.IsActive}
                                data-select-value: "{ch_opt.Value}"
                                data-select-label: "{ch_opt.Label}"
                                {ch_opt.Label}

            .context-panel-section
                .node-title-row
                    {if SelectedNode.IsStartScene}
                        span.node-start-indicator
                            data-hint: "{t_start_scene_badge_hint}"
                            svg.scene-star-icon src: /icons/star.svg
                    {elif SelectedNode.IsChapterEntry}
                        span.node-start-indicator.node-chapter-entry-indicator
                            data-hint: "{t_chapter_entry_hint}"
                            svg.scene-star-icon src: /icons/star.svg
                    input.node-inline-title
                        type: text
                        name: name
                        value: "{SelectedNode.Name}"
                        placeholder: "{t_scene_name_opt_placeholder}"
                        disabled?: {IsHistoryPreview}
                        readonly?: {IsHistoryPreview}

            .context-panel-section
                .node-meta-fields
                    .node-meta-row
                        span.node-meta-key "scene:"
                        input.node-meta-input#context-scene-id-input
                            type: text
                            name: explicit_id
                            value: "{SelectedNode.ExplicitID}"
                            placeholder: "{t_scene_id_placeholder}"
                            spellcheck: "false"
                            autocorrect: "off"
                            autocapitalize: "off"
                            autocomplete: "off"
                            disabled?: {IsHistoryPreview}
                            readonly?: {IsHistoryPreview}
                    {if HasBookVariables}
                        .node-meta-row.node-meta-row-effects .is-empty-rule?: {!SelectedNode.HasEffects}
                            span.node-meta-key "set:"
                            .syn-input-wrap .is-invalid?: {SelectedNode.IsEffectsInvalid}
                                data-hint?: {SelectedNode.EffectsErrorHint}
                                input.node-meta-input.syn-inner-input
                                    type: text
                                    name: scene_effects
                                    value: "{SelectedNode.Effects}"
                                    placeholder: "odkryta_prawda = true; sumienie++"
                                    spellcheck: "false"
                                    autocorrect: "off"
                                    autocapitalize: "off"
                                    autocomplete: "off"
                                    data-hint?: {SelectedNode.EffectsErrorHint}
                                    disabled?: {IsHistoryPreview}
                                    readonly?: {IsHistoryPreview}
                    {if IsScreenplayMode}
                        .node-meta-row
                            span.node-meta-key {t_sp_scene_key}
                            input.node-meta-input.node-slugline-input
                                type: text
                                name: slugline
                                value: "{SelectedNode.Slugline}"
                                placeholder: "{t_slugline_placeholder}"
                                disabled?: {IsHistoryPreview}
                                readonly?: {IsHistoryPreview}
                            {if SelectedNode.ScriptLengthLabel}
                                span.node-script-length-badge {SelectedNode.ScriptLengthLabel}
                        {if not IsHistoryPreview}
                            .slugline-helper-pills
                                {for p in SluglinePrefixPills}
                                    button.slugline-pill-btn
                                        data-scene: "{SelectedNode.ID}"
                                        data-prefix: "{p}"
                                        {p}
                                {for t in SluglineTimePills}
                                    button.slugline-pill-btn
                                        data-scene: "{SelectedNode.ID}"
                                        data-time: "{t}"
                                        {t}
                        .node-meta-row.node-meta-row-textarea
                            span.node-meta-key {t_desc_key}
                            textarea.node-meta-input.node-meta-textarea
                                name: synopsis
                                rows: 1
                                placeholder: "{t_synopsis_placeholder}"
                                disabled?: {IsHistoryPreview}
                                readonly?: {IsHistoryPreview}
                                {SelectedNode.Synopsis}
                    {if not IsScreenplayMode}
                        .node-meta-row
                            span.node-meta-key "image:"
                            .custom-select.node-media-select.node-image-select
                                data-select-root: "true"
                                button.custom-select-trigger
                                    data-select-toggle: "true"
                                    disabled?: {IsHistoryPreview}
                                    span.custom-select-label {SelectedNode.CurrentImageLabel}
                                    svg.custom-select-caret src: /icons/chevron-down.svg
                                .custom-select-menu.hidden
                                    data-select-menu: "true"
                                    {for img_opt in SelectedNode.ImageOptions}
                                        button.custom-select-option .active?: {img_opt.IsActive}
                                            data-select-value: "{img_opt.Value}"
                                            data-select-label: "{img_opt.Label}"
                                            {img_opt.Label}
                            {if not IsHistoryPreview}
                                button.node-icon-btn.node-media-add-btn
                                    data-kind: image
                                    data-scene: "{SelectedNode.ID}"
                                    svg.media-plus-icon src: /icons/plus.svg
                    {if BookNarrated}
                        {if not IsHistoryPreview}
                            {if SelectedNode.HasVoiceSections}
                                .voice-rec-sections.inspector-voice-rec-sections .hidden?: {!VoiceRecorderOpen}
                                    .voice-rec-sections-tabs
                                        {for vsec in SelectedNode.VoiceSections}
                                            button.voice-rec-section-btn .active?: {vsec.IsActive} .is-recorded?: {vsec.HasAudio}
                                                data-scene: "{SelectedNode.ID}"
                                                data-voice-section: "{vsec.Key}"
                                                span.voice-rec-section-dot .is-recorded?: {vsec.HasAudio}
                                                span.voice-rec-section-label {vsec.InspectorLabel}
                        .node-meta-row
                            span.node-meta-key "voice:"
                            .custom-select.node-media-select.node-voice-select
                                data-select-root: "true"
                                button.custom-select-trigger
                                    data-select-toggle: "true"
                                    disabled?: {IsHistoryPreview}
                                    span.custom-select-label {SelectedNode.CurrentVoiceLabel}
                                    svg.custom-select-caret src: /icons/chevron-down.svg
                                .custom-select-menu.hidden
                                    data-select-menu: "true"
                                    {for voice_opt in SelectedNode.VoiceOptions}
                                        button.custom-select-option .active?: {voice_opt.IsActive}
                                            data-select-value: "{voice_opt.Value}"
                                            data-select-label: "{voice_opt.Label}"
                                            {voice_opt.Label}
                            {if not IsHistoryPreview}
                                button.node-icon-btn.node-media-add-btn
                                    data-kind: audio
                                    data-scene: "{SelectedNode.ID}"
                                    svg.media-plus-icon src: /icons/plus.svg
                                button.node-icon-btn.node-media-add-btn.icon-toggle-btn.voice-rec-toggle-btn#btn-inspector-voice-recorder .active?: {VoiceRecorderOpen}
                                    data-scene: "{SelectedNode.ID}"
                                    svg.media-plus-icon src: /icons/mic.svg
                        {if not IsHistoryPreview}
                            .voice-rec-strip#inspector-voice-rec-strip .hidden?: {!VoiceRecorderOpen}
                                data-voice-strip: "true"
                                data-voice-open: "{VoiceRecorderOpen}"
                                data-scene: "{SelectedNode.ID}"
                                data-voice-section: "{SelectedNode.ActiveVoiceSectionKey}"
                                data-voice-url: "{SelectedNode.CurrentVoiceURL}"
                                data-voice-file: "{SelectedNode.CurrentVoice}"
                                data-trim-start: "{SelectedNode.VoiceTrimStart}"
                                data-trim-end: "{SelectedNode.VoiceTrimEnd}"
                                button.voice-rec-transport-btn
                                    data-voice-transport: "true"
                                    svg.voice-rec-icon.voice-rec-icon-play .hidden?: {!SelectedNode.HasVoiceFile} data-voice-icon: "play" src: /icons/voice-play.svg
                                    svg.voice-rec-icon.voice-rec-icon-mic .hidden?: {SelectedNode.HasVoiceFile} data-voice-icon: "mic" src: /icons/mic.svg
                                    svg.voice-rec-icon.voice-rec-icon-pause.hidden data-voice-icon: "pause" src: /icons/voice-pause.svg
                                    svg.voice-rec-icon.voice-rec-icon-stop.hidden data-voice-icon: "stop" src: /icons/voice-stop.svg
                                .voice-rec-waveform-wrap
                                    data-voice-waveform-wrap: "true"
                                    canvas.voice-rec-waveform-canvas
                                        data-voice-canvas: "true"
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
                                    {SelectedNode.VoiceDurationLabel}
                                button.voice-rec-action-btn.voice-rec-cut-btn .is-disabled?: {!SelectedNode.HasVoiceTrim}
                                    data-voice-trim-destructive: "true"
                                    data-scene: "{SelectedNode.ID}"
                                    data-voice-section: "{SelectedNode.ActiveVoiceSectionKey}"
                                    data-hint: "{t_voice_trim_destructive_hint}"
                                    disabled?: {!SelectedNode.HasVoiceTrim}
                                    svg.voice-rec-btn-icon src: /icons/scissors.svg
                                button.voice-rec-action-btn.voice-rec-rerecord-btn .hidden?: {!SelectedNode.HasVoiceFile}
                                    data-voice-record-start: "true"
                                    svg.voice-rec-btn-icon src: /icons/mic.svg
                                button.voice-rec-action-btn.voice-rec-delete-btn .hidden?: {!SelectedNode.HasVoiceFile}
                                    data-voice-delete: "true"
                                    data-scene: "{SelectedNode.ID}"
                                    data-voice-section: "{SelectedNode.ActiveVoiceSectionKey}"
                                    svg.voice-rec-btn-icon src: /icons/trash.svg

            .context-panel-section
                button.btn.btn-sm.node-edit-text-btn.btn-open-scene-text
                    data-scene: "{SelectedNode.ID}"
                    data-hint: "{t_open_scene_text_hint}"
                    {if IsHistoryPreview}
                        "{t_preview_content_btn}"
                    {else}
                        "{t_edit_content_btn}"
                button.btn.btn-sm.node-edit-text-btn.btn-play-from-scene
                    data-scene: "{SelectedNode.ID}"
                    data-hint: "{t_play_from_scene_hint}"
                    "{t_play_from_scene_btn}"

            {if SelectedNode.HasNodeWarnings}
                ul.node-warnings-list
                    {for nw in SelectedNode.NodeWarnings}
                        li {nw}

            .context-panel-section.context-choices-section
                .context-choices-list
                    {for pt in SelectedNode.InspectorPorts}
                        .context-choice-item .is-rule-expanded?: {pt.IsRuleOpen} .has-active-rule?: {pt.HasRule}
                            {if not IsHistoryPreview}
                                span.choice-drag-grip
                                    data-choice-grip: "true"
                                    data-choice-idx: "{pt.Idx}"
                                    svg.choice-grip-icon src: /icons/grip.svg
                            .choice-triple-stack
                                .choice-triple-row.choice-triple-exit .is-invalid?: {pt.IsExitInvalid}
                                    data-hint?: {pt.ExitErrorHint}
                                    span.choice-row-key "exit:"
                                    input.choice-exit-input
                                        type: text
                                        name: "choice_target_{pt.Idx}"
                                        data-choice-idx: "{pt.Idx}"
                                        value: "{pt.SelectedValue}"
                                        placeholder: "{t_exit_target_placeholder}"
                                        spellcheck: "false"
                                        autocorrect: "off"
                                        autocapitalize: "off"
                                        autocomplete: "off"
                                        data-hint?: {pt.ExitErrorHint}
                                        disabled?: {IsHistoryPreview}
                                        readonly?: {IsHistoryPreview}
                                .choice-triple-row.choice-triple-label
                                    span.choice-row-key "text:"
                                    textarea.node-meta-input.node-meta-textarea.node-choice-label
                                        name: "choice_label_{pt.Idx}"
                                        data-choice-idx: "{pt.Idx}"
                                        rows: 1
                                        placeholder: "{t_choice_text_placeholder}"
                                        disabled?: {IsHistoryPreview}
                                        readonly?: {IsHistoryPreview}
                                        {pt.RawLabel}
                                {if pt.IsRuleOpen}
                                    .choice-triple-row.choice-triple-cond .is-empty-rule?: {!pt.HasCondition} .is-invalid?: {pt.IsCondInvalid}
                                        data-hint?: {pt.CondErrorHint}
                                        span.choice-row-key "if:"
                                        input.choice-rule-input
                                            type: text
                                            name: "choice_cond_{pt.Idx}"
                                            data-choice-idx: "{pt.Idx}"
                                            value: "{pt.Condition}"
                                            placeholder: ""
                                            spellcheck: "false"
                                            autocorrect: "off"
                                            autocapitalize: "off"
                                            autocomplete: "off"
                                            data-hint?: {pt.CondErrorHint}
                                            disabled?: {IsHistoryPreview}
                                            readonly?: {IsHistoryPreview}
                                    .choice-triple-row.choice-triple-eff .is-empty-rule?: {!pt.HasEffects} .is-invalid?: {pt.IsEffInvalid}
                                        data-hint?: {pt.EffErrorHint}
                                        span.choice-row-key "set:"
                                        input.choice-rule-input
                                            type: text
                                            name: "choice_effects_{pt.Idx}"
                                            data-choice-idx: "{pt.Idx}"
                                            value: "{pt.Effects}"
                                            placeholder: ""
                                            spellcheck: "false"
                                            autocorrect: "off"
                                            autocapitalize: "off"
                                            autocomplete: "off"
                                            data-hint?: {pt.EffErrorHint}
                                            disabled?: {IsHistoryPreview}
                                            readonly?: {IsHistoryPreview}
                                {else}
                                    input
                                        type: hidden
                                        name: "choice_cond_{pt.Idx}"
                                        value: "{pt.Condition}"
                                    input
                                        type: hidden
                                        name: "choice_effects_{pt.Idx}"
                                        value: "{pt.Effects}"
                            {if not IsHistoryPreview}
                                button.node-icon-btn.node-choice-btn.node-choice-remove
                                    data-scene: "{SelectedNode.ID}"
                                    data-choice-idx: "{pt.Idx}"
                                    svg.choice-trash-icon src: /icons/trash.svg
                .syn-autocomplete-dropdown#inspector-syn-autocomplete-dropdown .hidden?: {!ShowSyntaxHelpPopover}
                        {if ShowSyntaxHelpPopover}
                            .syn-dropdown-help-box#syntax-help-popover
                                .syntax-help-section
                                    {for sh in SyntaxHelpCondRows}
                                        .syntax-help-row
                                            span.syntax-help-code {sh.Value}
                                            span.syntax-help-desc {sh.Label}
                        .syn-dropdown-list
                            {for vsug in BookVarSuggestions}
                                button.syn-dropdown-item
                                    data-syn-kind: "var"
                                    data-syn-var-name: "{vsug.Name}"
                                    span.syn-dropdown-var-name {vsug.Name}
                                    span.syn-dropdown-var-meta {vsug.Kind}
                            {for esug in BookEnumSuggestions}
                                button.syn-dropdown-item.hidden
                                    data-syn-kind: "enum_val"
                                    data-syn-enum-var: "{esug.VarName}"
                                    data-syn-var-name: "{esug.Value}"
                                    span.syn-dropdown-var-name {esug.Value}
                                    span.syn-dropdown-var-meta {esug.Meta}
                            {for tsug in BookTargetSuggestions}
                                button.syn-dropdown-item.hidden
                                    data-syn-kind: "target"
                                    data-syn-var-name: "{tsug.Value}"
                                    span.syn-dropdown-var-name {tsug.Value}
                                    span.syn-dropdown-var-meta {tsug.Meta}
                {if not IsHistoryPreview}
                    button.btn.btn-sm.node-add-choice-btn
                        data-scene: "{SelectedNode.ID}"
                        "{t_add_choice_btn}"

            {if not IsHistoryPreview}
                .context-danger-zone
                    button.btn.btn-sm.context-delete-btn.btn-prompt-delete-scene
                        data-scene: "{SelectedNode.ID}"
                        svg.btn-icon src: /icons/trash.svg
                        span {t_delete_scene_btn}

    {if SelectedChapter}
        form.editor-chapter-form#editor-chapter-form .hidden?: {!HasSelectedChapter}
            data-context-chapter: "{SelectedChapter.Name}"
            input
                type: hidden
                name: old_chapter
                value: "{SelectedChapter.Name}"
            input
                type: hidden
                name: chapter
                value: "{SelectedChapter.Name}"
            .history-panel-header
                {if IsScreenplayMode}
                    span.history-panel-title {t_act_settings_title}
                {else}
                    span.history-panel-title {t_chapter_settings_title}
                button.node-close-btn.btn-close-book-settings
                    svg.node-close-icon src: /icons/close.svg
            .history-panel-body
                .context-panel-section
                    input.node-inline-title.chapter-group-title-input
                        type: text
                        name: new_chapter
                        value: "{SelectedChapter.Name}"
                        data-old-chapter: "{SelectedChapter.Name}"
                        placeholder: "{t_chapter_name_placeholder}"
                        disabled?: {IsHistoryPreview}
                        readonly?: {IsHistoryPreview}
                .context-panel-section
                    .node-meta-fields
                        .node-meta-row
                            span.node-meta-key {t_entry_key}
                            .custom-select.node-media-select.chapter-start-scene-select
                                data-select-root: "true"
                                button.custom-select-trigger
                                    data-select-toggle: "true"
                                    disabled?: {IsHistoryPreview}
                                    span.custom-select-label {SelectedChapter.StartSceneLabel}
                                    svg.custom-select-caret src: /icons/chevron-down.svg
                                .custom-select-menu.hidden
                                    data-select-menu: "true"
                                    {for ch_st_opt in SelectedChapter.StartSceneOptions}
                                        button.custom-select-option .active?: {ch_st_opt.IsActive}
                                            data-select-value: "{ch_st_opt.Value}"
                                            data-select-label: "{ch_st_opt.Label}"
                                            {ch_st_opt.Label}
                .context-panel-section
                    .breakdown-metrics-grid
                        .breakdown-metric-cell
                            span.breakdown-metric-label {t_metric_scenes}
                            span.breakdown-metric-value {SelectedChapter.SceneCount}
                        .breakdown-metric-cell
                            span.breakdown-metric-label {t_metric_length}
                            span.breakdown-metric-value {SelectedChapter.LengthStat}
                        .breakdown-metric-cell
                            span.breakdown-metric-label {t_metric_branches}
                            span.breakdown-metric-value {SelectedChapter.BranchCount}
                        .breakdown-metric-cell
                            span.breakdown-metric-label {t_metric_exits}
                            span.breakdown-metric-value {SelectedChapter.ExitCount}

    {if HasEditorBook}
        form.editor-book-form#editor-book-form .hidden?: {!BookSettingsOpen}
            .history-panel-header
                span.history-panel-title {EditorSettingsHint}
                button.node-close-btn.btn-close-book-settings
                    svg.node-close-icon src: /icons/close.svg
            .history-panel-body
                .context-panel-section
                    input.node-inline-title#book-settings-title-input
                        type: text
                        name: title
                        value: "{Book.Title}"
                        data-old-title: "{Book.Title}"
                        placeholder: "{EditorTitlePlaceholder}"
                        disabled?: {IsHistoryPreview}
                        readonly?: {IsHistoryPreview}
                .context-panel-section
                    .node-meta-fields
                        {if IsScreenplayMode}
                            .node-meta-row.node-meta-row-radio
                                span.node-meta-key {t_standard_key}
                                .form-radio-group
                                    {for std_opt in ScriptStandardOptions}
                                        label.form-radio-option .active?: {std_opt.IsActive} .is-disabled?: {IsHistoryPreview}
                                            data-standard: "{std_opt.Value}"
                                            data-hint: "{std_opt.Label}"
                                            input.form-radio-input
                                                type: radio
                                                name: standard
                                                value: "{std_opt.Value}"
                                                checked?: {std_opt.IsActive}
                                                disabled?: {IsHistoryPreview}
                                            span.form-radio-label {std_opt.Label}
                                            span.form-radio-sq
                        .node-meta-row
                            {if IsScreenplayMode}
                                span.node-meta-key {t_act_key}
                            {else}
                                span.node-meta-key {t_chapter_key}
                            .custom-select.node-media-select.book-start-chapter-select#book-start-chapter-select
                                data-select-root: "true"
                                button.custom-select-trigger
                                    data-select-toggle: "true"
                                    disabled?: {IsHistoryPreview}
                                    span.custom-select-label {BookStartChapterName}
                                    svg.custom-select-caret src: /icons/chevron-down.svg
                                .custom-select-menu.hidden
                                    data-select-menu: "true"
                                    {for bk_ch_opt in BookStartChapterOptions}
                                        button.custom-select-option .active?: {bk_ch_opt.IsActive}
                                            data-select-value: "{bk_ch_opt.Value}"
                                            data-select-label: "{bk_ch_opt.Label}"
                                            {bk_ch_opt.Label}
                        .node-meta-row
                            span.node-meta-key "start:"
                            .custom-select.node-media-select.book-start-scene-select#book-start-scene-select
                                data-select-root: "true"
                                data-select-name: "start_scene"
                                button.custom-select-trigger
                                    data-select-toggle: "true"
                                    disabled?: {IsHistoryPreview}
                                    span.custom-select-label {BookStartSceneLabel}
                                    svg.custom-select-caret src: /icons/chevron-down.svg
                                .custom-select-menu.hidden
                                    data-select-menu: "true"
                                    {for bk_st_opt in BookStartSceneOptions}
                                        button.custom-select-option .active?: {bk_st_opt.IsActive}
                                            data-select-value: "{bk_st_opt.Value}"
                                            data-select-label: "{bk_st_opt.Label}"
                                            {bk_st_opt.Label}
                        {if not IsScreenplayMode}
                            .node-meta-row
                                span.node-meta-key {t_narration_key}
                                button.book-setting-toggle-btn.btn-toggle-book-narrated .active?: {BookNarrated} .is-disabled?: {IsHistoryPreview}
                                    data-hint: "{BookNarratedLabel}"
                                    disabled?: {IsHistoryPreview}
                                    span.toggle-right
                                        span.toggle-switch
                                            span.toggle-thumb
                                        {if BookNarrated}
                                            svg.action-icon src: /icons/mic.svg
                                        {else}
                                            svg.action-icon src: /icons/mic-off.svg
                                    span.custom-select-label {BookNarratedShortLabel}
                            .node-meta-row
                                span.node-meta-key {t_images_key}
                                button.book-setting-toggle-btn.btn-toggle-book-imageless .active?: {!BookImageless} .is-disabled?: {IsHistoryPreview}
                                    data-hint: "{BookImagelessLabel}"
                                    disabled?: {IsHistoryPreview}
                                    span.toggle-right
                                        span.toggle-switch
                                            span.toggle-thumb
                                        {if BookImageless}
                                            svg.action-icon src: /icons/image-off.svg
                                        {else}
                                            svg.action-icon src: /icons/image.svg
                                    span.custom-select-label {BookImagelessShortLabel}
                        .node-meta-row
                            span.node-meta-key {t_author_key}
                            input.node-meta-input
                                type: text
                                name: author
                                value: "{Book.Author}"
                                placeholder: "{EditorAuthorPlaceholder}"
                                disabled?: {IsHistoryPreview}
                                readonly?: {IsHistoryPreview}
                        .node-meta-row
                            span.node-meta-key {t_license_key}
                            input.node-meta-input#book-settings-license-input
                                type: text
                                name: license
                                value: "{Book.License}"
                                placeholder: "{t_license_placeholder}"
                                disabled?: {IsHistoryPreview}
                                readonly?: {IsHistoryPreview}
                        .node-meta-row.node-meta-row-textarea
                            span.node-meta-key {t_desc_key}
                            textarea.node-meta-input.node-meta-textarea
                                name: description
                                rows: 1
                                placeholder: "{EditorDescriptionPlaceholder}"
                                disabled?: {IsHistoryPreview}
                                readonly?: {IsHistoryPreview}
                                {Book.Description}
                        .node-meta-row
                            span.node-meta-key {t_folder_dir_key}
                            .node-media-picker-row
                                input.node-meta-input
                                    type: text
                                    name: book_save_dir
                                    value: "{Book.RootPath}"
                                    placeholder: "{DefaultNewBookDir}"
                                    readonly: true
                                {if not IsHistoryPreview}
                                    button.node-icon-btn.node-folder-btn.btn-browse-book-save-folder
                                        data-hint: "{t_change_project_dir_hint}"
                                        svg.folder-browse-icon src: /icons/folder.svg
                        .node-meta-row
                            span.node-meta-key {t_export_key}
                            .node-media-picker-row
                                input.node-meta-input
                                    type: text
                                    name: export_dir
                                    value: "{Book.ExportDir}"
                                    placeholder: "{DefaultExportDir}"
                                    disabled?: {IsHistoryPreview}
                                    readonly?: {IsHistoryPreview}
                                {if not IsHistoryPreview}
                                    button.node-icon-btn.node-folder-btn.btn-browse-book-export-folder
                                        data-hint: "{t_change_export_dir_hint}"
                                        svg.folder-browse-icon src: /icons/folder.svg

    .editor-multi-panel.hidden#editor-multi-panel
        .context-panel-section
            label.node-micro-label#multi-select-count-label {t_selected_scenes_label}
            .context-multi-scene-list#context-multi-scene-list
                {for ms in MultiSceneItems}
                    .context-multi-scene-item.hidden
                        data-multi-scene-id: "{ms.Value}"
                        button.context-multi-scene-select-btn
                            data-multi-select-scene: "{ms.Value}"
                            span.context-multi-scene-title {ms.Label}
                            span.context-multi-scene-meta {ms.Meta}
                        button.node-icon-btn.context-multi-scene-deselect-btn
                            data-multi-deselect-scene: "{ms.Value}"
                            "✕"
        {if not IsHistoryPreview}
            .context-danger-zone
                button.btn.btn-sm.context-delete-btn#btn-delete-selected-scenes
                    data-scenes: ""
                    svg.btn-icon src: /icons/trash.svg
                    span#btn-delete-selected-scenes-label {t_delete_selected_scenes}
