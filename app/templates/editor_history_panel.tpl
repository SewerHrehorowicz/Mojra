aside.editor-history-panel#editor-history-panel .hidden?: {!show_editor_history_panel}
    .history-panel-header
        span.history-panel-title "Historia"
        button.node-close-btn
            type: button
            action: CloseEditorHistoryPanel
            svg.node-close-icon src: /icons/close.svg

    form.history-compact-form
        action: SaveHistoryVersion
        input.history-version-input
            type: text
            name: version_name
            placeholder: "{default_next_version_name}"
            disabled?: {is_history_preview}
        button.btn.btn-primary.btn-sm.history-version-save-btn .is-disabled?: {is_history_preview}
            type: submit
            disabled?: {is_history_preview}
            "Zapisz wersję"

    .history-active-branch-bar
        span.history-active-branch-dot
        span.history-active-branch-name {active_branch_name}

    .history-graph-scroll#history-graph-scroll
        .history-drop-preview-row.hidden#history-drop-preview-row
            span.history-drop-preview-dot
            span.history-drop-preview-stem
            .history-drop-preview-box
                span.history-row-title#history-drop-preview-title
                span.history-row-meta#history-drop-preview-meta

        .history-graph-list#history-graph-list
            {for row in history_graph_rows}
                .history-graph-row .is-unnamed-step?: {row.is_unnamed_step} .is-current-row?: {row.is_current} .is-active-head?: {row.is_active_head} .is-preview-row?: {row.is_preview} .is-undone-row?: {row.is_undone} .is-active-lineage?: {row.is_active_lineage} .is-inactive-branch?: {!row.is_active_lineage} .is-branch-head?: {row.is_branch_head}
                    data-revision: "{row.id}"
                    data-branch: "{row.branch}"
                    data-label: "{row.label}"
                    .history-graph-lanes
                        {for cell in row.lanes}
                            .history-lane-cell .has-node?: {cell.has_node} .is-branch-rail?: {cell.is_branch_rail} .is-other-branch?: {cell.is_other_branch}
                                data-branch?: {cell.branch}
                                data-hint?: {cell.hover_hint}
                                {if cell.has_rail_up}
                                    span.history-lane-rail-up .is-active?: {cell.is_active_rail_up} .is-dashed?: {cell.is_dashed_rail_up}
                                {if cell.has_rail_down}
                                    span.history-lane-rail-down .is-active?: {cell.is_active_rail_down} .is-dashed?: {cell.is_dashed_rail_down}
                                {if cell.is_fork_corner}
                                    span.history-lane-fork.is-corner .is-active?: {cell.is_active_fork}
                                {elif cell.has_fork_horiz}
                                    span.history-lane-fork.is-horiz .is-active?: {cell.is_active_fork}
                                {if cell.is_compact_in}
                                    span.history-lane-compact.is-in .is-active?: {cell.is_active_compact}
                                {if cell.is_compact_out}
                                    span.history-lane-compact.is-out .is-active?: {cell.is_active_compact}
                                {if cell.has_compact_horiz}
                                    span.history-lane-compact.is-horiz .is-active?: {cell.is_active_compact}
                                {if cell.has_stem_start}
                                    span.history-lane-stem.is-start
                                {elif cell.has_stem_through}
                                    span.history-lane-stem.is-through
                                {if cell.is_other_branch}
                                    button.history-rail-hit
                                        type: button
                                        action: SwitchHistoryBranch
                                        data-branch: "{cell.branch}"
                                        data-hint?: {cell.hover_hint}
                                {if cell.has_node}
                                    {if row.is_active_branch}
                                        button.history-rail-node .is-current-node?: {row.is_current} .is-unnamed-node?: {row.is_unnamed_step} .is-active-lineage-node?: {row.is_active_lineage}
                                            type: button
                                            action: PreviewHistoryRevision
                                            data-revision: "{row.id}"
                                            data-branch: "{row.branch}"
                                            data-label: "{row.label}"
                                            data-hint?: {row.hover_hint}
                                    {else}
                                        button.history-rail-node .is-current-node?: {row.is_current} .is-unnamed-node?: {row.is_unnamed_step} .is-active-lineage-node?: {row.is_active_lineage}
                                            type: button
                                            action: SwitchHistoryBranch
                                            data-revision: "{row.id}"
                                            data-branch: "{row.branch}"
                                            data-label: "{row.label}"
                                            data-hint?: {row.hover_hint}
                    span.history-row-stem
                    .history-row-box
                        action: PreviewHistoryRevision
                        data-revision: "{row.id}"
                        data-branch: "{row.branch}"
                        data-label: "{row.label}"
                        data-hint?: {row.hover_hint}
                        span.history-row-grip
                            data-history-drag-grip: "{row.id}"
                            "⋮⋮"
                        span.history-row-title {row.label}
                        span.history-row-meta {row.meta_label}

        .scroll-more-cue.hidden#history-scroll-more-cue
            span.scroll-more-dots "..."

    form.history-compact-form.history-branch-create-form
        action: PromptCreateHistoryBranch
        input.history-version-input
            type: text
            name: branch_name
            placeholder: "{default_next_branch_name}"
        button.btn.btn-sm.history-version-save-btn
            type: submit
            "+ Wariant"
