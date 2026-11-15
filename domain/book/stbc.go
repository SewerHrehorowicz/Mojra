package book

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"strings"
)

const (
	stbcHeaderMagic  = "STBC\x01"
	stbcTrailerMagic = "STBCWASM"
	stbcTrailerSize  = 148 // 8 (offset) + 8 (len) + 32 (key) + 12 (nonce) + 16 (permSeed) + 64 (sig) + 8 (magic)
)

// Logical STBC record opcodes (permuted per build via random Fisher-Yates shuffle).
const (
	stbcOpBookMeta byte = iota
	stbcOpEnumDef
	stbcOpVarDef
	stbcOpChapter
	stbcOpScene
	stbcOpChoice
	stbcOpMedia
	stbcOpEnd
	stbcLogicalOpCount = 8
)

var stbcSigningDomain = []byte("MOJRA-STUDIO-STBC-SIGNING-KEY-V1")

func stbcKeypair() (ed25519.PublicKey, ed25519.PrivateKey) {
	seed := sha256.Sum256(stbcSigningDomain)
	priv := ed25519.NewKeyFromSeed(seed[:])
	pub := priv.Public().(ed25519.PublicKey)
	return pub, priv
}

// stbcOpcodeTable deterministically generates a 256-byte permutation from a 16-byte random per-build seed.
func stbcOpcodeTable(permSeed [16]byte) (encode [stbcLogicalOpCount]byte, decode [256]byte) {
	for i := range decode {
		decode[i] = 0xff
	}
	var pool [256]byte
	for i := 0; i < 256; i++ {
		pool[i] = byte(i)
	}
	state := sha256.Sum256(permSeed[:])
	for i := 255; i > 0; i-- {
		state = sha256.Sum256(state[:])
		j := int(binary.LittleEndian.Uint32(state[:4])) % (i + 1)
		pool[i], pool[j] = pool[j], pool[i]
	}
	for op := byte(0); op < stbcLogicalOpCount; op++ {
		phys := pool[op]
		encode[op] = phys
		decode[phys] = op
	}
	return encode, decode
}

func stbcSignatureDigest(ciphertext []byte, key [32]byte, nonce [12]byte, permSeed [16]byte) [32]byte {
	h := sha256.New()
	h.Write([]byte(stbcTrailerMagic))
	h.Write(key[:])
	h.Write(nonce[:])
	h.Write(permSeed[:])
	h.Write(ciphertext)
	var out [32]byte
	copy(out[:], h.Sum(nil))
	return out
}

