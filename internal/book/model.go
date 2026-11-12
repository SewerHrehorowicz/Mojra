package book

import (
	"fmt"
	"strconv"
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
	TargetEnd                = "END"
	DefaultAudioDuration     = "0:00"
	DefaultTrimStartStr      = "0.0000"
	DefaultTrimEndStr        = "1.0000"

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

// ResolveChoiceNavigation resolves the destination scene ID and choice effects for choiceIdx.
func (s *Scene) ResolveChoiceNavigation(choiceIdx int, fallbackTargetID, startSceneID string) (string, string) {
	if s == nil || choiceIdx < 0 || choiceIdx >= len(s.Choices) {
		return fallbackTargetID, ""
	}
	ch := s.Choices[choiceIdx]
	targetID := fallbackTargetID
	if ch.IsEnding || strings.EqualFold(ch.TargetSceneID, TargetEnd) || strings.EqualFold(ch.Target, TargetEnd) {
		targetID = startSceneID
	} else if ch.TargetSceneID != "" {
		targetID = ch.TargetSceneID
	} else if targetID == "" {
		targetID = ch.Target
	}
	return targetID, ch.Effects
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
	HasPersistedAutoIDs  bool
	NextAutoSceneSeq     int
	NextSceneNameSeq     int
	RetiredSceneIDs      map[string]bool
	AutoSceneIDByName    map[string]string
	Warnings             []string
	deferProseHTML       bool
	cachedBreakdown      *ScreenplayBreakdown
	cachedBreakdownStamp uint64
}

// ParseAutoSceneSeq extracts N from an auto scene ID of the form "s<N>" (N >= 1), or returns 0.
func ParseAutoSceneSeq(raw string) int {
	trimmed := strings.ToLower(strings.TrimSpace(raw))
	if len(trimmed) < 2 || trimmed[0] != 's' {
		return 0
	}
	for i := 1; i < len(trimmed); i++ {
		if trimmed[i] < '0' || trimmed[i] > '9' {
			return 0
		}
	}
	n, err := strconv.Atoi(trimmed[1:])
	if err != nil || n <= 0 {
		return 0
	}
	return n
}

// ParseDefaultSceneNameSeq extracts N from "Nowa scena N", "Scena N", "nowa_scena_N", or "scena_N", or returns 0.
func ParseDefaultSceneNameSeq(raw string) int {
	lower := strings.ToLower(strings.TrimSpace(raw))
	for _, prefix := range []string{"nowa scena ", "nowa_scena_", "scena ", "scena_"} {
		if strings.HasPrefix(lower, prefix) {
			rest := strings.TrimSpace(lower[len(prefix):])
			if n, err := strconv.Atoi(rest); err == nil && n > 0 {
				return n
			}
		}
	}
	return 0
}

// RetireSceneID records one or more scene IDs, explicit IDs, or scene names as permanently allocated
// so they are never returned to the pool for reuse.
func (b *Book) RetireSceneID(ids ...string) {
	if b == nil {
		return
	}
	if b.RetiredSceneIDs == nil {
		b.RetiredSceneIDs = make(map[string]bool)
	}
	for _, raw := range ids {
		trimmed := strings.ToLower(strings.TrimSpace(raw))
		if trimmed == "" {
			continue
		}
		b.RetiredSceneIDs[trimmed] = true
		if seq := ParseAutoSceneSeq(trimmed); seq > b.NextAutoSceneSeq {
			b.NextAutoSceneSeq = seq
		}
		if nameSeq := ParseDefaultSceneNameSeq(trimmed); nameSeq > b.NextSceneNameSeq {
			b.NextSceneNameSeq = nameSeq
		}
		if slug := strings.ToLower(SanitizeSceneID(raw)); slug != "" {
			b.RetiredSceneIDs[slug] = true
			if seq := ParseAutoSceneSeq(slug); seq > b.NextAutoSceneSeq {
				b.NextAutoSceneSeq = seq
			}
			if nameSeq := ParseDefaultSceneNameSeq(slug); nameSeq > b.NextSceneNameSeq {
				b.NextSceneNameSeq = nameSeq
			}
		}
		if norm := NormalizeLookupKey(raw); norm != "" {
			b.RetiredSceneIDs[norm] = true
		}
	}
}

// IsSceneIDRetired reports whether cand was ever allocated and retired in this book.
func (b *Book) IsSceneIDRetired(cand string) bool {
	if b == nil || len(b.RetiredSceneIDs) == 0 {
		return false
	}
	trimmed := strings.ToLower(strings.TrimSpace(cand))
	if trimmed != "" && b.RetiredSceneIDs[trimmed] {
		return true
	}
	if slug := strings.ToLower(SanitizeSceneID(cand)); slug != "" && b.RetiredSceneIDs[slug] {
		return true
	}
	if norm := NormalizeLookupKey(cand); norm != "" && b.RetiredSceneIDs[norm] {
		return true
	}
	return false
}

// BindAutoSceneID associates an unnamed/placeholder scene's normalized Name with its allocated s<N> auto-ID.
func (b *Book) BindAutoSceneID(sceneName, autoID string) {
	if b == nil {
		return
	}
	norm := NormalizeLookupKey(sceneName)
	idLower := strings.ToLower(strings.TrimSpace(autoID))
	if norm == "" || idLower == "" {
		return
	}
	if b.AutoSceneIDByName == nil {
		b.AutoSceneIDByName = make(map[string]string)
	}
	b.AutoSceneIDByName[norm] = idLower
	b.HasPersistedAutoIDs = true
	b.RetireSceneID(idLower)
}

// LookupBoundAutoSceneID returns any previously bound auto-ID for sceneName.
func (b *Book) LookupBoundAutoSceneID(sceneName string) string {
	if b == nil || len(b.AutoSceneIDByName) == 0 {
		return ""
	}
	norm := NormalizeLookupKey(sceneName)
	if norm == "" {
		return ""
	}
	return b.AutoSceneIDByName[norm]
}

// UnbindAutoSceneName removes a sceneName from AutoSceneIDByName while keeping its old auto-ID retired forever.
func (b *Book) UnbindAutoSceneName(sceneName string) {
	if b == nil || len(b.AutoSceneIDByName) == 0 {
		return
	}
	norm := NormalizeLookupKey(sceneName)
	if norm == "" {
		return
	}
	if oldID, ok := b.AutoSceneIDByName[norm]; ok {
		b.RetireSceneID(oldID)
		delete(b.AutoSceneIDByName, norm)
	}
}

// NextUniqueAutoSceneID allocates the next monotonic "s<N>" scene ID that has never been used in this book.
func (b *Book) NextUniqueAutoSceneID(usedIDs map[string]bool) string {
	if b == nil {
		return "s1"
	}
	start := b.NextAutoSceneSeq + 1
	if start < 1 {
		start = 1
	}
	maxTries := len(b.Scenes) + len(b.RetiredSceneIDs) + 64
	for n := start; n < start+maxTries; n++ {
		cand := fmt.Sprintf("s%d", n)
		if usedIDs != nil && usedIDs[cand] {
			continue
		}
		if b.IsSceneIDRetired(cand) {
			continue
		}
		takenInScenes := false
		for _, sc := range b.Scenes {
			if sc != nil && (strings.EqualFold(sc.ID, cand) || strings.EqualFold(sc.ExplicitID, cand)) {
				takenInScenes = true
				break
			}
		}
		if takenInScenes {
			continue
		}
		b.NextAutoSceneSeq = n
		b.RetireSceneID(cand)
		if usedIDs != nil {
			usedIDs[cand] = true
		}
		return cand
	}
	b.NextAutoSceneSeq++
	cand := fmt.Sprintf("s%d", b.NextAutoSceneSeq)
	b.RetireSceneID(cand)
	if usedIDs != nil {
		usedIDs[cand] = true
	}
	return cand
}

// CopySceneIDPoolStateFrom merges monotonic counters and retired scene IDs from prev into b.
func (b *Book) CopySceneIDPoolStateFrom(prev *Book) {
	if b == nil || prev == nil {
		return
	}
	b.HasPersistedAutoIDs = true
	if prev.NextAutoSceneSeq > b.NextAutoSceneSeq {
		b.NextAutoSceneSeq = prev.NextAutoSceneSeq
	}
	if prev.NextSceneNameSeq > b.NextSceneNameSeq {
		b.NextSceneNameSeq = prev.NextSceneNameSeq
	}
	for k, v := range prev.RetiredSceneIDs {
		if v {
			b.RetireSceneID(k)
		}
	}
	if len(prev.AutoSceneIDByName) > 0 {
		if b.AutoSceneIDByName == nil {
			b.AutoSceneIDByName = make(map[string]string, len(prev.AutoSceneIDByName))
		}
		for k, v := range prev.AutoSceneIDByName {
			if _, exists := b.AutoSceneIDByName[k]; !exists {
				b.AutoSceneIDByName[k] = v
			}
			b.RetireSceneID(v)
		}
	}
	for _, sc := range prev.Scenes {
		if sc == nil {
			continue
		}
		if ParseAutoSceneSeq(sc.ID) > 0 {
			b.RetireSceneID(sc.ID)
		}
		if ParseAutoSceneSeq(sc.ExplicitID) > 0 {
			b.RetireSceneID(sc.ExplicitID)
		}
		if sc.ExplicitID == "" && sc.ID != "" && sc.Name != "" {
			if b.AutoSceneIDByName == nil {
				b.AutoSceneIDByName = make(map[string]string)
			}
			norm := NormalizeLookupKey(sc.Name)
			if norm != "" {
				if _, exists := b.AutoSceneIDByName[norm]; !exists {
					b.AutoSceneIDByName[norm] = strings.ToLower(sc.ID)
				}
			}
		}
	}
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

// IsNumberedStandard reports whether the screenplay standard numbers scene sluglines ("pl" or "eu").
func IsNumberedStandard(raw string) bool {
	norm := NormalizeScriptStandard(raw)
	return norm == "pl" || norm == "eu"
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

// FindScene finds a scene in the book by ID, ExplicitID, or normalized Name.
func (b *Book) FindScene(sceneID string) *Scene {
	if b == nil || sceneID == "" {
		return nil
	}
	for _, sc := range b.Scenes {
		if sc.ID == sceneID {
			return sc
		}
	}
	for _, sc := range b.Scenes {
		if strings.EqualFold(sc.ID, sceneID) || (sc.ExplicitID != "" && strings.EqualFold(sc.ExplicitID, sceneID)) {
			return sc
		}
	}
	normKey := NormalizeLookupKey(sceneID)
	for _, sc := range b.Scenes {
		if sc.Name != "" && (strings.EqualFold(sc.Name, sceneID) || (normKey != "" && NormalizeLookupKey(sc.Name) == normKey)) {
			return sc
		}
	}
	return nil
}

// CurrentScene resolves the active scene in b for the given save state.
func (b *Book) CurrentScene(save *SaveState) *Scene {
	if b == nil || len(b.Scenes) == 0 {
		return nil
	}
	if save != nil {
		if sc := b.FindScene(save.CurrentSceneID); sc != nil {
			return sc
		}
	}
	if b.StartSceneID != "" {
		if sc := b.FindScene(b.StartSceneID); sc != nil {
			return sc
		}
	}
	return b.Scenes[0]
}

// IsImageless reports whether the book enforces imageless mode (explicitly or as a screenplay).
func (b *Book) IsImageless() bool {
	return b != nil && (b.Imageless || b.IsScreenplay())
}

// IsEffectiveImageless reports whether imageless mode is active for this book and settings.
func (b *Book) IsEffectiveImageless(settings *AppSettings) bool {
	if b.IsImageless() {
		return true
	}
	return settings != nil && settings.Imageless
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
