package qr

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"strings"
)

// Pure-Go ISO/IEC 18004 QR Code generator (Byte mode, Error Correction Level L, Versions 1..4).

type versionSpec struct {
	version    int
	size       int
	dataBytes  int
	ecBytes    int
	alignCoord []int
}

var versions = []versionSpec{
	{version: 1, size: 21, dataBytes: 19, ecBytes: 7, alignCoord: nil},
	{version: 2, size: 25, dataBytes: 34, ecBytes: 10, alignCoord: []int{6, 18}},
	{version: 3, size: 29, dataBytes: 55, ecBytes: 15, alignCoord: []int{6, 22}},
	{version: 4, size: 33, dataBytes: 80, ecBytes: 20, alignCoord: []int{6, 26}},
}

// EncodeMatrix encodes text into a QR code boolean matrix (true = dark module).
func EncodeMatrix(text string) ([][]bool, error) {
	payload := []byte(text)
	var spec versionSpec
	for _, v := range versions {
		if len(payload)+3 <= v.dataBytes {
			spec = v
			break
		}
	}
	if spec.version == 0 {
		// Truncate or fallback to version 4 max capacity
		spec = versions[len(versions)-1]
		if len(payload) > spec.dataBytes-3 {
			payload = payload[:spec.dataBytes-3]
		}
	}

	codewords := buildDataCodewords(payload, spec.dataBytes)
	ecWords := reedSolomonRemainder(codewords, spec.ecBytes)
	allWords := append(codewords, ecWords...)

	size := spec.size
	modules := make([][]bool, size)
	isFunc := make([][]bool, size)
	for i := range modules {
		modules[i] = make([]bool, size)
		isFunc[i] = make([]bool, size)
	}

	drawFunctionPatterns(modules, isFunc, spec)
	drawCodewords(modules, isFunc, allWords)

	// Choose the mask (0..7) with the lowest ISO/IEC 18004 penalty score
	bestMask := 0
	bestPenalty := int(^uint(0) >> 1)
	for mask := 0; mask < 8; mask++ {
		applyMask(modules, isFunc, mask)
		drawFormatBits(modules, isFunc, mask)
		if p := penaltyScore(modules); p < bestPenalty {
			bestPenalty = p
			bestMask = mask
		}
		applyMask(modules, isFunc, mask) // revert mask
	}
	applyMask(modules, isFunc, bestMask)
	drawFormatBits(modules, isFunc, bestMask)

	return modules, nil
}

