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
