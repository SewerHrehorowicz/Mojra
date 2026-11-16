section.view-panel.license-view#license-view
    .license-container
        .license-card
            .license-header
                h2.license-title {t_license_title}
                p.license-subtitle {t_license_subtitle}
            {if IsActive}
                .license-status-box#license-activated-summary
                    .node-meta-row
                        span.node-meta-key {t_license_status_active}
                        span.license-holder-value "{HolderName} ({Email})"
                    .node-meta-row
                        span.node-meta-key {t_license_tier_key}
                        span.license-tier-value "{Tier}"
                    .node-meta-row
                        span.node-meta-key {t_license_expires_key}
                        span.license-tier-value "{ExpiresText}"
                    .license-actions
                        button.btn#btn-deactivate-license
                            "{t_license_deactivate_btn}"
                        button.btn.btn-primary#btn-close-license-summary
                            "{t_close}"
            {else}
                form.license-form#form-license
                    .node-meta-fields
                        .node-meta-row
                            span.node-meta-key {t_license_key}
                            .node-media-picker-row
                                input.node-meta-input.field-input#license-key-input
                                    type: text
                                    name: key
                                    placeholder: "{t_license_key_placeholder}"
                                    autocomplete: "off"
                                    spellcheck: "false"
                                button.node-icon-btn.node-folder-btn#btn-browse-license-file
                                    svg.folder-browse-icon src: /icons/folder.svg
                    {if HasError}
                        p.license-error#license-error-msg {ErrorMsg}
                    .license-actions
                        button.btn.btn-primary#btn-activate-license
                            type: submit
                            "{t_license_activate_btn}"
        .home-mode-footer
            button.home-mode-switch-btn#btn-license-to-reader
                data-view: library
                svg.home-mode-switch-icon src: /icons/mojra.svg
                span.home-mode-switch-label
                    "{t_home_switch_to_reader_prefix} "
                    strong.home-mode-switch-target "Mojra"
                    span.home-mode-switch-arrow "  →"
