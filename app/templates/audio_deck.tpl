footer.audio-deck#audio-deck
    audio#story-audio
        preload: auto
    .audio-deck-inner
        .audio-transport
            button.deck-play-btn#btn-audio-play
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
                span#audio-time-total "{AudioDurationLabel}"

        .audio-options-col
            button.deck-chip#btn-audio-speed "{AudioSpeed}x"
            button.deck-chip.icon-toggle-btn#btn-auto-choice-toggle .active?: {AutoChoiceEnabled}
                data-hint: "{AutoChoiceLabel}"
                svg.deck-chip-icon src: /icons/branch.svg
