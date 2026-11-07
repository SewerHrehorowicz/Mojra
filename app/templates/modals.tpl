{if show_qr_modal}
    .modal-backdrop
        data-backdrop-action: CloseQRModal
        .modal-card
            .modal-header
                h3.modal-title {t_settings_modal_title}
            .modal-body.qr-modal-body
                .node-meta-fields.modal-meta-fields
                    .node-meta-row
                        span.node-meta-key {t_settings_lang_key}
                        .form-radio-group.is-inline
                            {for opt in lang_options}
                                button.form-radio-option .active?: {opt.is_active}
                                    type: button
                                    action: SetLanguage
                                    data-lang: "{opt.code}"
                                    span.form-radio-label "{opt.upper}"
                                    span.form-radio-sq
                    .node-meta-row
                        span.node-meta-key {t_theme_key}
                        button.book-setting-toggle-btn.form-toggle-btn .active?: {is_dark}
                            type: button
                            action: ToggleTheme
                            span.custom-select-label {theme_toggle_label}
                            span.toggle-right
                                {if is_dark}
                                    svg.action-icon src: /icons/theme.svg
                                {else}
                                    svg.action-icon src: /icons/sun.svg
                                span.toggle-switch
                                    span.toggle-thumb
                    .node-meta-row
                        span.node-meta-key {t_images_key}
                        button.book-setting-toggle-btn.form-toggle-btn .active?: {!imageless} .is-disabled?: {imageless_locked}
                            type: button
                            action: ToggleImageless
                            disabled?: {imageless_locked}
                            span.custom-select-label {imageless_toggle_label}
                            span.toggle-right
                                {if imageless}
                                    svg.action-icon src: /icons/image-off.svg
                                {else}
                                    svg.action-icon src: /icons/image.svg
                                span.toggle-switch
                                    span.toggle-thumb
                    .node-meta-row
                        span.node-meta-key {t_scale_key}
                        .form-radio-group.is-inline
                            button.form-radio-option .active?: {is_small_ui}
                                type: button
                                action: SetUIScale
                                data-scale: small
                                span.form-radio-label {t_scale_default}
                                span.form-radio-sq
                            button.form-radio-option .active?: {is_large_ui}
                                type: button
                                action: SetUIScale
                                data-scale: large
                                span.form-radio-label {t_scale_large}
                                span.form-radio-sq
                            button.form-radio-option .active?: {is_zen_ui}
                                type: button
                                action: SetUIScale
                                data-scale: zen
                                span.form-radio-label "Zen"
                                span.form-radio-sq
                    .node-meta-row
                        span.node-meta-key {t_scroll_sensitivity_key}
                        .form-radio-group.is-inline
                            button.form-radio-option .active?: {is_scroll_sens_high}
                                type: button
                                action: SetScrollSensitivity
                                data-sensitivity: high
                                span.form-radio-label {t_scroll_sens_high}
                                span.form-radio-sq
                            button.form-radio-option .active?: {is_scroll_sens_medium}
                                type: button
                                action: SetScrollSensitivity
                                data-sensitivity: medium
                                span.form-radio-label {t_scroll_sens_medium}
                                span.form-radio-sq
                            button.form-radio-option .active?: {is_scroll_sens_low}
                                type: button
                                action: SetScrollSensitivity
                                data-sensitivity: low
                                span.form-radio-label {t_scroll_sens_low}
                                span.form-radio-sq
                            button.form-radio-option .active?: {is_scroll_sens_off}
                                type: button
                                action: SetScrollSensitivity
                                data-sensitivity: off
                                span.form-radio-label {t_scroll_sens_off}
                                span.form-radio-sq
                    .node-meta-row
                        span.node-meta-key {t_auto_choice_delay_key}
                        .form-radio-group.is-inline
                            button.form-radio-option .active?: {is_auto_delay_1}
                                type: button
                                action: SetAutoChoiceDelay
                                data-delay: "1"
                                span.form-radio-label "1s"
                                span.form-radio-sq
                            button.form-radio-option .active?: {is_auto_delay_2}
                                type: button
                                action: SetAutoChoiceDelay
                                data-delay: "2"
                                span.form-radio-label "2s"
                                span.form-radio-sq
                            button.form-radio-option .active?: {is_auto_delay_3}
                                type: button
                                action: SetAutoChoiceDelay
                                data-delay: "3"
                                span.form-radio-label "3s"
                                span.form-radio-sq
                            button.form-radio-option .active?: {is_auto_delay_5}
                                type: button
                                action: SetAutoChoiceDelay
                                data-delay: "5"
                                span.form-radio-label "5s"
                                span.form-radio-sq
                {if dev_mode}
                    .settings-modal-actions
                        button.btn.btn-sm
                            type: button
                            action: ExtractEmbeddedBook
                            "{t_extract_embedded_book}"
                    p.modal-desc {t_lan_qr_desc}
                    .qr-frame
                        img.qr-image
                            src: /api/lan-qr.png
                            alt: "{t_lan_qr_alt}"
                    .lan-url-box {lan_url}
                .modal-actions
                    button.btn
                        type: button
                        action: CloseQRModal
                        "{t_close}"

