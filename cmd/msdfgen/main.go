package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

const (
	msdfAtlasSize  = 512
	msdfBaseEmPx   = 23.0
	msdfPixelRange = 3.0
	msdfSteps      = 6
	msdfWindingSeg = 4

	flagAppName   = "app"
	flagAppDef    = "app"
	flagAppUsage  = "Path to app directory"
	flagOutName   = "out"
	flagOutDef    = "build/app"
	flagOutUsage  = "Path to build output directory"
	flagLangsName = "langs"
	flagLangsDef  = "pl,en"
	flagLangsDesc = "Comma-separated language codes for glyph set"

	charsetEN = " !\"#$%&'()*+,-./0123456789:;<=>?@ABCDEFGHIJKLMNOPQRSTUVWXYZ[\\]^_`abcdefghijklmnopqrstuvwxyz{|}~…—–„”«»★"
	charsetPL = "ĄĆĘŁŃÓŚŹŻąćęłńóśźż"

	langCodeEN = "en"
	langCodePL = "pl"

	fontDirName     = "fonts"
	jsDirName       = "js"
	fontAleoRegular = "Aleo-Regular.ttf"
	fontLatoRegular = "Lato-Regular.ttf"
	fontLatoBold    = "Lato-Bold.ttf"
	atlasFileName   = "studio-atlas.png"
	metricsFileName = "msdf_metrics.js"

	ttfTagHead = "head"
	ttfTagHhea = "hhea"
	ttfTagMaxp = "maxp"
	ttfTagCmap = "cmap"
	ttfTagLoca = "loca"
	ttfTagGlyf = "glyf"
	ttfTagHmtx = "hmtx"

	errTTFShort    = "ttf too short"
	errTTFTrunc    = "ttf table dir truncated"
	errTTFTables   = "missing required ttf tables"
	errReadFontFmt = "read font %s: %w"
	errParseTTFfmt = "parse font %s: %w"
	errOverflowFmt = "msdf 512x512 font atlas overflow at font %d rune %q (y=%d, gh=%d)"
	errGenFailed   = "msdfgen error: %v\n"

	jsHeaderPrefix = "window.__STUDIO_MSDF_DATA = \""
	jsHeaderSuffix = "\";\n"
	metricRowFmt   = "%d|%s|%.5f|%.5f|%.5f|%.5f|%.2f|%.2f|%.2f|%.2f|%.2f"
	escBackslash   = "\\\\"
	escQuote       = "\\\""
	escNewline     = "\\n"
)

type ttfPoint struct {
	x, y    float64
	onCurve bool
}

type msdfEdge struct {
	isQuad     bool
	p0, p1, p2 ttfPoint
	colorMask  int
	minX, minY float64
	maxX, maxY float64
}

type msdfContour struct {
	edges  []msdfEdge
	isHole bool
}

type ttfFont struct {
	data             []byte
	unitsPerEm       float64
	ascender         float64
	descender        float64
	indexToLocFormat int16
	numGlyphs        int
	numOfLongHorMet  int
	cmapOffset       int
	locaOffset       int
	glyfOffset       int
	glyfLength       int
	hmtxOffset       int
}

type msdfGlyphMetric struct {
	FontID int
	Rune   rune
	U0, V0 float64
	U1, V1 float64
	W, H   float64
	OX, OY float64
	Adv    float64
}

type ttfCompositeTask struct {
	gid                int
	depth              int
	a, b, c, d, dx, dy float64
	identity           bool
}

type edgeEval struct {
	dist       float64
	ortho      float64
	pseudoDist float64
	winding    int
}

func main() {
	appDir := flag.String(flagAppName, flagAppDef, flagAppUsage)
	outDir := flag.String(flagOutName, flagOutDef, flagOutUsage)
	langs := flag.String(flagLangsName, flagLangsDef, flagLangsDesc)
	flag.Parse()

	runes := buildCharsetRunes(*langs)
	pngBytes, metricsJS, err := generateStudioMSDFAtlas(*appDir, runes)
	if err != nil {
		fmt.Fprintf(os.Stderr, errGenFailed, err)
		os.Exit(1)
	}

	appAtlasPath := filepath.Join(*appDir, atlasFileName)
	if err := os.WriteFile(appAtlasPath, pngBytes, 0644); err != nil {
		fmt.Fprintf(os.Stderr, errGenFailed, err)
		os.Exit(1)
	}

	metricsPath := filepath.Join(*appDir, jsDirName, metricsFileName)
	if err := os.WriteFile(metricsPath, []byte(metricsJS), 0644); err != nil {
		fmt.Fprintf(os.Stderr, errGenFailed, err)
		os.Exit(1)
	}

	if strings.TrimSpace(*outDir) != "" {
		if err := os.MkdirAll(*outDir, 0755); err != nil {
			fmt.Fprintf(os.Stderr, errGenFailed, err)
			os.Exit(1)
		}
		outAtlasPath := filepath.Join(*outDir, atlasFileName)
		if err := os.WriteFile(outAtlasPath, pngBytes, 0644); err != nil {
			fmt.Fprintf(os.Stderr, errGenFailed, err)
			os.Exit(1)
		}
	}
}

