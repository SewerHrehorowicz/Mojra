{if ShowQRModal}
    .modal-backdrop#modal-backdrop-qr
        .modal-card
            .modal-header
                h3.modal-title {t_settings_modal_title}
            .modal-body.qr-modal-body
                .node-meta-fields.modal-meta-fields
                    .node-meta-row
                        span.node-meta-key {t_settings_lang_key}
                        .form-radio-group.is-inline
                            {for opt in lang_options}
                                button.form-radio-option.lang-radio-btn .active?: {opt.is_active}
                                    data-lang: "{opt.code}"
                                    span.form-radio-label "{opt.upper}"
                                    span.form-radio-sq
                    .node-meta-row
                        span.node-meta-key {t_theme_key}
                        button.book-setting-toggle-btn.form-toggle-btn#btn-toggle-theme .active?: {IsDark}
                            span.custom-select-label {ThemeToggleLabel}
                            span.toggle-right
                                svg.action-icon#icon-theme-moon .hidden?: {!IsDark} src: /icons/theme.svg
                                svg.action-icon#icon-theme-sun .hidden?: {IsDark} src: /icons/sun.svg
                                span.toggle-switch
                                    span.toggle-thumb
                    .node-meta-row
                        span.node-meta-key {t_images_key}
                        button.book-setting-toggle-btn.form-toggle-btn#btn-toggle-imageless .active?: {!Imageless} .is-disabled?: {ImagelessLocked}
                            disabled?: {ImagelessLocked}
                            span.custom-select-label {ImagelessToggleLabel}
                            span.toggle-right
                                svg.action-icon#icon-imageless-on .hidden?: {!Imageless} src: /icons/image-off.svg
                                svg.action-icon#icon-imageless-off .hidden?: {Imageless} src: /icons/image.svg
                                span.toggle-switch
                                    span.toggle-thumb
                    .node-meta-row
                        span.node-meta-key {t_scale_key}
                        .form-radio-group.is-inline
                            button.form-radio-option.ui-scale-radio-btn .active?: {IsSmallUI}
                                data-scale: small
                                span.form-radio-label {t_scale_default}
                                span.form-radio-sq
                            button.form-radio-option.ui-scale-radio-btn .active?: {IsLargeUI}
                                data-scale: large
                                span.form-radio-label {t_scale_large}
                                span.form-radio-sq
                    .node-meta-row
                        span.node-meta-key {t_scroll_sensitivity_key}
                        .form-radio-group.is-inline
                            button.form-radio-option.scroll-sens-radio-btn .active?: {IsScrollSensHigh}
                                data-sensitivity: high
                                span.form-radio-label {t_scroll_sens_high}
                                span.form-radio-sq
                            button.form-radio-option.scroll-sens-radio-btn .active?: {IsScrollSensMedium}
                                data-sensitivity: medium
                                span.form-radio-label {t_scroll_sens_medium}
                                span.form-radio-sq
                            button.form-radio-option.scroll-sens-radio-btn .active?: {IsScrollSensLow}
                                data-sensitivity: low
                                span.form-radio-label {t_scroll_sens_low}
                                span.form-radio-sq
                            button.form-radio-option.scroll-sens-radio-btn .active?: {IsScrollSensOff}
                                data-sensitivity: off
                                span.form-radio-label {t_scroll_sens_off}
                                span.form-radio-sq
                    .node-meta-row
                        span.node-meta-key {t_auto_choice_delay_key}
                        .form-radio-group.is-inline
                            button.form-radio-option.auto-delay-radio-btn .active?: {IsAutoDelay1}
                                data-delay: "1"
                                span.form-radio-label "1s"
                                span.form-radio-sq
                            button.form-radio-option.auto-delay-radio-btn .active?: {IsAutoDelay2}
                                data-delay: "2"
                                span.form-radio-label "2s"
                                span.form-radio-sq
                            button.form-radio-option.auto-delay-radio-btn .active?: {IsAutoDelay3}
                                data-delay: "3"
                                span.form-radio-label "3s"
                                span.form-radio-sq
                            button.form-radio-option.auto-delay-radio-btn .active?: {IsAutoDelay5}
                                data-delay: "5"
                                span.form-radio-label "5s"
                                span.form-radio-sq
                {if DevMode}
                    .settings-modal-actions
                        button.btn.btn-sm.btn-extract-embedded-book
                            "{t_extract_embedded_book}"
                    p.modal-desc {t_lan_qr_desc}
                    .qr-frame
                        img.qr-image
                            src: /api/lan-qr.png
                            alt: "{t_lan_qr_alt}"
                    .lan-url-box {LANURL}
                .modal-actions
                    button.btn.btn-close-qr
                        "{t_close}"
