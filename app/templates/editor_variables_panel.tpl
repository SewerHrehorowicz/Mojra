aside.editor-variables-panel#editor-variables-panel .hidden?: {!ShowEditorVariablesPanel}
    .history-panel-header
        h3.history-panel-title {t_variables_title}
        button.node-close-btn.btn-close-variables
            data-hint: "{t_close_variables_panel}"
            svg.node-close-icon src: /icons/close.svg

    .history-panel-body.breakdown-panel-body.variables-panel-body
        .breakdown-section
            .breakdown-section-header
                span.breakdown-section-title "bool"
                {if not IsHistoryPreview}
                    button.node-icon-btn.var-section-add-btn
                        data-kind: bool
                        svg.media-plus-icon src: /icons/plus.svg
            {if HasVarFlags}
                .breakdown-entity-list
                    {for vf in VarFlags}
                        .var-panel-row
                            form.var-panel-inline-form
                                input
                                    type: hidden
                                    name: var_idx
                                    value: "{vf.Idx}"
                                input.node-meta-input.var-name-input
                                    type: text
                                    name: var_name
                                    value: "{vf.Name}"
                                    spellcheck: "false"
                                    autocorrect: "off"
                                    autocapitalize: "off"
                                    autocomplete: "off"
                                    disabled?: {IsHistoryPreview}
                                    readonly?: {IsHistoryPreview}
                            button.var-bool-toggle-btn .active?: {vf.BoolVal}
                                data-var-idx: "{vf.Idx}"
                                disabled?: {IsHistoryPreview}
                                span.ui-toggle-track
                                    span.ui-toggle-thumb
                                span.var-bool-label {vf.BoolLabel}
                            {if not IsHistoryPreview}
                                button.node-icon-btn.var-remove-btn
                                    data-var-idx: "{vf.Idx}"
                                    svg.choice-trash-icon src: /icons/trash.svg

        .breakdown-section
            .breakdown-section-header
                span.breakdown-section-title "int"
                {if not IsHistoryPreview}
                    button.node-icon-btn.var-section-add-btn
                        data-kind: int
                        svg.media-plus-icon src: /icons/plus.svg
            {if HasVarCounters}
                .breakdown-entity-list
                    {for vc in VarCounters}
                        .var-panel-row
                            form.var-panel-inline-form
                                input
                                    type: hidden
                                    name: var_idx
                                    value: "{vc.Idx}"
                                input.node-meta-input.var-name-input
                                    type: text
                                    name: var_name
                                    value: "{vc.Name}"
                                    spellcheck: "false"
                                    autocorrect: "off"
                                    autocapitalize: "off"
                                    autocomplete: "off"
                                    disabled?: {IsHistoryPreview}
                                    readonly?: {IsHistoryPreview}
                                input.node-meta-input.var-val-input
                                    type: text
                                    name: var_value
                                    value: "{vc.IntVal}"
                                    spellcheck: "false"
                                    autocorrect: "off"
                                    autocapitalize: "off"
                                    autocomplete: "off"
                                    disabled?: {IsHistoryPreview}
                                    readonly?: {IsHistoryPreview}
                            {if not IsHistoryPreview}
                                button.node-icon-btn.var-remove-btn
                                    data-var-idx: "{vc.Idx}"
                                    svg.choice-trash-icon src: /icons/trash.svg

        .breakdown-section
            .breakdown-section-header
                span.breakdown-section-title "enum"
                {if not IsHistoryPreview}
                    button.node-icon-btn.var-section-add-btn
                        data-kind: enum
                        svg.media-plus-icon src: /icons/plus.svg
            {if HasVarStates}
                .breakdown-entity-list
                    {for vs in VarStates}
                        form.var-panel-inline-form.var-enum-form
                            input
                                type: hidden
                                name: var_idx
                                value: "{vs.Idx}"
                            input
                                type: hidden
                                name: opt_count
                                value: "{vs.OptCount}"
                            .var-panel-row.var-enum-header-row
                                input.node-meta-input.var-name-input.var-enum-name-input
                                    type: text
                                    name: var_name
                                    value: "{vs.Name}"
                                    spellcheck: "false"
                                    autocorrect: "off"
                                    autocapitalize: "off"
                                    autocomplete: "off"
                                    disabled?: {IsHistoryPreview}
                                    readonly?: {IsHistoryPreview}
                                {if not IsHistoryPreview}
                                    button.node-icon-btn.var-enum-add-opt-btn
                                        data-var-idx: "{vs.Idx}"
                                        data-hint: "{t_add_enum_opt_hint}"
                                        svg.media-plus-icon src: /icons/plus.svg
                                    button.node-icon-btn.var-remove-btn
                                        data-var-idx: "{vs.Idx}"
                                        svg.choice-trash-icon src: /icons/trash.svg
                            .var-enum-options-list
                                {for opt in vs.OptionItems}
                                    .var-enum-option-row
                                        input.node-meta-input.var-val-input.var-options-input
                                            type: text
                                            name: "enum_opt_{opt.OptIdx}"
                                            value: "{opt.Value}"
                                            placeholder: "{t_enum_opt_placeholder}"
                                            spellcheck: "false"
                                            autocorrect: "off"
                                            autocapitalize: "off"
                                            autocomplete: "off"
                                            disabled?: {IsHistoryPreview}
                                            readonly?: {IsHistoryPreview}
                                        {if vs.CanRemoveOpt}
                                            {if not IsHistoryPreview}
                                                button.node-icon-btn.var-enum-opt-remove-btn
                                                    data-var-idx: "{vs.Idx}"
                                                    data-opt-idx: "{opt.OptIdx}"
                                                    data-hint: "{t_remove_enum_opt_hint}"
                                                    svg.choice-trash-icon src: /icons/trash.svg