func buildCharsetRunes(langsFlag string) []rune {
	seen := make(map[rune]bool, 160)
	var out []rune
	appendStr := func(s string) {
		for _, r := range s {
			if !seen[r] {
				seen[r] = true
				out = append(out, r)
			}
		}
	}
	appendStr(charsetEN)
	for _, part := range strings.Split(langsFlag, ",") {
		switch strings.ToLower(strings.TrimSpace(part)) {
		case langCodePL:
			appendStr(charsetPL)
		case langCodeEN:
			appendStr(charsetEN)
		}
	}
	return out
}

func parseTTF(data []byte) (*ttfFont, error) {
	if len(data) < 12 {
		return nil, fmt.Errorf(errTTFShort)
	}
	numTables := int(binary.BigEndian.Uint16(data[4:6]))
	if len(data) < 12+numTables*16 {
		return nil, fmt.Errorf(errTTFTrunc)
	}
	f := &ttfFont{data: data}
	var headOff, hheaOff, maxpOff int
	for i := 0; i < numTables; i++ {
		off := 12 + i*16
		tag := string(data[off : off+4])
		tOff := int(binary.BigEndian.Uint32(data[off+8 : off+12]))
		tLen := int(binary.BigEndian.Uint32(data[off+12 : off+16]))
		if tOff+tLen > len(data) {
			continue
		}
		switch tag {
		case ttfTagHead:
			headOff = tOff
		case ttfTagHhea:
			hheaOff = tOff
		case ttfTagMaxp:
			maxpOff = tOff
		case ttfTagCmap:
			f.cmapOffset = tOff
		case ttfTagLoca:
			f.locaOffset = tOff
		case ttfTagGlyf:
			f.glyfOffset = tOff
			f.glyfLength = tLen
		case ttfTagHmtx:
			f.hmtxOffset = tOff
		}
	}
	if headOff == 0 || hheaOff == 0 || maxpOff == 0 || f.cmapOffset == 0 || f.locaOffset == 0 || f.glyfOffset == 0 || f.hmtxOffset == 0 {
		return nil, fmt.Errorf(errTTFTables)
	}
	f.unitsPerEm = float64(binary.BigEndian.Uint16(data[headOff+18 : headOff+20]))
	f.indexToLocFormat = int16(binary.BigEndian.Uint16(data[headOff+50 : headOff+52]))
	f.ascender = float64(int16(binary.BigEndian.Uint16(data[hheaOff+4 : hheaOff+6])))
	f.descender = float64(int16(binary.BigEndian.Uint16(data[hheaOff+6 : hheaOff+8])))
	f.numOfLongHorMet = int(binary.BigEndian.Uint16(data[hheaOff+34 : hheaOff+36]))
	f.numGlyphs = int(binary.BigEndian.Uint16(data[maxpOff+4 : maxpOff+6]))
	return f, nil
}

func (f *ttfFont) lookupGlyphIndex(r rune) int {
	off := f.cmapOffset
	if off+4 > len(f.data) {
		return 0
	}
	numSub := int(binary.BigEndian.Uint16(f.data[off+2 : off+4]))
	var fmt4Off, fmt12Off int
	for i := 0; i < numSub; i++ {
		rec := off + 4 + i*8
		if rec+8 > len(f.data) {
			break
		}
		pid := binary.BigEndian.Uint16(f.data[rec : rec+2])
		eid := binary.BigEndian.Uint16(f.data[rec+2 : rec+4])
		subOff := off + int(binary.BigEndian.Uint32(f.data[rec+4:rec+8]))
		if subOff+4 > len(f.data) {
			continue
		}
		if pid == 0 || (pid == 3 && (eid == 1 || eid == 10)) {
			format := binary.BigEndian.Uint16(f.data[subOff : subOff+2])
			if format == 12 && fmt12Off == 0 {
				fmt12Off = subOff
			} else if format == 4 && fmt4Off == 0 {
				fmt4Off = subOff
			}
		}
	}
	if fmt12Off != 0 {
		nGroups := int(binary.BigEndian.Uint32(f.data[fmt12Off+12 : fmt12Off+16]))
		code := uint32(r)
		for i := 0; i < nGroups; i++ {
			gOff := fmt12Off + 16 + i*12
			if gOff+12 > len(f.data) {
				break
			}
			startCode := binary.BigEndian.Uint32(f.data[gOff : gOff+4])
			endCode := binary.BigEndian.Uint32(f.data[gOff+4 : gOff+8])
			startGID := binary.BigEndian.Uint32(f.data[gOff+8 : gOff+12])
			if code >= startCode && code <= endCode {
				return int(startGID + (code - startCode))
			}
		}
	}
	if fmt4Off != 0 && r <= 0xFFFF {
		code := uint16(r)
		segCountX2 := int(binary.BigEndian.Uint16(f.data[fmt4Off+6 : fmt4Off+8]))
		segCount := segCountX2 / 2
		endCountOff := fmt4Off + 14
		startCountOff := endCountOff + segCountX2 + 2
		idDeltaOff := startCountOff + segCountX2
		idRangeOff := idDeltaOff + segCountX2
		for i := 0; i < segCount; i++ {
			endCode := binary.BigEndian.Uint16(f.data[endCountOff+i*2 : endCountOff+i*2+2])
			if code > endCode {
				continue
			}
			startCode := binary.BigEndian.Uint16(f.data[startCountOff+i*2 : startCountOff+i*2+2])
			if code < startCode {
				return 0
			}
			idDelta := int16(binary.BigEndian.Uint16(f.data[idDeltaOff+i*2 : idDeltaOff+i*2+2]))
			rangeOffPos := idRangeOff + i*2
			idRangeOffset := int(binary.BigEndian.Uint16(f.data[rangeOffPos : rangeOffPos+2]))
			if idRangeOffset == 0 {
				return int(uint16(int(code) + int(idDelta)))
			}
			glyphIdAddr := rangeOffPos + idRangeOffset + int(code-startCode)*2
			if glyphIdAddr+2 > len(f.data) {
				return 0
			}
			gid := binary.BigEndian.Uint16(f.data[glyphIdAddr : glyphIdAddr+2])
			if gid == 0 {
				return 0
			}
			return int(uint16(int(gid) + int(idDelta)))
		}
	}
	return 0
}

