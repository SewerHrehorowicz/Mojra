{if ShowEditorHelpModal}
    .modal-backdrop#modal-backdrop-editor-help
        .modal-card.modal-card-lg
            .modal-header
                h3.modal-title {t_help_panel_title}
                button.node-close-btn.btn-close-editor-help
                    svg.node-close-icon src: /icons/close.svg
            .modal-body
                .help-shortcuts-list
                    .help-shortcut-row
                        span.help-shortcut-key {t_help_m_k1}
                        span.help-shortcut-desc {t_help_m_d1}
                    .help-shortcut-row
                        span.help-shortcut-key {t_help_m_k2}
                        span.help-shortcut-desc {t_help_m_d2}
                    .help-shortcut-row
                        span.help-shortcut-key {t_help_m_k3}
                        span.help-shortcut-desc {t_help_m_d3}
                    .help-shortcut-row
                        span.help-shortcut-key {t_help_m_k4}
                        span.help-shortcut-desc {t_help_m_d4}
                    .help-shortcut-row
                        span.help-shortcut-key {t_help_m_k5}
                        span.help-shortcut-desc {t_help_m_d5}
                    .help-shortcut-row
                        span.help-shortcut-key {t_help_m_k6}
                        span.help-shortcut-desc {t_help_m_d6}
                    .help-shortcut-row
                        span.help-shortcut-key {t_help_m_k7}
                        span.help-shortcut-desc {t_help_m_d7}
                    .help-shortcut-row
                        span.help-shortcut-key {t_help_m_k8}
                        span.help-shortcut-desc {t_help_m_d8}
                .modal-actions
                    button.btn.btn-primary.btn-close-editor-help
                        "{t_got_it_btn}"