// DecodeEncryptedSTBC verifies Ed25519 signature, decrypts AES-256-GCM payload, and decodes the permuted STBC bytecode.
func DecodeEncryptedSTBC(ciphertext []byte, key [32]byte, nonce [12]byte, permSeed [16]byte, sig [64]byte) (*Book, map[string][]byte, error) {
	pub, _ := stbcKeypair()
	msgHash := stbcSignatureDigest(ciphertext, key, nonce, permSeed)
	if !ed25519.Verify(pub, msgHash[:], sig[:]) {
		return nil, nil, fmt.Errorf("nieprawidłowy podpis kryptograficzny STBC (naruszona integralność binarki)")
	}

	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, err
	}
	plain, err := gcm.Open(nil, nonce[:], ciphertext, permSeed[:])
	if err != nil {
		return nil, nil, fmt.Errorf("błąd deszyfrowania strumienia STBC: %w", err)
	}

	if len(plain) < len(stbcHeaderMagic) || string(plain[:len(stbcHeaderMagic)]) != stbcHeaderMagic {
		return nil, nil, fmt.Errorf("uszkodzony nagłówek strumienia STBC")
	}

	_, decOps := stbcOpcodeTable(permSeed)
	r := bytes.NewReader(plain[len(stbcHeaderMagic):])

	b := &Book{
		IsPacked:       true,
		Layout:         make(map[string]NodePosition),
		PackedMedia:    make(map[string]PackedMediaEntry),
		deferProseHTML: true,
	}
	media := make(map[string][]byte)

	for r.Len() > 0 {
		physOp, err := r.ReadByte()
		if err != nil {
			return nil, nil, err
		}
		op := decOps[physOp]
		switch op {
		case stbcOpBookMeta:
			b.ID, _ = readSTBCString(r)
			b.Title, _ = readSTBCString(r)
			b.Author, _ = readSTBCString(r)
			b.License, _ = readSTBCString(r)
			b.Description, _ = readSTBCString(r)
			b.Mode, _ = readSTBCString(r)
			b.ScriptStandard, _ = readSTBCString(r)
			b.StartSceneID, _ = readSTBCString(r)
			b.CoverFile, _ = readSTBCString(r)
			flags, err := r.ReadByte()
			if err != nil {
				return nil, nil, err
			}
			b.Narrated = (flags & 1) != 0
			b.Imageless = (flags & 2) != 0
			if b.CoverFile != "" {
				b.CoverURL = encodeMediaURL(b.ID, b.CoverFile)
			}

		case stbcOpEnumDef:
			name, _ := readSTBCString(r)
			var count uint16
			if err := binary.Read(r, binary.LittleEndian, &count); err != nil {
				return nil, nil, err
			}
			vals := make([]string, 0, count)
			for i := uint16(0); i < count; i++ {
				v, _ := readSTBCString(r)
				vals = append(vals, v)
			}
			b.Enums = append(b.Enums, EnumDef{Name: name, Values: vals})

		case stbcOpVarDef:
			var vd VariableDef
			vd.Name, _ = readSTBCString(r)
			vd.Kind, _ = readSTBCString(r)
			bByte, _ := r.ReadByte()
			vd.BoolVal = bByte != 0
			var iv int64
			_ = binary.Read(r, binary.LittleEndian, &iv)
			vd.IntVal = int(iv)
			vd.EnumName, _ = readSTBCString(r)
			vd.EnumVal, _ = readSTBCString(r)
			var optCount uint16
			if err := binary.Read(r, binary.LittleEndian, &optCount); err != nil {
				return nil, nil, err
			}
			vd.Options = make([]string, 0, optCount)
			for i := uint16(0); i < optCount; i++ {
				opt, _ := readSTBCString(r)
				vd.Options = append(vd.Options, opt)
			}
			b.Variables = append(b.Variables, vd)

		case stbcOpChapter:
			var ch Chapter
			ch.ID, _ = readSTBCString(r)
			ch.ExplicitID, _ = readSTBCString(r)
			ch.Name, _ = readSTBCString(r)
			ch.StartSceneID, _ = readSTBCString(r)
			var sCount uint16
			if err := binary.Read(r, binary.LittleEndian, &sCount); err != nil {
				return nil, nil, err
			}
			ch.SceneIDs = make([]string, 0, sCount)
			for i := uint16(0); i < sCount; i++ {
				sid, _ := readSTBCString(r)
				ch.SceneIDs = append(ch.SceneIDs, sid)
			}
			b.Chapters = append(b.Chapters, ch)

		case stbcOpScene:
			sc := &Scene{}
			sc.ID, _ = readSTBCString(r)
			sc.ExplicitID, _ = readSTBCString(r)
			sc.Name, _ = readSTBCString(r)
			sc.Slugline, _ = readSTBCString(r)
			sc.Synopsis, _ = readSTBCString(r)
			sc.Effects, _ = readSTBCString(r)
			sc.Chapter, _ = readSTBCString(r)
			sc.ChapterID, _ = readSTBCString(r)
			sc.Text, _ = readSTBCString(r)
			sc.AudioFile, _ = readSTBCString(r)
			sc.ImageFile, _ = readSTBCString(r)
			var ord int32
			_ = binary.Read(r, binary.LittleEndian, &ord)
			sc.OrderIndex = int(ord)
			scFlags, _ := r.ReadByte()
			sc.ExplicitEnd = (scFlags & 1) != 0
			if (scFlags & 2) != 0 {
				var ts, te float32
				_ = binary.Read(r, binary.LittleEndian, &ts)
				_ = binary.Read(r, binary.LittleEndian, &te)
				sc.VoiceTrimStart = float64(ts)
				sc.VoiceTrimEnd = float64(te)
			}
			if sc.AudioFile != "" && b.Narrated {
				sc.AudioURL = encodeMediaURL(b.ID, sc.AudioFile)
			}
			if sc.ImageFile != "" {
				sc.ImageURL = encodeMediaURL(b.ID, sc.ImageFile)
			}
			var chCount uint16
			if err := binary.Read(r, binary.LittleEndian, &chCount); err != nil {
				return nil, nil, err
			}
			sc.Choices = make([]Choice, 0, chCount)
			for i := uint16(0); i < chCount; i++ {
				cPhys, err := r.ReadByte()
				if err != nil || decOps[cPhys] != stbcOpChoice {
					return nil, nil, fmt.Errorf("nieprawidłowy opcode wyboru w STBC")
				}
				var ch Choice
				ch.Label, _ = readSTBCString(r)
				ch.Target, _ = readSTBCString(r)
				ch.TargetSceneID, _ = readSTBCString(r)
				ch.TargetChapterID, _ = readSTBCString(r)
				ch.Condition, _ = readSTBCString(r)
				ch.Effects, _ = readSTBCString(r)
				cFlags, _ := r.ReadByte()
				ch.ShowWhenLocked = (cFlags & 1) != 0
				ch.IsChapterLink = (cFlags & 2) != 0
				ch.IsImplicit = (cFlags & 4) != 0
				ch.IsEnding = (cFlags & 8) != 0
				sc.Choices = append(sc.Choices, ch)
			}
			b.Scenes = append(b.Scenes, sc)

		case stbcOpMedia:
			rel, err := readSTBCString(r)
			if err != nil {
				return nil, nil, err
			}
			var mLen uint32
			if err := binary.Read(r, binary.LittleEndian, &mLen); err != nil {
				return nil, nil, err
			}
			if mLen > maxMediaEntryBytes {
				return nil, nil, fmt.Errorf("plik multimedialny STBC przekracza limit")
			}
			raw := make([]byte, mLen)
			if _, err := io.ReadFull(r, raw); err != nil {
				return nil, nil, err
			}
			if !validateMediaHeader(rel, raw) {
				return nil, nil, fmt.Errorf("uszkodzony nagłówek pliku multimedialnego w STBC: %s", rel)
			}
			media[rel] = raw
			var pmEntry PackedMediaEntry
			pmEntry.Size = int64(len(raw))
			hdrLen := len(raw)
			if hdrLen > len(pmEntry.Header512) {
				hdrLen = len(pmEntry.Header512)
			}
			copy(pmEntry.Header512[:], raw[:hdrLen])
			pmEntry.HeaderLen = hdrLen
			pmEntry.ValidHeader = true
			b.PackedMedia[strings.ToLower(rel)] = pmEntry
			if isAudioFilename(rel) {
				b.AudioFiles = append(b.AudioFiles, rel)
			} else if isRasterImageFilename(rel) {
				b.ImageFiles = append(b.ImageFiles, rel)
			}

		case stbcOpEnd:
			for _, sc := range b.Scenes {
				NormalizeSceneInlineSectionVoices(sc)
				ExtractSceneVoiceSections(sc, b)
			}
			return b, media, nil

		default:
			return nil, nil, fmt.Errorf("nieznana instrukcja w strumieniu STBC: 0x%02x", physOp)
		}
	}

	for _, sc := range b.Scenes {
		NormalizeSceneInlineSectionVoices(sc)
		ExtractSceneVoiceSections(sc, b)
	}
	return b, media, nil
}

