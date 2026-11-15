//go:build js && wasm

package main

import (
	"syscall/js"

	bookplayeractions "mojra/domain/shell/book_player/actions"
	"mojra/domain/shell/book_player/state"
	"mojra/engine/osbridge"
)

const (
	jsWasmBootFn    = "__wasmBoot"
	jsWasmActionFn  = "__wasmAction"
	errBootPrefix   = "ERR:"
	errExpectedWasm = "ERR:expected Uint8Array wasm binary"
	errShortWasm    = "ERR:wasm binary too short"
	minWasmBytes    = 132
)

var activeShell *state.ShellState

func jsBoot(_ js.Value, args []js.Value) any {
	if len(args) < 1 || args[0].Type() != js.TypeObject {
		return errExpectedWasm
	}
	byteLen := args[0].Get("byteLength").Int()
	if byteLen < minWasmBytes {
		return errShortWasm
	}
	wasmBytes := make([]byte, byteLen)
	js.CopyBytesToGo(wasmBytes, args[0])

	savedHex := ""
	if len(args) >= 2 && args[1].Type() == js.TypeString {
		savedHex = args[1].String()
	}

	storage := osbridge.NewWasmStorage()
	osbridge.BindBrowserWasmHooks(storage)

	shell, initialFDP, bootErr := bookplayeractions.BootFromWasmBytes(wasmBytes, savedHex, storage)
	if bootErr != nil {
		return errBootPrefix + bootErr.Error()
	}
	activeShell = shell
	return initialFDP
}

func jsAction(_ js.Value, args []js.Value) any {
	if activeShell == nil || len(args) < 1 {
		return ""
	}
	return bookplayeractions.DispatchAction(activeShell, args[0].String())
}

func main() {
	js.Global().Set(jsWasmBootFn, js.FuncOf(jsBoot))
	js.Global().Set(jsWasmActionFn, js.FuncOf(jsAction))
	select {}
}
