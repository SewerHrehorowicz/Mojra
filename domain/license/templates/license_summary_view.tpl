{if IsActive}
    .node-meta-row#license-summary-view
        span.node-meta-key {t_license_key}
        .license-summary-inline
            span.license-summary-holder "{HolderName} ({Email}) · {t_license_tier_key} {Tier} · {t_license_expires_key} {ExpiresText}"
            button.btn.btn-sm#btn-settings-deactivate-license
                "{t_license_deactivate_btn}"
