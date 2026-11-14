section.view-panel#view-library
    .library-top-actions
        button.btn.btn-sm#btn-open-add-book
            "{t_add_book_btn}"
        {if DevMode}
            button.btn.btn-sm#btn-extract-embedded-book
                "{t_extract_embedded_book}"
    {if not Books}
        p.bookshelf-empty {t_bookshelf_empty}
    .bookshelf-grid
        {for b in Books}
            article.book-card
                button.book-cover-wrap
                    data-book: "{b.ID}"
                    data-view: player
                    {if b.CoverURL}
                        img.book-cover-img
                            alt: "{b.Title}"
                            src: {b.CoverURL}
                    span.book-cover-play-overlay .is-default-cover?: {!b.CoverURL}
                        svg.book-cover-play-icon src: /icons/playbook.svg
                .book-card-body
                    .book-card-meta
                        span.book-card-author {b.Author}
                        progress.book-progress-bar
                            value: "{b.ProgressPct}"
                            max: 100
                    h3.book-card-title {b.Title}
                    p.book-card-desc {b.Description}
    .home-mode-footer
        button.home-mode-switch-btn#btn-switch-to-studio
            data-view: studio
            svg.home-mode-switch-icon src: /icons/studio.svg
            span.home-mode-switch-label
                "{t_home_switch_to_studio_prefix} "
                strong.home-mode-switch-target "Playbook Studio"
                span.home-mode-switch-arrow "  →"
