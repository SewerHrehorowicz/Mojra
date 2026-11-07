package book

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"mojra/internal/tpl"
)

const (
	saveMagic   = "STSV"
	saveVersion = byte(1)

	// Byte offset of float64 AudioTime inside .sav header ("STSV" + version byte = offset 5)
	audioTimeOffset = 5
)

func saveFilePath(savesDir, bookID string) string {
	safeID := SanitizeBookID(bookID)
	return filepath.Join(savesDir, safeID+".sav")
}

func settingsIniPath(savesDir string) string {
	return filepath.Join(savesDir, "settings.ini")
}

// NormalizeLanguage normalizes a UI language code to one of the registered languages in tpl.SupportedLanguages.
func NormalizeLanguage(raw string) string {
	return tpl.NormalizeLangCode(raw)
}

const DefaultScrollSensitivity = "medium"

// NormalizeScrollSensitivity normalizes scroll sensitivity to "high", "medium", "low", or "off".
func NormalizeScrollSensitivity(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "high", "wysoka", "fast":
		return "high"
	case "low", "niska", "slow":
		return "low"
	case "off", "none", "false", "0", "disabled", "wyl", "wył":
		return "off"
	default:
		return DefaultScrollSensitivity
	}
}

// DefaultAppSettings returns default human-readable INI settings.
func DefaultAppSettings() *AppSettings {
	return &AppSettings{
		Theme:              "dark",
		Language:           "pl",
		PreviewMode:        "full",
		ActiveBookID:       "",
		ActiveView:         "library",
		Imageless:          false,
		UIScale:            "small",
		ScrollSensitivity:  DefaultScrollSensitivity,
		PlaybackSpeed:      1.0,
		AutoChoice:         false,
		AutoChoiceStrategy: "first",
		AutoChoiceDelay:    DefaultAutoChoiceDelay,
		BookExportDirs:     make(map[string]string),
	}
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

// LoadAppSettings parses saves/settings.ini.
func LoadAppSettings(savesDir string) *AppSettings {
	cfg := DefaultAppSettings()
	raw, err := os.ReadFile(settingsIniPath(savesDir))
	if err != nil {
		return cfg
	}

	section := ""
	lines := strings.Split(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, ";") || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			section = strings.ToLower(strings.TrimSpace(trimmed[1 : len(trimmed)-1]))
			continue
		}
		idx := strings.IndexByte(trimmed, '=')
		if idx <= 0 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(trimmed[:idx]))
		val := strings.TrimSpace(trimmed[idx+1:])

		switch section + "." + key {
		case "display.theme", ".theme":
			if val == "light" {
				cfg.Theme = "light"
			} else {
				cfg.Theme = "dark"
			}
		case "display.lang", "display.language", ".lang", ".language":
			cfg.Language = NormalizeLanguage(val)
		case "display.preview_mode", ".preview_mode":
			switch val {
			case "phone", "portrait":
				cfg.PreviewMode = "portrait"
			case "tablet", "ipad":
				cfg.PreviewMode = "tablet"
			case "landscape":
				cfg.PreviewMode = "landscape"
			case "full", "desktop":
				cfg.PreviewMode = "full"
			}
		case "display.active_book", ".active_book":
			cfg.ActiveBookID = SanitizeBookID(val)
		case "display.editor_book", ".editor_book":
			cfg.EditorBookID = SanitizeBookID(val)
		case "display.active_view", ".active_view":
			if val == "library" || val == "player" || val == "editor" {
				cfg.ActiveView = val
			}
		case "display.imageless", ".imageless":
			cfg.Imageless = strings.EqualFold(val, "true") || val == "1" || strings.EqualFold(val, "yes")
		case "display.ui_scale", ".ui_scale":
			if strings.EqualFold(val, "large") {
				cfg.UIScale = "large"
			} else if strings.EqualFold(val, "zen") {
				cfg.UIScale = "zen"
			} else {
				cfg.UIScale = "small"
			}
		case "display.scroll_sensitivity", ".scroll_sensitivity":
			cfg.ScrollSensitivity = NormalizeScrollSensitivity(val)
		case "audio.speed", ".speed":
			if f, err := strconv.ParseFloat(val, 64); err == nil && f >= 0.5 && f <= 3.0 {
				cfg.PlaybackSpeed = f
			}
		case "audio.auto_choice", ".auto_choice":
			cfg.AutoChoice = strings.EqualFold(val, "true") || val == "1" || strings.EqualFold(val, "yes")
		case "audio.auto_strategy", ".auto_strategy":
			if val == "random" {
				cfg.AutoChoiceStrategy = "random"
			} else {
				cfg.AutoChoiceStrategy = "first"
			}
		case "audio.auto_delay", ".auto_delay":
			if f, err := strconv.ParseFloat(val, 64); err == nil {
				cfg.AutoChoiceDelay = NormalizeAutoChoiceDelay(f)
			}
		case "library.book", "library.path", ".external_book":
			if val != "" {
				cfg.AddExternalBook(val)
			}
		case "library.new_book_dir", ".new_book_dir":
			cleanDir := filepath.Clean(strings.TrimSpace(val))
			if cleanDir != "" && cleanDir != "." {
				cfg.NewBookDir = cleanDir
			}
		case "library.export_dir", ".export_dir":
			cleanDir := filepath.Clean(strings.TrimSpace(val))
			if cleanDir != "" && cleanDir != "." {
				cfg.ExportDir = cleanDir
			}
		default:
			if strings.HasPrefix(key, "book_export.") {
				bID := strings.TrimPrefix(key, "book_export.")
				cfg.SetBookExportDir(bID, val)
			}
		}
	}
	return cfg
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

