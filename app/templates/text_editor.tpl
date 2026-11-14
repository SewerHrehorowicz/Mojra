{if ShowSceneTextModal}
    .modal-backdrop.scene-text-fullscreen-backdrop#modal-scene-text .mode-half?: {SceneTextIsHalfWidth} .mode-full?: {SceneTextIsFullWidth} .has-right-panel?: {SceneTextHasRightPanel}
        .player-layout.scene-text-book-layout .is-imageless?: {SceneTextModalIsImageless}
            .player-bg-stage
                {if SceneTextModalHasImage}
                    img.player-bg-img
                        alt: "{SceneTextModalName}"
                        src: {SceneTextModalImageURL}
            .story-scroll-feed.scene-text-editor-feed
                .reader-global-meta
                    span.scene-chapter-pill {SceneTextModalHeaderMeta}
                    .scene-text-editor-top-bar
                        button.scene-text-back-btn.btn-play-from-scene
                            data-scene: "{SceneTextModalID}"
                            svg.btn-icon src: /icons/play.svg
                        {if not IsHistoryPreview}
                            button.scene-text-back-btn.scene-text-delete-btn.btn-prompt-delete-scene
                                data-scene: "{SceneTextModalID}"
                                svg.btn-icon src: /icons/trash.svg
                        button.scene-text-back-btn#btn-collapse-scene-text
                            svg.btn-icon src: /icons/chevrons-left.svg
                        button.scene-text-back-btn#btn-expand-scene-text-full .hidden?: {SceneTextIsFullWidth}
                            data-mode: full
                            svg.btn-icon src: /icons/chevrons-right.svg
                .active-scene-stage.scene-text-editor-stage
                    form.scene-card.scene-focused.scene-text-editor-card#form-scene-text .has-illustration?: {SceneTextModalHasImage}
                        data-book-vars: "{BookVarsSpec}"
                        data-test-vars: "{SceneTestVarsSpec}"
                        input
                            type: hidden
                            name: scene_id
                            value: "{SceneTextModalID}"
                        input#scene-settings-open-input
                            type: hidden
                            name: scene_settings_open
                            value: "{SceneTextModalSettingsOpenVal}"
                        {if SceneTextModalHasIncoming}
                            .scene-incoming-list
                                {for inc in SceneTextModalIncoming}
                                    button.choice-btn.incoming-choice-btn
                                        data-scene: "{inc.Value}"
                                        span.incoming-choice-content
                                            span.incoming-scene-name {inc.Label}
                                            {if inc.IsActive}
                                                span.incoming-choice-sep "—"
                                                span.incoming-choice-label {inc.Meta}
                                        span.choice-arrow.incoming-choice-arrow "↑"
                        .scene-header-block
                            .scene-title-row
                                .scene-title-with-badge
                                    span.present-scene-badge
                                        svg.present-scene-icon src: /icons/asterisk.svg
                                    {if SceneTextModalIsStart}
                                        span.node-start-indicator
                                            data-hint: "{t_start_scene_root_hint}"
                                            svg.scene-star-icon src: /icons/star.svg
                                    input.scene-title.scene-title-present.node-inline-title
                                        type: text
                                        name: name
                                        value: "{SceneTextModalRawName}"
                                        placeholder: "{t_scene_name_opt_placeholder}"
                                        disabled?: {IsHistoryPreview}
                                        readonly?: {IsHistoryPreview}
                                {if IsHistoryPreview}
                                    span.history-step-badge {t_preview_readonly_badge}
                                {else}
                                    .scene-title-right-controls
                                        {if HasBookVariables}
                                            button.contextual-rewind-btn.scene-preview-mode-btn.icon-toggle-btn#btn-scene-preview-output .active?: {!IsScenePreviewNormal}
                                                svg.scene-settings-icon src: /icons/preview-output.svg
                                            button.contextual-rewind-btn.scene-preview-mode-btn.icon-toggle-btn#btn-scene-test-vars .active?: {ShowSceneTestVarsBar}
                                                svg.scene-settings-icon src: /icons/braces.svg
                                        {if SceneTextModalBookNarrated}
                                            button.contextual-rewind-btn.icon-toggle-btn.voice-rec-toggle-btn#btn-modal-voice-recorder .active?: {VoiceRecorderOpen}
                                                data-scene: "{SceneTextModalID}"
                                                svg.scene-settings-icon src: /icons/mic.svg
                                        button.contextual-rewind-btn.scene-settings-toggle-btn.icon-toggle-btn .active?: {SceneTextModalSettingsOpen}
                                            data-scene-settings-toggle: "true"
                                            data-hint: "{t_scene_settings_hint}"
                                            svg.scene-settings-icon src: /icons/settings.svg
                            {if ShowSceneTestVarsBar}
                                .scene-test-vars-popover#scene-test-vars-bar
                                    .history-panel-header.scene-test-vars-header
                                        span.history-panel-title {t_test_state_title}
                                        button.node-close-btn.btn-close-test-vars
                                            svg.node-close-icon src: /icons/close.svg
                                    {if HasSceneTestVars}
                                        .scene-test-vars-items
                                            {for tv in SceneTestVars}
                                                .node-meta-row.test-var-row
                                                    span.node-meta-key.test-var-key "{tv.Name}:"
                                                    {if tv.IsBool}
                                                        button.book-setting-toggle-btn.test-var-bool-btn .active?: {tv.BoolVal}
                                                            data-var-name: "{tv.Name}"
                                                            data-toggle: "true"
                                                            span.toggle-right
                                                                span.toggle-switch
                                                                    span.toggle-thumb
                                                            span.custom-select-label {tv.Value}
                                                    {elif tv.IsInt}
                                                        input.node-meta-input.test-var-int-input
                                                            type: text
                                                            name: var_value
                                                            inputmode: numeric
                                                            value: "{tv.Value}"
                                                            data-test-var-input: "{tv.Name}"
                                                            spellcheck: "false"
                                                            autocorrect: "off"
                                                            autocapitalize: "off"
                                                            autocomplete: "off"
                                                    {elif tv.IsEnum}
                                                        .custom-select.node-media-select.test-var-enum-select
                                                            data-select-root: "true"
                                                            data-var-name: "{tv.Name}"
                                                            button.custom-select-trigger
                                                                data-select-toggle: "true"
                                                                span.custom-select-label {tv.Value}
                                                                svg.custom-select-caret src: /icons/chevron-down.svg
                                                            .custom-select-menu.hidden
                                                                data-select-menu: "true"
                                                                {for eopt in tv.EnumOptions}
                                                                    button.custom-select-option .active?: {eopt.IsActive}
                                                                        data-select-value: "{eopt.Value}"
                                                                        data-select-label: "{eopt.Label}"
                                                                        {eopt.Label}
                                    {else}
                                        span.scene-test-vars-empty {t_no_vars_in_project}
                            .scene-settings-block#modal-scene-settings-block .hidden?: {!SceneTextModalSettingsOpen}
                                .scene-settings-row
                                    span.node-meta-key {SceneTextModalChapterKey}
                                    .custom-select.node-chapter-select
                                        data-select-root: "true"
                                        .node-chapter-input-wrap
                                            input.node-inline-chapter
                                                type: text
                                                name: chapter
                                                value: "{SceneTextModalChapter}"
                                                placeholder: "{t_chapter_placeholder}"
                                                disabled?: {IsHistoryPreview}
                                                readonly?: {IsHistoryPreview}
                                            {if not IsHistoryPreview}
                                                button.node-chapter-caret-btn
                                                    data-select-toggle: "true"
                                                    svg.custom-select-caret src: /icons/chevron-down.svg
                                        .custom-select-menu.hidden
                                            data-select-menu: "true"
                                            {for ch_opt in SceneTextModalChapterOptions}
                                                button.custom-select-option .active?: {ch_opt.IsActive}
                                                    data-select-value: "{ch_opt.Value}"
                                                    data-select-label: "{ch_opt.Label}"
                                                    {ch_opt.Label}
                                .scene-settings-row
                                    span.node-meta-key "scene:"
                                    input.node-meta-input
                                        type: text
                                        name: explicit_id
                                        value: "{SceneTextModalExplicitID}"
                                        placeholder: "{t_scene_id_placeholder}"
                                        spellcheck: "false"
                                        autocorrect: "off"
                                        autocapitalize: "off"
                                        autocomplete: "off"
                                        disabled?: {IsHistoryPreview}
                                        readonly?: {IsHistoryPreview}
                                {if HasBookVariables}
                                    .scene-settings-row .is-empty-rule?: {!SceneTextModalHasEffects}
                                        span.node-meta-key "set:"
                                        .syn-input-wrap .is-invalid?: {SceneTextModalEffectsInvalid}
                                            data-hint?: {SceneTextModalEffectsErrorHint}
                                            input.node-meta-input.syn-inner-input
                                                type: text
                                                name: scene_effects
                                                value: "{SceneTextModalEffects}"
                                                placeholder: "odkryta_prawda = true; sumienie++"
                                                spellcheck: "false"
                                                autocorrect: "off"
                                                autocapitalize: "off"
                                                autocomplete: "off"
                                                data-hint?: {SceneTextModalEffectsErrorHint}
                                                disabled?: {IsHistoryPreview}
                                                readonly?: {IsHistoryPreview}
                                {if not IsScreenplayMode}
                                    .scene-settings-row
                                        span.node-meta-key "image:"
                                        .custom-select.node-media-select.modal-image-select
                                            data-select-root: "true"
                                            button.custom-select-trigger
                                                data-select-toggle: "true"
                                                disabled?: {IsHistoryPreview}
                                                span.custom-select-label {SceneTextModalCurrentImageLabel}
                                                svg.custom-select-caret src: /icons/chevron-down.svg
                                            .custom-select-menu.hidden
                                                data-select-menu: "true"
                                                {for img_opt in SceneTextModalImageOptions}
                                                    button.custom-select-option .active?: {img_opt.IsActive}
                                                        data-select-value: "{img_opt.Value}"
                                                        data-select-label: "{img_opt.Label}"
                                                        {img_opt.Label}
                                        {if not IsHistoryPreview}
                                            button.node-icon-btn.node-media-add-btn
                                                data-kind: image
                                                data-scene: "{SceneTextModalID}"
                                                svg.media-plus-icon src: /icons/plus.svg
                                {if SceneTextModalBookNarrated}
                                    .scene-settings-row
                                        span.node-meta-key "voice:"
                                        .custom-select.node-media-select.modal-voice-select
                                            data-select-root: "true"
                                            button.custom-select-trigger
                                                data-select-toggle: "true"
                                                disabled?: {IsHistoryPreview}
                                                span.custom-select-label {SceneTextModalCurrentVoiceLabel}
                                                svg.custom-select-caret src: /icons/chevron-down.svg
                                            .custom-select-menu.hidden
                                                data-select-menu: "true"
                                                {for voice_opt in SceneTextModalVoiceOptions}
                                                    button.custom-select-option .active?: {voice_opt.IsActive}
                                                        data-select-value: "{voice_opt.Value}"
                                                        data-select-label: "{voice_opt.Label}"
                                                        {voice_opt.Label}
                                        {if not IsHistoryPreview}
                                            button.node-icon-btn.node-media-add-btn
                                                data-kind: audio
                                                data-scene: "{SceneTextModalID}"
                                                svg.media-plus-icon src: /icons/plus.svg
                                            button.node-icon-btn.node-media-add-btn.icon-toggle-btn.voice-rec-toggle-btn .active?: {VoiceRecorderOpen}
                                                data-scene: "{SceneTextModalID}"
                                                svg.media-plus-icon src: /icons/mic.svg
                            {if SceneTextModalBookNarrated}
                                {if not IsHistoryPreview}
                                    {if SceneTextModalHasVoiceSections}
                                        .voice-rec-sections.modal-voice-rec-sections#modal-voice-rec-sections .hidden?: {!VoiceRecorderOpen}
                                            .voice-rec-sections-tabs
                                                {for vsec in SceneTextModalVoiceSections}
                                                    button.voice-rec-section-btn .active?: {vsec.IsActive} .is-recorded?: {vsec.HasAudio}
                                                        data-scene: "{SceneTextModalID}"
                                                        data-voice-section: "{vsec.Key}"
                                                        span.voice-rec-section-dot .is-recorded?: {vsec.HasAudio}
                                                        span.voice-rec-section-label {vsec.Label}
                                                        span.voice-rec-section-time {vsec.DurationLabel}
                                            span.voice-rec-section-file {SceneTextModalActiveVoiceSectionFile}
                                    .voice-rec-strip.modal-voice-rec-strip#modal-voice-rec-strip .hidden?: {!VoiceRecorderOpen}
                                        data-voice-strip: "true"
                                        data-voice-open: "{VoiceRecorderOpen}"
                                        data-scene: "{SceneTextModalID}"
                                        data-voice-section: "{SceneTextModalActiveVoiceSectionKey}"
                                        data-voice-url: "{SceneTextModalVoiceURL}"
                                        data-voice-file: "{SceneTextModalCurrentVoice}"
                                        data-trim-start: "{SceneTextModalVoiceTrimStart}"
                                        data-trim-end: "{SceneTextModalVoiceTrimEnd}"
                                        button.voice-rec-transport-btn
                                            data-voice-transport: "true"
                                            svg.voice-rec-icon.voice-rec-icon-play .hidden?: {!SceneTextModalHasVoice} data-voice-icon: "play" src: /icons/voice-play.svg
                                            svg.voice-rec-icon.voice-rec-icon-mic .hidden?: {SceneTextModalHasVoice} data-voice-icon: "mic" src: /icons/mic.svg
                                            svg.voice-rec-icon.voice-rec-icon-pause.hidden data-voice-icon: "pause" src: /icons/voice-pause.svg
                                            svg.voice-rec-icon.voice-rec-icon-stop.hidden data-voice-icon: "stop" src: /icons/voice-stop.svg
                                        .voice-rec-waveform-wrap
                                            data-voice-waveform-wrap: "true"
                                            canvas.voice-rec-waveform-canvas
                                                data-voice-canvas: "true"
                                                width: "510"
                                                height: "30"
                                            span.voice-rec-playhead
                                                data-voice-playhead: "true"
                                            span.voice-rec-trim-handle.trim-start
                                                data-voice-trim: "start"
                                            span.voice-rec-trim-handle.trim-end
                                                data-voice-trim: "end"
                                        span.voice-rec-timer
                                            data-voice-timer: "true"
                                            {SceneTextModalVoiceDurationLabel}
                                        button.voice-rec-action-btn.voice-rec-cut-btn .is-disabled?: {!SceneTextModalHasVoiceTrim}
                                            data-voice-trim-destructive: "true"
                                            data-scene: "{SceneTextModalID}"
                                            data-voice-section: "{SceneTextModalActiveVoiceSectionKey}"
                                            data-hint: "{t_voice_trim_destructive_hint}"
                                            disabled?: {!SceneTextModalHasVoiceTrim}
                                            svg.voice-rec-btn-icon src: /icons/scissors.svg
                                        button.voice-rec-action-btn.voice-rec-rerecord-btn .hidden?: {!SceneTextModalHasVoice}
                                            data-voice-record-start: "true"
                                            svg.voice-rec-btn-icon src: /icons/mic.svg
                                        button.voice-rec-action-btn.voice-rec-delete-btn .hidden?: {!SceneTextModalHasVoice}
                                            data-voice-delete: "true"
                                            data-scene: "{SceneTextModalID}"
                                            data-voice-section: "{SceneTextModalActiveVoiceSectionKey}"
                                            svg.voice-rec-btn-icon src: /icons/trash.svg
                        {if SceneTextModalHasImage}
                            .scene-illustration-spacer
                                img.scene-portrait-img
                                    alt: "{SceneTextModalName}"
                                    src: {SceneTextModalImageURL}
                        .scene-body-block.scene-text-editor-body
                            {if IsScreenplayMode}
                                .modal-screenplay-meta-bar
                                    .modal-slugline-row
                                        span.node-meta-key {t_sp_scene_upper_key}
                                        input.node-meta-input.node-slugline-input#modal-slugline-input
                                            type: text
                                            name: slugline
                                            value: "{SceneTextModalSlugline}"
                                            placeholder: "{t_slugline_placeholder}"
                                            disabled?: {IsHistoryPreview}
                                            readonly?: {IsHistoryPreview}
                                        {if SceneTextModalLengthLabel}
                                            span.node-script-length-badge {SceneTextModalLengthLabel}
                                    {if not IsHistoryPreview}
                                        .slugline-helper-pills
                                            {for p in SluglinePrefixPills}
                                                button.slugline-pill-btn
                                                    data-scene: "{SceneTextModalID}"
                                                    data-prefix: "{p}"
                                                    {p}
                                            {for t in SluglineTimePills}
                                                button.slugline-pill-btn
                                                    data-scene: "{SceneTextModalID}"
                                                    data-time: "{t}"
                                                    {t}
                                    .modal-slugline-row.node-meta-row-textarea
                                        span.node-meta-key {t_desc_upper_key}
                                        textarea.node-meta-input.node-meta-textarea
                                            name: synopsis
                                            rows: 1
                                            placeholder: "{t_synopsis_placeholder}"
                                            disabled?: {IsHistoryPreview}
                                            readonly?: {IsHistoryPreview}
                                            {SceneTextModalSynopsis}
                                {if not IsHistoryPreview}
                                    .modal-screenplay-format-bar#modal-screenplay-format-bar
                                        .modal-format-row
                                            span.node-meta-key {t_insert_upper_key}
                                            .slugline-helper-pills
                                                {for fp in ScreenplayInsertPills}
                                                    button.slugline-pill-btn.script-format-pill-btn
                                                        data-insert-script-block: "{fp.Kind}"
                                                        data-script-standard: "{ScriptStandard}"
                                                        data-snippet: "{fp.Snippet}"
                                                        data-select-target: "{fp.SelectTarget}"
                                                        data-double-newline: "{fp.DoubleNewline}"
                                                        data-scene: "{SceneTextModalID}"
                                                        data-hint: "{fp.Hint}"
                                                        {fp.Label}
                                        {if HasScreenplayCharacterPills}
                                            .modal-format-row
                                                span.node-meta-key {t_char_upper_key}
                                                .slugline-helper-pills
                                                    {for cp in ScreenplayCharacterPills}
                                                        button.slugline-pill-btn.script-char-pill-btn
                                                            data-insert-script-char: "{cp.Name}"
                                                            data-snippet: "{cp.Snippet}"
                                                            data-select-target: "{cp.SelectTarget}"
                                                            data-double-newline: "{cp.DoubleNewline}"
                                                            data-scene: "{SceneTextModalID}"
                                                            data-hint: "{cp.Hint}"
                                                            {cp.Label}
                            {if IsVoiceSectionIsolated}
                                input
                                    type: hidden
                                    name: isolated_voice_section
                                    value: "{IsolatedVoiceSectionKey}"
                                .voice-section-isolation-bar#voice-section-isolation-bar
                                    span.voice-section-isolation-title {IsolatedVoiceSectionHeader}
                            .scene-prose-edit-wrap .hidden?: {!IsScenePreviewNormal}
                                .scene-prose.modal-scene-prose-hl#modal-scene-prose-hl .is-screenplay?: {IsScreenplayMode}
                                    aria-hidden: "true"
                                    {for seg in SceneTextModalHlSegments}
                                        span.prose-hl-seg .is-syntax?: {seg.is_syntax} {seg.text}
                                textarea.scene-prose.modal-scene-textarea#modal-scene-textarea .is-screenplay?: {IsScreenplayMode} .hidden?: {!IsScenePreviewNormal}
                                    name: text
                                    rows: 1
                                    spellcheck: "false"
                                    autocorrect: "off"
                                    autocapitalize: "off"
                                    autocomplete: "off"
                                    placeholder: "{t_scene_text_placeholder}"
                                    disabled?: {IsHistoryPreview}
                                    readonly?: {IsHistoryPreview}
                                    {SceneTextModalText}
                            {if not IsScenePreviewNormal}
                                .scene-prose.scene-prose-preview-stage#scene-prose-preview-stage .is-screenplay?: {IsScreenplayMode} .mode-highlight?: {IsScenePreviewHighlight} .mode-output?: {IsScenePreviewOutput}
                                    {if IsScenePreviewOutput}
                                        {prose SceneTextModalOutputProse}
                                    {else}
                                        {for para in SceneTextModalPreviewParagraphs}
                                            {if para.is_divider}
                                                hr.scene-divider
                                            {else}
                                                p.scene-preview-para .is-cond-active-block?: {para.is_active} .is-cond-inactive-block?: {para.is_inactive}
                                                    {for seg in para.segments}
                                                        {if seg.is_syntax}
                                                            span.prose-syn-brace {seg.text}
                                                        {elif seg.is_active}
                                                            span.prose-cond-active {seg.text}
                                                        {elif seg.is_inactive}
                                                            span.prose-cond-inactive {seg.text}
                                                        {elif seg.is_var_val}
                                                            span.prose-var-val {seg.text}
                                                        {else}
                                                            span {seg.text}
                            .scene-choices-list
                                {for ch in SceneTextModalChoices}
                                    .choice-btn.modal-choice-row .is-rule-expanded?: {ch.IsRuleOpen} .has-active-rule?: {ch.HasRule} .choice-cond-unmet?: {ch.IsCondUnmet} .choice-locked?: {ch.IsLockedPreview}
                                        {if not IsHistoryPreview}
                                            span.choice-drag-grip
                                                data-choice-grip: "true"
                                                data-choice-idx: "{ch.Idx}"
                                                svg.choice-grip-icon src: /icons/grip.svg
                                        .choice-triple-stack.modal-choice-triple-stack
                                            .choice-triple-row.choice-triple-exit .is-invalid?: {ch.IsExitInvalid}
                                                data-hint?: {ch.ExitErrorHint}
                                                span.choice-row-key "exit:"
                                                input.choice-exit-input
                                                    type: text
                                                    name: "choice_target_{ch.Idx}"
                                                    data-choice-idx: "{ch.Idx}"
                                                    value: "{ch.SelectedValue}"
                                                    placeholder: "{t_exit_target_placeholder}"
                                                    spellcheck: "false"
                                                    autocorrect: "off"
                                                    autocapitalize: "off"
                                                    autocomplete: "off"
                                                    data-hint?: {ch.ExitErrorHint}
                                                    disabled?: {IsHistoryPreview}
                                                    readonly?: {IsHistoryPreview}
                                            .choice-triple-row.choice-triple-label
                                                span.choice-row-key "text:"
                                                textarea.node-meta-input.node-meta-textarea.choice-label-text.node-inline-title
                                                    name: "choice_label_{ch.Idx}"
                                                    data-choice-idx: "{ch.Idx}"
                                                    rows: 1
                                                    placeholder: "{t_choice_text_placeholder}"
                                                    disabled?: {IsHistoryPreview}
                                                    readonly?: {IsHistoryPreview}
                                                    {ch.Label}
                                            {if ch.IsRuleOpen}
                                                .choice-triple-row.choice-triple-cond .is-empty-rule?: {!ch.HasCondition} .is-invalid?: {ch.IsCondInvalid}
                                                    data-hint?: {ch.CondErrorHint}
                                                    span.choice-row-key "if:"
                                                    input.choice-rule-input
                                                        type: text
                                                        name: "choice_cond_{ch.Idx}"
                                                        data-choice-idx: "{ch.Idx}"
                                                        value: "{ch.Condition}"
                                                        placeholder: ""
                                                        spellcheck: "false"
                                                        autocorrect: "off"
                                                        autocapitalize: "off"
                                                        autocomplete: "off"
                                                        data-hint?: {ch.CondErrorHint}
                                                        disabled?: {IsHistoryPreview}
                                                        readonly?: {IsHistoryPreview}
                                                .choice-triple-row.choice-triple-eff .is-empty-rule?: {!ch.HasEffects} .is-invalid?: {ch.IsEffInvalid}
                                                    data-hint?: {ch.EffErrorHint}
                                                    span.choice-row-key "set:"
                                                    input.choice-rule-input
                                                        type: text
                                                        name: "choice_effects_{ch.Idx}"
                                                        data-choice-idx: "{ch.Idx}"
                                                        value: "{ch.Effects}"
                                                        placeholder: ""
                                                        spellcheck: "false"
                                                        autocorrect: "off"
                                                        autocapitalize: "off"
                                                        autocomplete: "off"
                                                        data-hint?: {ch.EffErrorHint}
                                                        disabled?: {IsHistoryPreview}
                                                        readonly?: {IsHistoryPreview}
                                            {else}
                                                input
                                                    type: hidden
                                                    name: "choice_cond_{ch.Idx}"
                                                    value: "{ch.Condition}"
                                                input
                                                    type: hidden
                                                    name: "choice_effects_{ch.Idx}"
                                                    value: "{ch.Effects}"
                                        .modal-choice-controls
                                            {if not IsHistoryPreview}
                                                button.node-icon-btn.node-choice-btn.node-choice-remove
                                                    data-scene: "{SceneTextModalID}"
                                                    data-choice-idx: "{ch.Idx}"
                                                    svg.choice-trash-icon src: /icons/trash.svg
                                            button.contextual-rewind-btn.modal-choice-nav-btn
                                                data-scene: "{ch.TargetSceneID}"
                                                data-choice-idx: "{ch.Idx}"
                                                data-hint: "{t_go_to_target_scene_hint}"
                                                @scene_choice_arrow
                                {if not IsHistoryPreview}
                                    button.node-icon-btn.node-media-add-btn.modal-add-choice-btn
                                        data-scene: "{SceneTextModalID}"
                                        svg.media-plus-icon src: /icons/plus.svg
                            .syn-autocomplete-dropdown#modal-syn-autocomplete-dropdown .hidden?: {!ShowSyntaxHelpPopover}
                                {if ShowSyntaxHelpPopover}
                                    .syn-dropdown-help-box#modal-syntax-help-popover
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
                .scroll-more-cue.hidden#modal-scroll-more-cue
                    span.scroll-more-dots "..."