func (f *ttfFont) advanceWidth(gid int) float64 {
	if gid < 0 || f.hmtxOffset == 0 {
		return f.unitsPerEm * 0.5
	}
	idx := gid
	if idx >= f.numOfLongHorMet {
		idx = f.numOfLongHorMet - 1
	}
	pos := f.hmtxOffset + idx*4
	if pos+2 > len(f.data) {
		return f.unitsPerEm * 0.5
	}
	return float64(binary.BigEndian.Uint16(f.data[pos : pos+2]))
}

func (f *ttfFont) glyphLocRange(gid int) (int, int) {
	if gid < 0 || gid >= f.numGlyphs {
		return 0, 0
	}
	if f.indexToLocFormat == 0 {
		pos := f.locaOffset + gid*2
		if pos+4 > len(f.data) {
			return 0, 0
		}
		o1 := int(binary.BigEndian.Uint16(f.data[pos:pos+2])) * 2
		o2 := int(binary.BigEndian.Uint16(f.data[pos+2:pos+4])) * 2
		return f.glyfOffset + o1, f.glyfOffset + o2
	}
	pos := f.locaOffset + gid*4
	if pos+8 > len(f.data) {
		return 0, 0
	}
	o1 := int(binary.BigEndian.Uint32(f.data[pos : pos+4]))
	o2 := int(binary.BigEndian.Uint32(f.data[pos+4 : pos+8]))
	return f.glyfOffset + o1, f.glyfOffset + o2
}

func (f *ttfFont) extractSimpleContours(start, end, numContours int) [][]ttfPoint {
	pos := start + 10
	endPts := make([]int, numContours)
	for i := 0; i < numContours; i++ {
		if pos+2 > end {
			return nil
		}
		endPts[i] = int(binary.BigEndian.Uint16(f.data[pos : pos+2]))
		pos += 2
	}
	numPts := endPts[len(endPts)-1] + 1
	if pos+2 > end {
		return nil
	}
	instrLen := int(binary.BigEndian.Uint16(f.data[pos : pos+2]))
	pos += 2 + instrLen
	if pos > end {
		return nil
	}
	flags := make([]byte, numPts)
	for i := 0; i < numPts; {
		if pos >= end {
			return nil
		}
		fl := f.data[pos]
		pos++
		flags[i] = fl
		i++
		if fl&0x08 != 0 {
			if pos >= end {
				return nil
			}
			rep := int(f.data[pos])
			pos++
			for r := 0; r < rep && i < numPts; r++ {
				flags[i] = fl
				i++
			}
		}
	}
	xs := make([]float64, numPts)
	var curX int16
	for i := 0; i < numPts; i++ {
		fl := flags[i]
		if fl&0x02 != 0 {
			if pos >= end {
				return nil
			}
			dx := int16(f.data[pos])
			pos++
			if fl&0x10 != 0 {
				curX += dx
			} else {
				curX -= dx
			}
		} else if fl&0x10 == 0 {
			if pos+2 > end {
				return nil
			}
			curX += int16(binary.BigEndian.Uint16(f.data[pos : pos+2]))
			pos += 2
		}
		xs[i] = float64(curX)
	}
	ys := make([]float64, numPts)
	var curY int16
	for i := 0; i < numPts; i++ {
		fl := flags[i]
		if fl&0x04 != 0 {
			if pos >= end {
				return nil
			}
			dy := int16(f.data[pos])
			pos++
			if fl&0x20 != 0 {
				curY += dy
			} else {
				curY -= dy
			}
		} else if fl&0x20 == 0 {
			if pos+2 > end {
				return nil
			}
			curY += int16(binary.BigEndian.Uint16(f.data[pos : pos+2]))
			pos += 2
		}
		ys[i] = float64(curY)
	}
	var contours [][]ttfPoint
	startIdx := 0
	for _, endIdx := range endPts {
		if endIdx >= numPts || endIdx < startIdx {
			break
		}
		c := make([]ttfPoint, 0, endIdx-startIdx+1)
		for i := startIdx; i <= endIdx; i++ {
			c = append(c, ttfPoint{x: xs[i], y: ys[i], onCurve: flags[i]&0x01 != 0})
		}
		contours = append(contours, c)
		startIdx = endIdx + 1
	}
	return contours
}

