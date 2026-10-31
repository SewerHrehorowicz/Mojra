section.view-panel#view-player
    .player-layout.is-imageless?=imageless_mode#player-layout
        .player-bg-stage#player-bg-stage
            -> if current_scene.has_bg_image
                img.player-bg-img#player-bg-img
                    alt: "{current_scene.rail_title}"
                    -> current_scene.bg_image_url

        .story-scroll-feed#story-scroll-feed
            .reader-global-meta#reader-global-meta
                span.scene-chapter-pill#reader-global-meta-text -> current_scene.header_meta

            -> if has_book_warnings
                .book-warnings-box
                    .book-warnings-title -> "⚠ Ostrzeżenia książki ({book_warning_count}):"
                    ul.book-warnings-list -> book_warnings.each() as w
                        li -> w.text

            -> if has_history
                .history-stack#history-stack -> history_items.each() as h
                    article.scene-card.scene-dimmed.has-illustration?=h.has_image.scene-untitled?=!h.has_title
                        data-history-idx: "{h.idx}"
                        data-scene-image: "{h.bg_image_url}"
                        data-scene-meta: "{h.header_meta}"
                        -> if h.has_title
                            .scene-header-block
                                .scene-title-row
                                    h2.scene-title -> h.name
                                    .scene-contextual-rewind
                                        button.contextual-rewind-btn
                                            type: button
                                            action: PromptRewindStep
                                            data-step: "{h.idx}"
                                            data-hint: "Wznów od tej sceny"
                                            svg.rewind-icon src: /icons/reset.svg

                        -> if h.has_image
                            .scene-illustration-spacer
                                img.scene-portrait-img
                                    loading: lazy
                                    decoding: async
                                    alt: "{h.rail_title}"
                                    -> h.image_url

                        .scene-body-block
                            -> if not h.has_title
                                .scene-contextual-rewind.scene-contextual-rewind-floating
                                    button.contextual-rewind-btn
                                        type: button
                                        action: PromptRewindStep
                                        data-step: "{h.idx}"
                                        data-hint: "Wznów od tej sceny"
                                        svg.rewind-icon src: /icons/reset.svg
                            .scene-prose -> html:h.html

            .active-scene-stage#active-scene-stage
                article.scene-card.scene-focused.has-illustration?=current_scene.has_image.scene-untitled?=!current_scene.has_title
                    data-present-scene: true
                    data-scene-image: "{current_scene.bg_image_url}"
                    data-scene-meta: "{current_scene.header_meta}"
                    -> if current_scene.has_title
                        .scene-header-block
                            .scene-title-row
                                .scene-title-with-badge
                                    span.present-scene-badge
                                        svg.present-scene-icon src: /icons/asterisk.svg
                                    h2.scene-title.scene-title-present -> current_scene.name
                    -> if current_scene.has_image
                        .scene-illustration-spacer
                            img.scene-portrait-img
                                alt: "{current_scene.rail_title}"
                                -> current_scene.image_url
                    .scene-body-block
                        .scene-prose -> html:current_scene.html
                        .scene-choices-list
                            -> if current_scene.is_terminal
                                button.choice-btn
                                    type: button
                                    action: PromptResetProgress
                                    span "Koniec opowieści — Zagraj od początku"
                                    span.choice-arrow "↺"
                            -> else
                                -> current_scene.choices.each() as ch
                                    button.choice-btn
                                        type: button
                                        action: ChooseBranch
                                        data-choice-idx: "{ch.idx}"
                                        data-implicit-next: "{ch.is_implicit}"
                                        @scene_choice_arrow

            .scroll-more-cue.hidden
                span.scroll-more-dots "..."

        -> if has_history
            nav.story-history-rail#story-history-rail
                aria-label: "Oś przeczytanych scen"
                -> history_items.each() as h
                    button.history-rail-tick
                        type: button
                        data-hint: "{h.rail_title}"
                        span.history-rail-bar
                button.history-rail-tick.present-tick.active
                    type: button
                    data-hint: "{current_scene.rail_title}"
                    span.history-rail-bar

        -> if show_audio_deck
            footer.audio-deck#audio-deck
                audio#story-audio
                    preload: metadata
                    data-audio-url: "{current_scene.audio_url}"
                    data-resume-time: "{audio_resume_time}"
                    data-speed: "{audio_speed}"
                    data-auto-choice: "{auto_choice_enabled}"
                .audio-deck-inner
                    .audio-transport
                        button.deck-play-btn#btn-audio-play
                            type: button
                            svg.deck-play-icon#icon-audio-play src: /icons/play.svg
                            svg.deck-play-icon.hidden#icon-audio-pause src: /icons/pause.svg

                    .audio-progress-wrap
                        input.audio-seek-slider#audio-seek-slider
                            type: range
                            min: 0
                            max: 1000
                            value: 0
                        span.audio-time-display
                            span#audio-time-current "0:00"
                            span " / "
                            span#audio-time-total "0:00"

                    .audio-options-col
                        button.deck-chip#btn-audio-speed
                            type: button
                            action: CycleAudioSpeed
                            -> "{audio_speed}x"
                        button.deck-chip.chip-active?=auto_choice_enabled#btn-auto-choice-toggle
                            type: button
                            action: ToggleAutoChoice
                            data-hint: "{auto_choice_label}"
                            svg.deck-chip-icon src: /icons/branch.svg
        -> else
            footer.reader-progress-footer#reader-progress-footer
                progress.reader-book-progress
                    value: "{book_progress_pct}"
                    max: 100