// EncodePNG renders a QR code for text as a crisp black-on-white PNG image with a 4-module quiet zone.
func EncodePNG(text string, modulePx int) ([]byte, error) {
	if modulePx < 4 {
		modulePx = 8
	}
	mat, err := EncodeMatrix(text)
	if err != nil {
		return nil, err
	}
	n := len(mat)
	quiet := 4
	total := (n + quiet*2) * modulePx

	img := image.NewRGBA(image.Rect(0, 0, total, total))
	white := color.RGBA{255, 255, 255, 255}
	dark := color.RGBA{0, 0, 0, 255}

	for y := 0; y < total; y++ {
		for x := 0; x < total; x++ {
			img.SetRGBA(x, y, white)
		}
	}

	for r := 0; r < n; r++ {
		for c := 0; c < n; c++ {
			if !mat[r][c] {
				continue
			}
			y0 := (r + quiet) * modulePx
			x0 := (c + quiet) * modulePx
			for dy := 0; dy < modulePx; dy++ {
				for dx := 0; dx < modulePx; dx++ {
					img.SetRGBA(x0+dx, y0+dy, dark)
				}
			}
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// EncodeSVG renders a QR code as an SVG string.
func EncodeSVG(text string) (string, error) {
	mat, err := EncodeMatrix(text)
	if err != nil {
		return "", err
	}
	n := len(mat)
	quiet := 4
	total := n + quiet*2

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d" shape-rendering="crispEdges">`, total, total))
	sb.WriteString(fmt.Sprintf(`<rect width="%d" height="%d" fill="#ffffff"/>`, total, total))
	for r := 0; r < n; r++ {
		for c := 0; c < n; c++ {
			if mat[r][c] {
				sb.WriteString(fmt.Sprintf(`<rect x="%d" y="%d" width="1" height="1" fill="#000000"/>`, c+quiet, r+quiet))
			}
		}
	}
	sb.WriteString(`</svg>`)
	return sb.String(), nil
}

func buildDataCodewords(payload []byte, capacityBytes int) []byte {
	var bits []uint8
	appendBits := func(val int, length int) {
		for i := length - 1; i >= 0; i-- {
			bits = append(bits, uint8((val>>i)&1))
		}
	}

	// Byte mode indicator (0100) + 8-bit character count (for Version 1..9)
	appendBits(0b0100, 4)
	appendBits(len(payload), 8)
	for _, b := range payload {
		appendBits(int(b), 8)
	}

	// Terminator up to 4 zero bits
	maxBits := capacityBytes * 8
	for i := 0; i < 4 && len(bits) < maxBits; i++ {
		bits = append(bits, 0)
	}
	for len(bits)%8 != 0 {
		bits = append(bits, 0)
	}

	out := make([]byte, 0, capacityBytes)
	for i := 0; i < len(bits); i += 8 {
		var b byte
		for j := 0; j < 8; j++ {
			b = (b << 1) | bits[i+j]
		}
		out = append(out, b)
	}

	padBytes := []byte{0xEC, 0x11}
	for idx := 0; len(out) < capacityBytes; idx++ {
		out = append(out, padBytes[idx%2])
	}
	return out
}

func gfMul(x, y byte) byte {
	var z byte
	for i := 7; i >= 0; i-- {
		z = (z << 1) ^ ((z >> 7) * 0x1D)
		z ^= ((y >> i) & 1) * x
	}
	return z
}

func reedSolomonRemainder(data []byte, degree int) []byte {
	divisor := make([]byte, degree)
	divisor[degree-1] = 1
	root := byte(1)
	for i := 0; i < degree; i++ {
		for j := 0; j < degree; j++ {
			divisor[j] = gfMul(divisor[j], root)
			if j+1 < degree {
				divisor[j] ^= divisor[j+1]
			}
		}
		root = gfMul(root, 0x02)
	}

	result := make([]byte, degree)
	for _, b := range data {
		factor := b ^ result[0]
		copy(result, result[1:])
		result[degree-1] = 0
		for j := 0; j < degree; j++ {
			result[j] ^= gfMul(divisor[j], factor)
		}
	}
	return result
}

func setFuncModule(modules, isFunc [][]bool, r, c int, dark bool) {
	modules[r][c] = dark
	isFunc[r][c] = true
}

func drawFinderPattern(modules, isFunc [][]bool, centerR, centerC int) {
	size := len(modules)
	for dy := -4; dy <= 4; dy++ {
		for dx := -4; dx <= 4; dx++ {
			r, c := centerR+dy, centerC+dx
			if r < 0 || r >= size || c < 0 || c >= size {
				continue
			}
			dist := absInt(dx)
			if absInt(dy) > dist {
				dist = absInt(dy)
			}
			dark := dist != 2 && dist != 4
			setFuncModule(modules, isFunc, r, c, dark)
		}
	}
}

func drawAlignmentPattern(modules, isFunc [][]bool, centerR, centerC int) {
	for dy := -2; dy <= 2; dy++ {
		for dx := -2; dx <= 2; dx++ {
			dist := absInt(dx)
			if absInt(dy) > dist {
				dist = absInt(dy)
			}
			setFuncModule(modules, isFunc, centerR+dy, centerC+dx, dist != 1)
		}
	}
}

func drawFunctionPatterns(modules, isFunc [][]bool, spec versionSpec) {
	size := spec.size

	// Finder patterns
	drawFinderPattern(modules, isFunc, 3, 3)
	drawFinderPattern(modules, isFunc, 3, size-4)
	drawFinderPattern(modules, isFunc, size-4, 3)

	// Timing patterns
	for i := 0; i < size; i++ {
		if !isFunc[6][i] {
			setFuncModule(modules, isFunc, 6, i, i%2 == 0)
		}
		if !isFunc[i][6] {
			setFuncModule(modules, isFunc, i, 6, i%2 == 0)
		}
	}

	// Alignment patterns
	for _, r := range spec.alignCoord {
		for _, c := range spec.alignCoord {
			if (r == 6 && c == 6) || (r == 6 && c == size-7) || (r == size-7 && c == 6) {
				continue
			}
			drawAlignmentPattern(modules, isFunc, r, c)
		}
	}

	// Reserve format info bits & dark module
	drawFormatBits(modules, isFunc, 0)
}

func drawFormatBits(modules, isFunc [][]bool, mask int) {
	size := len(modules)
	// EC Level L = 01
	data := (1 << 3) | mask
	rem := data
	for i := 0; i < 10; i++ {
		rem = (rem << 1) ^ ((rem >> 9) * 0x537)
	}
	bits := ((data << 10) | rem) ^ 0x5412

	bit := func(i int) bool {
		return ((bits >> i) & 1) != 0
	}

	for i := 0; i <= 5; i++ {
		setFuncModule(modules, isFunc, i, 8, bit(i))
	}
	setFuncModule(modules, isFunc, 7, 8, bit(6))
	setFuncModule(modules, isFunc, 8, 8, bit(7))
	setFuncModule(modules, isFunc, 8, 7, bit(8))
	for i := 9; i < 15; i++ {
		setFuncModule(modules, isFunc, 8, 14-i, bit(i))
	}

	for i := 0; i < 8; i++ {
		setFuncModule(modules, isFunc, 8, size-1-i, bit(i))
	}
	for i := 8; i < 15; i++ {
		setFuncModule(modules, isFunc, size-15+i, 8, bit(i))
	}
	setFuncModule(modules, isFunc, size-8, 8, true)
}

func drawCodewords(modules, isFunc [][]bool, codewords []byte) {
	size := len(modules)
	bitIdx := 0
	totalBits := len(codewords) * 8

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
					dark := ((codewords[bitIdx>>3] >> (7 - (bitIdx & 7))) & 1) != 0
					modules[r][c] = dark
					bitIdx++
				}
			}
		}
	}
}