// ExtractSTBCFromWasm reads and decrypts the embedded STBC book and media directly from a .wasm binary.
func ExtractSTBCFromWasm(wasmBytes []byte) (*Book, map[string][]byte, error) {
	if len(wasmBytes) < 8+stbcTrailerSize {
		return nil, nil, fmt.Errorf("binarka .wasm nie zawiera osadzonego bloku STBC")
	}
	trailer := wasmBytes[len(wasmBytes)-stbcTrailerSize:]
	if string(trailer[140:148]) != stbcTrailerMagic {
		return nil, nil, fmt.Errorf("brak sygnatury STBCWASM na końcu binarki .wasm")
	}
	payloadOffset := binary.LittleEndian.Uint64(trailer[0:8])
	payloadLen := binary.LittleEndian.Uint64(trailer[8:16])
	endOffset := uint64(len(wasmBytes) - stbcTrailerSize)
	if payloadOffset < 8 || payloadLen == 0 || payloadOffset+payloadLen != endOffset {
		return nil, nil, fmt.Errorf("uszkodzony zakres offsetu STBC w binarce .wasm")
	}

	var key [32]byte
	var nonce [12]byte
	var permSeed [16]byte
	var sig [64]byte
	copy(key[:], trailer[16:48])
	copy(nonce[:], trailer[48:60])
	copy(permSeed[:], trailer[60:76])
	copy(sig[:], trailer[76:140])

	ciphertext := wasmBytes[payloadOffset:endOffset]
	return DecodeEncryptedSTBC(ciphertext, key, nonce, permSeed, sig)
}

func readSTBCString(r *bytes.Reader) (string, error) {
	var n uint32
	if err := binary.Read(r, binary.LittleEndian, &n); err != nil {
		return "", err
	}
	if n > MaxZipSingleTextBytes {
		return "", fmt.Errorf("przekroczono limit długości łańcucha STBC")
	}
	return readBytesOfLen(r, int(n))
}
