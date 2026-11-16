package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	flagInit          = "init"
	flagInspect       = "inspect"
	flagName          = "name"
	flagEmail         = "email"
	flagTier          = "tier"
	flagDays          = "days"
	flagOut           = "out"
	descInit          = "Generate cmd/keygen/sign.key if missing and sync verifier key"
	descInspect       = "Inspect a leaked license token or license.txt file and print holder info"
	descName          = "License holder full name"
	descEmail         = "License holder email"
	descTier          = "License tier (pro, max, edu)"
	descDays          = "Validity in days (0 = lifetime)"
	descOut           = "Output path for generated license.txt"
	envSignKey        = "MOJRA_SIGN_KEY"
	goModFile         = "go.mod"
	keygenSubDir      = "cmd/keygen"
	signKeyFileName   = "sign.key"
	licenseFileName   = "license.txt"
	verifierRelPath   = "domain/license/state/verifier.go"
	serverTestRelPath = "domain/server_test.go"
	verifyConstPrefix = "\tpublicVerifyKeyHex = \""
	testKeyPrefix     = "const testSignedLicenseKey = \""
	quoteSuffix       = "\""
	testHolderName    = "Jan Kowalski"
	testHolderEmail   = "jan@mojra.pl"
	tierMax           = "max"
	tierYearly        = "yearly"
	tokenPrefix       = "MOJRA-"
	tokenDotSep       = "."
	hexMaskFmt        = "%016x"
	payloadJoinFmt    = "%s|%s|%s|%d|%s"
	outKeyPathFmt     = "KEY_FILE=%s\n"
	outVerifyKeyFmt   = "VERIFY_KEY_HEX=%s\n"
	outLicenseFileFmt = "LICENSE_FILE=%s\n"
	outTokenFmt       = "%s\n"
	outInspectFmt     = "SIGNATURE=%s\nHOLDER=%s\nEMAIL=%s\nTIER=%s\nEXPIRES=%s\n"
	sigStatusValid    = "VALID"
	sigStatusInvalid  = "INVALID"
	errKeyExistsMsg   = "cmd/keygen/sign.key already exists; remove it manually before generating a new keypair"
	errMissingKeyMsg  = "missing cmd/keygen/sign.key (copy from backup or run with --init)"
	errInvalidKeyMsg  = "invalid Ed25519 signing key hex"
	errMissingHolder  = "holder name and email are required"
	errInvalidToken   = "invalid or unreadable license token"
	studioUnlockSeed  = uint64(0x4D4F4A5241535455)
)

func main() {
	initFlag := flag.Bool(flagInit, false, descInit)
	inspectTarget := flag.String(flagInspect, "", descInspect)
	holderName := flag.String(flagName, "", descName)
	email := flag.String(flagEmail, "", descEmail)
	tier := flag.String(flagTier, tierMax, descTier)
	days := flag.Int(flagDays, 0, descDays)
	outFile := flag.String(flagOut, "", descOut)
	flag.Parse()

	rootDir := resolveRepoRoot()
	keyPath := filepath.Join(rootDir, filepath.FromSlash(keygenSubDir), signKeyFileName)

	if *initFlag {
		verifierPath := filepath.Join(rootDir, filepath.FromSlash(verifierRelPath))
		testFilePath := filepath.Join(rootDir, filepath.FromSlash(serverTestRelPath))
		if err := initKeypairAt(keyPath, verifierPath, testFilePath); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
		return
	}

	signKeyHex := loadSigningKeyHex(keyPath)
	if strings.TrimSpace(*inspectTarget) != "" {
		report, err := InspectLicenseToken(*inspectTarget, signKeyHex)
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
		fmt.Print(report)
		return
	}

	var expiresAt int64
	validDays := *days
	if validDays == 0 && strings.EqualFold(strings.TrimSpace(*tier), tierYearly) {
		validDays = 365
	}
	if validDays > 0 {
		expiresAt = time.Now().Add(time.Duration(validDays) * 24 * time.Hour).Unix()
	}
	signedToken, err := SignLicenseToken(signKeyHex, *holderName, *email, *tier, expiresAt)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	targetOutPath := strings.TrimSpace(*outFile)
	if targetOutPath == "" {
		targetOutPath = filepath.Join(rootDir, filepath.FromSlash(keygenSubDir), licenseFileName)
	}
	if writeErr := os.WriteFile(targetOutPath, []byte(signedToken+"\n"), 0644); writeErr == nil {
		fmt.Printf(outLicenseFileFmt, targetOutPath)
	}
	fmt.Printf(outTokenFmt, signedToken)
}

func resolveRepoRoot() string {
	curDir, err := os.Getwd()
	if err != nil {
		return "."
	}
	for step := 0; step < 6; step++ {
		if info, statErr := os.Stat(filepath.Join(curDir, goModFile)); statErr == nil && !info.IsDir() {
			return curDir
		}
		parentDir := filepath.Dir(curDir)
		if parentDir == curDir {
			break
		}
		curDir = parentDir
	}
	return "."
}

func loadSigningKeyHex(keyPath string) string {
	if rawBytes, err := os.ReadFile(keyPath); err == nil {
		if trimmed := strings.TrimSpace(string(rawBytes)); trimmed != "" {
			return trimmed
		}
	}
	return strings.TrimSpace(os.Getenv(envSignKey))
}

