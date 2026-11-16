package model

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

const (
	defaultChapterRef       = "rozdzial_1"
	defaultLicenseLabel     = "Wszelkie prawa zastrzeżone (All Rights Reserved)"
	modeKeywordScreenplay   = "screenplay"
	modeKeywordScenariusz   = "scenariusz"
	modeKeywordScript       = "script"
	modeKeywordFilm         = "film"
	stdKeywordEU            = "eu"
	stdKeywordEurope        = "europe"
	stdKeywordEuropa        = "europa"
	stdKeywordUS            = "us"
	stdKeywordUSA           = "usa"
	stdKeywordHollywood     = "hollywood"
	stdKeywordLetter        = "letter"
	autoSceneIDFormat       = "s%d"
	branchNameMain          = "main"
	errNoActiveBookPL       = "Brak aktywnej książki"
	errTargetChNotFoundPL   = "Nie znaleziono docelowego rozdziału"
	errSelfChapterLinkPL    = "Przejście rozdziału nie może wskazywać na ten sam rozdział"
	errLinkToRootChapterPL  = "Zmiana rozdziału jest jednokierunkowa — nie można wracać do pierwszego rozdziału"
	errBackwardChapterLnkPL = "Zmiana rozdziału jest jednokierunkowa — nie można wracać do wcześniejszego rozdziału"
	errMissingChStartFmtPL  = "Rozdział „%s”: scena początkowa „%s” nie istnieje w tym rozdziale"
)

// Domain hooks wired by domain/book/actions at init time so model remains 100% pure and OS-free.
var (
	FormatProseHTMLHook         func(rawText string) string
	EvaluateConditionHook       func(expr string, state map[string]string, targetBook *Book) bool
	AnalyzeScreenplayHook       func(targetBook *Book) *ScreenplayBreakdown
	ResolveExportDirHook        func(targetBook *Book, settings *AppSettings) string
	PersistExportDirHook        func(targetBook *Book, savesDir string, settings *AppSettings, dir string)
	ReloadFromDiskHook          func(targetBook *Book, booksDir string, settings *AppSettings) *Book
	HistoryExitPreviewHook      func(history *BookHistory, booksDir string, settings *AppSettings, studioBook **Book) string
	HistoryRefreshFromDiskHook  func(history *BookHistory, studioBook *Book) *BookHistory
	HistoryEnsureBaselineHook   func(history *BookHistory, studioBook *Book, selectedSceneID string)
	HistoryRecordStepHook       func(history *BookHistory, studioBook *Book, label, kind, coalesceKey, selectedSceneID string)
	SyncHistoryBranchesMetaHook func(history *BookHistory)
)

// Choice represents an outgoing transition or branching option on a Scene.
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
	Idx             int
	SceneID         string
	RawLabel        string
	Arrow           string
	IsSingle        bool
	IsEditable      bool
	IsBranch        bool
	IsCondUnmet     bool
	IsLockedPreview bool
	SelectedValue   string
	HasCondition    bool
	HasEffects      bool
	HasRule         bool
	IsRuleOpen      bool
	IsExitInvalid   bool
	ExitErrorHint   string
	IsCondInvalid   bool
	IsEffInvalid    bool
	CondErrorHint   string
	EffErrorHint    string
}

// ChoiceEval holds the evaluated state of a single choice for a player scene render.
type ChoiceEval struct {
	Choice         Choice
	ChoiceIdx      int
	ConditionMet   bool
	IsLocked       bool
	Label          string
	ShowWhenLocked bool
	IsEnding       bool
	HasEffects     bool
	Effects        string
}

// SceneChoicesEval summarizes the visible, unlocked, and terminal choice state of a scene.
type SceneChoicesEval struct {
	Visible           []Choice
	Unlocked          []Choice
	UnlockedNonEnding []Choice
	SingleUnlocked    *Choice
	IsBranching       bool
	IsTerminal        bool
}

// VoiceSectionMeta stores explicit voiceover path and trim ratios for a section key on a Scene.
type VoiceSectionMeta struct {
	ExplicitVoice string
	TrimStart     float64
	TrimEnd       float64
}

