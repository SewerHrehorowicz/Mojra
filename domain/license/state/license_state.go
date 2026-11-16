package state

import (
	"os"
	"path/filepath"
	"strings"

	commonstate "mojra/domain/common/state"
	"mojra/engine"
	"mojra/engine/osbridge"
)

const (
	licenseFileName = "license.txt"
	legacyFileName  = "license.ini"
)

// LicenseState holds the in-memory verification status and 64-bit XOR unlock mask for Playbook Studio.
type LicenseState struct {
	*commonstate.UIState
	HolderName           string
	Email                string
	Tier                 string
	RawKey               string
	ExpiresAt            int64
	UnlockMask           uint64
	ErrorKey             engine.TrKey
	ShowActivatedSummary bool
}

// BindUI links LicenseState to the shared UIState and loads any persisted key from license.txt (in RootDir or SavesDir).
func (lic *LicenseState) BindUI(ui *commonstate.UIState) {
	lic.UIState = ui
	if ui == nil {
		return
	}
	candidates := []string{
		filepath.Join(ui.Config.RootDir, licenseFileName),
		filepath.Join(ui.Config.SavesDir, licenseFileName),
		filepath.Join(ui.Config.SavesDir, legacyFileName),
	}
	for _, path := range candidates {
		if path == "" || path == licenseFileName {
			continue
		}
		if lic.LoadKeyFromFile(path) {
			_ = lic.SaveKeyToDisk(lic.RawKey)
			return
		}
	}
}

// LoadKeyFromFile reads a license.txt file from disk, extracts the MOJRA-... token, and verifies it.
func (lic *LicenseState) LoadKeyFromFile(filePath string) bool {
	if lic == nil || filePath == "" {
		return false
	}
	rawBytes, err := osbridge.DefaultStorage.ReadBytes(filePath)
	if err != nil || len(rawBytes) == 0 {
		return false
	}
	for _, rawLine := range strings.Split(string(rawBytes), "\n") {
		idx := strings.Index(rawLine, tokenPrefix)
		if idx >= 0 {
			savedKey := strings.TrimSpace(rawLine[idx:])
			if savedKey != "" {
				return lic.VerifyKey(savedKey)
			}
		}
	}
	return false
}

// SaveKeyToDisk persists the license key atomically to saves/license.txt.
func (lic *LicenseState) SaveKeyToDisk(rawKey string) error {
	if lic == nil || lic.UIState == nil || lic.Config.SavesDir == "" {
		return nil
	}
	if err := os.MkdirAll(lic.Config.SavesDir, 0755); err != nil {
		return err
	}
	cleanKey := strings.TrimSpace(rawKey)
	if cleanKey == "" && lic.Config.RootDir != "" {
		_ = os.Remove(filepath.Join(lic.Config.RootDir, licenseFileName))
	}
	targetPath := filepath.Join(lic.Config.SavesDir, licenseFileName)
	return osbridge.DefaultStorage.WriteAtomic(targetPath, []byte(cleanKey+"\n"))
}
