section.view-panel#view-library
    .library-top-actions
        button.btn.btn-sm
            type: button
            action: OpenAddBookModal
            "{t_add_book_btn}"
        {if dev_mode}
            button.btn.btn-sm
                type: button
                action: ExtractEmbeddedBook
                "{t_extract_embedded_book}"
    {if not books}
        p.bookshelf-empty {t_bookshelf_empty}
    .bookshelf-grid
        {for b in books}
            article.book-card
                button.book-cover-wrap
                    type: button
                    action: OpenBook
                    data-book: "{b.id}"
                    data-view: player
                    {if b.cover_url}
                        img.book-cover-img
                            alt: "{b.title}"
                            src: {b.cover_url}
                    span.book-cover-play-overlay .is-default-cover?: {!b.cover_url}
                        svg.book-cover-play-icon src: /icons/playbook.svg
                .book-card-body
                    .book-card-meta
                        span.book-card-author {b.author}
                        progress.book-progress-bar
                            value: "{b.progress_pct}"
                            max: 100
                    h3.book-card-title {b.title}
                    p.book-card-desc {b.description}
    .home-mode-footer
        button.home-mode-switch-btn#btn-switch-to-studio
            type: button
            action: SwitchView
            data-view: studio
            svg.home-mode-switch-icon src: /icons/studio.svg
            span.home-mode-switch-label
                "{t_home_switch_to_studio_prefix} "
                strong.home-mode-switch-target "Playbook Studio"
                span.home-mode-switch-arrow "  →"