// VoiceSection represents an ordered static or conditional voiceover section extracted from a scene's AST.
type VoiceSection struct {
	Index         int
	Key           string
	Kind          string
	Condition     string
	Label         string
	ShortLabel    string
	HeaderTitle   string
	Text          string
	ExplicitVoice  string
	TargetFile     string
	AudioFile      string
	AudioURL       string
	TrimStart      float64
	TrimEnd        float64
	TrimStartStr   string
	TrimEndStr     string
	DurationSec    float64
	DurationLabel  string
	HasAudio       bool
	HasTrim        bool
	IsActive       bool
	IsSelected     bool
	ShowDeleteBtn  bool
	IsStatic       bool
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

// SelectOption represents a dropdown option item in custom-select controls.
type SelectOption struct {
	OptIdx   int
	Value    string
	Label    string
	Meta     string
	VarName  string
	IsActive bool
}

// Scene represents a single story or screenplay scene node inside a Book.
type Scene struct {
	ID              string
	ExplicitID      string
	Name            string
	Slugline        string
	Synopsis        string
	Effects         string
	Chapter         string
	ChapterID       string
	SourceFile      string
	OrderIndex      int
	Row             int
	Col             int
	X               int
	Y               int
	Unreachable     bool
	ExplicitImage   string
	ExplicitVoice   string
	VoiceTrimStart  float64
	VoiceTrimEnd    float64
	SectionVoices   map[string]VoiceSectionMeta
	VoiceSections   []VoiceSection
	ExplicitNext    string
	ExplicitEnd     bool
	Text            string
	HTML            string
	HTMLSourceText  string
	AudioFile       string
	ImageFile       string
	AudioURL        string
	ImageURL        string
	Choices         []Choice
	Warnings        []string
	SluglineIntExt  string
	SluglineLoc     string
	SluglineTime    string
	ScriptPageLabel string
	IsScreenplay    bool
}

// DisplayName returns the scene's title if present, or falls back to its ExplicitID or ID.
func (sceneVal Scene) DisplayName() string {
	if name := strings.TrimSpace(sceneVal.Name); name != "" {
		return name
	}
	if expID := strings.TrimSpace(sceneVal.ExplicitID); expID != "" {
		return expID
	}
	return strings.TrimSpace(sceneVal.ID)
}

// ReferenceKey returns the preferred target reference for choices and start: (ExplicitID if set, otherwise Name, otherwise ID).
func (sceneVal Scene) ReferenceKey() string {
	if expID := strings.TrimSpace(sceneVal.ExplicitID); expID != "" {
		return expID
	}
	if name := strings.TrimSpace(sceneVal.Name); name != "" {
		return name
	}
	return strings.TrimSpace(sceneVal.ID)
}

// ResolveChoiceNavigation resolves the destination scene ID and choice effects for choiceIdx.
func (scenePtr *Scene) ResolveChoiceNavigation(choiceIdx int, fallbackTargetID, startSceneID string) (string, string) {
	if scenePtr == nil || choiceIdx < 0 || choiceIdx >= len(scenePtr.Choices) {
		return fallbackTargetID, ""
	}
	chosen := scenePtr.Choices[choiceIdx]
	targetID := fallbackTargetID
	if chosen.IsEnding || strings.EqualFold(chosen.TargetSceneID, TargetEnd) || strings.EqualFold(chosen.Target, TargetEnd) {
		targetID = startSceneID
	} else if chosen.TargetSceneID != "" {
		targetID = chosen.TargetSceneID
	} else if targetID == "" {
		targetID = chosen.Target
	}
	return targetID, chosen.Effects
}

// NormalizedVoiceTrimValues normalizes [start, end] ratios to [0, 1].
func NormalizedVoiceTrimValues(startRatio, endRatio float64) (float64, float64) {
	if startRatio < 0 || startRatio >= 1 {
		startRatio = 0
	}
	if endRatio <= 0 || endRatio > 1 {
		endRatio = 1
	}
	if endRatio <= startRatio+0.001 {
		return 0, 1
	}
	return startRatio, endRatio
}

// HasVoiceTrimValues reports whether [start, end] is narrower than [0, 1].
func HasVoiceTrimValues(startRatio, endRatio float64) bool {
	trimStart, trimEnd := NormalizedVoiceTrimValues(startRatio, endRatio)
	return trimStart > 0.002 || trimEnd < 0.998
}

// NormalizedVoiceTrim returns the effective non-destructive [startRatio, endRatio] in [0, 1].
func (scenePtr *Scene) NormalizedVoiceTrim() (float64, float64) {
	if scenePtr == nil {
		return 0, 1
	}
	return NormalizedVoiceTrimValues(scenePtr.VoiceTrimStart, scenePtr.VoiceTrimEnd)
}

// HasVoiceTrim reports whether the scene has a non-destructive audio trim range narrower than [0, 1].
func (scenePtr *Scene) HasVoiceTrim() bool {
	if scenePtr == nil {
		return false
	}
	return HasVoiceTrimValues(scenePtr.VoiceTrimStart, scenePtr.VoiceTrimEnd)
}

// FindVoiceSection returns a pointer to the VoiceSection matching sectionKey, or the first section as fallback.
func (scenePtr *Scene) FindVoiceSection(sectionKey string) *VoiceSection {
	if scenePtr == nil || len(scenePtr.VoiceSections) == 0 {
		return nil
	}
	cleanKey := strings.TrimSpace(sectionKey)
	for idx := range scenePtr.VoiceSections {
		if strings.EqualFold(scenePtr.VoiceSections[idx].Key, cleanKey) {
			return &scenePtr.VoiceSections[idx]
		}
	}
	return &scenePtr.VoiceSections[0]
}

// SetSectionVoiceMeta updates the explicit voice path and trim ratios for a section key on the scene.
func (scenePtr *Scene) SetSectionVoiceMeta(sectionKey, explicitVoice string, trimStart, trimEnd float64) {
	if scenePtr == nil {
		return
	}
	cleanKey := strings.ToLower(strings.TrimSpace(sectionKey))
	if cleanKey == "" {
		return
	}
	if scenePtr.SectionVoices == nil {
		scenePtr.SectionVoices = make(map[string]VoiceSectionMeta)
	}
	normStart, normEnd := NormalizedVoiceTrimValues(trimStart, trimEnd)
	if !HasVoiceTrimValues(normStart, normEnd) {
		normStart = 0
		normEnd = 0
	}
	scenePtr.SectionVoices[cleanKey] = VoiceSectionMeta{
		ExplicitVoice: SanitizeRelativeMediaPath(explicitVoice),
		TrimStart:     normStart,
		TrimEnd:       normEnd,
	}
}

// DeleteSectionVoiceMeta removes explicit voiceover metadata for a specific section key.
func (scenePtr *Scene) DeleteSectionVoiceMeta(sectionKey string) {
	if scenePtr == nil || scenePtr.SectionVoices == nil {
		return
	}
	delete(scenePtr.SectionVoices, strings.ToLower(strings.TrimSpace(sectionKey)))
}

// EnsureHTML lazily renders and caches safe HTML paragraphs from scenePtr.Text if not already rendered.
func (scenePtr *Scene) EnsureHTML() string {
	if scenePtr == nil {
		return ""
	}
	if scenePtr.Text == "" {
		scenePtr.HTML = ""
		scenePtr.HTMLSourceText = ""
		return ""
	}
	if scenePtr.HTML == "" || scenePtr.HTMLSourceText != scenePtr.Text {
		if FormatProseHTMLHook != nil {
			scenePtr.HTML = FormatProseHTMLHook(scenePtr.Text)
		} else {
			scenePtr.HTML = "<p>" + scenePtr.Text + "</p>"
		}
		scenePtr.HTMLSourceText = scenePtr.Text
	}
	return scenePtr.HTML
}

// Chapter represents a chapter or screenplay act inside a Book.
type Chapter struct {
	ID                string
	ExplicitID        string
	Name              string
	ExplicitStart     string
	StartSceneID      string
	StartSceneLabel   string
	SourceFile        string
	SceneIDs          []string
	SceneCount        int
	LengthStat        string
	BranchCount       int
	ExitCount         int
	StartSceneValue   string
	StartSceneOptions []SelectOption
	SceneStartOptions []SelectOption
}

// DisplayName returns the chapter's title if present, or falls back to its ExplicitID or ID.
func (chapterVal Chapter) DisplayName() string {
	if name := strings.TrimSpace(chapterVal.Name); name != "" {
		return name
	}
	if expID := strings.TrimSpace(chapterVal.ExplicitID); expID != "" {
		return expID
	}
	return strings.TrimSpace(chapterVal.ID)
}

// ReferenceKey returns the preferred target reference for chapter links.
func (chapterVal Chapter) ReferenceKey() string {
	if expID := SanitizeChapterID(chapterVal.ExplicitID); expID != "" {
		return expID
	}
	if chapID := SanitizeChapterID(chapterVal.ID); chapID != "" {
		return chapID
	}
	if nameID := SanitizeChapterID(chapterVal.Name); nameID != "" {
		return nameID
	}
	return defaultChapterRef
}

// FileName returns the preferred .playtxt file name for this chapter.
func (chapterVal Chapter) FileName() string {
	if expID := SanitizeChapterID(chapterVal.ExplicitID); expID != "" && expID != fallbackBookID {
		return expID + PlaytxtExt
	}
	if chapID := SanitizeChapterID(chapterVal.ID); chapID != "" && chapID != fallbackBookID {
		return chapID + PlaytxtExt
	}
	if srcFile := strings.TrimSpace(chapterVal.SourceFile); srcFile != "" && !strings.EqualFold(srcFile, DefaultStoryFile) && !strings.EqualFold(srcFile, BookDataFileName) {
		return srcFile
	}
	if nameID := SanitizeChapterID(chapterVal.Name); nameID != "" && nameID != fallbackBookID {
		return nameID + PlaytxtExt
	}
	return DefaultChapterFile
}

// PackedMediaEntry indexes a media file inside a read-only .playbook archive or STBC container.
type PackedMediaEntry struct {
	DataOffset  int64
	Size        int64
	Method      uint16
	ModTime     time.Time
	Header512   [512]byte
	HeaderLen   int
	ValidHeader bool
}

// Book represents a complete interactive book or screenplay project.
type Book struct {
	ID                   string
	Title                string
	Author               string
	License              string
	Description          string
	Mode                 string
	ScriptStandard       string
	StartSceneID         string
	RawStart             string
	Narrated             bool
	Imageless            bool
	AutoChoiceDelay      float64
	CoverFile            string
	CoverURL             string
	ExportDir            string
	IsPacked             bool
	IsExternal           bool
	SourceURL            string
	RootPath             string
	TextFiles            []string
	AudioFiles           []string
	ImageFiles           []string
	PackedMedia          map[string]PackedMediaEntry
	Enums                []EnumDef
	Variables            []VariableDef
	Chapters             []Chapter
	Scenes               []*Scene
	Layout               map[string]NodePosition
	WirePins             map[string][]WirePin
	EditorPanX           int
	EditorPanY           int
	EditorZoom           float64
	HasEditorView        bool
	HasPersistedAutoIDs  bool
	NextAutoSceneSeq     int
	NextSceneNameSeq     int
	RetiredSceneIDs      map[string]bool
	AutoSceneIDByName    map[string]string
	Warnings             []string
	ProgressPct          int
	SceneCount           int
	ModeBadge            string
	KindBadge            string
	MetaLine             string
	PathLine             string
	IsCurrent            bool
	IsUnpacked           bool
	DeferProseHTML       bool
	CachedBreakdown      *ScreenplayBreakdown
	CachedBreakdownStamp uint64
}

// ExportFormatItem describes an export format option in the Studio export dialog.
type ExportFormatItem struct {
	ID         string
	Label      string
	Ext        string
	IsSelected bool
}

// StudioSearchItem represents a scene search result in the Studio search bar.
type StudioSearchItem struct {
	ID             string
	DisplayTitle   string
	Title          string
	Slugline       string
	Chapter        string
	ChapterUpper   string
	Synopsis       string
	TextB64        string
	Choices        string
	DefaultBadge   string
	DefaultSnippet string
	IsSelected     bool
}

// EditorNodeView holds the prepared inspector view model for the selected scene node.
type EditorNodeView struct {
	ID                       string
	ExplicitID               string
	CurrentImage             string
	CurrentImageLabel        string
	ImageOptions             []SelectOption
	CurrentVoice             string
	CurrentVoiceLabel        string
	CurrentVoiceURL          string
	VoiceDurationLabel       string
	VoiceTrimStart           string
	VoiceTrimEnd             string
	HasVoiceFile             bool
	HasVoiceTrim             bool
	VoiceOptions             []SelectOption
	HasVoiceSections         bool
	ActiveVoiceSectionKey    string
	ActiveVoiceSectionFile   string
	ActiveVoiceSectionText   string
	ActiveVoiceSectionHeader string
	VoiceSections            []VoiceSection
	Name                     string
	Slugline                 string
	Synopsis                 string
	Effects                  string
	HasEffects               bool
	IsEffectsInvalid         bool
	EffectsErrorHint         string
	ScriptLengthLabel        string
	Chapter                  string
	ChapterOptions           []SelectOption
	SourceFile               string
	IsStartScene             bool
	IsChapterEntry           bool
	HasNodeWarnings          bool
	NodeWarnings             []string
	InspectorPorts           []Choice
	PortCount                int
}

// ScreenplayBlock represents a single parsed screenplay block within a scene.
type ScreenplayBlock struct {
	Kind          ScreenplayBlockKind
	Text          string
	CharacterName string
	CharacterExt  string
	WrappedLines  int
}

// SluglineInfo holds the deterministically parsed components of a scene slugline.
type SluglineInfo struct {
	Raw            string
	IntExt         string
	Location       string
	TimeOfDay      string
	HasValidPrefix bool
	HasTimeOfDay   bool
	IsInt          bool
	IsExt          bool
	IsDay          bool
	IsNight        bool
}

// SceneScriptMetrics holds deterministic page/eighths and timing metrics for a single scene.
type SceneScriptMetrics struct {
	SceneID          string
	SceneNumber      int
	Lines            int
	Eighths          int
	FormattedLength  string
	EstimatedSeconds int
}

// ScreenplayInsertPill represents a quick-insert screenplay block button in the editor.
type ScreenplayInsertPill struct {
	Name          string
	Kind          string
	Label         string
	Hint          string
	Snippet       string
	SelectTarget  string
	DoubleNewline bool
}

// LocationStat aggregates production breakdown statistics for a single location.
type LocationStat struct {
	Name               string
	IntExt             string
	SceneCount         int
	TotalEighths       int
	FormattedLength    string
	DayScenes          int
	NightScenes        int
	IntScenes          int
	ExtScenes          int
	EstimatedShootDays int
	SceneIDs           []string
	FirstSceneID       string
	DayNightBadge      string
}

// CharacterStat aggregates production breakdown statistics for a character role.
type CharacterStat struct {
	Name              string
	DialogueCues      int
	WordCount         int
	SpeakingSceneIDs  []string
	SilentSceneIDs    []string
	TotalScenes       int
	SpeakingScenes    int
	SilentScenes      int
	LocationsVisited  []string
	EstimatedCallDays int
	FirstSceneID      string
}

// ActStat aggregates page/scene metrics for an Act (chapter in screenplay mode).
type ActStat struct {
	ID              string
	Name            string
	SceneCount      int
	TotalEighths    int
	FormattedLength string
	SharePct        int
	IsDominant      bool
	IsEmpty         bool
}

// ScreenplayBreakdown contains the complete deterministic breakdown of a screenplay project.
type ScreenplayBreakdown struct {
	Standard              string
	TotalScenes           int
	TotalLines            int
	TotalEighths          int
	TotalPagesFormatted   string
	EstimatedMinutes      int
	EstimatedRuntimeLabel string
	IntScenes             int
	ExtScenes             int
	IntPct                int
	ExtPct                int
	DayScenes             int
	NightScenes           int
	DayPct                int
	NightPct              int
	EstimatedShootDays    int
	ToolbarSummary        string
	Acts                  []ActStat
	Characters            []CharacterStat
	Locations             []LocationStat
	SceneMetrics          map[string]SceneScriptMetrics
}

// HistoryBranch represents a named Git-style draft branch in version history.
type HistoryBranch struct {
	Name          string
	HeadRevID     string
	BaseRevID     string
	CreatedAtUnix int64
}

// BranchComparison summarizes structural, page/runtime, cast, and location deltas between two branches.
type BranchComparison struct {
	BranchA             string
	BranchB             string
	ScenesA             int
	ScenesB             int
	SceneDelta          int
	AddedScenes         []string
	RemovedScenes       []string
	ModifiedScenes      []string
	EighthsA            int
	EighthsB            int
	EighthsDelta        int
	PagesDeltaFormatted string
	MinutesA            int
	MinutesB            int
	MinutesDelta        int
	LocationsA          int
	LocationsB          int
	LocationDelta       int
	CharactersA         int
	CharactersB         int
	CharacterDelta      int
	SummaryLabel        string
}

// HistoryRevision represents a single snapshot in version history.
type HistoryRevision struct {
	ID              string
	Branch          string
	ParentID        string
	TimeUnix        int64
	Kind            string
	Named           bool
	Label           string
	SelectedSceneID string
	CoalesceKey     string
	ScenesCount     int
	TotalEighths    int
	Files           map[string]string
}

// HistoryGraphLane represents a single vertical lane cell inside a history graph row.
type HistoryGraphLane struct {
	Branch           string
	HoverHint        string
	IsBranchRail     bool
	IsOtherBranch    bool
	HasRailUp        bool
	HasRailDown      bool
	IsActiveRailUp   bool
	IsActiveRailDown bool
	IsDashedRailUp   bool
	IsDashedRailDown bool
	HasNode          bool
	IsForkCorner     bool
	HasForkHoriz     bool
	IsActiveFork     bool
	IsCompactIn      bool
	IsCompactOut     bool
	HasCompactHoriz  bool
	IsActiveCompact  bool
	HasStemStart     bool
	HasStemThrough   bool
}

// HistoryGraphRow represents a rendered revision row in the Studio history timeline.
type HistoryGraphRow struct {
	ID              string
	Label           string
	MetaLabel       string
	Branch          string
	HoverHint       string
	IsUnnamedStep   bool
	IsCurrent       bool
	IsActiveHead    bool
	IsPreview       bool
	IsUndone        bool
	IsActiveLineage bool
	IsActiveBranch  bool
	IsBranchHead    bool
	Lanes           []HistoryGraphLane
}

// HistoryPreview holds the read-only revision preview snapshot for a book in Studio.
type HistoryPreview struct {
	RevisionID      string
	RevisionLabel   string
	LiveBook        *Book
	SelectedSceneID string
}

// BookHistory holds the parsed state of a book's version history.
type BookHistory struct {
	ActiveBranch  string
	Cursor        string
	NextSeq       int
	Branches      []HistoryBranch
	Revisions     []HistoryRevision
	Preview       *HistoryPreview
	CachedBookID  string
	CachedModNano int64
	CachedSize    int64
}

// RetireSceneID records one or more scene IDs, explicit IDs, or scene names as permanently allocated.
func (targetBook *Book) RetireSceneID(rawIDs ...string) {
	if targetBook == nil {
		return
	}
	if targetBook.RetiredSceneIDs == nil {
		targetBook.RetiredSceneIDs = make(map[string]bool)
	}
	for _, raw := range rawIDs {
		trimmed := strings.ToLower(strings.TrimSpace(raw))
		if trimmed == "" {
			continue
		}
		targetBook.RetiredSceneIDs[trimmed] = true
		if seq := ParseAutoSceneSeq(trimmed); seq > targetBook.NextAutoSceneSeq {
			targetBook.NextAutoSceneSeq = seq
		}
		if nameSeq := ParseDefaultSceneNameSeq(trimmed); nameSeq > targetBook.NextSceneNameSeq {
			targetBook.NextSceneNameSeq = nameSeq
		}
		if slug := strings.ToLower(SanitizeSceneID(raw)); slug != "" {
			targetBook.RetiredSceneIDs[slug] = true
			if seq := ParseAutoSceneSeq(slug); seq > targetBook.NextAutoSceneSeq {
				targetBook.NextAutoSceneSeq = seq
			}
			if nameSeq := ParseDefaultSceneNameSeq(slug); nameSeq > targetBook.NextSceneNameSeq {
				targetBook.NextSceneNameSeq = nameSeq
			}
		}
		if norm := NormalizeLookupKey(raw); norm != "" {
			targetBook.RetiredSceneIDs[norm] = true
		}
	}
}

// IsSceneIDRetired reports whether candidate was ever allocated and retired in this book.
func (targetBook *Book) IsSceneIDRetired(candidate string) bool {
	if targetBook == nil || len(targetBook.RetiredSceneIDs) == 0 {
		return false
	}
	trimmed := strings.ToLower(strings.TrimSpace(candidate))
	if trimmed != "" && targetBook.RetiredSceneIDs[trimmed] {
		return true
	}
	if slug := strings.ToLower(SanitizeSceneID(candidate)); slug != "" && targetBook.RetiredSceneIDs[slug] {
		return true
	}
	if norm := NormalizeLookupKey(candidate); norm != "" && targetBook.RetiredSceneIDs[norm] {
		return true
	}
	return false
}

// BindAutoSceneID associates an unnamed/placeholder scene's normalized Name with its allocated s<N> auto-ID.
func (targetBook *Book) BindAutoSceneID(sceneName, autoID string) {
	if targetBook == nil {
		return
	}
	norm := NormalizeLookupKey(sceneName)
	cleanID := strings.ToLower(strings.TrimSpace(autoID))
	if norm == "" || cleanID == "" {
		return
	}
	if targetBook.AutoSceneIDByName == nil {
		targetBook.AutoSceneIDByName = make(map[string]string)
	}
	targetBook.AutoSceneIDByName[norm] = cleanID
	targetBook.RetireSceneID(cleanID)
}

// LookupBoundAutoSceneID returns a previously bound auto-ID for sceneName, or "".
func (targetBook *Book) LookupBoundAutoSceneID(sceneName string) string {
	if targetBook == nil || len(targetBook.AutoSceneIDByName) == 0 {
		return ""
	}
	norm := NormalizeLookupKey(sceneName)
	if norm == "" {
		return ""
	}
	return targetBook.AutoSceneIDByName[norm]
}

// UnbindAutoSceneName removes the name -> auto-ID binding when a scene is deleted or given an ExplicitID.
func (targetBook *Book) UnbindAutoSceneName(sceneName string) {
	if targetBook == nil || len(targetBook.AutoSceneIDByName) == 0 {
		return
	}
	norm := NormalizeLookupKey(sceneName)
	if norm != "" {
		delete(targetBook.AutoSceneIDByName, norm)
	}
}

// NextUniqueAutoSceneID allocates the next fresh "s<N>" scene ID that has never been used or retired in targetBook.
func (targetBook *Book) NextUniqueAutoSceneID(usedIDs map[string]bool) string {
	if targetBook == nil {
		return "s1"
	}
	for _, sceneItem := range targetBook.Scenes {
		if sceneItem == nil {
			continue
		}
		if seq := ParseAutoSceneSeq(sceneItem.ID); seq > targetBook.NextAutoSceneSeq {
			targetBook.NextAutoSceneSeq = seq
		}
		if seq := ParseAutoSceneSeq(sceneItem.ExplicitID); seq > targetBook.NextAutoSceneSeq {
			targetBook.NextAutoSceneSeq = seq
		}
	}
	isTaken := func(cand string) bool {
		lower := strings.ToLower(cand)
		if usedIDs != nil && usedIDs[lower] {
			return true
		}
		if targetBook.IsSceneIDRetired(lower) {
			return true
		}
		for _, sceneItem := range targetBook.Scenes {
			if sceneItem != nil && (strings.EqualFold(sceneItem.ID, lower) || strings.EqualFold(sceneItem.ExplicitID, lower) || NormalizeLookupKey(sceneItem.Name) == lower) {
				return true
			}
		}
		return false
	}
	maxTries := len(targetBook.Scenes) + len(targetBook.RetiredSceneIDs) + 128
	for tries := 0; tries < maxTries; tries++ {
		targetBook.NextAutoSceneSeq++
		cand := fmt.Sprintf(autoSceneIDFormat, targetBook.NextAutoSceneSeq)
		if !isTaken(cand) {
			targetBook.RetireSceneID(cand)
			return cand
		}
	}
	targetBook.NextAutoSceneSeq++
	cand := fmt.Sprintf(autoSceneIDFormat, targetBook.NextAutoSceneSeq)
	targetBook.RetireSceneID(cand)
	return cand
}

// CopySceneIDPoolStateFrom merges monotonic counters and retired scene IDs from prevBook into targetBook.
func (targetBook *Book) CopySceneIDPoolStateFrom(prevBook *Book) {
	if targetBook == nil || prevBook == nil {
		return
	}
	targetBook.HasPersistedAutoIDs = true
	if prevBook.NextAutoSceneSeq > targetBook.NextAutoSceneSeq {
		targetBook.NextAutoSceneSeq = prevBook.NextAutoSceneSeq
	}
	if prevBook.NextSceneNameSeq > targetBook.NextSceneNameSeq {
		targetBook.NextSceneNameSeq = prevBook.NextSceneNameSeq
	}
	for retiredKey, isRetired := range prevBook.RetiredSceneIDs {
		if isRetired {
			targetBook.RetireSceneID(retiredKey)
		}
	}
	if len(prevBook.AutoSceneIDByName) > 0 {
		if targetBook.AutoSceneIDByName == nil {
			targetBook.AutoSceneIDByName = make(map[string]string, len(prevBook.AutoSceneIDByName))
		}
		for nameKey, idVal := range prevBook.AutoSceneIDByName {
			if _, exists := targetBook.AutoSceneIDByName[nameKey]; !exists {
				targetBook.AutoSceneIDByName[nameKey] = idVal
			}
			targetBook.RetireSceneID(idVal)
		}
	}
	for _, sceneItem := range prevBook.Scenes {
		if sceneItem == nil {
			continue
		}
		if ParseAutoSceneSeq(sceneItem.ID) > 0 {
			targetBook.RetireSceneID(sceneItem.ID)
		}
		if ParseAutoSceneSeq(sceneItem.ExplicitID) > 0 {
			targetBook.RetireSceneID(sceneItem.ExplicitID)
		}
		if sceneItem.ExplicitID == "" && sceneItem.ID != "" && sceneItem.Name != "" {
			if targetBook.AutoSceneIDByName == nil {
				targetBook.AutoSceneIDByName = make(map[string]string)
			}
			norm := NormalizeLookupKey(sceneItem.Name)
			if norm != "" {
				if _, exists := targetBook.AutoSceneIDByName[norm]; !exists {
					targetBook.AutoSceneIDByName[norm] = strings.ToLower(sceneItem.ID)
				}
			}
		}
	}
}

// NormalizeBookMode normalizes a project mode string to "screenplay" or "book".
func NormalizeBookMode(rawMode string) string {
	switch strings.ToLower(strings.TrimSpace(rawMode)) {
	case modeKeywordScreenplay, modeKeywordScenariusz, modeKeywordScript, modeKeywordFilm:
		return BookModeScreenplay
	default:
		return BookModeBook
	}
}

// NormalizeScriptStandard normalizes a regional screenplay standard string to "pl", "eu", or "us".
func NormalizeScriptStandard(rawStd string) string {
	switch strings.ToLower(strings.TrimSpace(rawStd)) {
	case stdKeywordEU, stdKeywordEurope, stdKeywordEuropa:
		return ScriptStandardEU
	case stdKeywordUS, stdKeywordUSA, stdKeywordHollywood, stdKeywordLetter:
		return ScriptStandardUS
	default:
		return ScriptStandardPL
	}
}

// IsScreenplay reports whether the book project is in Screenplay Mode.
func (targetBook *Book) IsScreenplay() bool {
	return targetBook != nil && NormalizeBookMode(targetBook.Mode) == BookModeScreenplay
}

// NormalizedMode returns "screenplay" or "book".
func (targetBook *Book) NormalizedMode() string {
	if targetBook == nil {
		return BookModeBook
	}
	return NormalizeBookMode(targetBook.Mode)
}

// NormalizedStandard returns "pl", "eu", or "us".
func (targetBook *Book) NormalizedStandard() string {
	if targetBook == nil {
		return ScriptStandardPL
	}
	return NormalizeScriptStandard(targetBook.ScriptStandard)
}

// IsNumberedStandard reports whether the screenplay standard numbers scene sluglines ("pl" or "eu").
func IsNumberedStandard(rawStd string) bool {
	norm := NormalizeScriptStandard(rawStd)
	return norm == ScriptStandardPL || norm == ScriptStandardEU
}

// DefaultChapterNameForKind returns the default first chapter or act title.
func DefaultChapterNameForKind(isScreenplay bool) string {
	if isScreenplay {
		return DefaultScreenplayActName
	}
	return DefaultBookChapterName
}

// DefaultChapterName returns "Akt I" for screenplays or "Rozdział 1" for books.
func (targetBook *Book) DefaultChapterName() string {
	return DefaultChapterNameForKind(targetBook.IsScreenplay())
}

// EffectiveLicense returns the book's license string or the default Mojra license label.
func (targetBook *Book) EffectiveLicense() string {
	if targetBook != nil && strings.TrimSpace(targetBook.License) != "" {
		return strings.TrimSpace(targetBook.License)
	}
	return defaultLicenseLabel
}

// FindScene looks up a Scene by ID, ExplicitID, or normalized Name.
func (targetBook *Book) FindScene(sceneID string) *Scene {
	if targetBook == nil || len(targetBook.Scenes) == 0 {
		return nil
	}
	trimmed := strings.TrimSpace(sceneID)
	if trimmed == "" {
		return nil
	}
	for _, sceneItem := range targetBook.Scenes {
		if strings.EqualFold(sceneItem.ID, trimmed) || (sceneItem.ExplicitID != "" && strings.EqualFold(sceneItem.ExplicitID, trimmed)) {
			return sceneItem
		}
	}
	norm := NormalizeLookupKey(trimmed)
	for _, sceneItem := range targetBook.Scenes {
		if norm != "" && NormalizeLookupKey(sceneItem.Name) == norm {
			return sceneItem
		}
	}
	return nil
}

// CurrentScene resolves the active Scene for save, falling back to StartSceneID or the first scene.
func (targetBook *Book) CurrentScene(save *SaveState) *Scene {
	if targetBook == nil || len(targetBook.Scenes) == 0 {
		return nil
	}
	if save != nil && save.CurrentSceneID != "" {
		if found := targetBook.FindScene(save.CurrentSceneID); found != nil {
			return found
		}
	}
	if targetBook.StartSceneID != "" {
		if found := targetBook.FindScene(targetBook.StartSceneID); found != nil {
			return found
		}
	}
	return targetBook.Scenes[0]
}

// ResolveValidSceneID returns preferredID if it exists in targetBook, or falls back to StartSceneID / first scene ID.
func (targetBook *Book) ResolveValidSceneID(preferredID string) string {
	if targetBook == nil || len(targetBook.Scenes) == 0 {
		return ""
	}
	if preferredID != "" && targetBook.FindScene(preferredID) != nil {
		return preferredID
	}
	if targetBook.StartSceneID != "" && targetBook.FindScene(targetBook.StartSceneID) != nil {
		return targetBook.StartSceneID
	}
	return targetBook.Scenes[0].ID
}

// EvaluateSceneChoices evaluates all choices of targetScene against variables.
func (targetBook *Book) EvaluateSceneChoices(targetScene *Scene, variables map[string]string, defaultNextLabel string) SceneChoicesEval {
	var eval SceneChoicesEval
	if targetScene == nil {
		eval.IsTerminal = true
		return eval
	}
	for choiceIdx, choice := range targetScene.Choices {
		if choice.IsEnding || strings.EqualFold(choice.TargetSceneID, TargetEnd) || strings.EqualFold(choice.Target, TargetEnd) {
			continue
		}
		condMet := true
		if choice.Condition != "" && EvaluateConditionHook != nil {
			condMet = EvaluateConditionHook(choice.Condition, variables, targetBook)
		}
		if !condMet && !choice.ShowWhenLocked {
			continue
		}
		item := choice
		item.Idx = choiceIdx
		item.IsCondUnmet = !condMet
		eval.Visible = append(eval.Visible, item)
		if condMet {
			eval.Unlocked = append(eval.Unlocked, item)
			if !item.IsEnding && !strings.EqualFold(item.TargetSceneID, TargetEnd) && !strings.EqualFold(item.Target, TargetEnd) {
				eval.UnlockedNonEnding = append(eval.UnlockedNonEnding, item)
			}
		}
	}
	hasMultiple := len(eval.Visible) > 1
	for idx := range eval.Visible {
		eval.Visible[idx].IsSingle = !hasMultiple
		if !hasMultiple && defaultNextLabel != "" {
			eval.Visible[idx].Label = defaultNextLabel
		}
	}
	if len(eval.Unlocked) == 1 {
		eval.SingleUnlocked = &eval.Unlocked[0]
	}
	eval.IsBranching = hasMultiple
	eval.IsTerminal = len(eval.Visible) == 0
	return eval
}

// IsImageless reports whether the book enforces imageless mode (explicitly or as a screenplay).
func (targetBook *Book) IsImageless() bool {
	return targetBook != nil && (targetBook.Imageless || targetBook.IsScreenplay())
}

// IsEffectiveImageless reports whether imageless mode is active for this book and settings.
func (targetBook *Book) IsEffectiveImageless(settings *AppSettings) bool {
	if targetBook.IsImageless() {
		return true
	}
	return settings != nil && settings.Imageless
}

// FindVariable looks up a declared VariableDef by name (case-insensitive).
func (targetBook *Book) FindVariable(varName string) *VariableDef {
	if targetBook == nil || len(targetBook.Variables) == 0 {
		return nil
	}
	clean := SanitizeVarName(varName)
	if clean == "" {
		return nil
	}
	for idx := range targetBook.Variables {
		if strings.EqualFold(SanitizeVarName(targetBook.Variables[idx].Name), clean) {
			return &targetBook.Variables[idx]
		}
	}
	return nil
}

// FindChapter returns the Chapter matching the given name or ID, or nil if not found.
func (targetBook *Book) FindChapter(nameOrID string) *Chapter {
	if targetBook == nil {
		return nil
	}
	key := strings.TrimSpace(nameOrID)
	if chRef, isCh := ParseChapterTarget(key); isCh {
		key = chRef
	}
	if key == "" {
		return nil
	}
	idKey := SanitizeSceneID(key)
	chSlug := SanitizeChapterID(key)
	for idx := range targetBook.Chapters {
		chap := &targetBook.Chapters[idx]
		if strings.EqualFold(strings.TrimSpace(chap.Name), key) ||
			(idKey != "" && (strings.EqualFold(chap.ExplicitID, idKey) || strings.EqualFold(chap.ID, idKey) || strings.EqualFold(SanitizeSceneID(chap.Name), idKey))) ||
			(chSlug != "" && (strings.EqualFold(SanitizeChapterID(chap.ExplicitID), chSlug) || strings.EqualFold(SanitizeChapterID(chap.ID), chSlug) || strings.EqualFold(SanitizeChapterID(chap.Name), chSlug))) {
			return chap
		}
	}
	return nil
}

// RootChapterKey returns the lowercase reference key of the book's starting chapter.
func (targetBook *Book) RootChapterKey() string {
	if targetBook == nil || len(targetBook.Scenes) == 0 {
		return ""
	}
	startID := targetBook.StartSceneID
	for _, sceneItem := range targetBook.Scenes {
		if strings.EqualFold(sceneItem.ID, startID) {
			if chap := targetBook.FindChapter(sceneItem.Chapter); chap != nil {
				return strings.ToLower(chap.ReferenceKey())
			}
			return strings.ToLower(SanitizeSceneID(sceneItem.Chapter))
		}
	}
	if len(targetBook.Chapters) > 0 {
		return strings.ToLower(targetBook.Chapters[0].ReferenceKey())
	}
	return strings.ToLower(SanitizeSceneID(targetBook.Scenes[0].Chapter))
}

// ChapterCanReach reports whether targetKey is reachable from startKey in a chapter DAG.
func ChapterCanReach(edges map[string][]string, startKey, targetKey string) bool {
	startKey = strings.ToLower(strings.TrimSpace(startKey))
	targetKey = strings.ToLower(strings.TrimSpace(targetKey))
	if startKey == "" || targetKey == "" {
		return false
	}
	if startKey == targetKey {
		return true
	}
	visited := map[string]bool{startKey: true}
	queue := []string{startKey}
	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]
		for _, nxt := range edges[curr] {
			if nxt == targetKey {
				return true
			}
			if !visited[nxt] {
				visited[nxt] = true
				queue = append(queue, nxt)
			}
		}
	}
	return false
}

