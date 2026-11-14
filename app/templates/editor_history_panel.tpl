aside.editor-history-panel#editor-history-panel .hidden?: {!ShowEditorHistoryPanel}
    .history-panel-header
        span.history-panel-title {t_history_panel_title}
        button.node-close-btn.btn-close-history
            svg.node-close-icon src: /icons/close.svg

    form.history-compact-form.history-version-save-form
        input.history-version-input
            type: text
            name: version_name
            placeholder: "{DefaultNextVersionName}"
            disabled?: {IsHistoryPreview}
        button.btn.btn-primary.btn-sm.history-version-save-btn .is-disabled?: {IsHistoryPreview}
            type: submit
            disabled?: {IsHistoryPreview}
            "{t_save_version_btn}"

    .history-active-branch-bar
        span.history-active-branch-dot
        span.history-active-branch-name {ActiveBranchName}

    .history-graph-scroll#history-graph-scroll
        .history-drop-preview-row.hidden#history-drop-preview-row
            span.history-drop-preview-dot
            span.history-drop-preview-stem
            .history-drop-preview-box
                span.history-row-title#history-drop-preview-title
                span.history-row-meta#history-drop-preview-meta

        .history-graph-list#history-graph-list
            {for row in HistoryGraphRows}
                .history-graph-row .is-unnamed-step?: {row.IsUnnamedStep} .is-current-row?: {row.IsCurrent} .is-active-head?: {row.IsActiveHead} .is-preview-row?: {row.IsPreview} .is-undone-row?: {row.IsUndone} .is-active-lineage?: {row.IsActiveLineage} .is-inactive-branch?: {!row.IsActiveLineage} .is-branch-head?: {row.IsBranchHead}
                    drop-zone: "true"
                    data-revision: "{row.ID}"
                    data-branch: "{row.Branch}"
                    data-label: "{row.Label}"
                    .history-graph-lanes
                        {for cell in row.Lanes}
                            .history-lane-cell .has-node?: {cell.HasNode} .is-branch-rail?: {cell.IsBranchRail} .is-other-branch?: {cell.IsOtherBranch}
                                drop-zone?: {cell.Branch}
                                data-branch?: {cell.Branch}
                                data-hint?: {cell.HoverHint}
                                {if cell.HasRailUp}
                                    span.history-lane-rail-up .is-active?: {cell.IsActiveRailUp} .is-dashed?: {cell.IsDashedRailUp}
                                {if cell.HasRailDown}
                                    span.history-lane-rail-down .is-active?: {cell.IsActiveRailDown} .is-dashed?: {cell.IsDashedRailDown}
                                {if cell.IsForkCorner}
                                    span.history-lane-fork.is-corner .is-active?: {cell.IsActiveFork}
                                {elif cell.HasForkHoriz}
                                    span.history-lane-fork.is-horiz .is-active?: {cell.IsActiveFork}
                                {if cell.IsCompactIn}
                                    span.history-lane-compact.is-in .is-active?: {cell.IsActiveCompact}
                                {if cell.IsCompactOut}
                                    span.history-lane-compact.is-out .is-active?: {cell.IsActiveCompact}
                                {if cell.HasCompactHoriz}
                                    span.history-lane-compact.is-horiz .is-active?: {cell.IsActiveCompact}
                                {if cell.HasStemStart}
                                    span.history-lane-stem.is-start
                                {elif cell.HasStemThrough}
                                    span.history-lane-stem.is-through
                                {if cell.IsOtherBranch}
                                    button.history-rail-hit
                                        data-branch: "{cell.Branch}"
                                        data-hint?: {cell.HoverHint}
                                {if cell.HasNode}
                                    {if row.IsActiveBranch}
                                        button.history-rail-node.history-rail-node-active-branch .is-current-node?: {row.IsCurrent} .is-unnamed-node?: {row.IsUnnamedStep} .is-active-lineage-node?: {row.IsActiveLineage}
                                            data-revision: "{row.ID}"
                                            data-branch: "{row.Branch}"
                                            data-label: "{row.Label}"
                                            data-is-current: "{row.IsCurrent}"
                                            data-hint?: {row.HoverHint}
                                    {else}
                                        button.history-rail-node.history-rail-node-other-branch .is-current-node?: {row.IsCurrent} .is-unnamed-node?: {row.IsUnnamedStep} .is-active-lineage-node?: {row.IsActiveLineage}
                                            data-revision: "{row.ID}"
                                            data-branch: "{row.Branch}"
                                            data-label: "{row.Label}"
                                            data-is-current: "{row.IsCurrent}"
                                            data-hint?: {row.HoverHint}
                    span.history-row-stem
                    .history-row-box
                        data-revision: "{row.ID}"
                        data-branch: "{row.Branch}"
                        data-label: "{row.Label}"
                        data-hint?: {row.HoverHint}
                        span.history-row-grip
                            data-revision: "{row.ID}"
                            data-branch: "{row.Branch}"
                            data-label: "{row.Label}"
                            data-is-current: "{row.IsCurrent}"
                            data-history-drag-grip: "{row.ID}"
                            "⋮⋮"
                        span.history-row-title {row.Label}
                        span.history-row-meta {row.MetaLabel}

        .scroll-more-cue.hidden#history-scroll-more-cue
            span.scroll-more-dots "..."

    form.history-compact-form.history-branch-create-form
        input.history-version-input
            type: text
            name: branch_name
            placeholder: "{DefaultNextBranchName}"
        button.btn.btn-sm.history-version-save-btn
            type: submit
            "{t_add_branch_btn}"
