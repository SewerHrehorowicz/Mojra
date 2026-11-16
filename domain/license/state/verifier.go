package state

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"strconv"
	"strings"
	"time"

	"mojra/domain/i18n"
)

const (
	publicVerifyKeyHex = "c8c65e1db86483a6ddc6b99870bbff2f2c737825ff74bbc095367e34d6c17421"
	tokenPrefix        = "MOJRA-"
)

// VerifyKey validates an Ed25519-signed license token and derives the 64-bit XOR UnlockMask.
func (lic *LicenseState) VerifyKey(rawKey string) bool {
	if lic == nil {
		return false
	}
	lic.HolderName = ""
	lic.Email = ""
	lic.Tier = ""
	lic.RawKey = ""
	lic.ExpiresAt = 0
	lic.UnlockMask = 0
	lic.ErrorKey = ""

	cleanKey := strings.TrimSpace(rawKey)
	if !strings.HasPrefix(cleanKey, tokenPrefix) {
		lic.ErrorKey = i18n.TrLicenseErrorInvalid
		return false
	}
	bodyPart := cleanKey[len(tokenPrefix):]
	dotIdx := strings.IndexByte(bodyPart, '.')
	if dotIdx <= 0 || dotIdx >= len(bodyPart)-1 {
		lic.ErrorKey = i18n.TrLicenseErrorInvalid
		return false
	}
	payloadBytes, errPayload := base64.RawURLEncoding.DecodeString(bodyPart[:dotIdx])
	sigBytes, errSig := base64.RawURLEncoding.DecodeString(bodyPart[dotIdx+1:])
	pubKeyBytes, errPub := hex.DecodeString(publicVerifyKeyHex)
	if errPayload != nil || errSig != nil || errPub != nil || len(pubKeyBytes) != ed25519.PublicKeySize || len(sigBytes) != ed25519.SignatureSize {
		lic.ErrorKey = i18n.TrLicenseErrorInvalid
		return false
	}
	if !ed25519.Verify(ed25519.PublicKey(pubKeyBytes), payloadBytes, sigBytes) {
		lic.ErrorKey = i18n.TrLicenseErrorInvalid
		return false
	}
	tokenMask, ok := parseSignedLicensePayload(lic, string(payloadBytes))
	if !ok {
		lic.ErrorKey = i18n.TrLicenseErrorInvalid
		return false
	}
	if lic.ExpiresAt > 0 && time.Now().Unix() > lic.ExpiresAt {
		lic.HolderName = ""
		lic.Email = ""
		lic.Tier = ""
		lic.ExpiresAt = 0
		lic.ErrorKey = i18n.TrLicenseErrorExpired
		return false
	}
	digest := sha256.Sum256(pubKeyBytes)
	pad := binary.LittleEndian.Uint64(digest[:8])
	unlockMask := tokenMask ^ pad
	if unlockMask == 0 {
		lic.HolderName = ""
		lic.Email = ""
		lic.Tier = ""
		lic.ExpiresAt = 0
		lic.ErrorKey = i18n.TrLicenseErrorInvalid
		return false
	}
	lic.RawKey = cleanKey
	lic.UnlockMask = unlockMask
	return true
}

func parseSignedLicensePayload(lic *LicenseState, rawPayload string) (uint64, bool) {
	fields := strings.Split(rawPayload, "|")
	if len(fields) != 5 {
		return 0, false
	}
	holderName := strings.TrimSpace(fields[0])
	email := strings.TrimSpace(fields[1])
	tier := strings.TrimSpace(fields[2])
	if holderName == "" || email == "" || tier == "" {
		return 0, false
	}
	expUnix, errExp := strconv.ParseInt(fields[3], 10, 64)
	if errExp != nil || expUnix < 0 {
		return 0, false
	}
	tokenMask, errMask := strconv.ParseUint(fields[4], 16, 64)
	if errMask != nil || tokenMask == 0 {
		return 0, false
	}
	lic.HolderName = holderName
	lic.Email = email
	lic.Tier = tier
	lic.ExpiresAt = expUnix
	return tokenMask, true
}

// IsActive reports whether a verified, non-expired Studio license is active.
func (lic *LicenseState) IsActive() bool {
	if lic == nil || lic.UnlockMask == 0 {
		return false
	}
	if lic.ExpiresAt > 0 && time.Now().Unix() > lic.ExpiresAt {
		return false
	}
	return true
}

// UnmaskInt decodes an execution constant using the shifted byte of UnlockMask.
func (lic *LicenseState) UnmaskInt(masked uint64, shift uint) int {
	if lic == nil {
		return int(masked & 0xFF)
	}
	maskByte := (lic.UnlockMask >> shift) & 0xFF
	return int((masked ^ maskByte) & 0xFF)
}
