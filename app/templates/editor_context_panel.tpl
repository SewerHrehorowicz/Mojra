aside.editor-context-panel#editor-context-panel
    {if has_selected_node}
        form.editor-context-form#editor-context-form .hidden?: {is_node_inspector_hidden}
            action: SaveScene
            data-context-scene-id: "{selected_node.id}"
            input
                type: hidden
                name: scene_id
                value: "{selected_node.id}"
            input
                type: hidden
                name: source_file
                value: "{selected_node.source_file}"
            input
                type: hidden
                name: choice_count
                value: "{selected_node.port_count}"

            .context-panel-section
                .custom-select.node-chapter-select
                    .node-chapter-input-wrap
                        input.node-inline-chapter.custom-select-input
                            type: text
                            name: chapter
                            value: "{selected_node.chapter}"
                            placeholder: "Rozdział..."
                        button.node-chapter-caret-btn
                            type: button
                            data-select-toggle: "true"
                            "▾"
                    .custom-select-menu.hidden
                        {for ch_opt in selected_node.chapter_options}
                            button.custom-select-option .active?: {ch_opt.is_active}
                                type: button
                                data-select-value: "{ch_opt.value}"
                                data-select-label: "{ch_opt.label}"
                                {ch_opt.label}

            .context-panel-section
                .node-title-row
                    {if selected_node.is_start_scene}
                        span.node-start-indicator
                            data-hint: "Scena początkowa (START)"
                            svg.scene-star-icon src: /icons/star.svg
                    {elif selected_node.is_chapter_entry}
                        span.node-start-indicator.node-chapter-entry-indicator
                            data-hint: "Wejście rozdziału"
                            svg.scene-star-icon src: /icons/star.svg
                    input.node-inline-title
                        type: text
                        name: name
                        value: "{selected_node.name}"
                        placeholder: "Nazwa sceny (opcjonalna przy ID)..."

            .context-panel-section
                .node-meta-fields
                    .node-meta-row
                        span.node-meta-key "id:"
                        input.node-meta-input
                            type: text
                            name: explicit_id
                            value: "{selected_node.explicit_id}"
                            placeholder: "opcjonalne id (np. warsztat)"
                    .node-meta-row
                        span.node-meta-key "image:"
                        .custom-select.node-media-select
                            input.custom-select-input
                                type: hidden
                                name: explicit_image
                                value: "{selected_node.current_image}"
                            button.custom-select-trigger
                                type: button
                                data-select-toggle: "true"
                                span.custom-select-label {selected_node.current_image_label}
                                span.custom-select-caret "▾"
                            .custom-select-menu.hidden
                                {for img_opt in selected_node.image_options}
                                    button.custom-select-option .active?: {img_opt.is_active}
                                        type: button
                                        data-select-value: "{img_opt.value}"
                                        data-select-label: "{img_opt.label}"
                                        {img_opt.label}
                        button.node-icon-btn.node-media-add-btn
                            type: button
                            action: BrowseAddBookMedia
                            data-kind: image
                            data-scene: "{selected_node.id}"
                            svg.media-plus-icon src: /icons/plus.svg
                    {if book_narrated}
                        .node-meta-row
                            span.node-meta-key "voice:"
                            .custom-select.node-media-select
                                input.custom-select-input
                                    type: hidden
                                    name: explicit_voice
                                    value: "{selected_node.current_voice}"
                                button.custom-select-trigger
                                    type: button
                                    data-select-toggle: "true"
                                    span.custom-select-label {selected_node.current_voice_label}
                                    span.custom-select-caret "▾"
                                .custom-select-menu.hidden
                                    {for voice_opt in selected_node.voice_options}
                                        button.custom-select-option .active?: {voice_opt.is_active}
                                            type: button
                                            data-select-value: "{voice_opt.value}"
                                            data-select-label: "{voice_opt.label}"
                                            {voice_opt.label}
                            button.node-icon-btn.node-media-add-btn
                                type: button
                                action: BrowseAddBookMedia
                                data-kind: audio
                                data-scene: "{selected_node.id}"
                                svg.media-plus-icon src: /icons/plus.svg

            .context-panel-section
                button.btn.btn-sm.node-edit-text-btn
                    type: button
                    action: OpenSceneTextModal
                    data-scene: "{selected_node.id}"
                    data-hint: "Edytuj treść sceny w widoku książki"
                    "Edytuj treść"

            {if selected_node.has_node_warnings}
                ul.node-warnings-list
                    {for nw in selected_node.node_warnings}
                        li {nw.text}

            .context-panel-section.context-choices-section
                {if selected_node.has_choices}
                    .context-choices-list
                        {for pt in selected_node.ports}
                            .context-choice-item
                                input.node-choice-label
                                    type: text
                                    name: "choice_label_{pt.idx}"
                                    data-choice-idx: "{pt.idx}"
                                    value: "{pt.raw_label}"
                                    placeholder: "Tekst wyboru..."
                                .custom-select.node-choice-select
                                    input.custom-select-input
                                        type: hidden
                                        name: "choice_target_{pt.idx}"
                                        value: "{pt.selected_value}"
                                    button.custom-select-trigger.choice-port-trigger
                                        type: button
                                        data-select-toggle: "true"
                                        data-hint: "{pt.selected_label}"
                                        span.choice-port-icon .port-filled?: {pt.is_filled}
                                    .custom-select-menu.hidden
                                        {for opt in pt.target_options}
                                            button.custom-select-option .active?: {opt.is_active}
                                                type: button
                                                data-select-value: "{opt.value}"
                                                data-select-label: "{opt.short_label}"
                                                {opt.label}
                                button.node-icon-btn.node-choice-btn.node-choice-remove
                                    type: button
                                    action: PromptRemoveChoice
                                    data-scene: "{selected_node.id}"
                                    data-choice-idx: "{pt.idx}"
                                    svg.choice-trash-icon src: /icons/trash.svg
                button.btn.btn-sm.node-add-choice-btn
                    type: button
                    action: AddChoice
                    data-scene: "{selected_node.id}"
                    "+ Wybór"

            .context-danger-zone
                button.btn.btn-sm.context-delete-btn
                    type: button
                    action: PromptDeleteScene
                    data-scene: "{selected_node.id}"
                    svg.btn-icon src: /icons/trash.svg
                    span "Usuń scenę"

    {if selected_chapter}
        form.editor-chapter-form#editor-chapter-form .hidden?: {!has_selected_chapter}
            action: SaveChapterSettings
            data-context-chapter: "{selected_chapter.name}"
            input
                type: hidden
                name: old_chapter
                value: "{selected_chapter.name}"
            input
                type: hidden
                name: chapter
                value: "{selected_chapter.name}"
            .context-panel-section
                input.node-inline-title.chapter-group-title-input
                    type: text
                    name: new_chapter
                    value: "{selected_chapter.name}"
                    data-old-chapter: "{selected_chapter.name}"
                    placeholder: "Nazwa rozdziału..."
            .context-panel-section
                .node-meta-fields
                    .node-meta-row
                        span.node-meta-key "start:"
                        .custom-select.node-media-select.chapter-start-scene-select
                            input.custom-select-input
                                type: hidden
                                name: start_scene
                                value: "{selected_chapter.start_scene_id}"
                            button.custom-select-trigger
                                type: button
                                data-select-toggle: "true"
                                span.custom-select-label {selected_chapter.start_scene_label}
                                span.custom-select-caret "▾"
                            .custom-select-menu.hidden
                                {for ch_st_opt in selected_chapter.start_scene_options}
                                    button.custom-select-option .active?: {ch_st_opt.is_active}
                                        type: button
                                        data-select-value: "{ch_st_opt.value}"
                                        data-select-label: "{ch_st_opt.label}"
                                        {ch_st_opt.label}

    {if has_active_book}
        form.editor-book-form#editor-book-form .hidden?: {!book_settings_open}
            action: SaveBookSettings
            .context-panel-section
                input.node-inline-title#book-settings-title-input
                    type: text
                    name: title
                    value: "{active_book_title}"
                    data-old-title: "{active_book_title}"
                    placeholder: "Tytuł książki..."
            .context-panel-section
                .node-meta-fields
                    .node-meta-row
                        span.node-meta-key "rozdział:"
                        .custom-select.node-media-select.book-start-chapter-select#book-start-chapter-select
                            input.custom-select-input
                                type: hidden
                                name: start_chapter
                                value: "{book_start_chapter_name}"
                            button.custom-select-trigger
                                type: button
                                data-select-toggle: "true"
                                span.custom-select-label {book_start_chapter_name}
                                span.custom-select-caret "▾"
                            .custom-select-menu.hidden
                                {for bk_ch_opt in book_start_chapter_options}
                                    button.custom-select-option .active?: {bk_ch_opt.is_active}
                                        type: button
                                        data-select-value: "{bk_ch_opt.value}"
                                        data-select-label: "{bk_ch_opt.label}"
                                        {bk_ch_opt.label}
                    .node-meta-row
                        span.node-meta-key "start:"
                        .custom-select.node-media-select.book-start-scene-select#book-start-scene-select
                            input.custom-select-input
                                type: hidden
                                name: start_scene
                                value: "{book_start_scene_id}"
                            button.custom-select-trigger
                                type: button
                                data-select-toggle: "true"
                                span.custom-select-label {book_start_scene_label}
                                span.custom-select-caret "▾"
                            .custom-select-menu.hidden
                                {for bk_st_opt in book_start_scene_options}
                                    button.custom-select-option .active?: {bk_st_opt.is_active}
                                        type: button
                                        data-select-value: "{bk_st_opt.value}"
                                        data-select-label: "{bk_st_opt.label}"
                                        {bk_st_opt.label}
                    .node-meta-row
                        span.node-meta-key "author:"
                        input.node-meta-input
                            type: text
                            name: author
                            value: "{active_book_author}"
                            placeholder: "Autor książki..."
                    .node-meta-row.node-meta-row-textarea
                        span.node-meta-key "opis:"
                        textarea.node-meta-input.node-meta-textarea
                            name: description
                            rows: 4
                            placeholder: "Krótki opis książki..."
                            {active_book_description}

    .editor-multi-panel.hidden#editor-multi-panel
        .context-panel-section
            label.node-micro-label#multi-select-count-label "Zaznaczone sceny"
            .context-multi-scene-list#context-multi-scene-list
                {for ms in multi_scene_items}
                    .context-multi-scene-item.hidden
                        data-multi-scene-id: "{ms.id}"
                        button.context-multi-scene-select-btn
                            type: button
                            data-multi-select-scene: "{ms.id}"
                            span.context-multi-scene-title {ms.title}
                            span.context-multi-scene-meta {ms.chapter}
                        button.node-icon-btn.context-multi-scene-deselect-btn
                            type: button
                            data-multi-deselect-scene: "{ms.id}"
                            "✕"
        .context-danger-zone
            button.btn.btn-sm.context-delete-btn#btn-delete-selected-scenes
                type: button
                action: PromptDeleteScenes
                data-scenes: ""
                svg.btn-icon src: /icons/trash.svg
                span#btn-delete-selected-scenes-label "Usuń zaznaczone sceny"
