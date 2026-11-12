{if show_studio_project_modal}
    .modal-backdrop#modal-backdrop-studio-projects
        data-backdrop-action: CloseStudioProjectModal
        .modal-card.modal-card-lg#modal-studio-project
            .modal-header
                h3.modal-title {t_open_project_modal_title}
                button.node-close-btn
                    type: button
                    action: CloseStudioProjectModal
                    svg.node-close-icon src: /icons/close.svg
            .modal-body
                {if has_studio_projects}
                    .add-book-section
                        .studio-projects-list
                            {for sp in studio_projects}
                                button.studio-project-row .active?: {sp.is_current}
                                    type: button
                                    action: OpenStudioProject
                                    data-book: "{sp.id}"
                                    span.studio-project-main
                                        span.studio-project-title {sp.title}
                                        span.studio-project-author {sp.author}
                                    span.studio-project-badge {sp.kind_badge}
                .add-book-section
                    .add-book-browse-row
                        button.btn.btn-sm
                            type: button
                            action: BrowseOpenStudioProject
                            data-mode: dir
                            "{t_choose_project_dir_btn}"
                        button.btn.btn-sm
                            type: button
                            action: BrowseOpenStudioProject
                            data-mode: file
                            "{t_unpack_playbook_copy_btn}"
                    form.add-book-inline-form
                        action: OpenStudioProjectFromDisk
                        input.field-input#studio-project-path-input
                            type: text
                            name: path
                            required: true
                            placeholder: "{t_studio_project_path_placeholder}"
                        button.btn.btn-primary
                            type: submit
                            "{t_open_btn}"
                .modal-actions
                    button.btn
                        type: button
                        action: CloseStudioProjectModal
                        "{t_close}"