func (f *ttfFont) extractContours(gid int, depth int) [][]ttfPoint {
	if depth > 6 {
		return nil
	}
	var allContours [][]ttfPoint
	queue := []ttfCompositeTask{{gid: gid, depth: depth, a: 1, d: 1, identity: true}}
	for qIdx := 0; qIdx < len(queue) && qIdx < 64; qIdx++ {
		task := queue[qIdx]
		if task.depth > 6 {
			continue
		}
		start, end := f.glyphLocRange(task.gid)
		if start >= end || end > len(f.data) || start+10 > len(f.data) {
			continue
		}
		numContours := int16(binary.BigEndian.Uint16(f.data[start : start+2]))
		if numContours == 0 {
			continue
		}
		if numContours > 0 {
			subContours := f.extractSimpleContours(start, end, int(numContours))
			if task.identity {
				allContours = append(allContours, subContours...)
			} else {
				for _, sc := range subContours {
					tc := make([]ttfPoint, len(sc))
					for i, pt := range sc {
						tc[i] = ttfPoint{
							x:       task.a*pt.x + task.c*pt.y + task.dx,
							y:       task.b*pt.x + task.d*pt.y + task.dy,
							onCurve: pt.onCurve,
						}
					}
					allContours = append(allContours, tc)
				}
			}
			continue
		}

		pos := start + 10
		for pos+4 <= end {
			flags := binary.BigEndian.Uint16(f.data[pos : pos+2])
			subGID := int(binary.BigEndian.Uint16(f.data[pos+2 : pos+4]))
			pos += 4
			var dx, dy float64
			if flags&0x0001 != 0 {
				if pos+4 > end {
					break
				}
				if flags&0x0002 != 0 {
					dx = float64(int16(binary.BigEndian.Uint16(f.data[pos : pos+2])))
					dy = float64(int16(binary.BigEndian.Uint16(f.data[pos+2 : pos+4])))
				}
				pos += 4
			} else {
				if pos+2 > end {
					break
				}
				if flags&0x0002 != 0 {
					dx = float64(int8(f.data[pos]))
					dy = float64(int8(f.data[pos+1]))
				}
				pos += 2
			}
			a, b, c, d := 1.0, 0.0, 0.0, 1.0
			if flags&0x0008 != 0 {
				if pos+2 > end {
					break
				}
				s := float64(int16(binary.BigEndian.Uint16(f.data[pos:pos+2]))) / 16384.0
				a, d = s, s
				pos += 2
			} else if flags&0x0040 != 0 {
				if pos+4 > end {
					break
				}
				a = float64(int16(binary.BigEndian.Uint16(f.data[pos:pos+2]))) / 16384.0
				d = float64(int16(binary.BigEndian.Uint16(f.data[pos+2:pos+4]))) / 16384.0
				pos += 4
			} else if flags&0x0080 != 0 {
				if pos+8 > end {
					break
				}
				a = float64(int16(binary.BigEndian.Uint16(f.data[pos:pos+2]))) / 16384.0
				b = float64(int16(binary.BigEndian.Uint16(f.data[pos+2:pos+4]))) / 16384.0
				c = float64(int16(binary.BigEndian.Uint16(f.data[pos+4:pos+6]))) / 16384.0
				d = float64(int16(binary.BigEndian.Uint16(f.data[pos+6:pos+8]))) / 16384.0
				pos += 8
			}
			if len(queue) < 64 {
				queue = append(queue, ttfCompositeTask{
					gid:   subGID,
					depth: task.depth + 1,
					a:     task.a*a + task.c*b,
					b:     task.b*a + task.d*b,
					c:     task.a*c + task.c*d,
					d:     task.b*c + task.d*d,
					dx:    task.a*dx + task.c*dy + task.dx,
					dy:    task.b*dx + task.d*dy + task.dy,
				})
			}
			if flags&0x0020 == 0 {
				break
			}
		}
	}
	return allContours
}

