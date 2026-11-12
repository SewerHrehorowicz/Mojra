//go:build !(js && wasm)

package actions

import (
	"net/http"

	"mojra/internal/state"
)

func openQRModal(s *state.AppState, _ *http.Request) string {
	s.Modals.QR = true
	return ""
}

func closeQRModal(s *state.AppState, _ *http.Request) string {
	s.Modals.QR = false
	return ""
}
