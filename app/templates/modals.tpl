{if show_qr_modal}
    .modal-backdrop
        data-backdrop-action: CloseQRModal
        .modal-card
            .modal-header
                h3.modal-title "Ustawienia"
            .modal-body.qr-modal-body
                .settings-modal-actions
                    button.btn.btn-sm
                        type: button
                        action: ToggleTheme
                        "Motyw: {theme_toggle_label}"
                    button.btn.btn-sm
                        type: button
                        action: ToggleImageless
                        disabled?: {imageless_locked}
                        "Obrazy: {imageless_toggle_label}"
                    button.btn.btn-sm
                        type: button
                        action: SetUIScale
                        data-scale: small
                        "− Pomniejsz UI"
                    button.btn.btn-sm
                        type: button
                        action: SetUIScale
                        data-scale: large
                        "+ Powiększ UI"
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
                .form-field
                    {if new_book_is_screenplay}
                        label.field-label#new-book-title-label "Tytuł scenariusza"
                    {else}
                        label.field-label#new-book-title-label "Tytuł opowieści"
                    input.field-input#new-book-title
                        type: text
                        name: title
                        value: "{new_book_title}"
                        placeholder: "{new_book_title_placeholder}"
                .form-field
                    label.field-label "Autor"
                    input.field-input#new-book-author
                        type: text
                        name: author
                        value: "{new_book_author}"
                        placeholder: "Autor"
                .form-field
                    label.field-label "Tryb projektu"
                    .custom-select.node-media-select.new-book-mode-select
                        input.custom-select-input
                            type: hidden
                            name: mode
                            value: "{new_book_mode}"
                        button.custom-select-trigger.modal-field-select-trigger
                            type: button
                            data-select-toggle: "true"
                            span.custom-select-label {new_book_mode_label}
                            span.custom-select-caret "▾"
                        .custom-select-menu.hidden
                            {for md_opt in new_book_mode_options}
                                button.custom-select-option .active?: {md_opt.is_active}
                                    type: button
                                    data-select-value: "{md_opt.value}"
                                    data-select-label: "{md_opt.label}"
                                    {md_opt.label}
                .form-field#new-book-standard-field .hidden?: {!new_book_is_screenplay}
                    label.field-label "Standard scenariusza"
                    .custom-select.node-media-select
                        input.custom-select-input
                            type: hidden
                            name: standard
                            value: "{new_book_standard}"
                        button.custom-select-trigger.modal-field-select-trigger
                            type: button
                            data-select-toggle: "true"
                            span.custom-select-label {new_book_standard_label}
                            span.custom-select-caret "▾"
                        .custom-select-menu.hidden
                            {for std_opt in new_book_standard_options}
                                button.custom-select-option .active?: {std_opt.is_active}
                                    type: button
                                    data-select-value: "{std_opt.value}"
                                    data-select-label: "{std_opt.label}"
                                    {std_opt.label}
                .form-field
                    label.field-label#new-book-location-label "Lokalizacja zapisu na dysku"
                    .add-book-input-row
                        input.field-input#new-book-path
                            type: text
                            name: save_path
                            value: "{new_book_default_dir}"
                            placeholder: "{new_book_default_dir}"
                        button.btn.btn-sm
                            type: button
                            action: BrowseNewBookFolder
                            "Wybierz folder..."
                .form-field.checkbox-field#new-book-narrated-field .hidden?: {new_book_is_screenplay}
                    label.checkbox-label
                        input
                            type: checkbox
                            name: narrated
                            value: true
                            checked?: {new_book_narrated}
                        span "Pełna narracja głosowa (narrated: true — wymaga nagrania voice dla każdej sceny)"
                .form-field.checkbox-field#new-book-imageless-field .hidden?: {new_book_is_screenplay}
                    label.checkbox-label
                        input
                            type: checkbox
                            name: imageless
                            value: true
                            checked?: {new_book_imageless}
                        span "Książka bez obrazów (imageless: true — wymusza tryb bez ilustracji)"
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
                    p.modal-desc "Wskaż istniejący folder książki lub archiwum .stbook / .zip na dysku. Książka pozostanie w oryginalnej lokalizacji."
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
                            "Wybierz plik .stbook / .zip..."
                    form.add-book-inline-form
                        action: AddBookFromDisk
                        input.field-input
                            type: text
                            name: path
                            required: true
                            placeholder: "/ścieżka/do/folderu-książki lub /ścieżka/do/pliku.stbook"
                        button.btn.btn-primary
                            type: submit
                            "Dodaj z dysku"
                .add-book-section
                    .field-label "2. Pobierz z adresu URL (.stbook / .zip)"
                    p.modal-desc "Podaj bezpośredni link HTTP/HTTPS do paczki książki (.stbook lub .zip), aby pobrać ją do biblioteki."
                    form.add-book-inline-form
                        action: AddBookFromURL
                        input.field-input
                            type: url
                            name: url
                            required: true
                            placeholder: "https://example.com/opowiesc.stbook lub .zip"
                        button.btn.btn-primary
                            type: submit
                            "Pobierz z URL"
                .modal-actions
                    button.btn
                        type: button
                        action: CloseAddBookModal
                        "Zamknij"

