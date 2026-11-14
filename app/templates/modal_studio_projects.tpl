{if ShowStudioProjectModal}
    .modal-backdrop#modal-backdrop-studio-projects
        .modal-card.modal-card-lg#modal-studio-project
            .modal-header
                h3.modal-title {t_open_project_modal_title}
                button.node-close-btn.btn-close-studio-projects
                    svg.node-close-icon src: /icons/close.svg
            .modal-body
                {if HasStudioProjects}
                    .add-book-section
                        .studio-projects-list
                            {for sp in StudioProjects}
                                button.studio-project-row .active?: {sp.IsCurrent}
                                    data-book: "{sp.ID}"
                                    span.studio-project-main
                                        span.studio-project-title {sp.Title}
                                        span.studio-project-author {sp.Author}
                                    span.studio-project-badge {sp.KindBadge}
                .add-book-section
                    .add-book-browse-row
                        button.btn.btn-sm.btn-browse-studio-project
                            data-mode: dir
                            "{t_choose_project_dir_btn}"
                        button.btn.btn-sm.btn-browse-studio-project
                            data-mode: file
                            "{t_unpack_playbook_copy_btn}"
                    form.add-book-inline-form#form-open-studio-project-disk
                        input.field-input#studio-project-path-input
                            type: text
                            name: path
                            required: true
                            placeholder: "{t_studio_project_path_placeholder}"
                        button.btn.btn-primary
                            type: submit
                            "{t_open_btn}"
                .modal-actions
                    button.btn.btn-close-studio-projects
                        "{t_close}"