// CanLinkChapters checks whether a chapter transition from fromChapter to toChapter is valid under one-way DAG rules.
func (targetBook *Book) CanLinkChapters(fromChapterNameOrID, toChapterNameOrID string) (bool, string) {
	if targetBook == nil {
		return false, errNoActiveBookPL
	}
	fromCh := targetBook.FindChapter(fromChapterNameOrID)
	toCh := targetBook.FindChapter(toChapterNameOrID)
	if fromCh == nil || toCh == nil {
		return false, errTargetChNotFoundPL
	}
	fromKey := strings.ToLower(strings.TrimSpace(fromCh.ID))
	toKey := strings.ToLower(strings.TrimSpace(toCh.ID))
	if fromKey == "" {
		fromKey = strings.ToLower(fromCh.ReferenceKey())
	}
	if toKey == "" {
		toKey = strings.ToLower(toCh.ReferenceKey())
	}
	if fromKey == toKey || strings.EqualFold(fromCh.Name, toCh.Name) {
		return false, errSelfChapterLinkPL
	}

	rootKey := targetBook.RootChapterKey()
	if rootKey != "" && toKey == rootKey {
		return false, errLinkToRootChapterPL
	}

	edges := make(map[string][]string, len(targetBook.Chapters))
	for _, sceneItem := range targetBook.Scenes {
		scCh := targetBook.FindChapter(sceneItem.Chapter)
		if scCh == nil {
			continue
		}
		srcKey := strings.ToLower(scCh.ReferenceKey())
		for _, choice := range sceneItem.Choices {
			if choice.IsEnding || !choice.IsChapterLink || choice.TargetChapterID == "" {
				continue
			}
			if dstCh := targetBook.FindChapter(choice.TargetChapterID); dstCh != nil {
				dstKey := strings.ToLower(dstCh.ReferenceKey())
				if srcKey != "" && dstKey != "" && srcKey != dstKey {
					edges[srcKey] = append(edges[srcKey], dstKey)
				}
			}
		}
	}

	if ChapterCanReach(edges, toKey, fromKey) {
		return false, errBackwardChapterLnkPL
	}
	return true, ""
}

