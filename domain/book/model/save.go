package model

import (
	"path/filepath"
	"strings"
)

const (
	widthModeFull = "full"
	widthModeHalf = "half"
)

// HistoryEntry records a single visited scene step in player save history.
type HistoryEntry struct {
	Step          int
	SceneID       string
	SceneName     string
	Chapter       string
	ChoiceLabel   string
	TargetSceneID string
	Variables     map[string]string
}

// NormalizeAutoChoiceDelay clamps an auto-choice delay in seconds to a valid range (default 2.0s).
func NormalizeAutoChoiceDelay(delaySec float64) float64 {
	if delaySec <= 0 || delaySec > 30 {
		return DefaultAutoChoiceDelay
	}
	return delaySec
}

// SaveState holds the persisted player progress for a single book.
type SaveState struct {
	BookID             string
	CurrentSceneID     string
	AudioTime          float64
	PlaybackSpeed      float64
	AutoChoice         bool
	AutoChoiceStrategy string
	AutoChoiceDelay    float64
	Variables          map[string]string
	History            []HistoryEntry
	UpdatedAtUnix      int64
}

// EditorBookUIState stores remembered Studio UI state for a specific book.
type EditorBookUIState struct {
	SelectedSceneID       string
	SceneTextModalID      string
	SceneTextWidthMode    string
	SceneTextSettingsOpen bool
	RightPanel            string
	SelectedChapter       string
}

// AppSettings holds user display, audio, library, and studio preferences stored in settings.ini.
type AppSettings struct {
	Theme                  string
	Language               string
	PreviewMode            string
	ActiveBookID           string
	EditorBookID           string
	EditorStudioHome       bool
	EditorSelectedSceneID  string
	EditorSceneTextModalID string
	EditorSceneTextWidth   string
	EditorSceneSettings    bool
	EditorRightPanel       string
	EditorSelectedChapter  string
	EditorBookStates       map[string]EditorBookUIState
	ActiveView             string
	Imageless              bool
	UIScale                string
	ScrollSensitivity      string
	PlaybackSpeed          float64
	AutoChoice             bool
	AutoChoiceStrategy     string
	AutoChoiceDelay        float64
	NewBookDir             string
	ExportDir              string
	BookExportDirs         map[string]string
	ExternalBooks          []string
}

// SetEditorBookUIState stores the Studio UI state for a specific book ID in settings.ini.
func (cfg *AppSettings) SetEditorBookUIState(bookID string, uiState EditorBookUIState) {
	if cfg == nil {
		return
	}
	safeID := SanitizeBookID(bookID)
	if safeID == "" {
		return
	}
	if cfg.EditorBookStates == nil {
		cfg.EditorBookStates = make(map[string]EditorBookUIState)
	}
	uiState.SelectedSceneID = strings.TrimSpace(uiState.SelectedSceneID)
	uiState.SceneTextModalID = strings.TrimSpace(uiState.SceneTextModalID)
	if uiState.SceneTextWidthMode == widthModeFull {
		uiState.SceneTextWidthMode = widthModeFull
	} else if uiState.SceneTextWidthMode == widthModeHalf {
		uiState.SceneTextWidthMode = widthModeHalf
	} else {
		uiState.SceneTextWidthMode = ""
	}
	uiState.RightPanel = strings.TrimSpace(uiState.RightPanel)
	uiState.SelectedChapter = strings.TrimSpace(uiState.SelectedChapter)
	cfg.EditorBookStates[safeID] = uiState
}

// GetEditorBookUIState returns the remembered Studio UI state for a specific book ID.
func (cfg *AppSettings) GetEditorBookUIState(bookID string) (EditorBookUIState, bool) {
	if cfg == nil || len(cfg.EditorBookStates) == 0 {
		return EditorBookUIState{}, false
	}
	safeID := SanitizeBookID(bookID)
	if safeID == "" {
		return EditorBookUIState{}, false
	}
	uiState, found := cfg.EditorBookStates[safeID]
	return uiState, found
}

// SetBookExportDir stores or clears the remembered export directory for a specific book ID in settings.ini.
func (cfg *AppSettings) SetBookExportDir(bookID, rawDir string) {
	if cfg == nil {
		return
	}
	safeID := SanitizeBookID(bookID)
	if safeID == "" {
		return
	}
	if cfg.BookExportDirs == nil {
		cfg.BookExportDirs = make(map[string]string)
	}
	cleanDir := filepath.Clean(strings.TrimSpace(rawDir))
	if cleanDir == "" || cleanDir == "." {
		delete(cfg.BookExportDirs, safeID)
		return
	}
	cfg.BookExportDirs[safeID] = cleanDir
}

// GetBookExportDir returns the remembered export directory for a specific book ID from settings.ini.
func (cfg *AppSettings) GetBookExportDir(bookID string) string {
	if cfg == nil || len(cfg.BookExportDirs) == 0 {
		return ""
	}
	return cfg.BookExportDirs[SanitizeBookID(bookID)]
}

// AddExternalBook registers a book path on disk (without copying files) if not already registered.
func (cfg *AppSettings) AddExternalBook(rawPath string) bool {
	cleaned := filepath.Clean(strings.TrimSpace(rawPath))
	if cleaned == "" || cleaned == "." {
		return false
	}
	for _, existing := range cfg.ExternalBooks {
		if filepath.Clean(existing) == cleaned {
			return false
		}
	}
	cfg.ExternalBooks = append(cfg.ExternalBooks, cleaned)
	return true
}

// RemoveExternalBook removes a registered external book path from settings.
func (cfg *AppSettings) RemoveExternalBook(rawPath string) bool {
	cleaned := filepath.Clean(strings.TrimSpace(rawPath))
	if cleaned == "" {
		return false
	}
	var kept []string
	removed := false
	for _, existing := range cfg.ExternalBooks {
		if filepath.Clean(existing) == cleaned {
			removed = true
			continue
		}
		kept = append(kept, existing)
	}
	cfg.ExternalBooks = kept
	return removed
}
