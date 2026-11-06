{if show_qr_modal}
    .modal-backdrop
        data-backdrop-action: CloseQRModal
        .modal-card
            .modal-header
                h3.modal-title "Ustawienia"
            .modal-body.qr-modal-body
                .node-meta-fields.modal-meta-fields
                    .node-meta-row
                        span.node-meta-key "motyw:"
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
                        span.node-meta-key "obrazy:"
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
                        span.node-meta-key "skala:"
                        .form-radio-group.is-inline
                            button.form-radio-option .active?: {is_small_ui}
                                type: button
                                action: SetUIScale
                                data-scale: small
                                span.form-radio-label "Domyślna"
                                span.form-radio-sq
                            button.form-radio-option .active?: {is_large_ui}
                                type: button
                                action: SetUIScale
                                data-scale: large
                                span.form-radio-label "Powiększona"
                                span.form-radio-sq
                            button.form-radio-option .active?: {is_zen_ui}
                                type: button
                                action: SetUIScale
                                data-scale: zen
                                span.form-radio-label "Zen"
                                span.form-radio-sq
                {if dev_mode}
                    .settings-modal-actions
                        button.btn.btn-sm
                            type: button
                            action: ExtractEmbeddedBook
                            "Wypakuj wbudowaną książkę"
                    p.modal-desc "Zeskanuj kod QR aparatem telefonu w tej samej sieci Wi-Fi:"
                    .qr-frame
                        img.qr-image
                            src: /api/lan-qr.png
                            alt: "Kod QR połączenia LAN"
                    .lan-url-box {lan_url}
                .modal-actions
                    button.btn
                        type: button
                        action: CloseQRModal
                        "Zamknij"

{if show_new_book_modal}
    .modal-backdrop
        data-backdrop-action: CloseNewBookModal
        .modal-card
            .modal-header
                {if new_book_is_screenplay}
                    h3.modal-title#new-book-modal-title "Utwórz Nowy Scenariusz"
                {else}
                    h3.modal-title#new-book-modal-title "Utwórz Nową Książkę"
            form.modal-body#form-new-book
                action: CreateNewBook
                .node-meta-fields.modal-meta-fields
                    .node-meta-row
                        span.node-meta-key#new-book-title-label "tytuł:"
                        input.node-meta-input.field-input#new-book-title
                            type: text
                            name: title
                            value: "{new_book_title}"
                            placeholder: "{new_book_title_placeholder}"
                    .node-meta-row
                        span.node-meta-key "autor:"
                        input.node-meta-input.field-input#new-book-author
                            type: text
                            name: author
                            value: "{new_book_author}"
                            placeholder: "Autor"
                    .node-meta-row
                        span.node-meta-key "tryb:"
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
                        span.node-meta-key "norma:"
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
                        span.node-meta-key "narracja:"
                        input
                            type: hidden
                            name: narrated
                            value: "{new_book_narrated_val}"
                        button.book-setting-toggle-btn.form-toggle-btn .active?: {new_book_narrated}
                            type: button
                            data-form-toggle: narrated
                            data-active-when: "true"
                            data-label-on: "Wymagana"
                            data-label-off: "Wyłączona"
                            span.toggle-right
                                span.toggle-switch
                                    span.toggle-thumb
                                svg.action-icon.toggle-icon-on .hidden?: {!new_book_narrated} src: /icons/mic.svg
                                svg.action-icon.toggle-icon-off .hidden?: {new_book_narrated} src: /icons/mic-off.svg
                            span.custom-select-label {new_book_narrated_short_label}
                    .node-meta-row#new-book-imageless-field .hidden?: {new_book_is_screenplay}
                        span.node-meta-key "obrazy:"
                        input
                            type: hidden
                            name: imageless
                            value: "{new_book_imageless_val}"
                        button.book-setting-toggle-btn.form-toggle-btn .active?: {!new_book_imageless}
                            type: button
                            data-form-toggle: imageless
                            data-active-when: "false"
                            data-label-on: "Z obrazami"
                            data-label-off: "Bez obrazów"
                            span.toggle-right
                                span.toggle-switch
                                    span.toggle-thumb
                                svg.action-icon.toggle-icon-on .hidden?: {new_book_imageless} src: /icons/image.svg
                                svg.action-icon.toggle-icon-off .hidden?: {!new_book_imageless} src: /icons/image-off.svg
                            span.custom-select-label {new_book_imageless_short_label}
                    .node-meta-row
                        span.node-meta-key#new-book-location-label "folder:"
                        .node-media-picker-row
                            input.node-meta-input.field-input#new-book-path
                                type: text
                                name: save_path
                                value: "{new_book_default_dir}"
                                placeholder: "{new_book_default_dir}"
                            button.node-icon-btn.node-folder-btn
                                type: button
                                action: BrowseNewBookFolder
                                data-hint: "Wybierz folder"
                                svg.folder-browse-icon src: /icons/folder.svg
                .modal-actions
                    button.btn
                        type: button
                        action: CloseNewBookModal
                        "Anuluj"
                    button.btn.btn-primary#new-book-submit-btn
                        type: submit
                        {if new_book_is_screenplay}
                            "Utwórz scenariusz"
                        {else}
                            "Utwórz książkę"