func initKeypairAt(keyPath, verifierPath, testFilePath string) error {
	if _, err := os.Stat(keyPath); err == nil {
		return errors.New(errKeyExistsMsg)
	}
	pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(keyPath), 0755); err != nil {
		return err
	}
	signHex := hex.EncodeToString(privKey)
	verifyHex := hex.EncodeToString(pubKey)
	if err := os.WriteFile(keyPath, []byte(signHex+"\n"), 0600); err != nil {
		return err
	}
	if verifierPath != "" {
		_ = replacePrefixedLineInFile(verifierPath, verifyConstPrefix, verifyConstPrefix+verifyHex+quoteSuffix)
	}
	if testFilePath != "" {
		if testTok, signErr := SignLicenseToken(signHex, testHolderName, testHolderEmail, tierMax, 0); signErr == nil {
			_ = replacePrefixedLineInFile(testFilePath, testKeyPrefix, testKeyPrefix+testTok+quoteSuffix)
		}
	}
	fmt.Printf(outKeyPathFmt, keyPath)
	fmt.Printf(outVerifyKeyFmt, verifyHex)
	return nil
}

func replacePrefixedLineInFile(filePath, linePrefix, replacementLine string) error {
	rawBytes, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}
	lines := strings.Split(string(rawBytes), "\n")
	changed := false
	for idx, line := range lines {
		if strings.HasPrefix(line, linePrefix) {
			lines[idx] = replacementLine
			changed = true
			break
		}
	}
	if !changed {
		return nil
	}
	return os.WriteFile(filePath, []byte(strings.Join(lines, "\n")), 0644)
}

// InspectLicenseToken decodes a MOJRA-... token or license.txt file and verifies its Ed25519 signature.
func InspectLicenseToken(rawInput, signKeyHex string) (string, error) {
	candidate := strings.TrimSpace(rawInput)
	if !strings.HasPrefix(candidate, tokenPrefix) {
		if fileBytes, err := os.ReadFile(candidate); err == nil {
			for _, rawLine := range strings.Split(string(fileBytes), "\n") {
				if idx := strings.Index(rawLine, tokenPrefix); idx >= 0 {
					candidate = strings.TrimSpace(rawLine[idx:])
					break
				}
			}
		}
	}
	if !strings.HasPrefix(candidate, tokenPrefix) {
		return "", errors.New(errInvalidToken)
	}
	bodyPart := candidate[len(tokenPrefix):]
	dotIdx := strings.IndexByte(bodyPart, '.')
	if dotIdx <= 0 || dotIdx >= len(bodyPart)-1 {
		return "", errors.New(errInvalidToken)
	}
	payloadBytes, errPayload := base64.RawURLEncoding.DecodeString(bodyPart[:dotIdx])
	sigBytes, errSig := base64.RawURLEncoding.DecodeString(bodyPart[dotIdx+1:])
	if errPayload != nil || errSig != nil {
		return "", errors.New(errInvalidToken)
	}
	fields := strings.Split(string(payloadBytes), "|")
	if len(fields) != 5 {
		return "", errors.New(errInvalidToken)
	}
	sigStatus := sigStatusInvalid
	if pubKey := resolvePublicKeyFromHex(signKeyHex); len(pubKey) == ed25519.PublicKeySize && len(sigBytes) == ed25519.SignatureSize {
		if ed25519.Verify(pubKey, payloadBytes, sigBytes) {
			sigStatus = sigStatusValid
		}
	}
	return fmt.Sprintf(outInspectFmt, sigStatus, fields[0], fields[1], fields[2], fields[3]), nil
}

func resolvePublicKeyFromHex(signKeyHex string) ed25519.PublicKey {
	rawKeyBytes, err := hex.DecodeString(strings.TrimSpace(signKeyHex))
	if err != nil {
		return nil
	}
	switch len(rawKeyBytes) {
	case ed25519.SeedSize:
		return ed25519.NewKeyFromSeed(rawKeyBytes).Public().(ed25519.PublicKey)
	case ed25519.PrivateKeySize:
		return ed25519.PrivateKey(rawKeyBytes).Public().(ed25519.PublicKey)
	default:
		return nil
	}
}

// SignLicenseToken signs a 5-field license payload containing the 64-bit XOR studio unlock mask.
func SignLicenseToken(signKeyHex, holderName, email, tier string, expiresAt int64) (string, error) {
	if signKeyHex == "" {
		return "", errors.New(errMissingKeyMsg)
	}
	rawKeyBytes, err := hex.DecodeString(signKeyHex)
	if err != nil {
		return "", errors.New(errInvalidKeyMsg)
	}
	var privKey ed25519.PrivateKey
	switch len(rawKeyBytes) {
	case ed25519.SeedSize:
		privKey = ed25519.NewKeyFromSeed(rawKeyBytes)
	case ed25519.PrivateKeySize:
		privKey = ed25519.PrivateKey(rawKeyBytes)
	default:
		return "", errors.New(errInvalidKeyMsg)
	}
	cleanName := strings.TrimSpace(holderName)
	cleanEmail := strings.TrimSpace(email)
	if cleanName == "" || cleanEmail == "" {
		return "", errors.New(errMissingHolder)
	}
	cleanTier := strings.ToLower(strings.TrimSpace(tier))
	if cleanTier == "" {
		cleanTier = tierMax
	}
	pubKey := privKey.Public().(ed25519.PublicKey)
	digest := sha256.Sum256(pubKey)
	pad := binary.LittleEndian.Uint64(digest[:8])
	tokenMask := pad ^ studioUnlockSeed
	maskHex := fmt.Sprintf(hexMaskFmt, tokenMask)
	payload := fmt.Sprintf(payloadJoinFmt, cleanName, cleanEmail, cleanTier, expiresAt, maskHex)
	signature := ed25519.Sign(privKey, []byte(payload))
	payloadB64 := base64.RawURLEncoding.EncodeToString([]byte(payload))
	sigB64 := base64.RawURLEncoding.EncodeToString(signature)
	return tokenPrefix + payloadB64 + tokenDotSep + sigB64, nil
}