{if show_new_book_modal}
    .modal-backdrop
        data-backdrop-action: CloseNewBookModal
        .modal-card
            .modal-header
                {if new_book_is_screenplay}
                    h3.modal-title#new-book-modal-title {t_new_screenplay_modal_title}
                {else}
                    h3.modal-title#new-book-modal-title {t_new_book_modal_title}
            form.modal-body#form-new-book
                action: CreateNewBook
                .node-meta-fields.modal-meta-fields
                    .node-meta-row
                        span.node-meta-key#new-book-title-label {t_title_key}
                        input.node-meta-input.field-input#new-book-title
                            type: text
                            name: title
                            value: "{new_book_title}"
                            placeholder: "{new_book_title_placeholder}"
                    .node-meta-row
                        span.node-meta-key {t_author_key}
                        input.node-meta-input.field-input#new-book-author
                            type: text
                            name: author
                            value: "{new_book_author}"
                            placeholder: "{t_author_placeholder}"
                    .node-meta-row
                        span.node-meta-key {t_mode_key}
                        .custom-select.node-media-select.new-book-mode-select
                            input.custom-select-input
                                type: hidden
                                name: mode
                                value: "{new_book_mode}"
                            button.custom-select-trigger.modal-field-select-trigger
                                type: button
                                data-select-toggle: "true"
                                span.custom-select-label {new_book_mode_label}
                                svg.custom-select-caret src: /icons/chevron-down.svg
                            .custom-select-menu.hidden
                                {for md_opt in new_book_mode_options}
                                    button.custom-select-option .active?: {md_opt.is_active}
                                        type: button
                                        data-select-value: "{md_opt.value}"
                                        data-select-label: "{md_opt.label}"
                                        {md_opt.label}
                    .node-meta-row.node-meta-row-radio#new-book-standard-field .hidden?: {!new_book_is_screenplay}
                        span.node-meta-key {t_standard_key}
                        .form-radio-group
                            {for std_opt in new_book_standard_options}
                                label.form-radio-option .active?: {std_opt.is_active}
                                    input.form-radio-input
                                        type: radio
                                        name: standard
                                        value: "{std_opt.value}"
                                        checked?: {std_opt.is_active}
                                    span.form-radio-label {std_opt.label}
                                    span.form-radio-sq
                    .node-meta-row#new-book-narrated-field .hidden?: {new_book_is_screenplay}
                        span.node-meta-key {t_narration_key}
                        input
                            type: hidden
                            name: narrated
                            value: "{new_book_narrated_val}"
                        button.book-setting-toggle-btn.form-toggle-btn .active?: {new_book_narrated}
                            type: button
                            data-form-toggle: narrated
                            data-active-when: "true"
                            data-label-on: "{t_required_label}"
                            data-label-off: "{t_disabled_label}"
                            span.toggle-right
                                span.toggle-switch
                                    span.toggle-thumb
                                svg.action-icon.toggle-icon-on .hidden?: {!new_book_narrated} src: /icons/mic.svg
                                svg.action-icon.toggle-icon-off .hidden?: {new_book_narrated} src: /icons/mic-off.svg
                            span.custom-select-label {new_book_narrated_short_label}
                    .node-meta-row#new-book-imageless-field .hidden?: {new_book_is_screenplay}
                        span.node-meta-key {t_images_key}
                        input
                            type: hidden
                            name: imageless
                            value: "{new_book_imageless_val}"
                        button.book-setting-toggle-btn.form-toggle-btn .active?: {!new_book_imageless}
                            type: button
                            data-form-toggle: imageless
                            data-active-when: "false"
                            data-label-on: "{t_with_images_label}"
                            data-label-off: "{t_no_images_label}"
                            span.toggle-right
                                span.toggle-switch
                                    span.toggle-thumb
                                svg.action-icon.toggle-icon-on .hidden?: {new_book_imageless} src: /icons/image.svg
                                svg.action-icon.toggle-icon-off .hidden?: {!new_book_imageless} src: /icons/image-off.svg
                            span.custom-select-label {new_book_imageless_short_label}
                    .node-meta-row
                        span.node-meta-key#new-book-location-label {t_folder_key}
                        .node-media-picker-row
                            input.node-meta-input.field-input#new-book-path
                                type: text
                                name: save_path
                                value: "{new_book_default_dir}"
                                placeholder: "{new_book_default_dir}"
                            button.node-icon-btn.node-folder-btn
                                type: button
                                action: BrowseNewBookFolder
                                data-hint: "{t_choose_folder_hint}"
                                svg.folder-browse-icon src: /icons/folder.svg
                .modal-actions
                    button.btn
                        type: button
                        action: CloseNewBookModal
                        "{t_cancel}"
                    button.btn.btn-primary#new-book-submit-btn
                        type: submit
                        {if new_book_is_screenplay}
                            "{t_create_screenplay_btn}"
                        {else}
                            "{t_create_book_btn}"