{if show_export_modal}
    .modal-backdrop
        data-backdrop-action: CloseExportModal
        .modal-card#modal-export-project
            .modal-header
                h3.modal-title "Eksport projektu"
            form.modal-body#form-export-project
                action: ExportProject
                .node-meta-fields.modal-meta-fields
                    .node-meta-row
                        span.node-meta-key "format:"
                        .custom-select.node-media-select.export-format-select
                            input.custom-select-input
                                type: hidden
                                name: format
                                value: "{export_modal_format}"
                            button.custom-select-trigger.modal-field-select-trigger
                                type: button
                                data-select-toggle: "true"
                                span.custom-select-label {export_modal_format_label}
                                svg.custom-select-caret src: /icons/chevron-down.svg
                            .custom-select-menu.hidden
                                {for fmt_opt in export_modal_format_options}
                                    button.custom-select-option .active?: {fmt_opt.is_active}
                                        type: button
                                        data-select-value: "{fmt_opt.value}"
                                        data-select-label: "{fmt_opt.label}"
                                        {fmt_opt.label}
                    .node-meta-row
                        span.node-meta-key "folder:"
                        .node-media-picker-row
                            input.node-meta-input.field-input#export-modal-path
                                type: text
                                name: export_dir
                                value: "{export_modal_dir}"
                                placeholder: "{default_export_dir}"
                            button.node-icon-btn.node-folder-btn
                                type: button
                                action: BrowseExportModalFolder
                                data-hint: "Wybierz folder"
                                svg.folder-browse-icon src: /icons/folder.svg
                .modal-actions
                    button.btn
                        type: button
                        action: CloseExportModal
                        "Anuluj"
                    button.btn.btn-primary#export-modal-submit-btn
                        type: submit
                        "Eksportuj"

{if show_add_book_modal}
    .modal-backdrop
        data-backdrop-action: CloseAddBookModal
        .modal-card.modal-card-lg
            .modal-header
                h3.modal-title "Dodaj książkę do biblioteki"
                button.node-close-btn
                    type: button
                    action: CloseAddBookModal
                    svg.node-close-icon src: /icons/close.svg
            .modal-body
                .add-book-section
                    .field-label "1. Z dysku (bez kopiowania plików)"
                    p.modal-desc "Wskaż istniejący folder książki lub archiwum .playbook / .zip na dysku. Książka pozostanie w oryginalnej lokalizacji."
                    .add-book-browse-row
                        button.btn.btn-sm
                            type: button
                            action: BrowseAddBookFromDisk
                            data-mode: dir
                            "Wybierz folder z dysku..."
                        button.btn.btn-sm
                            type: button
                            action: BrowseAddBookFromDisk
                            data-mode: file
                            "Wybierz plik .playbook / .zip..."
                    form.add-book-inline-form
                        action: AddBookFromDisk
                        input.field-input
                            type: text
                            name: path
                            required: true
                            placeholder: "/ścieżka/do/folderu-książki lub /ścieżka/do/pliku.playbook"
                        button.btn.btn-primary
                            type: submit
                            "Dodaj z dysku"
                .add-book-section
                    .field-label "2. Pobierz z adresu URL (.playbook / .zip)"
                    p.modal-desc "Podaj bezpośredni link HTTP/HTTPS do paczki książki (.playbook lub .zip), aby pobrać ją do biblioteki."
                    form.add-book-inline-form
                        action: AddBookFromURL
                        input.field-input
                            type: url
                            name: url
                            required: true
                            placeholder: "https://example.com/opowiesc.playbook lub .zip"
                        button.btn.btn-primary
                            type: submit
                            "Pobierz z URL"
                .modal-actions
                    button.btn
                        type: button
                        action: CloseAddBookModal
                        "Zamknij"

