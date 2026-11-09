section.view-panel#view-player
    .player-layout#player-layout .is-imageless?: {imageless_mode}
        .player-bg-stage#player-bg-stage
            {if not imageless_mode}
                img.player-bg-img#player-bg-img .hidden?: {!current_scene.has_bg_image}
                    alt: "{current_scene.rail_title}"
                    src?: {current_scene.bg_image_url}

        .story-scroll-feed#story-scroll-feed
            {if is_history_preview}
                .studio-preview-banner.player-preview-banner#player-preview-banner
                    span.studio-preview-banner-text "{t_history_preview_banner}: {history_preview_label}"
                    .studio-preview-banner-actions
                        button.btn.btn-sm
                            type: button
                            action: SwitchView
                            data-view: editor
                            "{t_back_to_editor}"
                        button.btn.btn-primary.btn-sm
                            type: button
                            action: ExitHistoryPreview
                            "{t_back_to_current_version}"

            .reader-global-meta#reader-global-meta
                span.scene-chapter-pill#reader-global-meta-text {current_scene.header_meta}

            {if has_history}
                .history-stack#history-stack
                    {for h in history_items}
                        article.scene-card.scene-dimmed .has-illustration?: {h.has_image} .scene-untitled?: {!h.has_title}
                            data-history-idx: "{h.idx}"
                            data-scene-image: "{h.bg_image_url}"
                            data-scene-meta: "{h.header_meta}"
                            data-scene-audio: "{h.audio_url}"
                            data-scene-audio-queue: "{h.audio_queue}"
                            data-scene-audio-duration: "{h.audio_duration_label}"
                            data-scene-audio-trim-start: "{h.audio_trim_start}"
                            data-scene-audio-trim-end: "{h.audio_trim_end}"
                            {if h.has_title}
                                .scene-header-block
                                    .scene-title-row
                                        h2.scene-title {h.name}
                                        .scene-contextual-rewind
                                            button.contextual-rewind-btn
                                                type: button
                                                action: PromptRewindStep
                                                data-step: "{h.idx}"
                                                data-hint: "{t_rewind_scene_hint}"
                                                svg.rewind-icon src: /icons/reset.svg

                            {if h.has_image}
                                .scene-illustration-spacer
                                    img.scene-portrait-img
                                        loading: lazy
                                        decoding: async
                                        alt: "{h.rail_title}"
                                        src: {h.image_url}

                            .scene-body-block
                                {if not h.has_title}
                                    .scene-contextual-rewind.scene-contextual-rewind-floating
                                        button.contextual-rewind-btn
                                            type: button
                                            action: PromptRewindStep
                                            data-step: "{h.idx}"
                                            data-hint: "{t_rewind_scene_hint}"
                                            svg.rewind-icon src: /icons/reset.svg
                                .scene-prose .is-screenplay?: {is_screenplay_mode}
                                    {if is_screenplay_mode}
                                        {for blk in h.screenplay_blocks}
                                            {if blk.is_slugline}
                                                .sp-slugline
                                                    {if blk.show_scene_num}
                                                        span.sp-scene-num "{blk.scene_num}."
                                                    span.sp-slugline-text {blk.text}
                                                    {if blk.show_scene_num}
                                                        span.sp-scene-num-right "{blk.scene_num}."
                                            {elif blk.is_character}
                                                .sp-character {blk.text}
                                            {elif blk.is_parenthetical}
                                                .sp-parenthetical {blk.text}
                                            {elif blk.is_dialogue}
                                                .sp-dialogue {blk.text}
                                            {elif blk.is_transition}
                                                .sp-transition {blk.text}
                                            {else}
                                                p.sp-action {blk.text}
                                    {else}
                                        {prose h.prose}

            .active-scene-stage#active-scene-stage
                data-prefetch-urls: "{prefetch_media_urls}"
                article.scene-card.scene-focused .has-illustration?: {current_scene.has_image} .scene-untitled?: {!current_scene.has_title}
                    data-present-scene: true
                    data-scene-image: "{current_scene.bg_image_url}"
                    data-scene-meta: "{current_scene.header_meta}"
                    data-scene-audio: "{current_scene.audio_url}"
                    data-scene-audio-queue: "{current_scene.audio_queue}"
                    data-scene-audio-duration: "{current_scene.audio_duration_label}"
                    data-scene-audio-trim-start: "{current_scene.audio_trim_start}"
                    data-scene-audio-trim-end: "{current_scene.audio_trim_end}"
                    {if current_scene.has_title}
                        .scene-header-block
                            .scene-title-row
                                .scene-title-with-badge
                                    span.present-scene-badge
                                        svg.present-scene-icon src: /icons/asterisk.svg
                                    h2.scene-title.scene-title-present {current_scene.name}
                    {if current_scene.has_image}
                        .scene-illustration-spacer
                            img.scene-portrait-img
                                alt: "{current_scene.rail_title}"
                                src: {current_scene.image_url}
                    .scene-body-block
                        .scene-prose .is-screenplay?: {is_screenplay_mode}
                            {if is_screenplay_mode}
                                {for blk in current_scene.screenplay_blocks}
                                    {if blk.is_slugline}
                                        .sp-slugline
                                            {if blk.show_scene_num}
                                                span.sp-scene-num "{blk.scene_num}."
                                            span.sp-slugline-text {blk.text}
                                            {if blk.show_scene_num}
                                                span.sp-scene-num-right "{blk.scene_num}."
                                    {elif blk.is_character}
                                        .sp-character {blk.text}
                                    {elif blk.is_parenthetical}
                                        .sp-parenthetical {blk.text}
                                    {elif blk.is_dialogue}
                                        .sp-dialogue {blk.text}
                                    {elif blk.is_transition}
                                        .sp-transition {blk.text}
                                    {else}
                                        p.sp-action {blk.text}
                            {else}
                                {prose current_scene.prose}
                        .scene-choices-list
                            {if current_scene.is_terminal}
                                button.choice-btn
                                    type: button
                                    action: PromptResetProgress
                                    {if is_screenplay_mode}
                                        span {t_end_screenplay_reset}
                                    {else}
                                        span {t_end_story_reset}
                                    span.choice-arrow "↺"
                            {else}
                                {for ch in current_scene.choices}
                                    button.choice-btn .choice-locked?: {ch.is_locked}
                                        type: button
                                        action: ChooseBranch
                                        data-choice-idx: "{ch.idx}"
                                        data-implicit-next: "{ch.is_implicit}"
                                        disabled?: {ch.is_locked}
                                        @scene_choice_arrow

            .scroll-more-cue.hidden
                span.scroll-more-dots "..."

        {if has_history}
            nav.story-history-rail#story-history-rail
                aria-label: "{t_history_rail_aria}"
                {for h in history_items}
                    button.history-rail-tick
                        type: button
                        data-hint: "{h.rail_title}"
                        span.history-rail-bar
                button.history-rail-tick.present-tick.active
                    type: button
                    data-hint: "{current_scene.rail_title}"
                    span.history-rail-bar

        {if show_audio_deck}
            footer.audio-deck#audio-deck
                audio#story-audio
                    preload: auto
                    data-audio-url: "{current_scene.audio_url}"
                    data-scene-audio-queue: "{current_scene.audio_queue}"
                    data-trim-start: "{current_scene.audio_trim_start}"
                    data-trim-end: "{current_scene.audio_trim_end}"
                    data-resume-time: "{audio_resume_time}"
                    data-speed: "{audio_speed}"
                    data-auto-choice: "{auto_choice_enabled}"
                    data-auto-delay: "{auto_choice_delay}"
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
                            span#audio-time-total "{current_scene.audio_duration_label}"

                    .audio-options-col
                        button.deck-chip#btn-audio-speed
                            type: button
                            action: CycleAudioSpeed
                            "{audio_speed}x"
                        button.deck-chip#btn-auto-choice-toggle .chip-active?: {auto_choice_enabled}
                            type: button
                            action: ToggleAutoChoice
                            data-hint: "{auto_choice_label}"
                            svg.deck-chip-icon src: /icons/branch.svg
        {else}
            footer.reader-progress-footer#reader-progress-footer
                progress.reader-book-progress
                    value: "{book_progress_pct}"
                    max: 100

{def confirm_modal}
    {if show_confirm_modal}
        .modal-backdrop#modal-confirm
            data-backdrop-action: ConfirmModalClose
            .modal-card
                .modal-header
                    h3.modal-title {confirm_title}
                .modal-body
                    p.modal-desc {confirm_message}
                    .modal-actions
                        button.btn
                            type: button
                            action: ConfirmModalClose
                            "{t_cancel}"
                        button.btn .btn-danger?: {confirm_is_danger} .btn-primary?: {!confirm_is_danger}
                            type: button
                            action: ConfirmModalAccept
                            "{t_confirm_btn}"
