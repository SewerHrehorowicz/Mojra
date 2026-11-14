{if ShowConfirmModal}
    .modal-backdrop#confirm-view
        .modal-card
            .modal-header
                h3.modal-title {ConfirmTitle}
            .modal-body
                p.modal-desc {ConfirmMessage}
                .modal-actions
                    button.btn.btn-confirm-cancel
                        "{t_cancel}"
                    button.btn.btn-confirm-accept .btn-danger?: {ConfirmIsDanger} .btn-primary?: {!ConfirmIsDanger}
                        "{t_confirm_btn}"
