package book

import (
	"strings"
	"time"
)

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
	ID             string
	ExplicitID     string
	Name           string
	Slugline       string
	Synopsis       string
	Chapter        string
	ChapterID      string
	Text           string
	HTML           string
	Choices        []Choice
	ExplicitNext   string
	ExplicitEnd    bool
	ExplicitImage  string
	ExplicitVoice  string
	AudioFile      string
	ImageFile      string
	AudioURL       string
	ImageURL       string
	SourceFile     string
	OrderIndex     int
	Row            int
	Col            int
	X              int
	Y              int
	Unreachable    bool
	Warnings       []string
	htmlSourceText string
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

// EnsureHTML lazily renders and caches safe HTML paragraphs from s.Text if not already rendered.
func (s *Scene) EnsureHTML() string {
	if s == nil {
		return ""
	}
	if s.Text == "" {
		s.HTML = ""
		s.htmlSourceText = ""
		return ""
	}
	if s.HTML == "" || s.htmlSourceText != s.Text {
		s.HTML = RenderSafeProseHTML(s.Text)
		s.htmlSourceText = s.Text
	}
	return s.HTML
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

type PackedMediaEntry struct {
	DataOffset  int64
	Size        int64
	Method      uint16
	ModTime     time.Time
	Header512   [512]byte
	HeaderLen   int
	ValidHeader bool
}

type Book struct {
	ID             string
	Title          string
	Author         string
	Description    string
	Mode           string
	ScriptStandard string
	StartSceneID   string
	RawStart       string
	Narrated       bool
	Imageless      bool
	CoverFile      string
	CoverURL       string
	IsPacked       bool
	IsExternal     bool
	SourceURL      string
	RootPath       string
	TextFiles      []string
	AudioFiles     []string
	ImageFiles     []string
	PackedMedia    map[string]PackedMediaEntry
	Chapters       []Chapter
	Scenes         []*Scene
	Layout         map[string]NodePosition
	EditorPanX     int
	EditorPanY     int
	EditorZoom     float64
	HasEditorView  bool
	Warnings       []string
	deferProseHTML bool
}

// NormalizeBookMode normalizes a project mode string to "screenplay" or "book".
func NormalizeBookMode(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "screenplay", "scenariusz", "script", "film":
		return "screenplay"
	default:
		return "book"
	}
}

// NormalizeScriptStandard normalizes a regional screenplay standard string to "pl", "eu", or "us".
func NormalizeScriptStandard(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "eu", "europe", "europa":
		return "eu"
	case "us", "usa", "hollywood", "letter":
		return "us"
	default:
		return "pl"
	}
}

// IsScreenplay reports whether the book project is in Screenplay Mode.
func (b *Book) IsScreenplay() bool {
	return b != nil && NormalizeBookMode(b.Mode) == "screenplay"
}

// NormalizedMode returns "screenplay" or "book".
func (b *Book) NormalizedMode() string {
	if b.IsScreenplay() {
		return "screenplay"
	}
	return "book"
}

// NormalizedStandard returns "pl", "eu", or "us".
func (b *Book) NormalizedStandard() string {
	if b == nil {
		return "pl"
	}
	return NormalizeScriptStandard(b.ScriptStandard)
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
	NewBookDir         string
	ExternalBooks      []string
}
