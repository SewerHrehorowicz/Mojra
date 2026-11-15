.choice-triple-stack .modal-choice-triple-stack?: {ShowSceneTextModal}
    .choice-triple-row.choice-triple-exit .is-invalid?: {ch.IsExitInvalid}
        data-hint?: {ch.ExitErrorHint}
        span.choice-row-key "exit:"
        input.choice-exit-input
            type: text
            name: "choice_target_{ch.Idx}"
            data-choice-idx: "{ch.Idx}"
            value: "{ch.SelectedValue}"
            placeholder: "{t_exit_target_placeholder}"
            spellcheck: "false"
            autocorrect: "off"
            autocapitalize: "off"
            autocomplete: "off"
            data-hint?: {ch.ExitErrorHint}
            disabled?: {IsHistoryPreview}
            readonly?: {IsHistoryPreview}
    .choice-triple-row.choice-triple-label
        span.choice-row-key "text:"
        textarea.node-meta-input.node-meta-textarea.node-choice-label .choice-label-text?: {ShowSceneTextModal} .node-inline-title?: {ShowSceneTextModal}
            name: "choice_label_{ch.Idx}"
            data-choice-idx: "{ch.Idx}"
            rows: 1
            placeholder: "{t_choice_text_placeholder}"
            disabled?: {IsHistoryPreview}
            readonly?: {IsHistoryPreview}
            {ch.Label}
    {if ch.IsRuleOpen}
        .choice-triple-row.choice-triple-cond .is-empty-rule?: {!ch.HasCondition} .is-invalid?: {ch.IsCondInvalid}
            data-hint?: {ch.CondErrorHint}
            span.choice-row-key "if:"
            input.choice-rule-input
                type: text
                name: "choice_cond_{ch.Idx}"
                data-choice-idx: "{ch.Idx}"
                value: "{ch.Condition}"
                placeholder: ""
                spellcheck: "false"
                autocorrect: "off"
                autocapitalize: "off"
                autocomplete: "off"
                data-hint?: {ch.CondErrorHint}
                disabled?: {IsHistoryPreview}
                readonly?: {IsHistoryPreview}
        .choice-triple-row.choice-triple-eff .is-empty-rule?: {!ch.HasEffects} .is-invalid?: {ch.IsEffInvalid}
            data-hint?: {ch.EffErrorHint}
            span.choice-row-key "set:"
            input.choice-rule-input
                type: text
                name: "choice_effects_{ch.Idx}"
                data-choice-idx: "{ch.Idx}"
                value: "{ch.Effects}"
                placeholder: ""
                spellcheck: "false"
                autocorrect: "off"
                autocapitalize: "off"
                autocomplete: "off"
                data-hint?: {ch.EffErrorHint}
                disabled?: {IsHistoryPreview}
                readonly?: {IsHistoryPreview}
    {else}
        input
            type: hidden
            name: "choice_cond_{ch.Idx}"
            value: "{ch.Condition}"
        input
            type: hidden
            name: "choice_effects_{ch.Idx}"
            value: "{ch.Effects}"