func buildColoredMSDFContours(rawContours [][]ttfPoint, scale, offsetX, baselineY float64) []msdfContour {
	var out []msdfContour
	colorCycle := []int{3, 6, 5}
	colorIdx := 0

	tx := func(p ttfPoint) ttfPoint {
		return ttfPoint{
			x:       offsetX + p.x*scale,
			y:       baselineY - p.y*scale,
			onCurve: p.onCurve,
		}
	}

	for _, raw := range rawContours {
		n := len(raw)
		if n < 2 {
			continue
		}
		signedArea2 := 0.0
		for i := 0; i < n; i++ {
			p0 := raw[i]
			p1 := raw[(i+1)%n]
			signedArea2 += p0.x*p1.y - p1.x*p0.y
		}
		isHole := signedArea2 > 0

		var pts []ttfPoint
		for i := 0; i < n; i++ {
			cur := tx(raw[i])
			next := tx(raw[(i+1)%n])
			pts = append(pts, cur)
			if !cur.onCurve && !next.onCurve {
				pts = append(pts, ttfPoint{
					x:       0.5 * (cur.x + next.x),
					y:       0.5 * (cur.y + next.y),
					onCurve: true,
				})
			}
		}
		if len(pts) < 2 {
			continue
		}
		firstOn := -1
		for i, p := range pts {
			if p.onCurve {
				firstOn = i
				break
			}
		}
		if firstOn > 0 {
			pts = append(pts[firstOn:], pts[:firstOn]...)
		} else if firstOn < 0 {
			mid := ttfPoint{x: 0.5 * (pts[0].x + pts[len(pts)-1].x), y: 0.5 * (pts[0].y + pts[len(pts)-1].y), onCurve: true}
			pts = append([]ttfPoint{mid}, pts...)
		}

		var cEdges []msdfEdge
		m := len(pts)
		for i := 0; i < m; {
			p0 := pts[i]
			p1 := pts[(i+1)%m]
			if p1.onCurve {
				if math.Hypot(p1.x-p0.x, p1.y-p0.y) > 1e-4 {
					e := msdfEdge{isQuad: false, p0: p0, p1: p1}
					e.minX = math.Min(p0.x, p1.x)
					e.maxX = math.Max(p0.x, p1.x)
					e.minY = math.Min(p0.y, p1.y)
					e.maxY = math.Max(p0.y, p1.y)
					cEdges = append(cEdges, e)
				}
				i++
			} else {
				p2 := pts[(i+2)%m]
				e := msdfEdge{isQuad: true, p0: p0, p1: p1, p2: p2}
				e.minX = math.Min(p0.x, math.Min(p1.x, p2.x))
				e.maxX = math.Max(p0.x, math.Max(p1.x, p2.x))
				e.minY = math.Min(p0.y, math.Min(p1.y, p2.y))
				e.maxY = math.Max(p0.y, math.Max(p1.y, p2.y))
				cEdges = append(cEdges, e)
				i += 2
			}
		}
		if len(cEdges) == 0 {
			continue
		}
		if len(cEdges) == 1 {
			cEdges[0].colorMask = 7
			out = append(out, msdfContour{edges: cEdges, isHole: isHole})
			continue
		}

		edgeStartDir := func(e msdfEdge) (float64, float64) {
			dx, dy := e.p1.x-e.p0.x, e.p1.y-e.p0.y
			l := math.Hypot(dx, dy)
			if l < 1e-6 {
				return 1, 0
			}
			return dx / l, dy / l
		}
		edgeEndDir := func(e msdfEdge) (float64, float64) {
			var dx, dy float64
			if e.isQuad {
				dx, dy = e.p2.x-e.p1.x, e.p2.y-e.p1.y
			} else {
				dx, dy = e.p1.x-e.p0.x, e.p1.y-e.p0.y
			}
			l := math.Hypot(dx, dy)
			if l < 1e-6 {
				return 1, 0
			}
			return dx / l, dy / l
		}

		var corners []int
		ceCount := len(cEdges)
		for i := 0; i < ceCount; i++ {
			prev := cEdges[(i+ceCount-1)%ceCount]
			cur := cEdges[i]
			ax, ay := edgeEndDir(prev)
			bx, by := edgeStartDir(cur)
			dot := ax*bx + ay*by
			cross := ax*by - ay*bx
			if dot <= 0 || math.Abs(cross) > 0.28 {
				corners = append(corners, i)
			}
		}

		if len(corners) == 0 {
			for i := range cEdges {
				cEdges[i].colorMask = 7
			}
		} else if len(corners) == 1 {
			startCorner := corners[0]
			for i := 0; i < ceCount; i++ {
				idx := (startCorner + i) % ceCount
				t := (i * 3) / ceCount
				cEdges[idx].colorMask = colorCycle[(colorIdx+t)%3]
			}
			colorIdx += 3
		} else {
			numC := len(corners)
			for c := 0; c < numC; c++ {
				col := colorCycle[colorIdx%3]
				if c == numC-1 && (colorIdx%3) == ((colorIdx-c)%3) {
					col = colorCycle[(colorIdx+1)%3]
				}
				colorIdx++
				sIdx := corners[c]
				eIdx := corners[(c+1)%numC]
				k := sIdx
				for step := 0; step < ceCount; step++ {
					cEdges[k].colorMask = col
					if k == (eIdx+ceCount-1)%ceCount {
						break
					}
					k = (k + 1) % ceCount
				}
			}
		}
		out = append(out, msdfContour{edges: cEdges, isHole: isHole})
	}
	return out
}