// SaveAppSettings writes human-readable saves/settings.ini atomically.
func SaveAppSettings(savesDir string, cfg *AppSettings) error {
	if err := os.MkdirAll(savesDir, 0755); err != nil {
		return err
	}
	if cfg.Theme != "light" {
		cfg.Theme = "dark"
	}
	cfg.Language = NormalizeLanguage(cfg.Language)
	switch cfg.PreviewMode {
	case "phone":
		cfg.PreviewMode = "portrait"
	case "ipad":
		cfg.PreviewMode = "tablet"
	case "portrait", "tablet", "landscape":
	default:
		cfg.PreviewMode = "full"
	}
	if cfg.ActiveView != "player" && cfg.ActiveView != "editor" {
		cfg.ActiveView = "library"
	}
	if cfg.UIScale != "large" && cfg.UIScale != "zen" {
		cfg.UIScale = "small"
	}
	cfg.ScrollSensitivity = NormalizeScrollSensitivity(cfg.ScrollSensitivity)
	if math.IsNaN(cfg.PlaybackSpeed) || math.IsInf(cfg.PlaybackSpeed, 0) || cfg.PlaybackSpeed < 0.5 || cfg.PlaybackSpeed > 3.0 {
		cfg.PlaybackSpeed = 1.0
	}
	if cfg.AutoChoiceStrategy != "random" {
		cfg.AutoChoiceStrategy = "first"
	}
	cfg.AutoChoiceDelay = NormalizeAutoChoiceDelay(cfg.AutoChoiceDelay)

	var sb strings.Builder
	sb.WriteString("; Mojra — Ustawienia aplikacji (format INI)\n\n")
	sb.WriteString("[display]\n")
	fmt.Fprintf(&sb, "theme = %s\n", cfg.Theme)
	fmt.Fprintf(&sb, "lang = %s\n", cfg.Language)
	fmt.Fprintf(&sb, "preview_mode = %s\n", cfg.PreviewMode)
	fmt.Fprintf(&sb, "imageless = %t\n", cfg.Imageless)
	fmt.Fprintf(&sb, "ui_scale = %s\n", cfg.UIScale)
	fmt.Fprintf(&sb, "scroll_sensitivity = %s\n", cfg.ScrollSensitivity)
	fmt.Fprintf(&sb, "active_book = %s\n", cfg.ActiveBookID)
	if cfg.EditorBookID != "" {
		fmt.Fprintf(&sb, "editor_book = %s\n", cfg.EditorBookID)
	}
	fmt.Fprintf(&sb, "active_view = %s\n\n", cfg.ActiveView)

	sb.WriteString("[audio]\n")
	fmt.Fprintf(&sb, "speed = %.2f\n", cfg.PlaybackSpeed)
	fmt.Fprintf(&sb, "auto_choice = %t\n", cfg.AutoChoice)
	fmt.Fprintf(&sb, "auto_strategy = %s\n", cfg.AutoChoiceStrategy)
	fmt.Fprintf(&sb, "auto_delay = %.1f\n", cfg.AutoChoiceDelay)

	cleanNewBookDir := filepath.Clean(strings.TrimSpace(cfg.NewBookDir))
	if cleanNewBookDir == "." {
		cleanNewBookDir = ""
	}
	cleanExportDir := filepath.Clean(strings.TrimSpace(cfg.ExportDir))
	if cleanExportDir == "." {
		cleanExportDir = ""
	}
	if len(cfg.ExternalBooks) > 0 || cleanNewBookDir != "" || cleanExportDir != "" || len(cfg.BookExportDirs) > 0 {
		sb.WriteString("\n[library]\n")
		if cleanNewBookDir != "" {
			fmt.Fprintf(&sb, "new_book_dir = %s\n", cleanNewBookDir)
		}
		if cleanExportDir != "" {
			fmt.Fprintf(&sb, "export_dir = %s\n", cleanExportDir)
		}
		if len(cfg.BookExportDirs) > 0 {
			expKeys := make([]string, 0, len(cfg.BookExportDirs))
			for k, v := range cfg.BookExportDirs {
				cleanK := SanitizeBookID(k)
				cleanV := filepath.Clean(strings.TrimSpace(v))
				if cleanK != "" && cleanV != "" && cleanV != "." {
					expKeys = append(expKeys, cleanK)
				}
			}
			sort.Strings(expKeys)
			for _, k := range expKeys {
				fmt.Fprintf(&sb, "book_export.%s = %s\n", k, filepath.Clean(strings.TrimSpace(cfg.BookExportDirs[k])))
			}
		}
		seen := make(map[string]bool, len(cfg.ExternalBooks))
		for _, p := range cfg.ExternalBooks {
			clean := filepath.Clean(strings.TrimSpace(p))
			if clean == "" || clean == "." || seen[clean] {
				continue
			}
			seen[clean] = true
			fmt.Fprintf(&sb, "book = %s\n", clean)
		}
	}

	target := settingsIniPath(savesDir)
	tmp := target + ".tmp"
	if err := os.WriteFile(tmp, []byte(sb.String()), 0644); err != nil {
		return err
	}
	return os.Rename(tmp, target)
}