{if show_studio_project_modal}
    .modal-backdrop
        data-backdrop-action: CloseStudioProjectModal
        .modal-card.modal-card-lg#modal-studio-project
            .modal-header
                h3.modal-title "Otwórz projekt w Studio"
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
                            "Wybierz folder projektu..."
                        button.btn.btn-sm
                            type: button
                            action: BrowseOpenStudioProject
                            data-mode: file
                            "Wypakuj kopię z .playbook..."
                    form.add-book-inline-form
                        action: OpenStudioProjectFromDisk
                        input.field-input#studio-project-path-input
                            type: text
                            name: path
                            required: true
                            placeholder: "/ścieżka/do/projektu lub /ścieżka/do/pliku.playbook"
                        button.btn.btn-primary
                            type: submit
                            "Otwórz"
                .modal-actions
                    button.btn
                        type: button
                        action: CloseStudioProjectModal
                        "Zamknij"

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
                                            data-hint: "Ta scena jest punktem startowym (korzeniem) opowieści"
                                            svg.scene-star-icon src: /icons/star.svg
                                    input.scene-title.scene-title-present.node-inline-title
                                        type: text
                                        name: name
                                        value: "{scene_text_modal_raw_name}"
                                        placeholder: "Nazwa sceny (opcjonalna przy ID)..."
                                        disabled?: {is_history_preview}
                                        readonly?: {is_history_preview}
                                {if is_history_preview}
                                    span.history-step-badge "PODGLĄD (TYLKO DO ODCZYTU)"
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
                                        button.contextual-rewind-btn.scene-settings-toggle-btn .active?: {scene_text_modal_settings_open}
                                            type: button
                                            data-scene-settings-toggle: "true"
                                            data-hint: "Ustawienia sceny"
                                            svg.scene-settings-icon src: /icons/settings.svg
                            {if show_scene_test_vars_bar}
                                .scene-test-vars-popover#scene-test-vars-bar
                                    .history-panel-header.scene-test-vars-header
                                        span.history-panel-title "STAN TESTOWY"
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
                                        span.scene-test-vars-empty "Brak zmiennych w projekcie"
                            .scene-settings-block#modal-scene-settings-block .hidden?: {!scene_text_modal_settings_open}
                                .scene-settings-row
                                    span.node-meta-key {scene_text_modal_chapter_key}
                                    .custom-select.node-chapter-select
                                        .node-chapter-input-wrap
                                            input.node-inline-chapter.custom-select-input
                                                type: text
                                                name: chapter
                                                value: "{scene_text_modal_chapter}"
                                                placeholder: "Rozdział..."
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
                                        placeholder: "id sceny (np. warsztat)"
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
                        {if scene_text_modal_has_image}
                            .scene-illustration-spacer
                                img.scene-portrait-img
                                    alt: "{scene_text_modal_name}"
                                    src: {scene_text_modal_image_url}
                        .scene-body-block.scene-text-editor-body
                            {if is_screenplay_mode}
                                .modal-screenplay-meta-bar
                                    .modal-slugline-row
                                        span.node-meta-key "SCENA:"
                                        input.node-meta-input.node-slugline-input#modal-slugline-input
                                            type: text
                                            name: slugline
                                            value: "{scene_text_modal_slugline}"
                                            placeholder: "WN. LOKACJA - DZIEŃ"
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
                                        span.node-meta-key "OPIS:"
                                        textarea.node-meta-input.node-meta-textarea
                                            name: synopsis
                                            rows: 1
                                            placeholder: "Streszczenie sceny (1 zdanie)..."
                                            disabled?: {is_history_preview}
                                            readonly?: {is_history_preview}
                                            {scene_text_modal_synopsis}
                                {if not is_history_preview}
                                    .modal-screenplay-format-bar#modal-screenplay-format-bar
                                        .modal-format-row
                                            span.node-meta-key "WSTAW:"
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
                                                span.node-meta-key "POSTAĆ:"
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
                                    placeholder: "Wpisz treść sceny..."
                                    disabled?: {is_history_preview}
                                    readonly?: {is_history_preview}
                                    {scene_text_modal_text}
                            {if not is_scene_preview_normal}
                                .scene-prose.scene-prose-preview-stage#scene-prose-preview-stage .is-screenplay?: {is_screenplay_mode} .mode-highlight?: {is_scene_preview_highlight} .mode-output?: {is_scene_preview_output}
                                    {if is_scene_preview_output}
                                        {raw scene_text_modal_output_html}
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
                                            .choice-triple-row.choice-triple-exit
                                                span.choice-row-key "exit:"
                                                input.choice-exit-input
                                                    type: text
                                                    name: "choice_target_{ch.idx}"
                                                    data-choice-idx: "{ch.idx}"
                                                    value: "{ch.selected_value}"
                                                    placeholder: "cel (id lub END)"
                                                    spellcheck: "false"
                                                    autocorrect: "off"
                                                    autocapitalize: "off"
                                                    autocomplete: "off"
                                                    disabled?: {is_history_preview}
                                                    readonly?: {is_history_preview}
                                            .choice-triple-row.choice-triple-label
                                                span.choice-row-key "text:"
                                                textarea.node-meta-input.node-meta-textarea.choice-label-text.node-inline-title
                                                    name: "choice_label_{ch.idx}"
                                                    data-choice-idx: "{ch.idx}"
                                                    rows: 1
                                                    placeholder: "Tekst wyboru..."
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
                                                data-hint: "Przejdź do sceny docelowej"
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
                                            data-syn-target-value: "{tsug.value}"
                                            span.syn-dropdown-var-name {tsug.value}
                                            span.syn-dropdown-var-meta {tsug.meta}
                .scroll-more-cue.hidden#modal-scroll-more-cue
                    span.scroll-more-dots "..."

