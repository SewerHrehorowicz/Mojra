package book

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const betaTokenPrefix = "MOJRA-BETA1."

// BetaTokenClaims holds offline Closed Beta license metadata encoded in plain key=value lines (zero JSON).
type BetaTokenClaims struct {
	Holder        string
	ExpiresAtUnix int64
	Edition       string
}

// SignBetaAccessToken signs a Closed Beta access token using Ed25519 over plain key=value lines.
func SignBetaAccessToken(claims BetaTokenClaims, privKey ed25519.PrivateKey) (string, error) {
	if len(privKey) != ed25519.PrivateKeySize {
		return "", fmt.Errorf("nieprawidłowy klucz prywatny Ed25519")
	}
	holder := strings.ReplaceAll(strings.ReplaceAll(strings.TrimSpace(claims.Holder), "\r", ""), "\n", "")
	edition := strings.TrimSpace(claims.Edition)
	if edition == "" {
		edition = "closed-beta"
	}
	payload := fmt.Sprintf("holder=%s\nexpires=%d\nedition=%s", holder, claims.ExpiresAtUnix, edition)
	payloadBytes := []byte(payload)
	sig := ed25519.Sign(privKey, payloadBytes)
	return betaTokenPrefix +
		base64.RawURLEncoding.EncodeToString(payloadBytes) + "." +
		base64.RawURLEncoding.EncodeToString(sig), nil
}

// VerifyBetaAccessToken verifies an Ed25519-signed Closed Beta token and checks expiration against now.
func VerifyBetaAccessToken(token string, pubKey ed25519.PublicKey, now time.Time) (BetaTokenClaims, error) {
	var claims BetaTokenClaims
	trimmed := strings.TrimSpace(token)
	if !strings.HasPrefix(trimmed, betaTokenPrefix) {
		return claims, fmt.Errorf("nieprawidłowy prefiks klucza bety")
	}
	if len(pubKey) != ed25519.PublicKeySize {
		return claims, fmt.Errorf("nieprawidłowy klucz publiczny Ed25519")
	}
	rest := strings.TrimPrefix(trimmed, betaTokenPrefix)
	dot := strings.IndexByte(rest, '.')
	if dot <= 0 || dot >= len(rest)-1 {
		return claims, fmt.Errorf("nieprawidłowy format klucza bety")
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(rest[:dot])
	if err != nil {
		return claims, fmt.Errorf("błąd dekodowania ładunku klucza bety: %w", err)
	}
	sigBytes, err := base64.RawURLEncoding.DecodeString(rest[dot+1:])
	if err != nil || len(sigBytes) != ed25519.SignatureSize {
		return claims, fmt.Errorf("nieprawidłowy podpis klucza bety")
	}
	if !ed25519.Verify(pubKey, payloadBytes, sigBytes) {
		return claims, fmt.Errorf("weryfikacja podpisu Ed25519 klucza bety nie powiodła się")
	}

	for _, line := range strings.Split(string(payloadBytes), "\n") {
		eq := strings.IndexByte(line, '=')
		if eq <= 0 {
			continue
		}
		k := strings.TrimSpace(line[:eq])
		v := strings.TrimSpace(line[eq+1:])
		switch k {
		case "holder":
			claims.Holder = v
		case "expires":
			if ts, pErr := strconv.ParseInt(v, 10, 64); pErr == nil {
				claims.ExpiresAtUnix = ts
			}
		case "edition":
			claims.Edition = v
		}
	}
	if claims.ExpiresAtUnix <= 0 {
		return claims, fmt.Errorf("brak daty wygaśnięcia w kluczu bety")
	}
	if !now.IsZero() && now.Unix() > claims.ExpiresAtUnix {
		return claims, fmt.Errorf("klucz dostępu Closed Beta wygasł (%s)", time.Unix(claims.ExpiresAtUnix, 0).UTC().Format("2006-01-02"))
	}
	return claims, nil
}

// CheckBetaBuildExpiry validates an optional build expiry timestamp (RFC3339 or YYYY-MM-DD) or Ed25519 signed token.
func CheckBetaBuildExpiry(expirySpec, signedToken, pubKeyHex string, now time.Time) error {
	spec := strings.TrimSpace(expirySpec)
	tok := strings.TrimSpace(signedToken)
	if spec == "" && tok == "" {
		return nil
	}
	if now.IsZero() {
		now = time.Now()
	}
	if tok != "" && strings.TrimSpace(pubKeyHex) != "" {
		pubBytes, err := hex.DecodeString(strings.TrimSpace(pubKeyHex))
		if err != nil {
			return fmt.Errorf("błąd klucza publicznego bety: %w", err)
		}
		if _, err := VerifyBetaAccessToken(tok, ed25519.PublicKey(pubBytes), now); err == nil {
			return nil
		} else if spec == "" {
			return err
		}
	}
	if spec != "" {
		expTime, err := time.Parse(time.RFC3339, spec)
		if err != nil {
			expTime, err = time.Parse("2006-01-02", spec)
			if err != nil {
				return fmt.Errorf("nieprawidłowy znacznik wygaśnięcia buildu: %w", err)
			}
			expTime = expTime.Add(24*time.Hour - time.Second)
		}
		if now.After(expTime) {
			return fmt.Errorf("ten build testowy Closed Beta wygasł %s", expTime.UTC().Format("2006-01-02"))
		}
	}
	return nil
}