// LoadSaveState reads the binary SaveState from saves/<book-id>.sav.
func LoadSaveState(savesDir, bookID string, defaultStartSceneID string) *SaveState {
	safeID := SanitizeBookID(bookID)
	raw, err := os.ReadFile(saveFilePath(savesDir, safeID))
	if err != nil {
		return DecodeSaveStateBytes(safeID, nil, defaultStartSceneID)
	}
	return DecodeSaveStateBytes(safeID, raw, defaultStartSceneID)
}

// DecodeSaveStateBytes decodes a binary SaveState (STSV\x01) from raw bytes.
func DecodeSaveStateBytes(bookID string, raw []byte, defaultStartSceneID string) *SaveState {
	safeID := SanitizeBookID(bookID)
	st := &SaveState{
		BookID:             safeID,
		CurrentSceneID:     defaultStartSceneID,
		AudioTime:          0,
		PlaybackSpeed:      1.0,
		AutoChoice:         false,
		AutoChoiceStrategy: "first",
		AutoChoiceDelay:    DefaultAutoChoiceDelay,
		History:            []HistoryEntry{},
	}

	if len(raw) < 22 {
		return st
	}

	r := bytes.NewReader(raw)
	magic := make([]byte, 4)
	if _, err := io.ReadFull(r, magic); err != nil || string(magic) != saveMagic {
		return st
	}
	ver, err := r.ReadByte()
	if err != nil || ver != saveVersion {
		return st
	}

	var audioBits uint64
	if err := binary.Read(r, binary.LittleEndian, &audioBits); err != nil {
		return st
	}
	st.AudioTime = math.Float64frombits(audioBits)
	if math.IsNaN(st.AudioTime) || math.IsInf(st.AudioTime, 0) || st.AudioTime < 0 {
		st.AudioTime = 0
	}

	if err := binary.Read(r, binary.LittleEndian, &st.UpdatedAtUnix); err != nil {
		return st
	}

	var speedBits uint64
	if err := binary.Read(r, binary.LittleEndian, &speedBits); err != nil {
		return st
	}
	st.PlaybackSpeed = math.Float64frombits(speedBits)
	if math.IsNaN(st.PlaybackSpeed) || math.IsInf(st.PlaybackSpeed, 0) || st.PlaybackSpeed < 0.5 || st.PlaybackSpeed > 3.0 {
		st.PlaybackSpeed = 1.0
	}

	flags, err := r.ReadByte()
	if err != nil {
		return st
	}
	st.AutoChoice = (flags & 1) != 0
	if (flags & 2) != 0 {
		st.AutoChoiceStrategy = "random"
	} else {
		st.AutoChoiceStrategy = "first"
	}

	curScene, err := readLengthPrefixedString(r)
	if err == nil && curScene != "" {
		st.CurrentSceneID = curScene
	}

	var histCount uint32
	if err := binary.Read(r, binary.LittleEndian, &histCount); err != nil || histCount > 10000 {
		return st
	}

	st.History = make([]HistoryEntry, 0, histCount)
	for i := uint32(0); i < histCount; i++ {
		var step uint32
		if err := binary.Read(r, binary.LittleEndian, &step); err != nil {
			break
		}
		sceneID, _ := readLengthPrefixedString(r)
		sceneName, _ := readLengthPrefixedString(r)
		chapter, _ := readLengthPrefixedString(r)
		choiceLabel, _ := readLengthPrefixedString(r)
		targetSceneID, _ := readLengthPrefixedString(r)
		st.History = append(st.History, HistoryEntry{
			Step:          int(step),
			SceneID:       sceneID,
			SceneName:     sceneName,
			Chapter:       chapter,
			ChoiceLabel:   choiceLabel,
			TargetSceneID: targetSceneID,
		})
	}

	if r.Len() >= 4 {
		var varCount uint32
		if err := binary.Read(r, binary.LittleEndian, &varCount); err == nil && varCount <= 4096 {
			if varCount > 0 {
				st.Variables = make(map[string]string, varCount)
			}
			for i := uint32(0); i < varCount; i++ {
				k, err1 := readLengthPrefixedString(r)
				v, err2 := readLengthPrefixedString(r)
				if err1 != nil || err2 != nil {
					break
				}
				if k != "" {
					st.Variables[k] = v
				}
			}
		}
	}
	if r.Len() >= 4 {
		var histVarSteps uint32
		if err := binary.Read(r, binary.LittleEndian, &histVarSteps); err == nil && int(histVarSteps) <= len(st.History) {
			for i := uint32(0); i < histVarSteps; i++ {
				var stepVarCount uint32
				if err := binary.Read(r, binary.LittleEndian, &stepVarCount); err != nil || stepVarCount > 4096 {
					break
				}
				if stepVarCount > 0 {
					st.History[i].Variables = make(map[string]string, stepVarCount)
					for j := uint32(0); j < stepVarCount; j++ {
						k, err1 := readLengthPrefixedString(r)
						v, err2 := readLengthPrefixedString(r)
						if err1 != nil || err2 != nil {
							break
						}
						if k != "" {
							st.History[i].Variables[k] = v
						}
					}
				}
			}
		}
	}

	return st
}

