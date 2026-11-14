section.view-panel#view-player
    .player-layout#player-layout .is-imageless?: {ImagelessMode}
        .player-bg-stage#player-bg-stage
            {if not ImagelessMode}
                img.player-bg-img#player-bg-img .hidden?: {!HasBgImage}
                    alt: "{RailTitle}"
                    src?: {BgImageURL}

        .story-scroll-feed#story-scroll-feed
            {if IsHistoryPreview}
                .studio-preview-banner.player-preview-banner#player-preview-banner
                    span.studio-preview-banner-text "{t_history_preview_banner}: {HistoryPreviewLabel}"
                    .studio-preview-banner-actions
                        button.btn.btn-sm#btn-preview-back-editor "{t_back_to_editor}"
                        button.btn.btn-primary.btn-sm#btn-preview-exit "{t_back_to_current_version}"

            .reader-global-meta#reader-global-meta
                span.scene-chapter-pill#reader-global-meta-text {HeaderMeta}

            {if HasHistory}
                .history-stack#history-stack
                    @HistoryScenes

            .active-scene-stage#active-scene-stage
                data-prefetch-urls: "{PrefetchMediaURLs}"
                @CurrentScene

            .scroll-more-cue.hidden
                span.scroll-more-dots "..."

        {if HasHistory}
            nav.story-history-rail#story-history-rail
                aria-label: "{t_history_rail_aria}"
                {for tick in HistoryTicks}
                    button.history-rail-tick
                        data-hint: "{tick}"
                        span.history-rail-bar
                button.history-rail-tick.present-tick.active
                    data-hint: "{RailTitle}"
                    span.history-rail-bar

        {if ShowAudioDeck}
            @AudioDeck
        {else}
            footer.reader-progress-footer#reader-progress-footer
                progress.reader-book-progress
                    value: "{BookProgressPct}"
                    max: 100

{def confirm_modal}
    {if ShowConfirmModal}
        .modal-backdrop#modal-confirm
            .modal-card
                .modal-header
                    h3.modal-title {ConfirmTitle}
                .modal-body
                    p.modal-desc {ConfirmMessage}
                    .modal-actions
                        button.btn.btn-confirm-cancel
                            "{t_cancel}"
                        button.btn.btn-confirm-accept .btn-danger?: {ConfirmIsDanger} .btn-primary?: {!ConfirmIsDanger}
                            "{t_confirm_btn}"