// ChapterFileNameFor returns the target .playtxt filename for a chapter, using its ID primarily.
func (targetBook *Book) ChapterFileNameFor(chapterName, chapterID string) string {
	if cleanID := SanitizeChapterID(chapterID); cleanID != "" && cleanID != fallbackBookID {
		return cleanID + PlaytxtExt
	}
	if targetBook != nil {
		if chap := targetBook.FindChapter(chapterName); chap != nil {
			return chap.FileName()
		}
	}
	if cleanNameID := SanitizeChapterID(chapterName); cleanNameID != "" && cleanNameID != fallbackBookID {
		return cleanNameID + PlaytxtExt
	}
	return DefaultChapterFile
}

// EnsureChapterWithMeta ensures a Chapter entry exists in targetBook.Chapters with the given metadata.
func (targetBook *Book) EnsureChapterWithMeta(name, fallbackID, explicitID, sourceFile string) {
	cleanName := strings.TrimSpace(name)
	cleanExp := SanitizeChapterID(explicitID)
	if cleanExp == "" {
		cleanExp = SanitizeSceneID(explicitID)
	}
	cleanID := cleanExp
	if cleanID == "" {
		cleanID = SanitizeChapterID(fallbackID)
	}
	if cleanID == "" {
		cleanID = SanitizeChapterID(cleanName)
	}
	for idx := range targetBook.Chapters {
		chap := &targetBook.Chapters[idx]
		if (cleanName != "" && strings.EqualFold(chap.Name, cleanName)) ||
			(cleanExp != "" && (strings.EqualFold(chap.ExplicitID, cleanExp) || strings.EqualFold(chap.ID, cleanExp) || strings.EqualFold(chap.Name, cleanExp))) {
			if cleanName != "" && (chap.Name == "" || strings.EqualFold(chap.Name, chap.ExplicitID) || strings.EqualFold(chap.Name, chap.ID)) {
				chap.Name = cleanName
			}
			if cleanExp != "" {
				chap.ExplicitID = cleanExp
				chap.ID = cleanExp
			} else if chap.ID == "" && cleanID != "" {
				chap.ID = cleanID
			}
			if sourceFile != "" && chap.SourceFile == "" {
				chap.SourceFile = sourceFile
			}
			return
		}
	}
	targetBook.Chapters = append(targetBook.Chapters, Chapter{
		ID:         cleanID,
		ExplicitID: cleanExp,
		Name:       cleanName,
		SourceFile: sourceFile,
	})
}

