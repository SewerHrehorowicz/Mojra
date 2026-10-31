@topbar
main.workspace#workspace
    -> if is_view_library
        @library_view
    -> elif is_view_player
        @player_view
    -> elif is_view_editor
        @editor_view
.modals-root#modals-root
    @modals
.toast.hidden#toast-banner
.ui-tooltip.hidden#ui-tooltip
