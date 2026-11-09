package book

import (
	"strings"
	"time"
)

const (
	BookDataFileName         = "book.playtxt"
	DefaultStoryFile         = "story.playtxt"
	DefaultChapterFile       = "rozdzial_1.playtxt"
	DefaultActFile           = "akt_1.playtxt"
	DefaultBookChapterName   = "Rozdział 1"
	DefaultScreenplayActName = "Akt I"
	EditorStateFileName      = ".editor.cfg"
	PlaytxtExt               = ".playtxt"

	MaxZipTotalUncompressedBytes = 500 * 1024 * 1024
	MaxZipSingleTextBytes        = 4 * 1024 * 1024
)

type Choice struct {
	Label           string
	Target          string
	TargetSceneID   string
	TargetChapterID string
	Condition       string
	Effects         string
	ShowWhenLocked  bool
	IsChapterLink   bool
	IsImplicit      bool
	IsEnding        bool
}

type EnumDef struct {
	Name   string
	Values []string
}

type VariableDef struct {
	Name     string
	Kind     string // "bool", "int", "enum"
	BoolVal  bool
	IntVal   int
	EnumName string
	EnumVal  string
	Options  []string
}

// VoiceSectionMeta stores optional explicit audio file and trim bounds for a single voice section.
type VoiceSectionMeta struct {
	ExplicitVoice string
	TrimStart     float64
	TrimEnd       float64
}

// VoiceSection represents an ordered static or conditional voiceover section extracted from a scene's AST.
type VoiceSection struct {
	Index         int
	Key           string
	Kind          string // "static", "if", "elif", "else"
	Condition     string
	Label         string
	ShortLabel    string
	HeaderTitle   string
	Text          string
	ExplicitVoice string
	TargetFile    string
	AudioFile     string
	AudioURL      string
	TrimStart     float64
	TrimEnd       float64
	DurationSec   float64
	DurationLabel string
	HasAudio      bool
	HasTrim       bool
}

// VoiceQueueItem represents an active audio segment in the sequential player queue.
type VoiceQueueItem struct {
	Key         string
	Label       string
	AudioFile   string
	AudioURL    string
	TrimStart   float64
	TrimEnd     float64
	DurationSec float64
}

