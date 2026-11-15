//go:build !(js && wasm)

package actions

import (
	"net/http"

	"mojra/domain/state"
	"mojra/engine"
)

var _ = registerStandaloneActions(map[string]ActionHandler{
	"OpenQRModal":  openQRModal,
	"CloseQRModal": closeQRModal,
})

func openQRModal(s *state.AppState, _ *http.Request) []engine.ActionFrame {
	s.Overlays.QR = true
	return frameModals(s, "")
}

func closeQRModal(s *state.AppState, _ *http.Request) []engine.ActionFrame {
	s.Overlays.QR = false
	return frameModals(s, "")
}