{if show_export_modal}
    .modal-backdrop
        data-backdrop-action: CloseExportModal
        .modal-card#modal-export-project
            .modal-header
                h3.modal-title {t_export_modal_title}
                span.export-selection-count {export_selected_count_label}
            form.modal-body#form-export-project
                action: ExportProject
                .node-meta-fields.modal-meta-fields
                    .node-meta-row.export-formats-row
                        span.node-meta-key {t_formats_key}
                        .export-format-list
                            {for fmt_item in export_primary_formats}
                                button.export-format-item .active?: {fmt_item.is_selected}
                                    type: button
                                    action: ToggleExportFormat
                                    data-format: "{fmt_item.id}"
                                    span.export-format-name {fmt_item.label}
                                    span.export-format-right
                                        span.export-format-ext {fmt_item.ext}
                                        span.export-format-check
                    .node-meta-row
                        span.node-meta-key {t_folder_key}
                        .node-media-picker-row
                            input.node-meta-input.field-input#export-modal-path
                                type: text
                                name: export_dir
                                value: "{export_modal_dir}"
                                placeholder: "{default_export_dir}"
                            button.node-icon-btn.node-folder-btn
                                type: button
                                action: BrowseExportModalFolder
                                data-hint: "{t_choose_folder_hint}"
                                svg.folder-browse-icon src: /icons/folder.svg
                button.export-advanced-toggle .open?: {export_advanced_open}
                    type: button
                    action: ToggleExportModalAdvanced
                    span.export-advanced-label {t_advanced_upper}
                    svg.export-advanced-caret src: /icons/chevron-down.svg
                .export-advanced-body .hidden?: {!export_advanced_open}
                    .node-meta-fields.modal-meta-fields
                        .node-meta-row.export-formats-row
                            span.node-meta-key
                            .export-format-list
                                {for adv_fmt in export_advanced_formats}
                                    button.export-format-item .active?: {adv_fmt.is_selected}
                                        type: button
                                        action: ToggleExportFormat
                                        data-format: "{adv_fmt.id}"
                                        span.export-format-name {adv_fmt.label}
                                        span.export-format-right
                                            span.export-format-ext {adv_fmt.ext}
                                            span.export-format-check
                        .node-meta-row
                            span.node-meta-key {t_standard_key}
                            .custom-select.node-media-select.export-standard-select
                                input.custom-select-input
                                    type: hidden
                                    name: standard
                                    value: "{export_modal_standard}"
                                button.custom-select-trigger.modal-field-select-trigger
                                    type: button
                                    data-select-toggle: "true"
                                    span.custom-select-label {export_modal_standard_label}
                                    svg.custom-select-caret src: /icons/chevron-down.svg
                                .custom-select-menu.hidden
                                    {for std_opt in export_modal_standard_options}
                                        button.custom-select-option .active?: {std_opt.is_active}
                                            type: button
                                            data-select-value: "{std_opt.value}"
                                            data-select-label: "{std_opt.label}"
                                            {std_opt.label}
                .modal-actions
                    button.btn
                        type: button
                        action: CloseExportModal
                        "{t_cancel}"
                    button.btn.btn-primary#export-modal-submit-btn
                        type: submit
                        disabled?: {!export_has_selected}
                        "{t_export_btn} ({export_selected_count})"

{if show_add_book_modal}
    .modal-backdrop
        data-backdrop-action: CloseAddBookModal
        .modal-card.modal-card-lg
            .modal-header
                h3.modal-title {t_add_book_modal_title}
                button.node-close-btn
                    type: button
                    action: CloseAddBookModal
                    svg.node-close-icon src: /icons/close.svg
            .modal-body
                .add-book-section
                    .field-label {t_add_disk_hdr}
                    p.modal-desc {t_add_disk_desc}
                    .add-book-browse-row
                        button.btn.btn-sm
                            type: button
                            action: BrowseAddBookFromDisk
                            data-mode: dir
                            "{t_choose_book_dir_btn}"
                        button.btn.btn-sm
                            type: button
                            action: BrowseAddBookFromDisk
                            data-mode: file
                            "{t_choose_book_file_btn}"
                    form.add-book-inline-form
                        action: AddBookFromDisk
                        input.field-input
                            type: text
                            name: path
                            required: true
                            placeholder: "{t_add_disk_path_placeholder}"
                        button.btn.btn-primary
                            type: submit
                            "{t_add_from_disk_btn}"
                .add-book-section
                    .field-label {t_add_url_hdr}
                    p.modal-desc {t_add_url_desc}
                    form.add-book-inline-form
                        action: AddBookFromURL
                        input.field-input
                            type: url
                            name: url
                            required: true
                            placeholder: "{t_add_url_placeholder}"
                        button.btn.btn-primary
                            type: submit
                            "{t_download_from_url_btn}"
                .modal-actions
                    button.btn
                        type: button
                        action: CloseAddBookModal
                        "{t_close}"

{if show_studio_project_modal}
    .modal-backdrop
        data-backdrop-action: CloseStudioProjectModal
        .modal-card.modal-card-lg#modal-studio-project
            .modal-header
                h3.modal-title {t_open_project_modal_title}
                button.node-close-btn
                    type: button
                    action: CloseStudioProjectModal
                    svg.node-close-icon src: /icons/close.svg
            .modal-body
                {if has_studio_projects}
                    .add-book-section
                        .studio-projects-list
                            {for sp in studio_projects}
                                button.studio-project-row .active?: {sp.is_current}
                                    type: button
                                    action: OpenStudioProject
                                    data-book: "{sp.id}"
                                    span.studio-project-main
                                        span.studio-project-title {sp.title}
                                        span.studio-project-author {sp.author}
                                    span.studio-project-badge {sp.kind_badge}
                .add-book-section
                    .add-book-browse-row
                        button.btn.btn-sm
                            type: button
                            action: BrowseOpenStudioProject
                            data-mode: dir
                            "{t_choose_project_dir_btn}"
                        button.btn.btn-sm
                            type: button
                            action: BrowseOpenStudioProject
                            data-mode: file
                            "{t_unpack_playbook_copy_btn}"
                    form.add-book-inline-form
                        action: OpenStudioProjectFromDisk
                        input.field-input#studio-project-path-input
                            type: text
                            name: path
                            required: true
                            placeholder: "{t_studio_project_path_placeholder}"
                        button.btn.btn-primary
                            type: submit
                            "{t_open_btn}"
                .modal-actions
                    button.btn
                        type: button
                        action: CloseStudioProjectModal
                        "{t_close}"

