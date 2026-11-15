//go:build js && wasm

package osbridge

import "syscall/js"

const (
	jsLocalStorageKey = "localStorage"
	jsGetItemMethod   = "getItem"
	jsSetItemMethod   = "setItem"
	jsUint8ArrayClass = "Uint8Array"
	jsBlobClass       = "Blob"
	jsObjectClass     = "Object"
	jsURLClass        = "URL"
	jsCreateObjURL    = "createObjectURL"
	jsPropType        = "type"
	savePrefixKey     = "mojra_save_"
)

// BindBrowserWasmHooks attaches browser window.localStorage and URL.createObjectURL hooks onto storage.
func BindBrowserWasmHooks(storage *WasmStorage) {
	if storage == nil {
		return
	}
	storage.LoadKVHook = func(key string) (string, bool) {
		ls := js.Global().Get(jsLocalStorageKey)
		if !ls.Truthy() {
			return "", false
		}
		val := ls.Call(jsGetItemMethod, savePrefixKey+key)
		if !val.Truthy() || val.Type() != js.TypeString {
			return "", false
		}
		return val.String(), true
	}
	storage.StoreKVHook = func(key string, value string) {
		ls := js.Global().Get(jsLocalStorageKey)
		if ls.Truthy() {
			ls.Call(jsSetItemMethod, savePrefixKey+key, value)
		}
	}
	storage.BlobURLHook = func(_ string, payload []byte, mimeType string) string {
		u8 := js.Global().Get(jsUint8ArrayClass).New(len(payload))
		js.CopyBytesToJS(u8, payload)
		opts := js.Global().Get(jsObjectClass).New()
		opts.Set(jsPropType, mimeType)
		blob := js.Global().Get(jsBlobClass).New([]any{u8}, opts)
		return js.Global().Get(jsURLClass).Call(jsCreateObjURL, blob).String()
	}
}