// EncodeSaveStateBytes serializes SaveState into binary STSV\x01 format in memory.
func EncodeSaveStateBytes(state *SaveState) []byte {
	if state == nil {
		return nil
	}
	state.BookID = SanitizeBookID(state.BookID)
	state.CurrentSceneID = SanitizePlainText(state.CurrentSceneID)
	if math.IsNaN(state.AudioTime) || math.IsInf(state.AudioTime, 0) || state.AudioTime < 0 {
		state.AudioTime = 0
	}
	if math.IsNaN(state.PlaybackSpeed) || math.IsInf(state.PlaybackSpeed, 0) || state.PlaybackSpeed < 0.5 || state.PlaybackSpeed > 3.0 {
		state.PlaybackSpeed = 1.0
	}
	if state.AutoChoiceStrategy != "random" {
		state.AutoChoiceStrategy = "first"
	}
	state.UpdatedAtUnix = time.Now().Unix()

	var buf bytes.Buffer
	buf.WriteString(saveMagic)
	buf.WriteByte(saveVersion)

	_ = binary.Write(&buf, binary.LittleEndian, math.Float64bits(state.AudioTime))
	_ = binary.Write(&buf, binary.LittleEndian, state.UpdatedAtUnix)
	_ = binary.Write(&buf, binary.LittleEndian, math.Float64bits(state.PlaybackSpeed))

	var flags byte
	if state.AutoChoice {
		flags |= 1
	}
	if state.AutoChoiceStrategy == "random" {
		flags |= 2
	}
	buf.WriteByte(flags)

	writeLengthPrefixedString(&buf, state.CurrentSceneID)
	_ = binary.Write(&buf, binary.LittleEndian, uint32(len(state.History)))

	for i, h := range state.History {
		_ = binary.Write(&buf, binary.LittleEndian, uint32(i+1))
		writeLengthPrefixedString(&buf, SanitizePlainText(h.SceneID))
		writeLengthPrefixedString(&buf, SanitizePlainText(h.SceneName))
		writeLengthPrefixedString(&buf, SanitizePlainText(h.Chapter))
		writeLengthPrefixedString(&buf, SanitizePlainText(h.ChoiceLabel))
		writeLengthPrefixedString(&buf, SanitizePlainText(h.TargetSceneID))
	}

	writeVariablesMapBinary(&buf, state.Variables)
	_ = binary.Write(&buf, binary.LittleEndian, uint32(len(state.History)))
	for _, h := range state.History {
		writeVariablesMapBinary(&buf, h.Variables)
	}
	return buf.Bytes()
}

