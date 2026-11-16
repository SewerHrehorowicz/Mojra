package main

import (
	"crypto/ed25519"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSignLicenseTokenAndKeypair(t *testing.T) {
	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, signKeyFileName)
	verifierPath := filepath.Join(tmpDir, "verifier.go")
	testFilePath := filepath.Join(tmpDir, "server_test.go")

	_ = os.WriteFile(verifierPath, []byte("package state\nconst (\n\tpublicVerifyKeyHex = \"old\"\n)\n"), 0644)
	_ = os.WriteFile(testFilePath, []byte("package domain\nconst testSignedLicenseKey = \"old\"\n"), 0644)

	if err := initKeypairAt(keyPath, verifierPath, testFilePath); err != nil {
		t.Fatalf("initKeypairAt failed: %v", err)
	}
	if err := initKeypairAt(keyPath, verifierPath, testFilePath); err == nil {
		t.Fatalf("expected error when sign.key already exists")
	}

	loadedHex := loadSigningKeyHex(keyPath)
	if len(loadedHex) != ed25519.PrivateKeySize*2 {
		t.Fatalf("expected 128-char hex key in sign.key, got %d chars", len(loadedHex))
	}

	seed := make([]byte, ed25519.SeedSize)
	for i := range seed {
		seed[i] = byte(i + 1)
	}
	privKey := ed25519.NewKeyFromSeed(seed)
	signHex := hex.EncodeToString(privKey)

	tokMax, err := SignLicenseToken(signHex, "Jan Kowalski", "jan@mojra.pl", "max", 0)
	if err != nil {
		t.Fatalf("SignLicenseToken max failed: %v", err)
	}
	if !strings.HasPrefix(tokMax, tokenPrefix) || !strings.Contains(tokMax, tokenDotSep) {
		t.Fatalf("unexpected token format: %q", tokMax)
	}

	licTxtPath := filepath.Join(tmpDir, licenseFileName)
	_ = os.WriteFile(licTxtPath, []byte(tokMax+"\n"), 0644)
	report, err := InspectLicenseToken(licTxtPath, signHex)
	if err != nil {
		t.Fatalf("InspectLicenseToken failed: %v", err)
	}
	if !strings.Contains(report, "SIGNATURE=VALID") || !strings.Contains(report, "HOLDER=Jan Kowalski") || !strings.Contains(report, "TIER=max") {
		t.Fatalf("unexpected InspectLicenseToken report:\n%s", report)
	}

	expUnix := time.Now().Add(24 * time.Hour).Unix()
	tokEdu, err := SignLicenseToken(hex.EncodeToString(seed), "Anna Nowak", "anna@mojra.pl", "edu", expUnix)
	if err != nil {
		t.Fatalf("SignLicenseToken edu from seed failed: %v", err)
	}
	if !strings.HasPrefix(tokEdu, tokenPrefix) {
		t.Fatalf("unexpected edu token format: %q", tokEdu)
	}

	if _, err := SignLicenseToken("", "Jan", "jan@mojra.pl", "max", 0); err == nil {
		t.Fatalf("expected error for empty signing key")
	}
	if _, err := SignLicenseToken(signHex, "", "jan@mojra.pl", "max", 0); err == nil {
		t.Fatalf("expected error for empty holder name")
	}
}