// AddSceneToChapterWithMeta appends sceneID to the matching Chapter in targetBook.Chapters, creating the Chapter if needed.
func (targetBook *Book) AddSceneToChapterWithMeta(chapterName, explicitID, sourceFile, sceneID string) {
	cleanName := strings.TrimSpace(chapterName)
	cleanExp := SanitizeChapterID(explicitID)
	if cleanExp == "" {
		cleanExp = SanitizeSceneID(explicitID)
	}
	fallbackID := ""
	if sourceFile != "" && !strings.EqualFold(filepath.Base(sourceFile), DefaultStoryFile) && !strings.EqualFold(filepath.Base(sourceFile), BookDataFileName) {
		stem := strings.TrimSuffix(filepath.Base(sourceFile), filepath.Ext(sourceFile))
		fallbackID = SanitizeChapterID(stem)
	}
	cleanID := cleanExp
	if cleanID == "" {
		cleanID = fallbackID
	}
	if cleanID == "" {
		cleanID = SanitizeChapterID(cleanName)
	}

	for idx := range targetBook.Chapters {
		chap := &targetBook.Chapters[idx]
		if strings.EqualFold(chap.Name, cleanName) ||
			(cleanExp != "" && (strings.EqualFold(chap.ExplicitID, cleanExp) || strings.EqualFold(chap.ID, cleanExp))) {
			if cleanExp != "" && chap.ExplicitID == "" {
				chap.ExplicitID = cleanExp
				chap.ID = cleanExp
			}
			if chap.ID == "" && cleanID != "" {
				chap.ID = cleanID
			}
			if sourceFile != "" && chap.SourceFile == "" {
				chap.SourceFile = sourceFile
			}
			for _, existing := range chap.SceneIDs {
				if existing == sceneID {
					return
				}
			}
			chap.SceneIDs = append(chap.SceneIDs, sceneID)
			return
		}
	}
	targetBook.Chapters = append(targetBook.Chapters, Chapter{
		ID:         cleanID,
		ExplicitID: cleanExp,
		Name:       cleanName,
		SourceFile: sourceFile,
		SceneIDs:   []string{sceneID},
	})
}

