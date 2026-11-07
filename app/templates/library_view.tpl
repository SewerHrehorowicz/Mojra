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
                    span.book-cover-play-overlay
                        svg.book-cover-play-icon src: /icons/playbook.svg
                .book-card-body
                    .book-card-meta
                        span.book-card-author {b.author}
                        progress.book-progress-bar
                            value: "{b.progress_pct}"
                            max: 100
                    h3.book-card-title {b.title}
                    p.book-card-desc {b.description}
