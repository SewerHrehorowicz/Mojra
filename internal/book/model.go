package book

import "strings"

type Choice struct {
	Label           string
	Target          string
	TargetSceneID   string
	TargetChapterID string
	IsChapterLink   bool
	IsImplicit      bool
	IsEnding        bool
}

type Scene struct {
	ID            string
	ExplicitID    string
	Name          string
	Chapter       string
	ChapterID     string
	Text          string
	HTML          string
	Choices       []Choice
	ExplicitNext  string
	ExplicitEnd   bool
	ExplicitImage string
	ExplicitVoice string
	AudioFile     string
	ImageFile     string
	AudioURL      string
	ImageURL      string
	SourceFile    string
	OrderIndex    int
	Row           int
	Col           int
	X             int
	Y             int
	Unreachable   bool
	Warnings      []string
}

// DisplayName returns the scene's title if present, or falls back to its ExplicitID or ID.
func (s Scene) DisplayName() string {
	if name := strings.TrimSpace(s.Name); name != "" {
		return name
	}
	if expID := strings.TrimSpace(s.ExplicitID); expID != "" {
		return expID
	}
	return strings.TrimSpace(s.ID)
}

// ReferenceKey returns the preferred target reference for choices/links pointing to this scene
// (ExplicitID if set, otherwise Name, otherwise ID).
func (s Scene) ReferenceKey() string {
	if expID := strings.TrimSpace(s.ExplicitID); expID != "" {
		return expID
	}
	if name := strings.TrimSpace(s.Name); name != "" {
		return name
	}
	return strings.TrimSpace(s.ID)
}

type Chapter struct {
	ID            string
	ExplicitID    string
	Name          string
	ExplicitStart string
	StartSceneID  string
	SourceFile    string
	SceneIDs      []string
}

// DisplayName returns the chapter's title if present, or falls back to its ExplicitID or ID.
func (c Chapter) DisplayName() string {
	if name := strings.TrimSpace(c.Name); name != "" {
		return name
	}
	if expID := strings.TrimSpace(c.ExplicitID); expID != "" {
		return expID
	}
	return strings.TrimSpace(c.ID)
}

// ReferenceKey returns the preferred target reference for chapter links (ExplicitID if set, otherwise ID, otherwise sanitized Name).
func (c Chapter) ReferenceKey() string {
	if expID := SanitizeChapterID(c.ExplicitID); expID != "" {
		return expID
	}
	if id := SanitizeChapterID(c.ID); id != "" {
		return id
	}
	if nameID := SanitizeChapterID(c.Name); nameID != "" {
		return nameID
	}
	return "rozdzial_1"
}

// FileName returns the preferred .txt file name for this chapter, using its ID primarily.
func (c Chapter) FileName() string {
	if expID := SanitizeChapterID(c.ExplicitID); expID != "" {
		return expID + ".txt"
	}
	if id := SanitizeChapterID(c.ID); id != "" {
		return id + ".txt"
	}
	if sf := strings.TrimSpace(c.SourceFile); sf != "" && !strings.EqualFold(sf, "story.txt") {
		return sf
	}
	if nameID := SanitizeChapterID(c.Name); nameID != "" {
		return nameID + ".txt"
	}
	return "rozdzial_1.txt"
}

type NodePosition struct {
	Row   int
	Col   int
	X     int
	Y     int
	HasXY bool
}

type Book struct {
	ID            string
	Title         string
	Author        string
	Description   string
	StartSceneID  string
	RawStart      string
	Narrated      bool
	Imageless     bool
	CoverFile     string
	CoverURL      string
	IsPacked      bool
	IsExternal    bool
	SourceURL     string
	RootPath      string
	TextFiles     []string
	AudioFiles    []string
	ImageFiles    []string
	Chapters      []Chapter
	Scenes        []*Scene
	Layout        map[string]NodePosition
	EditorPanX    int
	EditorPanY    int
	EditorZoom    float64
	HasEditorView bool
	Warnings      []string
}

type HistoryEntry struct {
	Step          int
	SceneID       string
	SceneName     string
	Chapter       string
	ChoiceLabel   string
	TargetSceneID string
}

type SaveState struct {
	BookID             string
	CurrentSceneID     string
	AudioTime          float64
	PlaybackSpeed      float64
	AutoChoice         bool
	AutoChoiceStrategy string
	History            []HistoryEntry
	UpdatedAtUnix      int64
}

type AppSettings struct {
	Theme              string
	PreviewMode        string
	ActiveBookID       string
	ActiveView         string
	Imageless          bool
	UIScale            string
	PlaybackSpeed      float64
	AutoChoice         bool
	AutoChoiceStrategy string
	ExternalBooks      []string
}
