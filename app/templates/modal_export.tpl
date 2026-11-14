{if ShowExportModal}
    .modal-backdrop#modal-backdrop-export
        .modal-card#modal-export-project
            .modal-header
                h3.modal-title {t_export_modal_title}
                span.export-selection-count {ExportSelectedCountLabel}
            form.modal-body#form-export-project
                .node-meta-fields.modal-meta-fields
                    .node-meta-row.export-formats-row
                        span.node-meta-key {t_formats_key}
                        .export-format-list
                            {for fmt_item in ExportPrimaryFormats}
                                button.export-format-item .active?: {fmt_item.IsSelected}
                                    data-format: "{fmt_item.ID}"
                                    span.export-format-name {fmt_item.Label}
                                    span.export-format-right
                                        span.export-format-ext {fmt_item.Ext}
                                        span.export-format-check
                    .node-meta-row
                        span.node-meta-key {t_folder_key}
                        .node-media-picker-row
                            input.node-meta-input.field-input#export-modal-path
                                type: text
                                name: export_dir
                                value: "{ExportModalDir}"
                                placeholder: "{DefaultExportDir}"
                            button.node-icon-btn.node-folder-btn#btn-browse-export-folder
                                data-hint: "{t_choose_folder_hint}"
                                svg.folder-browse-icon src: /icons/folder.svg
                button.export-advanced-toggle .open?: {ExportAdvancedOpen}
                    span.export-advanced-label {t_advanced_upper}
                    svg.export-advanced-caret src: /icons/chevron-down.svg
                .export-advanced-body .hidden?: {!ExportAdvancedOpen}
                    .node-meta-fields.modal-meta-fields
                        .node-meta-row.export-formats-row
                            span.node-meta-key
                            .export-format-list
                                {for adv_fmt in ExportAdvancedFormats}
                                    button.export-format-item .active?: {adv_fmt.IsSelected}
                                        data-format: "{adv_fmt.ID}"
                                        span.export-format-name {adv_fmt.Label}
                                        span.export-format-right
                                            span.export-format-ext {adv_fmt.Ext}
                                            span.export-format-check
                        .node-meta-row
                            span.node-meta-key {t_standard_key}
                            .custom-select.node-media-select.export-standard-select
                                data-select-root: "true"
                                button.custom-select-trigger.modal-field-select-trigger
                                    data-select-toggle: "true"
                                    span.custom-select-label {ExportModalStandardLabel}
                                    svg.custom-select-caret src: /icons/chevron-down.svg
                                .custom-select-menu.hidden
                                    data-select-menu: "true"
                                    {for std_opt in ExportModalStandardOptions}
                                        button.custom-select-option .active?: {std_opt.IsActive}
                                            data-select-value: "{std_opt.Value}"
                                            data-select-label: "{std_opt.Label}"
                                            {std_opt.Label}
                .modal-actions
                    button.btn.btn-close-export
                        "{t_cancel}"
                    button.btn.btn-primary#export-modal-submit-btn
                        type: submit
                        disabled?: {!ExportHasSelected}
                        "{t_export_btn} ({ExportSelectedCount})"