// AppendSceneToChaptersFromMeta appends sceneItem to targetBook.Chapters using preserved metadata from prevChapterMeta.
func (targetBook *Book) AppendSceneToChaptersFromMeta(sceneItem *Scene, prevChapterMeta map[string]Chapter) {
	expID := sceneItem.ChapterID
	srcFile := sceneItem.SourceFile
	chKey := strings.ToLower(strings.TrimSpace(sceneItem.Chapter))
	if meta, found := prevChapterMeta[chKey]; found {
		if expID == "" {
			expID = meta.ExplicitID
		}
		if srcFile == "" {
			srcFile = meta.SourceFile
		}
	}
	targetBook.AddSceneToChapterWithMeta(sceneItem.Chapter, expID, srcFile, sceneItem.ID)
}

// ResolveChapterEntryPoints assigns StartSceneID on each Chapter in targetBook.Chapters.
func (targetBook *Book) ResolveChapterEntryPoints(prevChapterMeta map[string]Chapter, lookupScene func(sid string) *Scene, emitWarnings bool) {
	for idx := range targetBook.Chapters {
		chap := &targetBook.Chapters[idx]
		if chap.ID == "" {
			chap.ID = chap.ReferenceKey()
		}
		if chap.ExplicitStart == "" {
			for _, key := range []string{chap.Name, chap.ExplicitID, chap.ID} {
				if meta, found := prevChapterMeta[strings.ToLower(strings.TrimSpace(key))]; found && meta.ExplicitStart != "" {
					chap.ExplicitStart = meta.ExplicitStart
					break
				}
			}
		}
		if len(chap.SceneIDs) == 0 {
			continue
		}
		chap.StartSceneID = chap.SceneIDs[0]
		for _, sid := range chap.SceneIDs {
			if strings.EqualFold(sid, targetBook.StartSceneID) {
				chap.StartSceneID = targetBook.StartSceneID
				break
			}
		}
		if expStart := strings.TrimSpace(chap.ExplicitStart); expStart != "" {
			matched := ""
			normStart := NormalizeLookupKey(expStart)
			for _, sid := range chap.SceneIDs {
				sceneItem := lookupScene(sid)
				if sceneItem == nil {
					continue
				}
				if strings.EqualFold(sceneItem.ID, expStart) ||
					(sceneItem.ExplicitID != "" && strings.EqualFold(sceneItem.ExplicitID, expStart)) ||
					(normStart != "" && NormalizeLookupKey(sceneItem.Name) == normStart) {
					matched = sceneItem.ID
					break
				}
			}
			if matched != "" {
				chap.StartSceneID = matched
			} else if emitWarnings {
				targetBook.Warnings = append(targetBook.Warnings, fmt.Sprintf(errMissingChStartFmtPL, chap.DisplayName(), expStart))
			}
		}
	}
}

