//go:build !(js && wasm)

package book

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math"
	"sync"
)

const (
	MSDFPixelRange    = 3.0
	atlasWidth        = 256
	atlasHeight       = 128
	portRadius        = 6.5
	chapterPortRadius = 3.5
	spScale           = 32.0 / 24.0
	smScale           = 24.0 / 16.0
	chScale           = 16.0 / 10.0
	iconScale         = 28.0 / 24.0
	pinScale          = 28.0 / 16.0
	xBase             = -3.85
	xTip              = 4.65
	yHalf             = 4.25
	dxEdge            = xTip - xBase
)

var (
	cachedUIAtlasOnce sync.Once
	cachedUIAtlasPNG  []byte
	cachedUIAtlasErr  error
)

// BuildStudioUIAtlas generates the 256x128 SDF UI sprite texture (/studio-ui.png) at build time.
func BuildStudioUIAtlas() ([]byte, error) {
	cachedUIAtlasOnce.Do(func() {
		cachedUIAtlasPNG, cachedUIAtlasErr = buildStudioUIAtlasImpl()
	})
	return cachedUIAtlasPNG, cachedUIAtlasErr
}

func buildStudioUIAtlasImpl() ([]byte, error) {
	img := image.NewNRGBA(image.Rect(0, 0, atlasWidth, atlasHeight))
	setGray := func(pixelX, pixelY int, v uint8) {
		img.SetNRGBA(pixelX, pixelY, color.NRGBA{R: v, G: v, B: v, A: v})
	}
	distToByte := func(signedDist float64) uint8 {
		norm := math.Max(0.0, math.Min(1.0, 0.5+signedDist/(2.0*MSDFPixelRange)))
		return uint8(math.Round(norm * 255.0))
	}
	ptSegDist := func(px, py, x1, y1, x2, y2 float64) float64 {
		dx := x2 - x1
		dy := y2 - y1
		l2 := dx*dx + dy*dy
		if l2 < 0.0001 {
			return math.Hypot(px-x1, py-y1)
		}
		t := ((px-x1)*dx + (py-y1)*dy) / l2
		if t < 0 {
			t = 0
		} else if t > 1 {
			t = 1
		}
		return math.Hypot(px-(x1+t*dx), py-(y1+t*dy))
	}

	// 1. Solid white block at (2..8, 2..8)
	for pixelY := 2; pixelY < 8; pixelY++ {
		for pixelX := 2; pixelX < 8; pixelX++ {
			setGray(pixelX, pixelY, 255)
		}
	}

	// 2. Analytic SDF Ports & 90° Wire Corners
	for pixelY := 0; pixelY < 32; pixelY++ {
		for pixelX := 0; pixelX < 32; pixelX++ {
			wx := (float64(pixelX) + 0.5 - 16.0) / spScale
			wy := (float64(pixelY) + 0.5 - 16.0) / spScale
			r := math.Hypot(wx, wy)

			setGray(16+pixelX, 2+pixelY, distToByte(portRadius-r))
			setGray(52+pixelX, 2+pixelY, distToByte(1.0-math.Abs(r-portRadius)))
			setGray(88+pixelX, 2+pixelY, distToByte(portRadius+1.0-r))

			dr := (math.Abs(wx) + math.Abs(wy)) * 0.78
			setGray(16+pixelX, 66+pixelY, distToByte(portRadius-dr))
			setGray(52+pixelX, 66+pixelY, distToByte(1.0-math.Abs(dr-portRadius)))
			setGray(88+pixelX, 66+pixelY, distToByte(portRadius+1.0-dr))

			cwx := (float64(pixelX) + 0.5 - 16.0) * (18.0 / 32.0)
			cwy := (float64(pixelY) + 0.5 - 16.0) * (18.0 / 32.0)
			cr := math.Hypot(cwx, cwy)
			setGray(152+pixelX, 2+pixelY, distToByte(1.0-math.Abs(cr-8.0)))
		}
	}

	// 2b. Analytic 2x Smaller SDF Ports (r = 3.5px) for Chapter Ports & Wire Control Pins
	for pixelY := 0; pixelY < 24; pixelY++ {
		for pixelX := 0; pixelX < 24; pixelX++ {
			wx := (float64(pixelX) + 0.5 - 12.0) / smScale
			wy := (float64(pixelY) + 0.5 - 12.0) / smScale
			r := math.Hypot(wx, wy)

			setGray(16+pixelX, 36+pixelY, distToByte(chapterPortRadius-r))
			setGray(52+pixelX, 36+pixelY, distToByte(1.0-math.Abs(r-chapterPortRadius)))
			setGray(88+pixelX, 36+pixelY, distToByte(chapterPortRadius+0.8-r))
		}
	}

	// 3. Grip 2x3 dots at (124, 2, 20, 22)
	for row := 0; row < 3; row++ {
		cy := 2 + 5 + row*5
		for dy := 0; dy < 2; dy++ {
			for dx := 0; dx < 2; dx++ {
				setGray(124+6+dx, cy+dy, 255)
				setGray(124+11+dx, cy+dy, 255)
			}
		}
	}

	// 4. Open 4px directional chevrons: chevronH at (188,2,16x16), chevronV at (204,2,16x16)
	for pixelY := 0; pixelY < 16; pixelY++ {
		for pixelX := 0; pixelX < 16; pixelX++ {
			wx := (float64(pixelX) + 0.5 - 8.0) / chScale
			wy := (float64(pixelY) + 0.5 - 8.0) / chScale
			dH := math.Min(ptSegDist(wx, wy, -2.0, -3.0, 1.5, 0.0), ptSegDist(wx, wy, -2.0, 3.0, 1.5, 0.0))
			setGray(188+pixelX, 2+pixelY, distToByte(0.85-dH))
			dV := math.Min(ptSegDist(wx, wy, -3.0, -2.0, 0.0, 1.5), ptSegDist(wx, wy, 3.0, -2.0, 0.0, 1.5))
			setGray(204+pixelX, 2+pixelY, distToByte(0.85-dV))
		}
	}

	rasterizeSDFIcon := func(ox, oy int, segs [][4]float64, halfStroke float64) {
		for pixelY := 0; pixelY < 28; pixelY++ {
			for pixelX := 0; pixelX < 28; pixelX++ {
				ix := (float64(pixelX) + 0.5) / iconScale
				iy := (float64(pixelY) + 0.5) / iconScale
				minD := 999.0
				for _, sg := range segs {
					if d := ptSegDist(ix, iy, sg[0], sg[1], sg[2], sg[3]); d < minD {
						minD = d
					}
				}
				setGray(ox+pixelX, oy+pixelY, distToByte(halfStroke-minD))
			}
		}
	}
	rasterizeSDFIcon(120, 34, [][4]float64{
		{3, 12, 8, 12}, {8, 12, 12, 7}, {12, 7, 15, 7},
		{8, 12, 12, 17}, {12, 17, 18, 17}, {19, 4, 19, 10}, {16, 7, 22, 7},
	}, 0.95)
	rasterizeSDFIcon(152, 34, [][4]float64{
		{12, 20, 21, 20}, {16.5, 3.5, 20.5, 7.5}, {20.5, 7.5, 7, 21},
		{7, 21, 3, 21}, {3, 21, 3, 17}, {3, 17, 16.5, 3.5},
	}, 0.95)
	rasterizeSDFIcon(184, 34, [][4]float64{
		{3, 6, 21, 6}, {5, 6, 6, 19}, {6, 19, 7.5, 20.5}, {7.5, 20.5, 16.5, 20.5},
		{16.5, 20.5, 18, 19}, {18, 19, 19, 6}, {10, 11, 10, 17}, {14, 11, 14, 17},
	}, 0.95)

	// 5. Analytic SDF Directional Arrowheads (pinArrowH at 224,2; pinArrowV at 224,34)
	edgeLen := math.Hypot(dxEdge, yHalf)
	nx := yHalf / edgeLen
	ny := dxEdge / edgeLen
	sdPinArrow := func(along, perp float64) float64 {
		ay := math.Abs(perp)
		dBase := along - xBase
		dSlant := -((along-xTip)*nx + ay*ny)
		var dCore float64
		if dBase >= 0.0 && dSlant >= 0.0 {
			dCore = math.Min(dBase, dSlant)
		} else {
			dCore = -math.Min(ptSegDist(along, ay, xBase, 0.0, xBase, yHalf), ptSegDist(along, ay, xBase, yHalf, xTip, 0.0))
		}
		return dCore + 0.35
	}
	for pixelY := 0; pixelY < 28; pixelY++ {
		for pixelX := 0; pixelX < 28; pixelX++ {
			wx := (float64(pixelX) + 0.5 - 14.0) / pinScale
			wy := (float64(pixelY) + 0.5 - 14.0) / pinScale
			setGray(224+pixelX, 2+pixelY, distToByte(sdPinArrow(wx, wy)))
			setGray(224+pixelX, 34+pixelY, distToByte(sdPinArrow(wy, wx)))
		}
	}

	var pngBuf bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.BestSpeed}
	if err := enc.Encode(&pngBuf, img); err != nil {
		return nil, err
	}
	return pngBuf.Bytes(), nil
}