func evalEdgeDistanceAndWinding(e msdfEdge, px, py float64) edgeEval {
	var res edgeEval
	if !e.isQuad {
		ax, ay := e.p0.x, e.p0.y
		bx, by := e.p1.x, e.p1.y
		vx, vy := bx-ax, by-ay
		wx, wy := px-ax, py-ay
		lenSq := vx*vx + vy*vy
		t := 0.0
		if lenSq > 1e-9 {
			t = (wx*vx + wy*vy) / lenSq
		}
		clampedT := math.Max(0, math.Min(1, t))
		cx := ax + clampedT*vx
		cy := ay + clampedT*vy
		dx := px - cx
		dy := py - cy
		res.dist = math.Hypot(dx, dy)
		segLen := math.Sqrt(lenSq)
		if segLen > 1e-6 && res.dist > 1e-6 {
			res.ortho = math.Abs(vx*dy-vy*dx) / (segLen * res.dist)
		}
		res.pseudoDist = res.dist
		if (t < 0 || t > 1) && res.dist < 1.15 && segLen > 1e-6 {
			perp := math.Abs(vx*dy-vy*dx) / segLen
			if perp < res.dist {
				res.pseudoDist = perp
			}
		}
		if (ay <= py && by > py) || (by <= py && ay > py) {
			xInt := ax + (py-ay)*(bx-ax)/(by-ay)
			if xInt > px {
				if by > ay {
					res.winding = 1
				} else {
					res.winding = -1
				}
			}
		}
		return res
	}

	p0, p1, p2 := e.p0, e.p1, e.p2
	bestT := 0.0
	bestDistSq := (px-p0.x)*(px-p0.x) + (py-p0.y)*(py-p0.y)
	d2Sq := (px-p2.x)*(px-p2.x) + (py-p2.y)*(py-p2.y)
	if d2Sq < bestDistSq {
		bestDistSq = d2Sq
		bestT = 1.0
	}
	for s := 1; s < msdfSteps; s++ {
		t := float64(s) / float64(msdfSteps)
		mt := 1.0 - t
		qx := mt*mt*p0.x + 2*mt*t*p1.x + t*t*p2.x
		qy := mt*mt*p0.y + 2*mt*t*p1.y + t*t*p2.y
		dsq := (px-qx)*(px-qx) + (py-qy)*(py-qy)
		if dsq < bestDistSq {
			bestDistSq = dsq
			bestT = t
		}
	}
	ax := p0.x - 2*p1.x + p2.x
	ay := p0.y - 2*p1.y + p2.y
	bx := p1.x - p0.x
	by := p1.y - p0.y
	t := bestT
	for iter := 0; iter < 3; iter++ {
		mt := 1.0 - t
		qx := mt*mt*p0.x + 2*mt*t*p1.x + t*t*p2.x - px
		qy := mt*mt*p0.y + 2*mt*t*p1.y + t*t*p2.y - py
		dqx := 2 * (bx + ax*t)
		dqy := 2 * (by + ay*t)
		ddqx := 2 * ax
		ddqy := 2 * ay
		num := qx*dqx + qy*dqy
		den := dqx*dqx + dqy*dqy + qx*ddqx + qy*ddqy
		if math.Abs(den) > 1e-7 {
			cand := math.Max(0, math.Min(1, t-num/den))
			cmt := 1.0 - cand
			cqx := cmt*cmt*p0.x + 2*cmt*cand*p1.x + cand*cand*p2.x - px
			cqy := cmt*cmt*p0.y + 2*cmt*cand*p1.y + cand*cand*p2.y - py
			cdsq := cqx*cqx + cqy*cqy
			if cdsq <= bestDistSq {
				bestDistSq = cdsq
				t = cand
			}
		}
	}
	mt := 1.0 - t
	cx := mt*mt*p0.x + 2*mt*t*p1.x + t*t*p2.x
	cy := mt*mt*p0.y + 2*mt*t*p1.y + t*t*p2.y
	dx := px - cx
	dy := py - cy
	res.dist = math.Sqrt(bestDistSq)

	tx := 2 * (bx + ax*t)
	ty := 2 * (by + ay*t)
	tLen := math.Hypot(tx, ty)
	if tLen > 1e-6 && res.dist > 1e-6 {
		res.ortho = math.Abs(tx*dy-ty*dx) / (tLen * res.dist)
	}
	res.pseudoDist = res.dist
	if res.dist < 1.15 {
		if t <= 1e-4 {
			tx0, ty0 := p1.x-p0.x, p1.y-p0.y
			tl0 := math.Hypot(tx0, ty0)
			if tl0 > 1e-6 && ((px-p0.x)*tx0+(py-p0.y)*ty0) < 0 {
				perp := math.Abs(tx0*(py-p0.y)-ty0*(px-p0.x)) / tl0
				if perp < res.dist {
					res.pseudoDist = perp
				}
			}
		} else if t >= 1.0-1e-4 {
			tx1, ty1 := p2.x-p1.x, p2.y-p1.y
			tl1 := math.Hypot(tx1, ty1)
			if tl1 > 1e-6 && ((px-p2.x)*tx1+(py-p2.y)*ty1) > 0 {
				perp := math.Abs(tx1*(py-p2.y)-ty1*(px-p2.x)) / tl1
				if perp < res.dist {
					res.pseudoDist = perp
				}
			}
		}
	}

	prevX, prevY := p0.x, p0.y
	for s := 1; s <= msdfWindingSeg; s++ {
		st := float64(s) / float64(msdfWindingSeg)
		smt := 1.0 - st
		qx := smt*smt*p0.x + 2*smt*st*p1.x + st*st*p2.x
		qy := smt*smt*p0.y + 2*smt*st*p1.y + st*st*p2.y
		if (prevY <= py && qy > py) || (qy <= py && prevY > py) {
			xInt := prevX + (py-prevY)*(qx-prevX)/(qy-prevY)
			if xInt > px {
				if qy > prevY {
					res.winding++
				} else {
					res.winding--
				}
			}
		}
		prevX, prevY = qx, qy
	}
	return res
}

