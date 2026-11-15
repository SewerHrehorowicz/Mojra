//go:build !(js && wasm)

package actions

import (
	"mojra/domain/shell/studio_app/state"
	"mojra/engine/osbridge"
	"mojra/engine/transport"
)

var _ = registerActions(map[string]ActionHandler{
	"PromptQuitApp": promptQuitApp,
})

func promptQuitApp(s *state.AppState, _ *transport.ActionRequest) []transport.ActionFrame {
	s.Confirm.Reset()
	s.Overlays.NativeCmd = nativeCmdQuit
	go func() {
		s.Config.TriggerShutdown()
		osbridge.TerminateNativeWindow()
	}()
	return nil
}
