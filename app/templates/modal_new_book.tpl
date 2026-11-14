{if Open}
    .modal-backdrop#modal-backdrop-new-book
        .modal-card
            .modal-header
                {if Mode == "screenplay"}
                    h3.modal-title#new-book-modal-title {t_new_screenplay_modal_title}
                {else}
                    h3.modal-title#new-book-modal-title {t_new_book_modal_title}
            form.modal-body#form-new-book
                .node-meta-fields.modal-meta-fields
                    .node-meta-row
                        span.node-meta-key#new-book-title-label {t_title_key}
                        {if Mode == "screenplay"}
                            input.node-meta-input.field-input#new-book-title
                                type: text
                                name: title
                                value: "{Title}"
                                placeholder: "{t_new_screenplay_title_placeholder}"
                        {else}
                            input.node-meta-input.field-input#new-book-title
                                type: text
                                name: title
                                value: "{Title}"
                                placeholder: "{t_new_book_title_placeholder}"
                    .node-meta-row
                        span.node-meta-key {t_author_key}
                        input.node-meta-input.field-input#new-book-author
                            type: text
                            name: author
                            value: "{Author}"
                            placeholder: "{t_author_placeholder}"
                    .node-meta-row
                        span.node-meta-key {t_mode_key}
                        .custom-select.node-media-select.new-book-mode-select
                            data-select-root: "true"
                            input
                                type: hidden
                                name: mode
                                value: "{Mode}"
                            button.custom-select-trigger.modal-field-select-trigger
                                data-select-toggle: "true"
                                span.custom-select-label {ModeLabel}
                                svg.custom-select-caret src: /icons/chevron-down.svg
                            .custom-select-menu.hidden
                                data-select-menu: "true"
                                {for md_opt in ModeOptions}
                                    button.custom-select-option .active?: {md_opt.IsActive}
                                        data-select-value: "{md_opt.Value}"
                                        data-select-label: "{md_opt.Label}"
                                        {md_opt.Label}
                    .node-meta-row.node-meta-row-radio#new-book-standard-field .hidden?: {Mode != "screenplay"}
                        span.node-meta-key {t_standard_key}
                        .form-radio-group
                            {for std_opt in StandardOptions}
                                label.form-radio-option .active?: {std_opt.IsActive}
                                    data-radio-option: "true"
                                    input.form-radio-input
                                        type: radio
                                        name: standard
                                        value: "{std_opt.Value}"
                                        checked?: {std_opt.IsActive}
                                    span.form-radio-label {std_opt.Label}
                                    span.form-radio-sq
                    .node-meta-row#new-book-narrated-field .hidden?: {Mode == "screenplay"}
                        span.node-meta-key {t_narration_key}
                        input
                            type: hidden
                            name: narrated
                            value: "{Narrated}"
                        button.book-setting-toggle-btn.form-toggle-btn .active?: {Narrated}
                            data-form-toggle: narrated
                            data-active-when: "true"
                            data-label-on: "{t_required_label}"
                            data-label-off: "{t_disabled_label}"
                            span.toggle-right
                                span.toggle-switch
                                    span.toggle-thumb
                                svg.action-icon.toggle-icon-on .hidden?: {!Narrated} src: /icons/mic.svg
                                svg.action-icon.toggle-icon-off .hidden?: {Narrated} src: /icons/mic-off.svg
                            span.custom-select-label
                                {if Narrated}
                                    "{t_required_label}"
                                {else}
                                    "{t_disabled_label}"
                    .node-meta-row#new-book-imageless-field .hidden?: {Mode == "screenplay"}
                        span.node-meta-key {t_images_key}
                        input
                            type: hidden
                            name: imageless
                            value: "{Imageless}"
                        button.book-setting-toggle-btn.form-toggle-btn .active?: {!Imageless}
                            data-form-toggle: imageless
                            data-active-when: "false"
                            data-label-on: "{t_with_images_label}"
                            data-label-off: "{t_no_images_label}"
                            span.toggle-right
                                span.toggle-switch
                                    span.toggle-thumb
                                svg.action-icon.toggle-icon-on .hidden?: {Imageless} src: /icons/image.svg
                                svg.action-icon.toggle-icon-off .hidden?: {!Imageless} src: /icons/image-off.svg
                            span.custom-select-label
                                {if Imageless}
                                    "{t_no_images_label}"
                                {else}
                                    "{t_with_images_label}"
                    .node-meta-row
                        span.node-meta-key#new-book-location-label {t_folder_key}
                        .node-media-picker-row
                            input.node-meta-input.field-input#new-book-path
                                type: text
                                name: save_path
                                value: "{Dir}"
                                placeholder: "{Dir}"
                            button.node-icon-btn.node-folder-btn#btn-browse-new-book-folder
                                data-hint: "{t_choose_folder_hint}"
                                svg.folder-browse-icon src: /icons/folder.svg
                .modal-actions
                    button.btn.btn-close-new-book
                        "{t_cancel}"
                    button.btn.btn-primary#new-book-submit-btn
                        type: submit
                        {if Mode == "screenplay"}
                            "{t_create_screenplay_btn}"
                        {else}
                            "{t_create_book_btn}"
