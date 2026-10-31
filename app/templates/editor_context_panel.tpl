aside.editor-context-panel#editor-context-panel
    -> if has_selected_node
        form.editor-context-form#editor-context-form
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
                label.node-micro-label "Rozdział"
                .custom-select.node-chapter-select
                    .node-chapter-input-wrap
                        input.node-inline-chapter.custom-select-input
                            type: text
                            name: chapter
                            value: "{selected_node.chapter}"
                            placeholder: "Rozdział..."
                            title: "Wpisz nazwę rozdziału lub wybierz z listy"
                        button.node-chapter-caret-btn
                            type: button
                            data-select-toggle: "true"
                            title: "Wybierz istniejący rozdział"
                            "▾"
                    .custom-select-menu.hidden -> selected_node.chapter_options.each() as ch_opt
                        button.custom-select-option.{ch_opt.active_class}
                            type: button
                            data-select-value: "{ch_opt.value}"
                            data-select-label: "{ch_opt.label}"
                            -> ch_opt.label

            .context-panel-section
                label.node-micro-label "Tytuł"
                input.node-inline-title
                    type: text
                    name: name
                    value: "{selected_node.name}"
                    placeholder: "Nazwa sceny (opcjonalna przy ID)..."
                -> if selected_node.is_start_scene
                    button.btn.btn-sm.node-start-scene-btn.active#btn-set-start-scene
                        type: button
                        disabled: true
                        title: "Ta scena jest punktem startowym (korzeniem) opowieści"
                        "★ Scena początkowa (START)"
                -> else
                    button.btn.btn-sm.node-start-scene-btn#btn-set-start-scene
                        type: button
                        action: SetStartScene
                        data-scene: "{selected_node.id}"
                        title: "Ustaw tę scenę jako punkt startowy (korzeń) całej opowieści"
                        "☆ Ustaw jako scenę początkową"

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
                                title: "Wybierz obraz z biblioteki"
                                span.custom-select-label -> selected_node.current_image_label
                                span.custom-select-caret "▾"
                            .custom-select-menu.hidden -> selected_node.image_options.each() as img_opt
                                button.custom-select-option.{img_opt.active_class}
                                    type: button
                                    data-select-value: "{img_opt.value}"
                                    data-select-label: "{img_opt.label}"
                                    -> img_opt.label
                        button.node-icon-btn.node-media-add-btn#btn-add-scene-image
                            type: button
                            action: BrowseAddBookMedia
                            data-kind: image
                            data-scene: "{selected_node.id}"
                            title: "Dodaj obraz z dysku (.png, .jpg, .webp)"
                            svg.media-plus-icon src: /icons/plus.svg
                    -> if book_narrated
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
                                    title: "Wybierz nagranie z biblioteki"
                                    span.custom-select-label -> selected_node.current_voice_label
                                    span.custom-select-caret "▾"
                                .custom-select-menu.hidden -> selected_node.voice_options.each() as voice_opt
                                    button.custom-select-option.{voice_opt.active_class}
                                        type: button
                                        data-select-value: "{voice_opt.value}"
                                        data-select-label: "{voice_opt.label}"
                                        -> voice_opt.label
                            button.node-icon-btn.node-media-add-btn#btn-add-scene-voice
                                type: button
                                action: BrowseAddBookMedia
                                data-kind: audio
                                data-scene: "{selected_node.id}"
                                title: "Dodaj nagranie z dysku (.ogg, .flac, .mp3)"
                                svg.media-plus-icon src: /icons/plus.svg

            .context-panel-section
                label.node-micro-label "Treść"
                button.node-text-preview
                    type: button
                    action: OpenSceneTextModal
                    data-scene: "{selected_node.id}"
                    title: "Kliknij, aby edytować treść sceny w widoku książki"
                    span.node-text-snippet -> selected_node.first_sentence

            -> if selected_node.has_node_warnings
                ul.node-warnings-list -> selected_node.node_warnings.each() as nw
                    li -> nw.text

            .context-panel-section.context-choices-section
                label.node-micro-label "Wybory"
                -> if selected_node.has_choices
                    .context-choices-list -> selected_node.ports.each() as pt
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
                                    title: "{pt.selected_label}"
                                    span.choice-port-icon.{pt.port_class}
                                .custom-select-menu.hidden -> pt.target_options.each() as opt
                                    button.custom-select-option.{opt.active_class}
                                        type: button
                                        data-select-value: "{opt.value}"
                                        data-select-label: "{opt.short_label}"
                                        -> opt.label
                            button.node-icon-btn.node-choice-btn.node-choice-remove
                                type: button
                                action: PromptRemoveChoice
                                data-scene: "{selected_node.id}"
                                data-choice-idx: "{pt.idx}"
                                title: "Usuń ten wybór"
                                svg.choice-trash-icon src: /icons/trash.svg
                button.node-add-choice-btn
                    type: button
                    action: AddChoice
                    data-scene: "{selected_node.id}"
                    "+ Wybór"

            .context-danger-zone
                button.btn.btn-sm.context-delete-btn#btn-delete-scene
                    type: button
                    action: PromptDeleteScene
                    data-scene: "{selected_node.id}"
                    title: "Usuń scenę"
                    svg.btn-icon src: /icons/trash.svg
                    span "Usuń scenę"

    .editor-multi-panel.hidden#editor-multi-panel
        .context-panel-section
            label.node-micro-label#multi-select-count-label "Zaznaczone sceny"
            .context-multi-scene-list#context-multi-scene-list
        .context-danger-zone
            button.btn.btn-sm.context-delete-btn#btn-delete-selected-scenes
                type: button
                action: PromptDeleteScenes
                data-scenes: ""
                title: "Usuń zaznaczone sceny"
                svg.btn-icon src: /icons/trash.svg
                span#btn-delete-selected-scenes-label "Usuń zaznaczone sceny"