func evalSingleContourMSDF(c msdfContour, px, py float64) [4]float64 {
	bestDist := [3]float64{1e9, 1e9, 1e9}
	bestOrtho := [3]float64{-1, -1, -1}
	bestPseudo := [3]float64{1e9, 1e9, 1e9}
	trueMinDist := 1e9
	winding := 0

	for _, e := range c.edges {
		ev := evalEdgeDistanceAndWinding(e, px, py)
		winding += ev.winding
		if ev.dist < trueMinDist {
			trueMinDist = ev.dist
		}
		for ch := 0; ch < 3; ch++ {
			if e.colorMask&(1<<ch) == 0 {
				continue
			}
			diff := math.Abs(ev.dist - bestDist[ch])
			if ev.dist < bestDist[ch]-1e-4 || (diff <= 1e-4 && ev.ortho > bestOrtho[ch]) {
				bestDist[ch] = ev.dist
				bestOrtho[ch] = ev.ortho
				bestPseudo[ch] = ev.pseudoDist
			}
		}
	}

	inside := winding != 0
	var sign float64
	if c.isHole {
		if inside {
			sign = -1.0
		} else {
			sign = 1.0
		}
	} else {
		if inside {
			sign = 1.0
		} else {
			sign = -1.0
		}
	}
	return [4]float64{sign * bestPseudo[0], sign * bestPseudo[1], sign * bestPseudo[2], sign * trueMinDist}
}

func median3Float(r, g, b float64) float64 {
	return math.Max(math.Min(r, g), math.Min(math.Max(r, g), b))
}

func distToByte(signedDist float64) uint8 {
	norm := 0.5 + signedDist/(2.0*msdfPixelRange)
	if norm <= 0 {
		return 0
	}
	if norm >= 1 {
		return 255
	}
	return uint8(math.Round(norm * 255.0))
}

