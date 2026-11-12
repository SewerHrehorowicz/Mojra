{if show_export_modal}
    .modal-backdrop#modal-backdrop-export
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
