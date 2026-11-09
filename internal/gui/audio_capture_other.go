//go:build (!darwin || !cgo) && !(js && wasm)

package gui

import "fmt"

func StartNativeAudioCapture() error {
	return fmt.Errorf("natywne nagrywanie mikrofonu niedostępne na tej platformie")
}

func PollNativeAudioCapturePeak() (float64, bool) {
	return 0, false
}

func StopNativeAudioCapture() ([]byte, int, error) {
	return nil, 24000, fmt.Errorf("natywne nagrywanie mikrofonu niedostępne na tej platformie")
}