{if show_scene_text_modal}
    .modal-backdrop.scene-text-fullscreen-backdrop#modal-scene-text .mode-half?: {scene_text_is_half_width} .mode-full?: {scene_text_is_full_width} .has-right-panel?: {scene_text_has_right_panel}
        data-backdrop-action: CloseSceneTextModal
        .player-layout.scene-text-book-layout .is-imageless?: {scene_text_modal_is_imageless}
            .player-bg-stage
                {if scene_text_modal_has_image}
                    img.player-bg-img
                        alt: "{scene_text_modal_name}"
                        src: {scene_text_modal_image_url}
            .story-scroll-feed.scene-text-editor-feed
                .reader-global-meta
                    span.scene-chapter-pill {scene_text_modal_header_meta}
                    .scene-text-editor-top-bar
                        button.scene-text-back-btn
                            type: button
                            action: PlayFromScene
                            data-scene: "{scene_text_modal_id}"
                            svg.btn-icon src: /icons/play.svg
                        {if not is_history_preview}
                            button.scene-text-back-btn.scene-text-delete-btn
                                type: button
                                action: PromptDeleteScene
                                data-scene: "{scene_text_modal_id}"
                                svg.btn-icon src: /icons/trash.svg
                        button.scene-text-back-btn#btn-collapse-scene-text
                            type: button
                            action: CloseSceneTextModal
                            svg.btn-icon src: /icons/chevrons-left.svg
                        button.scene-text-back-btn#btn-expand-scene-text-full .hidden?: {scene_text_is_full_width}
                            type: button
                            action: SetSceneTextWidthMode
                            data-mode: full
                            svg.btn-icon src: /icons/chevrons-right.svg
                .active-scene-stage.scene-text-editor-stage
                    form.scene-card.scene-focused.scene-text-editor-card#form-scene-text .has-illustration?: {scene_text_modal_has_image}
                        action: SaveSceneTextModal
                        data-book-vars: "{book_vars_spec}"
                        data-test-vars: "{scene_test_vars_spec}"
                        input
                            type: hidden
                            name: scene_id
                            value: "{scene_text_modal_id}"
                        input#scene-settings-open-input
                            type: hidden
                            name: scene_settings_open
                            value: "{scene_text_modal_settings_open_val}"
                        {if scene_text_modal_has_incoming}
                            .scene-incoming-list
                                {for inc in scene_text_modal_incoming}
                                    button.choice-btn.incoming-choice-btn
                                        type: button
                                        action: NavigateSceneTextModal
                                        data-scene: "{inc.scene_id}"
                                        span.incoming-choice-content
                                            span.incoming-scene-name {inc.scene_name}
                                            {if inc.has_choice_label}
                                                span.incoming-choice-sep "—"
                                                span.incoming-choice-label {inc.choice_label}
                                        span.choice-arrow.incoming-choice-arrow "↑"
                        .scene-header-block
                            .scene-title-row
                                .scene-title-with-badge
                                    span.present-scene-badge
                                        svg.present-scene-icon src: /icons/asterisk.svg
                                    {if scene_text_modal_is_start}
                                        span.node-start-indicator
                                            data-hint: "{t_start_scene_root_hint}"
                                            svg.scene-star-icon src: /icons/star.svg
                                    input.scene-title.scene-title-present.node-inline-title
                                        type: text
                                        name: name
                                        value: "{scene_text_modal_raw_name}"
                                        placeholder: "{t_scene_name_opt_placeholder}"
                                        disabled?: {is_history_preview}
                                        readonly?: {is_history_preview}
                                {if is_history_preview}
                                    span.history-step-badge {t_preview_readonly_badge}
                                {else}
                                    .scene-title-right-controls
                                        {if has_book_variables}
                                            button.contextual-rewind-btn.scene-preview-mode-btn#btn-scene-preview-output .active?: {!is_scene_preview_normal}
                                                type: button
                                                action: SetSceneTextPreviewMode
                                                svg.scene-settings-icon src: /icons/preview-output.svg
                                            button.contextual-rewind-btn.scene-preview-mode-btn#btn-scene-test-vars .active?: {show_scene_test_vars_bar}
                                                type: button
                                                action: ToggleSceneTestVarsPopover
                                                svg.scene-settings-icon src: /icons/braces.svg
                                        {if scene_text_modal_book_narrated}
                                            button.contextual-rewind-btn.icon-toggle-btn.voice-rec-toggle-btn#btn-modal-voice-recorder .active?: {voice_recorder_open}
                                                type: button
                                                action: ToggleSceneVoiceRecorder
                                                data-scene: "{scene_text_modal_id}"
                                                svg.scene-settings-icon src: /icons/mic.svg
                                        button.contextual-rewind-btn.scene-settings-toggle-btn .active?: {scene_text_modal_settings_open}
                                            type: button
                                            data-scene-settings-toggle: "true"
                                            data-hint: "{t_scene_settings_hint}"
                                            svg.scene-settings-icon src: /icons/settings.svg
                            {if show_scene_test_vars_bar}
                                .scene-test-vars-popover#scene-test-vars-bar
                                    .history-panel-header.scene-test-vars-header
                                        span.history-panel-title {t_test_state_title}
                                        button.node-close-btn
                                            type: button
                                            action: ToggleSceneTestVarsPopover
                                            svg.node-close-icon src: /icons/close.svg
                                    {if has_scene_test_vars}
                                        .scene-test-vars-items
                                            {for tv in scene_test_vars}
                                                .node-meta-row.test-var-row
                                                    span.node-meta-key.test-var-key "{tv.name}:"
                                                    {if tv.is_bool}
                                                        button.book-setting-toggle-btn.test-var-bool-btn .active?: {tv.bool_val}
                                                            type: button
                                                            action: UpdateSceneTextTestVar
                                                            data-var-name: "{tv.name}"
                                                            data-toggle: "true"
                                                            span.toggle-right
                                                                span.toggle-switch
                                                                    span.toggle-thumb
                                                            span.custom-select-label {tv.value}
                                                    {elif tv.is_int}
                                                        input.node-meta-input.test-var-int-input
                                                            type: text
                                                            inputmode: numeric
                                                            value: "{tv.value}"
                                                            data-test-var-input: "{tv.name}"
                                                            spellcheck: "false"
                                                            autocorrect: "off"
                                                            autocapitalize: "off"
                                                            autocomplete: "off"
                                                    {elif tv.is_enum}
                                                        .custom-select.node-media-select.test-var-enum-select
                                                            data-var-name: "{tv.name}"
                                                            input.custom-select-input
                                                                type: hidden
                                                                value: "{tv.value}"
                                                            button.custom-select-trigger
                                                                type: button
                                                                data-select-toggle: "true"
                                                                span.custom-select-label {tv.value}
                                                                svg.custom-select-caret src: /icons/chevron-down.svg
                                                            .custom-select-menu.hidden
                                                                {for eopt in tv.enum_options}
                                                                    button.custom-select-option .active?: {eopt.is_active}
                                                                        type: button
                                                                        data-select-value: "{eopt.value}"
                                                                        data-select-label: "{eopt.label}"
                                                                        {eopt.label}
                                    {else}
                                        span.scene-test-vars-empty {t_no_vars_in_project}
                            .scene-settings-block#modal-scene-settings-block .hidden?: {!scene_text_modal_settings_open}
                                .scene-settings-row
                                    span.node-meta-key {scene_text_modal_chapter_key}
                                    .custom-select.node-chapter-select
                                        .node-chapter-input-wrap
                                            input.node-inline-chapter.custom-select-input
                                                type: text
                                                name: chapter
                                                value: "{scene_text_modal_chapter}"
                                                placeholder: "{t_chapter_placeholder}"
                                                disabled?: {is_history_preview}
                                                readonly?: {is_history_preview}
                                            {if not is_history_preview}
                                                button.node-chapter-caret-btn
                                                    type: button
                                                    data-select-toggle: "true"
                                                    svg.custom-select-caret src: /icons/chevron-down.svg
                                        .custom-select-menu.hidden
                                            {for ch_opt in scene_text_modal_chapter_options}
                                                button.custom-select-option .active?: {ch_opt.is_active}
                                                    type: button
                                                    data-select-value: "{ch_opt.value}"
                                                    data-select-label: "{ch_opt.label}"
                                                    {ch_opt.label}
                                .scene-settings-row
                                    span.node-meta-key "scene:"
                                    input.node-meta-input
                                        type: text
                                        name: explicit_id
                                        value: "{scene_text_modal_explicit_id}"
                                        placeholder: "{t_scene_id_placeholder}"
                                        spellcheck: "false"
                                        autocorrect: "off"
                                        autocapitalize: "off"
                                        autocomplete: "off"
                                        disabled?: {is_history_preview}
                                        readonly?: {is_history_preview}
                                {if has_book_variables}
                                    .scene-settings-row .is-empty-rule?: {!scene_text_modal_has_effects}
                                        span.node-meta-key "set:"
                                        .syn-input-wrap .is-invalid?: {scene_text_modal_effects_invalid}
                                            data-hint?: {scene_text_modal_effects_error_hint}
                                            input.node-meta-input.syn-inner-input
                                                type: text
                                                name: scene_effects
                                                value: "{scene_text_modal_effects}"
                                                placeholder: "odkryta_prawda = true; sumienie++"
                                                spellcheck: "false"
                                                autocorrect: "off"
                                                autocapitalize: "off"
                                                autocomplete: "off"
                                                data-hint?: {scene_text_modal_effects_error_hint}
                                                disabled?: {is_history_preview}
                                                readonly?: {is_history_preview}
                                {if not is_screenplay_mode}
                                    .scene-settings-row
                                        span.node-meta-key "image:"
                                        .custom-select.node-media-select
                                            input.custom-select-input
                                                type: hidden
                                                name: explicit_image
                                                value: "{scene_text_modal_current_image}"
                                            button.custom-select-trigger
                                                type: button
                                                data-select-toggle: "true"
                                                disabled?: {is_history_preview}
                                                span.custom-select-label {scene_text_modal_current_image_label}
                                                svg.custom-select-caret src: /icons/chevron-down.svg
                                            .custom-select-menu.hidden
                                                {for img_opt in scene_text_modal_image_options}
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
                                                data-scene: "{scene_text_modal_id}"
                                                svg.media-plus-icon src: /icons/plus.svg
                                {if scene_text_modal_book_narrated}
                                    .scene-settings-row
                                        span.node-meta-key "voice:"
                                        .custom-select.node-media-select
                                            input.custom-select-input
                                                type: hidden
                                                name: explicit_voice
                                                value: "{scene_text_modal_current_voice}"
                                            button.custom-select-trigger
                                                type: button
                                                data-select-toggle: "true"
                                                disabled?: {is_history_preview}
                                                span.custom-select-label {scene_text_modal_current_voice_label}
                                                svg.custom-select-caret src: /icons/chevron-down.svg
                                            .custom-select-menu.hidden
                                                {for voice_opt in scene_text_modal_voice_options}
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
                                                data-scene: "{scene_text_modal_id}"
                                                svg.media-plus-icon src: /icons/plus.svg
                                            button.node-icon-btn.node-media-add-btn.icon-toggle-btn.voice-rec-toggle-btn .active?: {voice_recorder_open}
                                                type: button
                                                action: ToggleSceneVoiceRecorder
                                                data-scene: "{scene_text_modal_id}"
                                                svg.media-plus-icon src: /icons/mic.svg
                            {if scene_text_modal_book_narrated}
                                {if not is_history_preview}
                                    .voice-rec-strip.modal-voice-rec-strip#modal-voice-rec-strip .hidden?: {!voice_recorder_open}
                                        data-scene: "{scene_text_modal_id}"
                                        data-voice-url: "{scene_text_modal_voice_url}"
                                        data-voice-file: "{scene_text_modal_current_voice}"
                                        data-trim-start: "{scene_text_modal_voice_trim_start}"
                                        data-trim-end: "{scene_text_modal_voice_trim_end}"
                                        button.voice-rec-transport-btn
                                            type: button
                                            data-voice-transport: "true"
                                            svg.voice-rec-icon.voice-rec-icon-play .hidden?: {!scene_text_modal_has_voice} src: /icons/voice-play.svg
                                            svg.voice-rec-icon.voice-rec-icon-mic .hidden?: {scene_text_modal_has_voice} src: /icons/mic.svg
                                            svg.voice-rec-icon.voice-rec-icon-pause.hidden src: /icons/voice-pause.svg
                                            svg.voice-rec-icon.voice-rec-icon-stop.hidden src: /icons/voice-stop.svg
                                        .voice-rec-waveform-wrap
                                            data-voice-waveform-wrap: "true"
                                            canvas.voice-rec-waveform-canvas
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
                                            {scene_text_modal_voice_duration_label}
                                        button.voice-rec-action-btn.voice-rec-cut-btn .hidden?: {!scene_text_modal_has_voice_trim}
                                            type: button
                                            data-voice-trim-destructive: "true"
                                            data-scene: "{scene_text_modal_id}"
                                            data-hint: "{t_voice_trim_destructive_hint}"
                                            svg.voice-rec-btn-icon src: /icons/scissors.svg
                                        button.voice-rec-action-btn.voice-rec-rerecord-btn .hidden?: {!scene_text_modal_has_voice}
                                            type: button
                                            data-voice-record-start: "true"
                                            svg.voice-rec-btn-icon src: /icons/mic.svg
                                        button.voice-rec-action-btn.voice-rec-delete-btn .hidden?: {!scene_text_modal_has_voice}
                                            type: button
                                            action: DeleteSceneVoice
                                            data-scene: "{scene_text_modal_id}"
                                            svg.voice-rec-btn-icon src: /icons/trash.svg
                        {if scene_text_modal_has_image}
                            .scene-illustration-spacer
                                img.scene-portrait-img
                                    alt: "{scene_text_modal_name}"
                                    src: {scene_text_modal_image_url}
                        .scene-body-block.scene-text-editor-body
                            {if is_screenplay_mode}
                                .modal-screenplay-meta-bar
                                    .modal-slugline-row
                                        span.node-meta-key {t_sp_scene_upper_key}
                                        input.node-meta-input.node-slugline-input#modal-slugline-input
                                            type: text
                                            name: slugline
                                            value: "{scene_text_modal_slugline}"
                                            placeholder: "{t_slugline_placeholder}"
                                            disabled?: {is_history_preview}
                                            readonly?: {is_history_preview}
                                        {if scene_text_modal_length_label}
                                            span.node-script-length-badge {scene_text_modal_length_label}
                                    {if not is_history_preview}
                                        .slugline-helper-pills
                                            {for p in slugline_prefix_pills}
                                                button.slugline-pill-btn
                                                    type: button
                                                    action: ApplySluglineToken
                                                    data-scene: "{scene_text_modal_id}"
                                                    data-prefix: "{p.token}"
                                                    {p.token}
                                            {for t in slugline_time_pills}
                                                button.slugline-pill-btn
                                                    type: button
                                                    action: ApplySluglineToken
                                                    data-scene: "{scene_text_modal_id}"
                                                    data-time: "{t.token}"
                                                    {t.token}
                                    .modal-slugline-row.node-meta-row-textarea
                                        span.node-meta-key {t_desc_upper_key}
                                        textarea.node-meta-input.node-meta-textarea
                                            name: synopsis
                                            rows: 1
                                            placeholder: "{t_synopsis_placeholder}"
                                            disabled?: {is_history_preview}
                                            readonly?: {is_history_preview}
                                            {scene_text_modal_synopsis}
                                {if not is_history_preview}
                                    .modal-screenplay-format-bar#modal-screenplay-format-bar
                                        .modal-format-row
                                            span.node-meta-key {t_insert_upper_key}
                                            .slugline-helper-pills
                                                {for fp in screenplay_insert_pills}
                                                    button.slugline-pill-btn.script-format-pill-btn
                                                        type: button
                                                        action: InsertScreenplayBlock
                                                        data-insert-script-block: "{fp.kind}"
                                                        data-script-standard: "{script_standard}"
                                                        data-scene: "{scene_text_modal_id}"
                                                        data-hint: "{fp.hint}"
                                                        {fp.label}
                                        {if has_screenplay_character_pills}
                                            .modal-format-row
                                                span.node-meta-key {t_char_upper_key}
                                                .slugline-helper-pills
                                                    {for cp in screenplay_character_pills}
                                                        button.slugline-pill-btn.script-char-pill-btn
                                                            type: button
                                                            action: InsertScreenplayBlock
                                                            data-insert-script-char: "{cp.name}"
                                                            data-scene: "{scene_text_modal_id}"
                                                            data-hint: "{cp.hint}"
                                                            {cp.label}
                            .scene-prose-edit-wrap .hidden?: {!is_scene_preview_normal}
                                .scene-prose.modal-scene-prose-hl#modal-scene-prose-hl .is-screenplay?: {is_screenplay_mode}
                                    aria-hidden: "true"
                                    {for seg in scene_text_modal_hl_segments}
                                        span.prose-hl-seg .is-syntax?: {seg.is_syntax} {seg.text}
                                textarea.scene-prose.modal-scene-textarea#modal-scene-textarea .is-screenplay?: {is_screenplay_mode} .hidden?: {!is_scene_preview_normal}
                                    name: text
                                    rows: 1
                                    spellcheck: "false"
                                    autocorrect: "off"
                                    autocapitalize: "off"
                                    autocomplete: "off"
                                    placeholder: "{t_scene_text_placeholder}"
                                    disabled?: {is_history_preview}
                                    readonly?: {is_history_preview}
                                    {scene_text_modal_text}
                            {if not is_scene_preview_normal}
                                .scene-prose.scene-prose-preview-stage#scene-prose-preview-stage .is-screenplay?: {is_screenplay_mode} .mode-highlight?: {is_scene_preview_highlight} .mode-output?: {is_scene_preview_output}
                                    {if is_scene_preview_output}
                                        {prose scene_text_modal_output_prose}
                                    {else}
                                        {for para in scene_text_modal_preview_paragraphs}
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
                                {for ch in scene_text_modal_choices}
                                    .choice-btn.modal-choice-row .is-rule-expanded?: {ch.is_rule_open} .has-active-rule?: {ch.has_rule} .choice-cond-unmet?: {ch.is_cond_unmet} .choice-locked?: {ch.is_locked_preview}
                                        {if not is_history_preview}
                                            span.choice-drag-grip
                                                data-choice-grip: "true"
                                                data-choice-idx: "{ch.idx}"
                                                svg.choice-grip-icon src: /icons/grip.svg
                                        .choice-triple-stack.modal-choice-triple-stack
                                            .choice-triple-row.choice-triple-exit .is-invalid?: {ch.is_exit_invalid}
                                                data-hint?: {ch.exit_error_hint}
                                                span.choice-row-key "exit:"
                                                input.choice-exit-input
                                                    type: text
                                                    name: "choice_target_{ch.idx}"
                                                    data-choice-idx: "{ch.idx}"
                                                    value: "{ch.selected_value}"
                                                    placeholder: "{t_exit_target_placeholder}"
                                                    spellcheck: "false"
                                                    autocorrect: "off"
                                                    autocapitalize: "off"
                                                    autocomplete: "off"
                                                    data-hint?: {ch.exit_error_hint}
                                                    disabled?: {is_history_preview}
                                                    readonly?: {is_history_preview}
                                            .choice-triple-row.choice-triple-label
                                                span.choice-row-key "text:"
                                                textarea.node-meta-input.node-meta-textarea.choice-label-text.node-inline-title
                                                    name: "choice_label_{ch.idx}"
                                                    data-choice-idx: "{ch.idx}"
                                                    rows: 1
                                                    placeholder: "{t_choice_text_placeholder}"
                                                    disabled?: {is_history_preview}
                                                    readonly?: {is_history_preview}
                                                    {ch.label}
                                            {if ch.is_rule_open}
                                                .choice-triple-row.choice-triple-cond .is-empty-rule?: {!ch.has_condition} .is-invalid?: {ch.is_cond_invalid}
                                                    data-hint?: {ch.cond_error_hint}
                                                    span.choice-row-key "if:"
                                                    input.choice-rule-input
                                                        type: text
                                                        name: "choice_cond_{ch.idx}"
                                                        data-choice-idx: "{ch.idx}"
                                                        value: "{ch.condition}"
                                                        placeholder: ""
                                                        spellcheck: "false"
                                                        autocorrect: "off"
                                                        autocapitalize: "off"
                                                        autocomplete: "off"
                                                        data-hint?: {ch.cond_error_hint}
                                                        disabled?: {is_history_preview}
                                                        readonly?: {is_history_preview}
                                                .choice-triple-row.choice-triple-eff .is-empty-rule?: {!ch.has_effects} .is-invalid?: {ch.is_eff_invalid}
                                                    data-hint?: {ch.eff_error_hint}
                                                    span.choice-row-key "set:"
                                                    input.choice-rule-input
                                                        type: text
                                                        name: "choice_effects_{ch.idx}"
                                                        data-choice-idx: "{ch.idx}"
                                                        value: "{ch.effects}"
                                                        placeholder: ""
                                                        spellcheck: "false"
                                                        autocorrect: "off"
                                                        autocapitalize: "off"
                                                        autocomplete: "off"
                                                        data-hint?: {ch.eff_error_hint}
                                                        disabled?: {is_history_preview}
                                                        readonly?: {is_history_preview}
                                            {else}
                                                input
                                                    type: hidden
                                                    name: "choice_cond_{ch.idx}"
                                                    value: "{ch.condition}"
                                                input
                                                    type: hidden
                                                    name: "choice_effects_{ch.idx}"
                                                    value: "{ch.effects}"
                                        .modal-choice-controls
                                            {if not is_history_preview}
                                                button.node-icon-btn.node-choice-btn.node-choice-remove
                                                    type: button
                                                    action: PromptRemoveChoice
                                                    data-scene: "{scene_text_modal_id}"
                                                    data-choice-idx: "{ch.idx}"
                                                    svg.choice-trash-icon src: /icons/trash.svg
                                            button.contextual-rewind-btn.modal-choice-nav-btn
                                                type: button
                                                action: NavigateSceneTextModal
                                                data-scene: "{ch.target_scene_id}"
                                                data-choice-idx: "{ch.idx}"
                                                data-hint: "{t_go_to_target_scene_hint}"
                                                @scene_choice_arrow
                                {if not is_history_preview}
                                    button.node-icon-btn.node-media-add-btn.modal-add-choice-btn
                                        type: button
                                        action: AddChoice
                                        data-scene: "{scene_text_modal_id}"
                                        svg.media-plus-icon src: /icons/plus.svg
                            .syn-autocomplete-dropdown#modal-syn-autocomplete-dropdown .hidden?: {!show_syntax_help_popover}
                                {if show_syntax_help_popover}
                                    .syn-dropdown-help-box#modal-syntax-help-popover
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
                .scroll-more-cue.hidden#modal-scroll-more-cue
                    span.scroll-more-dots "..."