func generateStudioMSDFAtlas(appDir string, runes []rune) ([]byte, string, error) {
	fontFiles := []string{
		filepath.Join(appDir, fontDirName, fontAleoRegular),
		filepath.Join(appDir, fontDirName, fontLatoRegular),
		filepath.Join(appDir, fontDirName, fontLatoBold),
	}
	fonts := make([]*ttfFont, len(fontFiles))
	for i, fp := range fontFiles {
		raw, err := os.ReadFile(fp)
		if err != nil {
			return nil, "", fmt.Errorf(errReadFontFmt, fp, err)
		}
		parsed, err := parseTTF(raw)
		if err != nil {
			return nil, "", fmt.Errorf(errParseTTFfmt, fp, err)
		}
		fonts[i] = parsed
	}

	img := image.NewNRGBA(image.Rect(0, 0, msdfAtlasSize, msdfAtlasSize))

	type glyphJob struct {
		orderIdx int
		fontID   int
		r        rune
		px, py   int
		gw, gh   int
		ox, oy   float64
		adv      float64
		contours []msdfContour
	}

	pad := int(math.Ceil(msdfPixelRange))
	var jobs []glyphJob
	var spaceMetrics []msdfGlyphMetric

	orderCounter := 0
	for fontID, f := range fonts {
		scale := msdfBaseEmPx / f.unitsPerEm
		for _, r := range runes {
			orderCounter++
			gid := f.lookupGlyphIndex(r)
			advPx := f.advanceWidth(gid) * scale
			rawContours := f.extractContours(gid, 0)
			if len(rawContours) == 0 {
				spaceMetrics = append(spaceMetrics, msdfGlyphMetric{
					FontID: fontID,
					Rune:   r,
					Adv:    advPx,
				})
				continue
			}
			minX, minY := 1e9, 1e9
			maxX, maxY := -1e9, -1e9
			for _, c := range rawContours {
				for _, pt := range c {
					sx := pt.x * scale
					sy := -pt.y * scale
					if sx < minX {
						minX = sx
					}
					if sx > maxX {
						maxX = sx
					}
					if sy < minY {
						minY = sy
					}
					if sy > maxY {
						maxY = sy
					}
				}
			}
			gw := int(math.Ceil(maxX-minX)) + pad*2
			gh := int(math.Ceil(maxY-minY)) + pad*2
			if gw < 6 {
				gw = 6
			}
			if gh < 6 {
				gh = 6
			}
			localOffsetX := float64(pad) - minX
			localBaselineY := float64(pad) - minY
			contours := buildColoredMSDFContours(rawContours, scale, localOffsetX, localBaselineY)

			jobs = append(jobs, glyphJob{
				orderIdx: orderCounter,
				fontID:   fontID,
				r:        r,
				gw:       gw,
				gh:       gh,
				ox:       minX - float64(pad),
				oy:       minY - float64(pad),
				adv:      advPx,
				contours: contours,
			})
		}
	}

	sort.Slice(jobs, func(i, j int) bool {
		if jobs[i].gh != jobs[j].gh {
			return jobs[i].gh < jobs[j].gh
		}
		return jobs[i].orderIdx < jobs[j].orderIdx
	})

	shelfX := 2
	shelfY := 2
	shelfRowH := 0
	metrics := make([]msdfGlyphMetric, 0, len(jobs)+len(spaceMetrics))
	metrics = append(metrics, spaceMetrics...)

	for i := range jobs {
		gw := jobs[i].gw
		gh := jobs[i].gh
		if shelfX+gw+1 > msdfAtlasSize {
			shelfX = 2
			shelfY += shelfRowH + 1
			shelfRowH = 0
		}
		if shelfY+gh+1 > msdfAtlasSize {
			return nil, "", fmt.Errorf(errOverflowFmt, jobs[i].fontID, jobs[i].r, shelfY, gh)
		}
		jobs[i].px = shelfX
		jobs[i].py = shelfY
		shelfX += gw + 1
		if gh > shelfRowH {
			shelfRowH = gh
		}
		metrics = append(metrics, msdfGlyphMetric{
			FontID: jobs[i].fontID,
			Rune:   jobs[i].r,
			U0:     float64(jobs[i].px) / float64(msdfAtlasSize),
			V0:     float64(jobs[i].py) / float64(msdfAtlasSize),
			U1:     float64(jobs[i].px+gw) / float64(msdfAtlasSize),
			V1:     float64(jobs[i].py+gh) / float64(msdfAtlasSize),
			W:      float64(gw),
			H:      float64(gh),
			OX:     jobs[i].ox,
			OY:     jobs[i].oy,
			Adv:    jobs[i].adv,
		})
	}

	var wg sync.WaitGroup
	workers := 8
	jobCh := make(chan glyphJob, len(jobs))
	for _, j := range jobs {
		jobCh <- j
	}
	close(jobCh)

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobCh {
				for y := 0; y < job.gh; y++ {
					py := float64(y) + 0.5
					for x := 0; x < job.gw; x++ {
						px := float64(x) + 0.5

						hasAdd := false
						addR, addG, addB, addA := -1e9, -1e9, -1e9, -1e9
						hasHole := false
						holeR, holeG, holeB, holeA := 1e9, 1e9, 1e9, 1e9

						for _, c := range job.contours {
							rgba := evalSingleContourMSDF(c, px, py)
							cr, cg, cb, ca := rgba[0], rgba[1], rgba[2], rgba[3]
							if !c.isHole {
								if !hasAdd || ca > addA {
									addA = ca
								}
								if !hasAdd || median3Float(cr, cg, cb) > median3Float(addR, addG, addB) {
									addR, addG, addB = cr, cg, cb
								}
								hasAdd = true
							} else {
								if !hasHole || ca < holeA {
									holeA = ca
								}
								if !hasHole || median3Float(cr, cg, cb) < median3Float(holeR, holeG, holeB) {
									holeR, holeG, holeB = cr, cg, cb
								}
								hasHole = true
							}
						}

						finalR, finalG, finalB, finalA := addR, addG, addB, addA
						if hasHole {
							if holeA < finalA {
								finalA = holeA
							}
							if median3Float(holeR, holeG, holeB) < median3Float(finalR, finalG, finalB) {
								finalR, finalG, finalB = holeR, holeG, holeB
							}
						}

						med := median3Float(finalR, finalG, finalB)
						if (finalA > 0 && med < 0) || (finalA < 0 && med > 0) {
							finalR, finalG, finalB = finalA, finalA, finalA
						}

						rVal := distToByte(finalR)
						gVal := distToByte(finalG)
						bVal := distToByte(finalB)
						aVal := distToByte(finalA)
						img.SetNRGBA(job.px+x, job.py+y, color.NRGBA{R: rVal, G: gVal, B: bVal, A: aVal})
					}
				}
			}
		}()
	}
	wg.Wait()

	var pngBuf bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.BestSpeed}
	if err := enc.Encode(&pngBuf, img); err != nil {
		return nil, "", err
	}

	var sb strings.Builder
	sb.WriteString(jsHeaderPrefix)
	for i, m := range metrics {
		if i > 0 {
			sb.WriteString(escNewline)
		}
		chEsc := string(m.Rune)
		switch m.Rune {
		case '\\':
			chEsc = escBackslash
		case '"':
			chEsc = escQuote
		}
		fmt.Fprintf(&sb, metricRowFmt,
			m.FontID, chEsc, m.U0, m.V0, m.U1, m.V1, m.W, m.H, m.OX, m.OY, m.Adv)
	}
	sb.WriteString(jsHeaderSuffix)

	return pngBuf.Bytes(), sb.String(), nil
}
