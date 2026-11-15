canvas.studio-canvas#studio-canvas
input.chapter-group-title-input.canvas-floating-chapter-input.hidden#canvas-chapter-input
    type: text
    placeholder: "{EditorChapterTitlePlaceholder}"
input.canvas-floating-node-input.hidden#canvas-node-title-input
    type: text
    placeholder: "{t_scene_title_placeholder}"
    autocomplete: "off"
input.canvas-floating-choice-input.hidden#canvas-choice-input
    type: text
    placeholder: "{t_choice_text_placeholder}"
    autocomplete: "off"
.canvas-context-menu.hidden#canvas-context-menu
    {if not IsHistoryPreview}
        button.canvas-context-item#btn-context-add-node
            "{t_ctx_add_node}"
        button.canvas-context-item#btn-context-add-chapter
            {if IsScreenplayMode}
                "{t_ctx_add_act}"
            {else}
                "{t_ctx_add_chapter}"
        button.canvas-context-item.btn-auto-layout-nodes
            "{t_auto_layout_hint}"
    button.canvas-context-item#btn-context-reset-view
        "{t_ctx_fit_view}"
