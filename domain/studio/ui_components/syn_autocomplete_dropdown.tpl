.syn-dropdown-list
    {for vsug in BookVarSuggestions}
        button.syn-dropdown-item
            data-syn-kind: "var"
            data-syn-var-name: "{vsug.Name}"
            span.syn-dropdown-var-name {vsug.Name}
            span.syn-dropdown-var-meta {vsug.Kind}
    {for esug in BookEnumSuggestions}
        button.syn-dropdown-item.hidden
            data-syn-kind: "enum_val"
            data-syn-enum-var: "{esug.VarName}"
            data-syn-var-name: "{esug.Value}"
            span.syn-dropdown-var-name {esug.Value}
            span.syn-dropdown-var-meta {esug.Meta}
    {for tsug in BookTargetSuggestions}
        button.syn-dropdown-item.hidden
            data-syn-kind: "target"
            data-syn-var-name: "{tsug.Value}"
            span.syn-dropdown-var-name {tsug.Value}
            span.syn-dropdown-var-meta {tsug.Meta}
