{if open}
    .modal-backdrop#modal-backdrop-new-book
        data-backdrop-action: CloseNewBookModal
        .modal-card
            .modal-header
                {if mode == "screenplay"}
                    h3.modal-title#new-book-modal-title {t_new_screenplay_modal_title}
                {else}
                    h3.modal-title#new-book-modal-title {t_new_book_modal_title}
            form.modal-body#form-new-book
                action: CreateNewBook
                .node-meta-fields.modal-meta-fields
                    .node-meta-row
                        span.node-meta-key#new-book-title-label {t_title_key}
                        {if mode == "screenplay"}
                            input.node-meta-input.field-input#new-book-title
                                type: text
                                name: title
                                value: "{title}"
                                placeholder: "{t_new_screenplay_title_placeholder}"
                        {else}
                            input.node-meta-input.field-input#new-book-title
                                type: text
                                name: title
                                value: "{title}"
                                placeholder: "{t_new_book_title_placeholder}"
                    .node-meta-row
                        span.node-meta-key {t_author_key}
                        input.node-meta-input.field-input#new-book-author
                            type: text
                            name: author
                            value: "{author}"
                            placeholder: "{t_author_placeholder}"
                    .node-meta-row
                        span.node-meta-key {t_mode_key}
                        .custom-select.node-media-select.new-book-mode-select
                            input.custom-select-input
                                type: hidden
                                name: mode
                                value: "{mode}"
                            button.custom-select-trigger.modal-field-select-trigger
                                type: button
                                data-select-toggle: "true"
                                span.custom-select-label {mode_label}
                                svg.custom-select-caret src: /icons/chevron-down.svg
                            .custom-select-menu.hidden
                                {for md_opt in mode_options}
                                    button.custom-select-option .active?: {md_opt.is_active}
                                        type: button
                                        data-select-value: "{md_opt.value}"
                                        data-select-label: "{md_opt.label}"
                                        {md_opt.label}
                    .node-meta-row.node-meta-row-radio#new-book-standard-field .hidden?: {mode != "screenplay"}
                        span.node-meta-key {t_standard_key}
                        .form-radio-group
                            {for std_opt in standard_options}
                                label.form-radio-option .active?: {std_opt.is_active}
                                    input.form-radio-input
                                        type: radio
                                        name: standard
                                        value: "{std_opt.value}"
                                        checked?: {std_opt.is_active}
                                    span.form-radio-label {std_opt.label}
                                    span.form-radio-sq
                    .node-meta-row#new-book-narrated-field .hidden?: {mode == "screenplay"}
                        span.node-meta-key {t_narration_key}
                        input
                            type: hidden
                            name: narrated
                            value: "{narrated}"
                        button.book-setting-toggle-btn.form-toggle-btn .active?: {narrated}
                            type: button
                            data-form-toggle: narrated
                            data-active-when: "true"
                            data-label-on: "{t_required_label}"
                            data-label-off: "{t_disabled_label}"
                            span.toggle-right
                                span.toggle-switch
                                    span.toggle-thumb
                                svg.action-icon.toggle-icon-on .hidden?: {!narrated} src: /icons/mic.svg
                                svg.action-icon.toggle-icon-off .hidden?: {narrated} src: /icons/mic-off.svg
                            span.custom-select-label
                                {if narrated}
                                    "{t_required_label}"
                                {else}
                                    "{t_disabled_label}"
                    .node-meta-row#new-book-imageless-field .hidden?: {mode == "screenplay"}
                        span.node-meta-key {t_images_key}
                        input
                            type: hidden
                            name: imageless
                            value: "{imageless}"
                        button.book-setting-toggle-btn.form-toggle-btn .active?: {!imageless}
                            type: button
                            data-form-toggle: imageless
                            data-active-when: "false"
                            data-label-on: "{t_with_images_label}"
                            data-label-off: "{t_no_images_label}"
                            span.toggle-right
                                span.toggle-switch
                                    span.toggle-thumb
                                svg.action-icon.toggle-icon-on .hidden?: {imageless} src: /icons/image.svg
                                svg.action-icon.toggle-icon-off .hidden?: {!imageless} src: /icons/image-off.svg
                            span.custom-select-label
                                {if imageless}
                                    "{t_no_images_label}"
                                {else}
                                    "{t_with_images_label}"
                    .node-meta-row
                        span.node-meta-key#new-book-location-label {t_folder_key}
                        .node-media-picker-row
                            input.node-meta-input.field-input#new-book-path
                                type: text
                                name: save_path
                                value: "{dir}"
                                placeholder: "{dir}"
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
                        {if mode == "screenplay"}
                            "{t_create_screenplay_btn}"
                        {else}
                            "{t_create_book_btn}"
