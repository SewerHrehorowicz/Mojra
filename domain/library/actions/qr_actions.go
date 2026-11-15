//go:build !(js && wasm)

package actions

import (
	"net/http"

	"mojra/domain/library/state"
	"mojra/engine/transport"
)

var _ = registerStandaloneActions(map[string]ActionHandler{
	"OpenQRModal":  openQRModal,
	"CloseQRModal": closeQRModal,
})

func openQRModal(s *state.LibraryState, _ *http.Request) []transport.ActionFrame {
	s.Overlays.QR = true
	return frameLibraryModals(s, "")
}

func closeQRModal(s *state.LibraryState, _ *http.Request) []transport.ActionFrame {
	s.Overlays.QR = false
	return frameLibraryModals(s, "")
}
