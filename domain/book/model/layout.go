package model

import (
	"strconv"
	"strings"
)

const (
	EditorGridSize           = 24
	maxCanvasCoord           = 1_000_000
	NodeCardWidth            = 192
	NodeCardHalfWidth        = 96
	NodeCardBaseHeight       = 72
	NodeChoiceRowHeight      = 24
	NodeMinLogicalHeight     = 120
	NodeColSpacing           = 264
	ChapterFramePadX         = 36
	ChapterFramePadX2        = 72
	ChapterFramePadTop       = 52
	ChapterFramePadBottom    = 44
	ChapterFramePadYTotal    = 96
	NewChapterDefaultOffsetX = 768
	NewChapterDefaultOffsetY = 552
)

// NodePosition stores the grid row/col and pixel (X, Y) coordinates of a scene node on the Studio canvas.
type NodePosition struct {
	Row   int
	Col   int
	X     int
	Y     int
	HasXY bool
}

// WirePin represents a user-placed control pin along a transition wire on the Studio canvas.
type WirePin struct {
	X int
	Y int
}

// ClampCanvasCoord bounds a canvas coordinate to [-maxCanvasCoord, maxCanvasCoord].
func ClampCanvasCoord(coord int) int {
	if coord > maxCanvasCoord {
		return maxCanvasCoord
	}
	if coord < -maxCanvasCoord {
		return -maxCanvasCoord
	}
	return coord
}

// SnapToGrid rounds an integer coordinate to the nearest multiple of EditorGridSize (24px).
func SnapToGrid(coord int) int {
	coord = ClampCanvasCoord(coord)
	if coord >= 0 {
		return ((coord + EditorGridSize/2) / EditorGridSize) * EditorGridSize
	}
	return -(((-coord + EditorGridSize/2) / EditorGridSize) * EditorGridSize)
}

// StoreSceneNodePosition records a scene's position in targetBook.Layout using stable keys.
func StoreSceneNodePosition(targetBook *Book, targetScene *Scene, pos NodePosition) {
	if targetBook == nil || targetScene == nil {
		return
	}
	if targetBook.Layout == nil {
		targetBook.Layout = make(map[string]NodePosition)
	}
	trimmedName := strings.TrimSpace(targetScene.Name)
	if targetScene.ExplicitID != "" {
		targetBook.Layout[targetScene.ExplicitID] = pos
		targetBook.Layout[strings.ToLower(targetScene.ExplicitID)] = pos
		if targetScene.ID != "" {
			targetBook.Layout[targetScene.ID] = pos
			targetBook.Layout[strings.ToLower(targetScene.ID)] = pos
		}
	} else if trimmedName != "" {
		if targetScene.ID != "" && !strings.EqualFold(targetScene.ID, trimmedName) && !strings.EqualFold(targetScene.ID, NormalizeLookupKey(trimmedName)) {
			delete(targetBook.Layout, targetScene.ID)
			delete(targetBook.Layout, strings.ToLower(targetScene.ID))
		}
	} else if targetScene.ID != "" {
		targetBook.Layout[targetScene.ID] = pos
		targetBook.Layout[strings.ToLower(targetScene.ID)] = pos
	}
	if trimmedName != "" {
		targetBook.Layout[targetScene.Name] = pos
		if norm := NormalizeLookupKey(targetScene.Name); norm != "" {
			targetBook.Layout[norm] = pos
		}
	}
}

// LookupNodePosition resolves a stored NodePosition for targetScene inside layout.
func LookupNodePosition(layout map[string]NodePosition, targetScene *Scene) (NodePosition, bool) {
	if len(layout) == 0 || targetScene == nil {
		return NodePosition{}, false
	}
	if targetScene.ExplicitID != "" {
		if pos, found := layout[strings.ToLower(targetScene.ExplicitID)]; found {
			return pos, true
		}
		if pos, found := layout[targetScene.ExplicitID]; found {
			return pos, true
		}
	}
	if targetScene.Name != "" {
		if pos, found := layout[NormalizeLookupKey(targetScene.Name)]; found {
			return pos, true
		}
		if pos, found := layout[targetScene.Name]; found {
			return pos, true
		}
	}
	if targetScene.ID != "" {
		if pos, found := layout[strings.ToLower(targetScene.ID)]; found {
			return pos, true
		}
		if pos, found := layout[targetScene.ID]; found {
			return pos, true
		}
	}
	return NodePosition{}, false
}

// ParseWirePinList parses a semicolon-separated list of "x,y" pin coordinates.
func ParseWirePinList(rawPins string) []WirePin {
	var pins []WirePin
	for _, segment := range strings.Split(rawPins, ";") {
		segment = strings.TrimSpace(segment)
		if segment == "" {
			continue
		}
		parts := strings.Split(segment, ",")
		if len(parts) != 2 {
			continue
		}
		coordX, errX := strconv.Atoi(strings.TrimSpace(parts[0]))
		coordY, errY := strconv.Atoi(strings.TrimSpace(parts[1]))
		if errX == nil && errY == nil {
			pins = append(pins, WirePin{X: ClampCanvasCoord(coordX), Y: ClampCanvasCoord(coordY)})
		}
	}
	return pins
}
