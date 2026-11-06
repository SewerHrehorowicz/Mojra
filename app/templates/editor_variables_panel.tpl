aside.editor-variables-panel#editor-variables-panel .hidden?: {!show_editor_variables_panel}
    .history-panel-header
        h3.history-panel-title "Zmienne"
        button.node-close-btn
            type: button
            action: CloseEditorVariablesPanel
            data-hint: "Zamknij panel zmiennych"
            svg.node-close-icon src: /icons/close.svg

    .history-panel-body.breakdown-panel-body.variables-panel-body
        .breakdown-section
            .breakdown-section-header
                span.breakdown-section-title "bool"
                {if not is_history_preview}
                    button.node-icon-btn.var-section-add-btn
                        type: button
                        action: AddBookVariable
                        data-kind: bool
                        svg.media-plus-icon src: /icons/plus.svg
            {if has_var_flags}
                .breakdown-entity-list
                    {for vf in var_flags}
                        .var-panel-row
                            form.var-panel-inline-form
                                action: UpdateBookVariable
                                input
                                    type: hidden
                                    name: var_idx
                                    value: "{vf.idx}"
                                input.node-meta-input.var-name-input
                                    type: text
                                    name: var_name
                                    value: "{vf.name}"
                                    spellcheck: "false"
                                    autocorrect: "off"
                                    autocapitalize: "off"
                                    autocomplete: "off"
                                    disabled?: {is_history_preview}
                                    readonly?: {is_history_preview}
                            button.var-bool-toggle-btn .active?: {vf.bool_val}
                                type: button
                                action: ToggleBookVariableBool
                                data-var-idx: "{vf.idx}"
                                disabled?: {is_history_preview}
                                span.ui-toggle-track
                                    span.ui-toggle-thumb
                                span.var-bool-label {vf.bool_label}
                            {if not is_history_preview}
                                button.node-icon-btn.var-remove-btn
                                    type: button
                                    action: RemoveBookVariable
                                    data-var-idx: "{vf.idx}"
                                    svg.choice-trash-icon src: /icons/trash.svg

        .breakdown-section
            .breakdown-section-header
                span.breakdown-section-title "int"
                {if not is_history_preview}
                    button.node-icon-btn.var-section-add-btn
                        type: button
                        action: AddBookVariable
                        data-kind: int
                        svg.media-plus-icon src: /icons/plus.svg
            {if has_var_counters}
                .breakdown-entity-list
                    {for vc in var_counters}
                        .var-panel-row
                            form.var-panel-inline-form
                                action: UpdateBookVariable
                                input
                                    type: hidden
                                    name: var_idx
                                    value: "{vc.idx}"
                                input.node-meta-input.var-name-input
                                    type: text
                                    name: var_name
                                    value: "{vc.name}"
                                    spellcheck: "false"
                                    autocorrect: "off"
                                    autocapitalize: "off"
                                    autocomplete: "off"
                                    disabled?: {is_history_preview}
                                    readonly?: {is_history_preview}
                                input.node-meta-input.var-val-input
                                    type: text
                                    name: var_value
                                    value: "{vc.int_val}"
                                    spellcheck: "false"
                                    autocorrect: "off"
                                    autocapitalize: "off"
                                    autocomplete: "off"
                                    disabled?: {is_history_preview}
                                    readonly?: {is_history_preview}
                            {if not is_history_preview}
                                button.node-icon-btn.var-remove-btn
                                    type: button
                                    action: RemoveBookVariable
                                    data-var-idx: "{vc.idx}"
                                    svg.choice-trash-icon src: /icons/trash.svg

        .breakdown-section
            .breakdown-section-header
                span.breakdown-section-title "enum"
                {if not is_history_preview}
                    button.node-icon-btn.var-section-add-btn
                        type: button
                        action: AddBookVariable
                        data-kind: enum
                        svg.media-plus-icon src: /icons/plus.svg
            {if has_var_states}
                .breakdown-entity-list
                    {for vs in var_states}
                        .var-panel-row
                            form.var-panel-inline-form
                                action: UpdateBookVariable
                                input
                                    type: hidden
                                    name: var_idx
                                    value: "{vs.idx}"
                                input.node-meta-input.var-name-input
                                    type: text
                                    name: var_name
                                    value: "{vs.name}"
                                    spellcheck: "false"
                                    autocorrect: "off"
                                    autocapitalize: "off"
                                    autocomplete: "off"
                                    disabled?: {is_history_preview}
                                    readonly?: {is_history_preview}
                                input.node-meta-input.var-val-input.var-options-input
                                    type: text
                                    name: var_value
                                    value: "{vs.options_csv}"
                                    spellcheck: "false"
                                    autocorrect: "off"
                                    autocapitalize: "off"
                                    autocomplete: "off"
                                    disabled?: {is_history_preview}
                                    readonly?: {is_history_preview}
                            {if not is_history_preview}
                                button.node-icon-btn.var-remove-btn
                                    type: button
                                    action: RemoveBookVariable
                                    data-var-idx: "{vs.idx}"
                                    svg.choice-trash-icon src: /icons/trash.svg
