{if ShowAddBookModal}
    .modal-backdrop#modal-backdrop-add-book
        .modal-card.modal-card-lg
            .modal-header
                h3.modal-title {t_add_book_modal_title}
                button.node-close-btn.btn-close-add-book
                    svg.node-close-icon src: /icons/close.svg
            .modal-body
                .add-book-section
                    .field-label {t_add_disk_hdr}
                    p.modal-desc {t_add_disk_desc}
                    .add-book-browse-row
                        button.btn.btn-sm.btn-browse-add-book
                            data-mode: dir
                            "{t_choose_book_dir_btn}"
                        button.btn.btn-sm.btn-browse-add-book
                            data-mode: file
                            "{t_choose_book_file_btn}"
                    form.add-book-inline-form#form-add-book-disk
                        input.field-input
                            type: text
                            name: path
                            required: true
                            placeholder: "{t_add_disk_path_placeholder}"
                        button.btn.btn-primary
                            type: submit
                            "{t_add_from_disk_btn}"
                .add-book-section
                    .field-label {t_add_url_hdr}
                    p.modal-desc {t_add_url_desc}
                    form.add-book-inline-form#form-add-book-url
                        input.field-input
                            type: url
                            name: url
                            required: true
                            placeholder: "{t_add_url_placeholder}"
                        button.btn.btn-primary
                            type: submit
                            "{t_download_from_url_btn}"
                .modal-actions
                    button.btn.btn-close-add-book
                        "{t_close}"