// ScreenplayBreakdown returns the cached screenplay breakdown for this book.
func (targetBook *Book) ScreenplayBreakdown() *ScreenplayBreakdown {
	if AnalyzeScreenplayHook != nil {
		return AnalyzeScreenplayHook(targetBook)
	}
	return &ScreenplayBreakdown{}
}

// ResolveExportDir returns the configured or default export directory for targetBook.
func (targetBook *Book) ResolveExportDir(settings *AppSettings) string {
	if ResolveExportDirHook != nil {
		return ResolveExportDirHook(targetBook, settings)
	}
	if targetBook != nil && strings.TrimSpace(targetBook.ExportDir) != "" {
		return strings.TrimSpace(targetBook.ExportDir)
	}
	return ""
}

// PersistExportDir updates the export directory for targetBook and persists it in settings.ini.
func (targetBook *Book) PersistExportDir(savesDir string, settings *AppSettings, dir string) {
	if PersistExportDirHook != nil {
		PersistExportDirHook(targetBook, savesDir, settings, dir)
		return
	}
	if targetBook != nil {
		targetBook.ExportDir = dir
	}
}

// ReloadFromDisk reloads the book from its RootPath or from booksDir while preserving external/export/ID pool metadata.
func (targetBook *Book) ReloadFromDisk(booksDir string, settings *AppSettings) *Book {
	if ReloadFromDiskHook != nil {
		return ReloadFromDiskHook(targetBook, booksDir, settings)
	}
	return targetBook
}