{if show_editor_help_modal}
    .modal-backdrop
        data-backdrop-action: CloseEditorHelpModal
        .modal-card.modal-card-lg
            .modal-header
                h3.modal-title {t_help_panel_title}
                button.node-close-btn
                    type: button
                    action: CloseEditorHelpModal
                    svg.node-close-icon src: /icons/close.svg
            .modal-body
                .help-shortcuts-list
                    .help-shortcut-row
                        span.help-shortcut-key {t_help_m_k1}
                        span.help-shortcut-desc {t_help_m_d1}
                    .help-shortcut-row
                        span.help-shortcut-key {t_help_m_k2}
                        span.help-shortcut-desc {t_help_m_d2}
                    .help-shortcut-row
                        span.help-shortcut-key {t_help_m_k3}
                        span.help-shortcut-desc {t_help_m_d3}
                    .help-shortcut-row
                        span.help-shortcut-key {t_help_m_k4}
                        span.help-shortcut-desc {t_help_m_d4}
                    .help-shortcut-row
                        span.help-shortcut-key {t_help_m_k5}
                        span.help-shortcut-desc {t_help_m_d5}
                    .help-shortcut-row
                        span.help-shortcut-key {t_help_m_k6}
                        span.help-shortcut-desc {t_help_m_d6}
                    .help-shortcut-row
                        span.help-shortcut-key {t_help_m_k7}
                        span.help-shortcut-desc {t_help_m_d7}
                    .help-shortcut-row
                        span.help-shortcut-key {t_help_m_k8}
                        span.help-shortcut-desc {t_help_m_d8}
                .modal-actions
                    button.btn.btn-primary
                        type: button
                        action: CloseEditorHelpModal
                        "{t_got_it_btn}"

{if show_confirm_modal}
    .modal-backdrop#modal-confirm
        data-backdrop-action: ConfirmModalClose
        .modal-card
            .modal-header
                h3.modal-title {confirm_title}
            .modal-body
                p.modal-desc {confirm_message}
                .modal-actions
                    button.btn
                        type: button
                        action: ConfirmModalClose
                        "{t_cancel}"
                    button.btn .btn-danger?: {confirm_is_danger} .btn-primary?: {!confirm_is_danger}
                        type: button
                        action: ConfirmModalAccept
                        "{t_confirm_btn}"
