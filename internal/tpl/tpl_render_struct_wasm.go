//go:build js && wasm

package tpl

func chunkTemplateName(_ any) string {
	return ""
}

func lookupStructField(_ any, _ string) (any, bool) {
	return nil, false
}

func isStructOrSliceTruthy(_ any) bool {
	return true
}

func reflectSliceLen(_ any) int {
	return 0
}

func reflectSliceItemAt(_ any, _ int) any {
	return nil
}