type Scene struct {
	ID             string
	ExplicitID     string
	Name           string
	Slugline       string
	Synopsis       string
	Effects        string
	Chapter        string
	ChapterID      string
	Text           string
	HTML           string
	Choices        []Choice
	ExplicitNext   string
	ExplicitEnd    bool
	ExplicitImage  string
	ExplicitVoice  string
	VoiceTrimStart float64
	VoiceTrimEnd   float64
	AudioFile      string
	ImageFile      string
	AudioURL       string
	ImageURL       string
	SectionVoices  map[string]VoiceSectionMeta
	VoiceSections  []VoiceSection
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

// NormalizedVoiceTrimValues normalizes [start, end] ratios to [0, 1].
func NormalizedVoiceTrimValues(start, end float64) (float64, float64) {
	if start < 0 || start >= 1 {
		start = 0
	}
	if end <= 0 || end > 1 {
		end = 1
	}
	if end <= start+0.001 {
		return 0, 1
	}
	return start, end
}

// HasVoiceTrimValues reports whether [start, end] is narrower than [0, 1].
func HasVoiceTrimValues(start, end float64) bool {
	ts, te := NormalizedVoiceTrimValues(start, end)
	return ts > 0.002 || te < 0.998
}

// NormalizedVoiceTrim returns the effective non-destructive [startRatio, endRatio] in [0, 1].
func (s *Scene) NormalizedVoiceTrim() (float64, float64) {
	if s == nil {
		return 0, 1
	}
	return NormalizedVoiceTrimValues(s.VoiceTrimStart, s.VoiceTrimEnd)
}

// HasVoiceTrim reports whether the scene has a non-destructive audio trim range narrower than [0, 1].
func (s *Scene) HasVoiceTrim() bool {
	if s == nil {
		return false
	}
	return HasVoiceTrimValues(s.VoiceTrimStart, s.VoiceTrimEnd)
}

// FindVoiceSection returns a pointer to the VoiceSection matching key, or the first section as fallback.
func (s *Scene) FindVoiceSection(key string) *VoiceSection {
	if s == nil || len(s.VoiceSections) == 0 {
		return nil
	}
	clean := strings.TrimSpace(key)
	for i := range s.VoiceSections {
		if strings.EqualFold(s.VoiceSections[i].Key, clean) {
			return &s.VoiceSections[i]
		}
	}
	return &s.VoiceSections[0]
}

// SetSectionVoiceMeta updates the explicit voice path and trim ratios for a section key on the scene.
func (s *Scene) SetSectionVoiceMeta(key, explicitVoice string, trimStart, trimEnd float64) {
	if s == nil {
		return
	}
	cleanKey := strings.ToLower(strings.TrimSpace(key))
	if cleanKey == "" {
		return
	}
	if s.SectionVoices == nil {
		s.SectionVoices = make(map[string]VoiceSectionMeta)
	}
	ts, te := NormalizedVoiceTrimValues(trimStart, trimEnd)
	if !HasVoiceTrimValues(ts, te) {
		ts = 0
		te = 0
	}
	s.SectionVoices[cleanKey] = VoiceSectionMeta{
		ExplicitVoice: SanitizeRelativeMediaPath(explicitVoice),
		TrimStart:     ts,
		TrimEnd:       te,
	}
}

// DeleteSectionVoiceMeta removes any explicit voice/trim metadata for a section key on the scene.
func (s *Scene) DeleteSectionVoiceMeta(key string) {
	if s == nil || s.SectionVoices == nil {
		return
	}
	delete(s.SectionVoices, strings.ToLower(strings.TrimSpace(key)))
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

// FileName returns the preferred .playtxt file name for this chapter, using its ID primarily.
func (c Chapter) FileName() string {
	if expID := SanitizeChapterID(c.ExplicitID); expID != "" && expID != "book" {
		return expID + PlaytxtExt
	}
	if id := SanitizeChapterID(c.ID); id != "" && id != "book" {
		return id + PlaytxtExt
	}
	if sf := strings.TrimSpace(c.SourceFile); sf != "" && !strings.EqualFold(sf, DefaultStoryFile) && !strings.EqualFold(sf, BookDataFileName) {
		return sf
	}
	if nameID := SanitizeChapterID(c.Name); nameID != "" && nameID != "book" {
		return nameID + PlaytxtExt
	}
	return DefaultChapterFile
}

type NodePosition struct {
	Row   int
	Col   int
	X     int
	Y     int
	HasXY bool
}

type WirePin struct {
	X int
	Y int
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
	License        string
	Description    string
	Mode           string
	ScriptStandard string
	StartSceneID   string
	RawStart       string
	Narrated       bool
	Imageless      bool
	AutoChoiceDelay float64
	CoverFile      string
	CoverURL       string
	ExportDir      string
	IsPacked       bool
	IsExternal     bool
	SourceURL      string
	RootPath       string
	TextFiles      []string
	AudioFiles     []string
	ImageFiles     []string
	PackedMedia    map[string]PackedMediaEntry
	Enums          []EnumDef
	Variables      []VariableDef
	Chapters       []Chapter
	Scenes         []*Scene
	Layout         map[string]NodePosition
	WirePins       map[string][]WirePin
	EditorPanX     int
	EditorPanY     int
	EditorZoom     float64
	HasEditorView        bool
	Warnings             []string
	deferProseHTML       bool
	cachedBreakdown      *ScreenplayBreakdown
	cachedBreakdownStamp uint64
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

// DefaultChapterNameForKind returns the default first chapter or act title.
func DefaultChapterNameForKind(isScreenplay bool) string {
	if isScreenplay {
		return DefaultScreenplayActName
	}
	return DefaultBookChapterName
}

// DefaultChapterName returns "Akt I" for screenplays or "Rozdział 1" for books.
func (b *Book) DefaultChapterName() string {
	return DefaultChapterNameForKind(b.IsScreenplay())
}

// EffectiveLicense returns the book's explicit license or the default copyright notice.
func (b *Book) EffectiveLicense() string {
	if b != nil && strings.TrimSpace(b.License) != "" {
		return strings.TrimSpace(b.License)
	}
	return "Wszelkie prawa zastrzeżone (All Rights Reserved)"
}

type HistoryEntry struct {
	Step          int
	SceneID       string
	SceneName     string
	Chapter       string
	ChoiceLabel   string
	TargetSceneID string
	Variables     map[string]string
}

const DefaultAutoChoiceDelay = 2.0

// NormalizeAutoChoiceDelay clamps an auto-choice delay in seconds to a valid range (default 2.0s).
func NormalizeAutoChoiceDelay(d float64) float64 {
	if d <= 0 || d > 30 {
		return DefaultAutoChoiceDelay
	}
	return d
}

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