func maskBit(mask, r, c int) bool {
	switch mask {
	case 0:
		return (r+c)%2 == 0
	case 1:
		return r%2 == 0
	case 2:
		return c%3 == 0
	case 3:
		return (r+c)%3 == 0
	case 4:
		return (r/2+c/3)%2 == 0
	case 5:
		return (r*c)%2+(r*c)%3 == 0
	case 6:
		return ((r*c)%2+(r*c)%3)%2 == 0
	case 7:
		return ((r+c)%2+(r*c)%3)%2 == 0
	default:
		return false
	}
}

func applyMask(modules, isFunc [][]bool, mask int) {
	size := len(modules)
	for r := 0; r < size; r++ {
		for c := 0; c < size; c++ {
			if !isFunc[r][c] && maskBit(mask, r, c) {
				modules[r][c] = !modules[r][c]
			}
		}
	}
}

func penaltyScore(modules [][]bool) int {
	size := len(modules)
	score := 0

	// N1: 5+ consecutive modules in row or column
	for r := 0; r < size; r++ {
		runColor := modules[r][0]
		runLen := 1
		for c := 1; c < size; c++ {
			if modules[r][c] == runColor {
				runLen++
				if runLen == 5 {
					score += 3
				} else if runLen > 5 {
					score++
				}
			} else {
				runColor = modules[r][c]
				runLen = 1
			}
		}
	}
	for c := 0; c < size; c++ {
		runColor := modules[0][c]
		runLen := 1
		for r := 1; r < size; r++ {
			if modules[r][c] == runColor {
				runLen++
				if runLen == 5 {
					score += 3
				} else if runLen > 5 {
					score++
				}
			} else {
				runColor = modules[r][c]
				runLen = 1
			}
		}
	}

	// N2: 2x2 blocks of same color
	for r := 0; r < size-1; r++ {
		for c := 0; c < size-1; c++ {
			v := modules[r][c]
			if modules[r][c+1] == v && modules[r+1][c] == v && modules[r+1][c+1] == v {
				score += 3
			}
		}
	}

	// N4: dark/light balance
	dark := 0
	for r := 0; r < size; r++ {
		for c := 0; c < size; c++ {
			if modules[r][c] {
				dark++
			}
		}
	}
	total := size * size
	k := (absInt(dark*20-total*10) + total - 1) / total
	if k > 0 {
		score += (k - 1) * 10
	}

	return score
}

func absInt(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
