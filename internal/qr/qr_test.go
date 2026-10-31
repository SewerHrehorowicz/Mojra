package qr

import (
	"bytes"
	"image/png"
	"testing"
)

func decodeMatrixPayload(t *testing.T, modules [][]bool) string {
	t.Helper()
	size := len(modules)

	var spec versionSpec
	for _, v := range versions {
		if v.size == size {
			spec = v
			break
		}
	}
	if spec.version == 0 {
		t.Fatalf("unknown QR matrix size: %d", size)
	}

	// Verify both copies of the 15-bit format information
	var copy1, copy2 int
	copy1Positions := [15][2]int{
		{0, 8}, {1, 8}, {2, 8}, {3, 8}, {4, 8}, {5, 8},
		{7, 8}, {8, 8}, {8, 7},
		{8, 5}, {8, 4}, {8, 3}, {8, 2}, {8, 1}, {8, 0},
	}
	for i, pos := range copy1Positions {
		if modules[pos[0]][pos[1]] {
			copy1 |= 1 << i
		}
	}

	for i := 0; i < 8; i++ {
		if modules[8][size-1-i] {
			copy2 |= 1 << i
		}
	}
	for i := 8; i < 15; i++ {
		if modules[size-15+i][8] {
			copy2 |= 1 << i
		}
	}

	if copy1 != copy2 {
		t.Fatalf("format info mismatch between copy1 (%015b) and copy2 (%015b)", copy1, copy2)
	}
	if !modules[size-8][8] {
		t.Fatalf("dark module at (%d, 8) is not set", size-8)
	}

	unmaskedFormat := copy1 ^ 0x5412
	dataBits := unmaskedFormat >> 10
	ecLevel := (dataBits >> 3) & 0b11
	mask := dataBits & 0b111
	if ecLevel != 0b01 {
		t.Fatalf("expected EC level L (01), got %02b", ecLevel)
	}
	if mask < 0 || mask > 7 {
		t.Fatalf("expected mask 0..7, got %d", mask)
	}

	// Build function pattern map to read codewords
	dummy := make([][]bool, size)
	isFunc := make([][]bool, size)
	for i := range dummy {
		dummy[i] = make([]bool, size)
		isFunc[i] = make([]bool, size)
	}
	drawFunctionPatterns(dummy, isFunc, spec)

	totalCodewords := spec.dataBytes + spec.ecBytes
	codewords := make([]byte, totalCodewords)
	bitIdx := 0
	totalBits := totalCodewords * 8

	for right := size - 1; right >= 1; right -= 2 {
		if right == 6 {
			right = 5
		}
		for vert := 0; vert < size; vert++ {
			for j := 0; j < 2; j++ {
				c := right - j
				upward := ((right + 1) & 2) == 0
				r := vert
				if upward {
					r = size - 1 - vert
				}
				if !isFunc[r][c] && bitIdx < totalBits {
					dark := modules[r][c]
					if maskBit(mask, r, c) {
						dark = !dark
					}
					if dark {
						codewords[bitIdx>>3] |= 1 << (7 - (bitIdx & 7))
					}
					bitIdx++
				}
			}
		}
	}

	if bitIdx != totalBits {
		t.Fatalf("expected to read %d bits, got %d", totalBits, bitIdx)
	}

	// Verify Reed-Solomon EC codewords match
	dataPart := codewords[:spec.dataBytes]
	ecPart := codewords[spec.dataBytes:]
	expectedEC := reedSolomonRemainder(dataPart, spec.ecBytes)
	if !bytes.Equal(ecPart, expectedEC) {
		t.Fatalf("Reed-Solomon EC mismatch: got %x, want %x", ecPart, expectedEC)
	}

	// Decode Byte mode stream
	mode := (dataPart[0] >> 4) & 0x0F
	if mode != 0b0100 {
		t.Fatalf("expected Byte mode (0100), got %04b", mode)
	}
	charCount := int(((dataPart[0] & 0x0F) << 4) | ((dataPart[1] >> 4) & 0x0F))
	out := make([]byte, charCount)
	for i := 0; i < charCount; i++ {
		out[i] = ((dataPart[i+1] & 0x0F) << 4) | ((dataPart[i+2] >> 4) & 0x0F)
	}
	return string(out)
}

func TestEncodeMatrixAndPNG(t *testing.T) {
	inputs := []string{
		"http://10.0.0.1:8484",
		"http://192.168.1.42:8484",
		"http://192.168.100.123:8484",
		"https://storyteller.local:8484/preview?mode=mobile",
	}

	for _, text := range inputs {
		mat, err := EncodeMatrix(text)
		if err != nil {
			t.Fatalf("EncodeMatrix(%q) failed: %v", text, err)
		}
		got := decodeMatrixPayload(t, mat)
		if got != text {
			t.Fatalf("decoded payload = %q, want %q", got, text)
		}

		pngBytes, err := EncodePNG(text, 8)
		if err != nil {
			t.Fatalf("EncodePNG(%q) failed: %v", text, err)
		}
		img, err := png.Decode(bytes.NewReader(pngBytes))
		if err != nil {
			t.Fatalf("png.Decode(%q) failed: %v", text, err)
		}
		wantDim := (len(mat) + 8) * 8
		if img.Bounds().Dx() != wantDim || img.Bounds().Dy() != wantDim {
			t.Fatalf("unexpected PNG dimensions %v, want %dx%d", img.Bounds(), wantDim, wantDim)
		}
	}
}
