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
)

const (
	saveMagic     = "STSV"
	saveVersion   = byte(1)
	layoutMagic   = "STLY"
	layoutVersion = byte(1)

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

// DefaultAppSettings returns default human-readable INI settings.
func DefaultAppSettings() *AppSettings {
	return &AppSettings{
		Theme:              "dark",
		PreviewMode:        "full",
		ActiveBookID:       "",
		ActiveView:         "library",
		Imageless:          false,
		UIScale:            "small",
		PlaybackSpeed:      1.0,
		AutoChoice:         false,
		AutoChoiceStrategy: "first",
	}
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
		case "display.preview_mode", ".preview_mode":
			if val == "portrait" || val == "landscape" || val == "full" {
				cfg.PreviewMode = val
			}
		case "display.active_book", ".active_book":
			cfg.ActiveBookID = SanitizeBookID(val)
		case "display.active_view", ".active_view":
			if val == "library" || val == "player" || val == "editor" {
				cfg.ActiveView = val
			}
		case "display.imageless", ".imageless":
			cfg.Imageless = strings.EqualFold(val, "true") || val == "1" || strings.EqualFold(val, "yes")
		case "display.ui_scale", ".ui_scale":
			if strings.EqualFold(val, "large") {
				cfg.UIScale = "large"
			} else {
				cfg.UIScale = "small"
			}
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
		case "library.book", "library.path", ".external_book":
			if val != "" {
				cfg.AddExternalBook(val)
			}
		case "library.new_book_dir", ".new_book_dir":
			cleanDir := filepath.Clean(strings.TrimSpace(val))
			if cleanDir != "" && cleanDir != "." {
				cfg.NewBookDir = cleanDir
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
	if cfg.PreviewMode != "portrait" && cfg.PreviewMode != "landscape" {
		cfg.PreviewMode = "full"
	}
	if cfg.ActiveView != "player" && cfg.ActiveView != "editor" {
		cfg.ActiveView = "library"
	}
	if cfg.UIScale != "large" {
		cfg.UIScale = "small"
	}
	if cfg.PlaybackSpeed < 0.5 || cfg.PlaybackSpeed > 3.0 {
		cfg.PlaybackSpeed = 1.0
	}
	if cfg.AutoChoiceStrategy != "random" {
		cfg.AutoChoiceStrategy = "first"
	}

	var sb strings.Builder
	sb.WriteString("; StoryTeller — Ustawienia aplikacji (format INI)\n\n")
	sb.WriteString("[display]\n")
	fmt.Fprintf(&sb, "theme = %s\n", cfg.Theme)
	fmt.Fprintf(&sb, "preview_mode = %s\n", cfg.PreviewMode)
	fmt.Fprintf(&sb, "imageless = %t\n", cfg.Imageless)
	fmt.Fprintf(&sb, "ui_scale = %s\n", cfg.UIScale)
	fmt.Fprintf(&sb, "active_book = %s\n", cfg.ActiveBookID)
	fmt.Fprintf(&sb, "active_view = %s\n\n", cfg.ActiveView)

	sb.WriteString("[audio]\n")
	fmt.Fprintf(&sb, "speed = %.2f\n", cfg.PlaybackSpeed)
	fmt.Fprintf(&sb, "auto_choice = %t\n", cfg.AutoChoice)
	fmt.Fprintf(&sb, "auto_strategy = %s\n", cfg.AutoChoiceStrategy)

	cleanNewBookDir := filepath.Clean(strings.TrimSpace(cfg.NewBookDir))
	if cleanNewBookDir == "." {
		cleanNewBookDir = ""
	}
	if len(cfg.ExternalBooks) > 0 || cleanNewBookDir != "" {
		sb.WriteString("\n[library]\n")
		if cleanNewBookDir != "" {
			fmt.Fprintf(&sb, "new_book_dir = %s\n", cleanNewBookDir)
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
	st := &SaveState{
		BookID:             safeID,
		CurrentSceneID:     defaultStartSceneID,
		AudioTime:          0,
		PlaybackSpeed:      1.0,
		AutoChoice:         false,
		AutoChoiceStrategy: "first",
		History:            []HistoryEntry{},
	}

	raw, err := os.ReadFile(saveFilePath(savesDir, safeID))
	if err != nil || len(raw) < 22 {
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

	if err := binary.Read(r, binary.LittleEndian, &st.UpdatedAtUnix); err != nil {
		return st
	}

	var speedBits uint64
	if err := binary.Read(r, binary.LittleEndian, &speedBits); err != nil {
		return st
	}
	st.PlaybackSpeed = math.Float64frombits(speedBits)
	if st.PlaybackSpeed < 0.5 || st.PlaybackSpeed > 3.0 {
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

	return st
}

// WriteSaveState atomically writes SaveState in binary format to saves/<book-id>.sav.
func WriteSaveState(savesDir string, state *SaveState) error {
	if err := os.MkdirAll(savesDir, 0755); err != nil {
		return err
	}
	state.BookID = SanitizeBookID(state.BookID)
	state.CurrentSceneID = SanitizePlainText(state.CurrentSceneID)
	if state.PlaybackSpeed < 0.5 || state.PlaybackSpeed > 3.0 {
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

	target := saveFilePath(savesDir, state.BookID)
	tmp := target + ".tmp"
	if err := os.WriteFile(tmp, buf.Bytes(), 0644); err != nil {
		return err
	}
	return os.Rename(tmp, target)
}

// UpdateSaveAudioTimeInPlace updates only the 8-byte float64 AudioTime and 8-byte UpdatedAt timestamp
// at fixed offset 5 inside saves/<book-id>.sav without rewriting the entire file.
func UpdateSaveAudioTimeInPlace(savesDir, bookID string, audioTime float64, defaultStartSceneID string) error {
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

// EncodeLayoutBinary serializes node studio timeline positions into binary .layout.dat format.
func EncodeLayoutBinary(layout map[string]NodePosition) []byte {
	var buf bytes.Buffer
	buf.WriteString(layoutMagic)
	buf.WriteByte(layoutVersion)

	keys := make([]string, 0, len(layout))
	for k := range layout {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	_ = binary.Write(&buf, binary.LittleEndian, uint32(len(keys)))
	for _, k := range keys {
		pos := layout[k]
		writeLengthPrefixedString(&buf, k)
		_ = binary.Write(&buf, binary.LittleEndian, int32(pos.Row))
		_ = binary.Write(&buf, binary.LittleEndian, int32(pos.Col))
	}
	return buf.Bytes()
}

// DecodeLayoutBinary parses binary .layout.dat bytes into map[string]NodePosition.
func DecodeLayoutBinary(raw []byte) map[string]NodePosition {
	out := make(map[string]NodePosition)
	if len(raw) < 9 {
		return out
	}
	r := bytes.NewReader(raw)
	magic := make([]byte, 4)
	if _, err := io.ReadFull(r, magic); err != nil || string(magic) != layoutMagic {
		return out
	}
	ver, err := r.ReadByte()
	if err != nil || ver != layoutVersion {
		return out
	}
	var count uint32
	if err := binary.Read(r, binary.LittleEndian, &count); err != nil || count > 10000 {
		return out
	}
	for i := uint32(0); i < count; i++ {
		id, err := readLengthPrefixedString(r)
		if err != nil {
			break
		}
		var row, col int32
		if err := binary.Read(r, binary.LittleEndian, &row); err != nil {
			break
		}
		if err := binary.Read(r, binary.LittleEndian, &col); err != nil {
			break
		}
		if id != "" && row > 0 && col > 0 {
			out[id] = NodePosition{Row: int(row), Col: int(col)}
		}
	}
	return out
}

func writeLengthPrefixedString(buf *bytes.Buffer, s string) {
	b := []byte(s)
	if len(b) > 65535 {
		b = b[:65535]
	}
	_ = binary.Write(buf, binary.LittleEndian, uint16(len(b)))
	buf.Write(b)
}

func readLengthPrefixedString(r io.Reader) (string, error) {
	var n uint16
	if err := binary.Read(r, binary.LittleEndian, &n); err != nil {
		return "", err
	}
	if n == 0 {
		return "", nil
	}
	b := make([]byte, n)
	if _, err := io.ReadFull(r, b); err != nil {
		return "", err
	}
	return string(b), nil
}