// IsPreview reports whether a read-only history revision preview is currently active.
func (history *BookHistory) IsPreview(studioBook *Book) bool {
	return history != nil && history.Preview != nil && history.Preview.RevisionID != "" && studioBook != nil && !studioBook.IsPacked
}

// ExitPreview exits read-only revision preview mode and restores the live book.
func (history *BookHistory) ExitPreview(booksDir string, settings *AppSettings, studioBook **Book) string {
	if HistoryExitPreviewHook != nil {
		return HistoryExitPreviewHook(history, booksDir, settings, studioBook)
	}
	if history != nil {
		history.Preview = nil
	}
	return ""
}

// InvalidateCache clears the mtime-cached parsed BookHistory snapshot metadata.
func (history *BookHistory) InvalidateCache() {
	if history == nil {
		return
	}
	history.CachedBookID = ""
	history.CachedModNano = 0
	history.CachedSize = 0
}

// RefreshFromDisk reloads history from disk when the underlying history file has changed.
func (history *BookHistory) RefreshFromDisk(studioBook *Book) *BookHistory {
	if HistoryRefreshFromDiskHook != nil {
		return HistoryRefreshFromDiskHook(history, studioBook)
	}
	return history
}

// EnsureBaseline ensures an initial baseline snapshot exists for studioBook.
func (history *BookHistory) EnsureBaseline(studioBook *Book, selectedSceneID string) {
	if HistoryEnsureBaselineHook != nil {
		HistoryEnsureBaselineHook(history, studioBook, selectedSceneID)
	}
}

// RecordStep records a new mutation snapshot in version history.
func (history *BookHistory) RecordStep(studioBook *Book, label, kind, coalesceKey, selectedSceneID string) {
	if HistoryRecordStepHook != nil {
		HistoryRecordStepHook(history, studioBook, label, kind, coalesceKey, selectedSceneID)
	}
}

// NormalizeRevisionBranch normalizes a history branch name, defaulting to "main".
func NormalizeRevisionBranch(rawBranch string) string {
	branchName := SanitizeBookID(strings.TrimSpace(rawBranch))
	if branchName == "" || branchName == fallbackBookID {
		return branchNameMain
	}
	return branchName
}

// NormalizedActiveBranch returns the sanitized active branch name, defaulting to "main".
func (history *BookHistory) NormalizedActiveBranch() string {
	if history == nil {
		return branchNameMain
	}
	return NormalizeRevisionBranch(history.ActiveBranch)
}

// ActiveBranchRevisions returns the slice indices in history.Revisions belonging to the active branch.
func (history *BookHistory) ActiveBranchRevisions() []int {
	if history == nil || len(history.Revisions) == 0 {
		return nil
	}
	activeBranch := history.NormalizedActiveBranch()
	var indices []int
	for idx := range history.Revisions {
		if NormalizeRevisionBranch(history.Revisions[idx].Branch) == activeBranch {
			indices = append(indices, idx)
		}
	}
	return indices
}

// SyncBranchesMetadata ensures main and the active branch exist in history.Branches and track history.Cursor.
func (history *BookHistory) SyncBranchesMetadata() {
	if history == nil {
		return
	}
	activeBranch := history.NormalizedActiveBranch()
	history.ActiveBranch = activeBranch

	hasMain := false
	for idx := range history.Branches {
		history.Branches[idx].Name = NormalizeRevisionBranch(history.Branches[idx].Name)
		if history.Branches[idx].Name == branchNameMain {
			hasMain = true
		}
	}
	if !hasMain && len(history.Revisions) > 0 {
		firstID := history.Revisions[0].ID
		headID := firstID
		for _, rev := range history.Revisions {
			if NormalizeRevisionBranch(rev.Branch) == branchNameMain {
				headID = rev.ID
			}
		}
		history.Branches = append([]HistoryBranch{{
			Name:          branchNameMain,
			HeadRevID:     headID,
			BaseRevID:     firstID,
			CreatedAtUnix: history.Revisions[0].TimeUnix,
		}}, history.Branches...)
	}
	if history.Cursor != "" {
		foundBranch := false
		for idx := range history.Branches {
			if history.Branches[idx].Name == activeBranch {
				history.Branches[idx].HeadRevID = history.Cursor
				foundBranch = true
				break
			}
		}
		if !foundBranch && len(history.Revisions) > 0 {
			history.Branches = append(history.Branches, HistoryBranch{
				Name:          activeBranch,
				HeadRevID:     history.Cursor,
				BaseRevID:     history.Cursor,
				CreatedAtUnix: time.Now().Unix(),
			})
		}
	}
}

// CursorIndex returns the index of the current cursor revision in history.Revisions, or -1 if empty.
func (history *BookHistory) CursorIndex() int {
	if history == nil || len(history.Revisions) == 0 {
		return -1
	}
	for idx := range history.Revisions {
		if history.Revisions[idx].ID == history.Cursor {
			return idx
		}
	}
	branchRevs := history.ActiveBranchRevisions()
	if len(branchRevs) > 0 {
		return branchRevs[len(branchRevs)-1]
	}
	return len(history.Revisions) - 1
}

// LastNamedIndex returns the index of the latest Named Version on the active branch, or -1 if none.
func (history *BookHistory) LastNamedIndex() int {
	if history == nil {
		return -1
	}
	branchRevs := history.ActiveBranchRevisions()
	for pos := len(branchRevs) - 1; pos >= 0; pos-- {
		revIdx := branchRevs[pos]
		if history.Revisions[revIdx].Named {
			return revIdx
		}
	}
	return -1
}

// CanUndo returns true if the cursor can step back on the active branch without crossing the latest Named Version floor.
func (history *BookHistory) CanUndo() bool {
	if history == nil || len(history.Revisions) == 0 {
		return false
	}
	branchRevs := history.ActiveBranchRevisions()
	if len(branchRevs) <= 1 {
		return false
	}
	curPos := len(branchRevs) - 1
	floorPos := 0
	for pos, revIdx := range branchRevs {
		if history.Revisions[revIdx].ID == history.Cursor {
			curPos = pos
		}
		if history.Revisions[revIdx].Named {
			floorPos = pos
		}
	}
	return curPos > floorPos
}

// CanRedo returns true if there is a newer revision after the current cursor on the active branch.
func (history *BookHistory) CanRedo() bool {
	if history == nil || len(history.Revisions) == 0 {
		return false
	}
	branchRevs := history.ActiveBranchRevisions()
	if len(branchRevs) <= 1 {
		return false
	}
	curPos := -1
	for pos, revIdx := range branchRevs {
		if history.Revisions[revIdx].ID == history.Cursor {
			curPos = pos
			break
		}
	}
	return curPos >= 0 && curPos < len(branchRevs)-1
}

// UnnamedEditsCount returns the number of active unnamed edits applied after the latest Named Version up to the cursor on the active branch.
func (history *BookHistory) UnnamedEditsCount() int {
	if history == nil || len(history.Revisions) == 0 {
		return 0
	}
	branchRevs := history.ActiveBranchRevisions()
	if len(branchRevs) == 0 {
		return 0
	}
	curPos := len(branchRevs) - 1
	floorPos := 0
	for pos, revIdx := range branchRevs {
		if history.Revisions[revIdx].ID == history.Cursor {
			curPos = pos
		}
		if history.Revisions[revIdx].Named {
			floorPos = pos
		}
	}
	if curPos <= floorPos {
		return 0
	}
	count := 0
	for pos := floorPos + 1; pos <= curPos && pos < len(branchRevs); pos++ {
		if !history.Revisions[branchRevs[pos]].Named {
			count++
		}
	}
	return count
}
