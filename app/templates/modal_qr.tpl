{if show_qr_modal}
    .modal-backdrop#modal-backdrop-qr
        data-backdrop-action: CloseQRModal
        .modal-card
            .modal-header
                h3.modal-title {t_settings_modal_title}
            .modal-body.qr-modal-body
                .node-meta-fields.modal-meta-fields
                    .node-meta-row
                        span.node-meta-key {t_settings_lang_key}
                        .form-radio-group.is-inline
                            {for opt in lang_options}
                                button.form-radio-option .active?: {opt.is_active}
                                    type: button
                                    action: SetLanguage
                                    data-lang: "{opt.code}"
                                    span.form-radio-label "{opt.upper}"
                                    span.form-radio-sq
                    .node-meta-row
                        span.node-meta-key {t_theme_key}
                        button.book-setting-toggle-btn.form-toggle-btn#btn-toggle-theme .active?: {is_dark}
                            type: button
                            action: ToggleTheme
                            span.custom-select-label {theme_toggle_label}
                            span.toggle-right
                                svg.action-icon#icon-theme-moon .hidden?: {!is_dark} src: /icons/theme.svg
                                svg.action-icon#icon-theme-sun .hidden?: {is_dark} src: /icons/sun.svg
                                span.toggle-switch
                                    span.toggle-thumb
                    .node-meta-row
                        span.node-meta-key {t_images_key}
                        button.book-setting-toggle-btn.form-toggle-btn#btn-toggle-imageless .active?: {!imageless} .is-disabled?: {imageless_locked}
                            type: button
                            action: ToggleImageless
                            disabled?: {imageless_locked}
                            span.custom-select-label {imageless_toggle_label}
                            span.toggle-right
                                svg.action-icon#icon-imageless-on .hidden?: {!imageless} src: /icons/image-off.svg
                                svg.action-icon#icon-imageless-off .hidden?: {imageless} src: /icons/image.svg
                                span.toggle-switch
                                    span.toggle-thumb
                    .node-meta-row
                        span.node-meta-key {t_scale_key}
                        .form-radio-group.is-inline
                            button.form-radio-option .active?: {is_small_ui}
                                type: button
                                action: SetUIScale
                                data-scale: small
                                span.form-radio-label {t_scale_default}
                                span.form-radio-sq
                            button.form-radio-option .active?: {is_large_ui}
                                type: button
                                action: SetUIScale
                                data-scale: large
                                span.form-radio-label {t_scale_large}
                                span.form-radio-sq
                    .node-meta-row
                        span.node-meta-key {t_scroll_sensitivity_key}
                        .form-radio-group.is-inline
                            button.form-radio-option .active?: {is_scroll_sens_high}
                                type: button
                                action: SetScrollSensitivity
                                data-sensitivity: high
                                span.form-radio-label {t_scroll_sens_high}
                                span.form-radio-sq
                            button.form-radio-option .active?: {is_scroll_sens_medium}
                                type: button
                                action: SetScrollSensitivity
                                data-sensitivity: medium
                                span.form-radio-label {t_scroll_sens_medium}
                                span.form-radio-sq
                            button.form-radio-option .active?: {is_scroll_sens_low}
                                type: button
                                action: SetScrollSensitivity
                                data-sensitivity: low
                                span.form-radio-label {t_scroll_sens_low}
                                span.form-radio-sq
                            button.form-radio-option .active?: {is_scroll_sens_off}
                                type: button
                                action: SetScrollSensitivity
                                data-sensitivity: off
                                span.form-radio-label {t_scroll_sens_off}
                                span.form-radio-sq
                    .node-meta-row
                        span.node-meta-key {t_auto_choice_delay_key}
                        .form-radio-group.is-inline
                            button.form-radio-option .active?: {is_auto_delay_1}
                                type: button
                                action: SetAutoChoiceDelay
                                data-delay: "1"
                                span.form-radio-label "1s"
                                span.form-radio-sq
                            button.form-radio-option .active?: {is_auto_delay_2}
                                type: button
                                action: SetAutoChoiceDelay
                                data-delay: "2"
                                span.form-radio-label "2s"
                                span.form-radio-sq
                            button.form-radio-option .active?: {is_auto_delay_3}
                                type: button
                                action: SetAutoChoiceDelay
                                data-delay: "3"
                                span.form-radio-label "3s"
                                span.form-radio-sq
                            button.form-radio-option .active?: {is_auto_delay_5}
                                type: button
                                action: SetAutoChoiceDelay
                                data-delay: "5"
                                span.form-radio-label "5s"
                                span.form-radio-sq
                {if dev_mode}
                    .settings-modal-actions
                        button.btn.btn-sm
                            type: button
                            action: ExtractEmbeddedBook
                            "{t_extract_embedded_book}"
                    p.modal-desc {t_lan_qr_desc}
                    .qr-frame
                        img.qr-image
                            src: /api/lan-qr.png
                            alt: "{t_lan_qr_alt}"
                    .lan-url-box {lan_url}
                .modal-actions
                    button.btn
                        type: button
                        action: CloseQRModal
                        "{t_close}"