{if show_editor_help_modal}
    .modal-backdrop
        data-backdrop-action: CloseEditorHelpModal
        .modal-card.modal-card-lg
            .modal-header
                h3.modal-title "Pomoc — Playbook Studio"
                button.node-close-btn
                    type: button
                    action: CloseEditorHelpModal
                    svg.node-close-icon src: /icons/close.svg
            .modal-body
                .help-shortcuts-list
                    .help-shortcut-row
                        span.help-shortcut-key "Chwyć uchwyt (⋮⋮)"
                        span.help-shortcut-desc "Przesuwa węzeł sceny na płótnie z automatycznym przyciąganiem do siatki (24 px). Kliknięcie węzła otwiera jego szczegóły w panelu po lewej."
                    .help-shortcut-row
                        span.help-shortcut-key "Przeciągnięcie po tle / Shift + Klik"
                        span.help-shortcut-desc "Zaznacza wiele węzłów naraz (ramka zaznaczenia lub dodawanie pojedynczych węzłów). Chwycenie jednego zaznaczonego węzła przesuwa całą grupę."
                    .help-shortcut-row
                        span.help-shortcut-key "Grupy rozdziałów"
                        span.help-shortcut-desc "Każdy rozdział tworzy ramkę grupy wokół swoich scen. Przeciągnij nagłówek rozdziału, aby przesunąć wszystkie jego sceny, zmień nazwę rozdziału w nagłówku lub kliknij „+ Scena”."
                    .help-shortcut-row
                        span.help-shortcut-key "2× Klik / PPM na tło"
                        span.help-shortcut-desc "Tworzy nową scenę we wskazanym miejscu płótna lub otwiera menu kontekstowe."
                    .help-shortcut-row
                        span.help-shortcut-key "Porty wyjściowe (dół / prawo)"
                        span.help-shortcut-desc "Scena bez rozwidleń posiada dolny port wyjściowy (domyślne przejście „Dalej”). Po dodaniu wyborów (+ Wybór, min. 2) porty wyjściowe pojawiają się po prawej stronie każdego wyboru."
                    .help-shortcut-row
                        span.help-shortcut-key "Spacja + przeciągnięcie / ŚPM"
                        span.help-shortcut-desc "Przesuwa widok płótna (Pan). Możesz również przesuwać widok prawym przyciskiem myszy (PPM + przeciągnięcie)."
                    .help-shortcut-row
                        span.help-shortcut-key "Rolka myszy / Gest"
                        span.help-shortcut-desc "Przybliża i oddala widok płótna (Zoom) względem kursora."
                    .help-shortcut-row
                        span.help-shortcut-key "Auto-układ"
                        span.help-shortcut-desc "Automatycznie porządkuje wszystkie sceny i rozdziały w przejrzyste drzewo na siatce."
                .modal-actions
                    button.btn.btn-primary
                        type: button
                        action: CloseEditorHelpModal
                        "Rozumiem"

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
                        "Anuluj"
                    button.btn .btn-danger?: {confirm_is_danger} .btn-primary?: {!confirm_is_danger}
                        type: button
                        action: ConfirmModalAccept
                        "Potwierdź"