// WriteSaveState atomically writes SaveState in binary format to saves/<book-id>.sav.
func WriteSaveState(savesDir string, state *SaveState) error {
	if err := os.MkdirAll(savesDir, 0755); err != nil {
		return err
	}
	raw := EncodeSaveStateBytes(state)
	target := saveFilePath(savesDir, state.BookID)
	tmp := target + ".tmp"
	if err := os.WriteFile(tmp, raw, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, target)
}

// UpdateSaveAudioTimeInPlace updates only the 8-byte float64 AudioTime and 8-byte UpdatedAt timestamp
// at fixed offset 5 inside saves/<book-id>.sav without rewriting the entire file.
func UpdateSaveAudioTimeInPlace(savesDir, bookID string, audioTime float64, defaultStartSceneID string) error {
	if math.IsNaN(audioTime) || math.IsInf(audioTime, 0) || audioTime < 0 {
		audioTime = 0
	}
	path := saveFilePath(savesDir, bookID)
	f, err := os.OpenFile(path, os.O_RDWR, 0644)
	if err != nil {
		st := LoadSaveState(savesDir, bookID, defaultStartSceneID)
		st.AudioTime = audioTime
		return WriteSaveState(savesDir, st)
	}
	defer f.Close()

	var patch [16]byte
	binary.LittleEndian.PutUint64(patch[0:8], math.Float64bits(audioTime))
	binary.LittleEndian.PutUint64(patch[8:16], uint64(time.Now().Unix()))
	_, err = f.WriteAt(patch[:], audioTimeOffset)
	return err
}

// DeleteSaveState removes saves/<book-id>.sav (and any legacy .save.json if present).
func DeleteSaveState(savesDir, bookID string) error {
	safeID := SanitizeBookID(bookID)
	_ = os.Remove(filepath.Join(savesDir, safeID+".save.json"))
	err := os.Remove(saveFilePath(savesDir, safeID))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func writeVariablesMapBinary(buf *bytes.Buffer, vars map[string]string) {
	if len(vars) == 0 {
		_ = binary.Write(buf, binary.LittleEndian, uint32(0))
		return
	}
	keys := make([]string, 0, len(vars))
	for k := range vars {
		if k != "" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	_ = binary.Write(buf, binary.LittleEndian, uint32(len(keys)))
	for _, k := range keys {
		writeLengthPrefixedString(buf, k)
		writeLengthPrefixedString(buf, vars[k])
	}
}

func writeLengthPrefixedString(buf *bytes.Buffer, s string) {
	b := []byte(s)
	if len(b) > 65535 {
		b = b[:65535]
	}
	_ = binary.Write(buf, binary.LittleEndian, uint16(len(b)))
	buf.Write(b)
}

func readBytesOfLen(r io.Reader, n int) (string, error) {
	if n <= 0 {
		return "", nil
	}
	b := make([]byte, n)
	if _, err := io.ReadFull(r, b); err != nil {
		return "", err
	}
	return string(b), nil
}

func readLengthPrefixedString(r io.Reader) (string, error) {
	var n uint16
	if err := binary.Read(r, binary.LittleEndian, &n); err != nil {
		return "", err
	}
	return readBytesOfLen(r, int(n))
}
