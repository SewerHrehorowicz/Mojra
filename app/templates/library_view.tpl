section.view-panel#view-library
    .library-top-actions
        button.btn.btn-sm
            type: button
            action: OpenAddBookModal
            "+ Dodaj książkę"
        {if dev_mode}
            button.btn.btn-sm
                type: button
                action: ExtractEmbeddedBook
                "Wypakuj wbudowaną książkę"
    {if not books}
        p.bookshelf-empty "No books in library page"
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
                        svg.book-cover-play-icon src: /icons/play.svg
                .book-card-body
                    .book-card-meta
                        span.book-card-author {b.author}
                        progress.book-progress-bar
                            value: "{b.progress_pct}"
                            max: 100
                    h3.book-card-title {b.title}
                    p.book-card-desc {b.description}
                    {if b.has_warnings}
                        .book-warnings-box
                            .book-warnings-title "⚠ Ostrzeżenia parsera i walidacji ({b.warning_count}):"
                            ul.book-warnings-list
                                {for w in b.warnings}
                                    li {w.text}