{if show_scene_text_modal}
    .modal-backdrop.scene-text-fullscreen-backdrop#modal-scene-text
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
                            action: CloseSceneTextModal
                            "← Wróć do edytora"
                .active-scene-stage.scene-text-editor-stage
                    form.scene-card.scene-focused.scene-text-editor-card#form-scene-text .has-illustration?: {scene_text_modal_has_image}
                        action: SaveSceneTextModal
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
                                    button.contextual-rewind-btn.scene-settings-toggle-btn .active?: {scene_text_modal_settings_open}
                                        type: button
                                        data-scene-settings-toggle: "true"
                                        data-hint: "Ustawienia sceny"
                                        svg.scene-settings-icon src: /icons/settings.svg
                            .scene-settings-block#modal-scene-settings-block .hidden?: {!scene_text_modal_settings_open}
                                .scene-settings-row
                                    span.node-meta-key "id:"
                                    input.node-meta-input
                                        type: text
                                        name: explicit_id
                                        value: "{scene_text_modal_explicit_id}"
                                        placeholder: "opcjonalne id (np. warsztat)"
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
                                                span.custom-select-caret "▾"
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
                                                span.custom-select-caret "▾"
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
                                    .modal-slugline-row
                                        span.node-meta-key "OPIS:"
                                        input.node-meta-input
                                            type: text
                                            name: synopsis
                                            value: "{scene_text_modal_synopsis}"
                                            placeholder: "Streszczenie sceny (1 zdanie)..."
                                            disabled?: {is_history_preview}
                                            readonly?: {is_history_preview}
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
                            textarea.scene-prose.modal-scene-textarea#modal-scene-textarea .is-screenplay?: {is_screenplay_mode}
                                name: text
                                rows: 14
                                placeholder: "Wpisz treść sceny..."
                                disabled?: {is_history_preview}
                                readonly?: {is_history_preview}
                                {scene_text_modal_text}
                            .scene-choices-list
                                {if scene_text_modal_is_terminal}
                                    .choice-btn.modal-choice-row
                                        button.modal-choice-lead-btn
                                            type: button
                                            action: NavigateSceneTextModal
                                            data-scene: "{scene_text_modal_start_id}"
                                            span.choice-label-text "Koniec opowieści — Wróć na początek"
                                        .modal-choice-controls
                                            {if not is_history_preview}
                                                .custom-select.node-choice-select.modal-choice-select
                                                    input.custom-select-input
                                                        type: hidden
                                                        name: "choice_target_0"
                                                        value: "{scene_text_modal_terminal_selected_value}"
                                                    button.custom-select-trigger.choice-port-trigger
                                                        type: button
                                                        data-select-toggle: "true"
                                                        data-hint: "{scene_text_modal_terminal_selected_label}"
                                                        span.choice-port-icon .port-filled?: {scene_text_modal_terminal_port_filled}
                                                    .custom-select-menu.hidden
                                                        {for opt in scene_text_modal_terminal_target_options}
                                                            button.custom-select-option .active?: {opt.is_active}
                                                                type: button
                                                                data-select-value: "{opt.value}"
                                                                data-select-label: "{opt.short_label}"
                                                                {opt.label}
                                            button.contextual-rewind-btn.modal-choice-nav-btn
                                                type: button
                                                action: NavigateSceneTextModal
                                                data-scene: "{scene_text_modal_start_id}"
                                                span.choice-arrow "↺"
                                {else}
                                    {for ch in scene_text_modal_choices}
                                        {if ch.is_editable}
                                            .choice-btn.modal-choice-row
                                                input.choice-label-text.node-inline-title
                                                    type: text
                                                    name: "choice_label_{ch.idx}"
                                                    data-choice-idx: "{ch.idx}"
                                                    value: "{ch.label}"
                                                    placeholder: "Tekst wyboru..."
                                                    disabled?: {is_history_preview}
                                                    readonly?: {is_history_preview}
                                                .modal-choice-controls
                                                    {if not is_history_preview}
                                                        .custom-select.node-choice-select.modal-choice-select
                                                            input.custom-select-input
                                                                type: hidden
                                                                name: "choice_target_{ch.idx}"
                                                                value: "{ch.selected_value}"
                                                            button.custom-select-trigger.choice-port-trigger
                                                                type: button
                                                                data-select-toggle: "true"
                                                                data-hint: "{ch.selected_label}"
                                                                span.choice-port-icon .port-filled?: {ch.is_filled}
                                                            .custom-select-menu.hidden
                                                                {for opt in ch.target_options}
                                                                    button.custom-select-option .active?: {opt.is_active}
                                                                        type: button
                                                                        data-select-value: "{opt.value}"
                                                                        data-select-label: "{opt.short_label}"
                                                                        {opt.label}
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
                                        {else}
                                            .choice-btn.modal-choice-row
                                                button.modal-choice-lead-btn
                                                    type: button
                                                    action: NavigateSceneTextModal
                                                    data-scene: "{ch.target_scene_id}"
                                                    data-choice-idx: "{ch.idx}"
                                                    span.choice-label-text {ch.label}
                                                .modal-choice-controls
                                                    {if not is_history_preview}
                                                        .custom-select.node-choice-select.modal-choice-select
                                                            input.custom-select-input
                                                                type: hidden
                                                                name: "choice_target_{ch.idx}"
                                                                value: "{ch.selected_value}"
                                                            button.custom-select-trigger.choice-port-trigger
                                                                type: button
                                                                data-select-toggle: "true"
                                                                data-hint: "{ch.selected_label}"
                                                                span.choice-port-icon .port-filled?: {ch.is_filled}
                                                            .custom-select-menu.hidden
                                                                {for opt in ch.target_options}
                                                                    button.custom-select-option .active?: {opt.is_active}
                                                                        type: button
                                                                        data-select-value: "{opt.value}"
                                                                        data-select-label: "{opt.short_label}"
                                                                        {opt.label}
                                                    button.contextual-rewind-btn.modal-choice-nav-btn
                                                        type: button
                                                        action: NavigateSceneTextModal
                                                        data-scene: "{ch.target_scene_id}"
                                                        data-choice-idx: "{ch.idx}"
                                                        data-hint: "Przejdź do sceny docelowej"
                                                        span.choice-arrow {ch.arrow}
                                {if not is_history_preview}
                                    button.node-icon-btn.node-media-add-btn.modal-add-choice-btn
                                        type: button
                                        action: AddChoice
                                        data-scene: "{scene_text_modal_id}"
                                        svg.media-plus-icon src: /icons/plus.svg
                .scroll-more-cue.hidden#modal-scroll-more-cue
                    span.scroll-more-dots "..."

{if show_editor_help_modal}
    .modal-backdrop
        data-backdrop-action: CloseEditorHelpModal
        .modal-card.modal-card-lg
            .modal-header
                h3.modal-title "Pomoc — Edytor Węzłów (Node Studio)"
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
