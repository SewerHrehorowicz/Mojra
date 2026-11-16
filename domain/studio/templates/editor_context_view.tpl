aside.editor-context-panel#editor-context-view .hidden?: {IsContextPanelHidden}
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
                            svg.node-close-icon src: /icons/close.svg
        {if not IsHistoryPreview}
            .context-danger-zone
                button.btn.btn-sm.context-delete-btn#btn-delete-selected-scenes
                    data-scenes: ""
                    svg.btn-icon src: /icons/trash.svg
                    span#btn-delete-selected-scenes-label {t_delete_selected_scenes}
