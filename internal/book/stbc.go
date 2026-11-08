package book

import (
	"archive/zip"
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
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

// CollectBookMediaBytes reads all validated audio and raster image files referenced by or present in b.
func CollectBookMediaBytes(b *Book) (map[string][]byte, error) {
	out := make(map[string][]byte)
	if b == nil {
		return out, nil
	}
	var totalBytes int64
	addFile := func(rel string) error {
		clean := SanitizeRelativeMediaPath(rel)
		if clean == "" {
			return nil
		}
		if _, exists := out[clean]; exists {
			return nil
		}
		if !b.Narrated && isAudioFilename(clean) {
			return nil
		}
		if !isAudioFilename(clean) && !isRasterImageFilename(clean) {
			return nil
		}
		data, err := readBookMediaRawBytes(b, clean)
		if err != nil {
			return nil
		}
		totalBytes += int64(len(data))
		if totalBytes > MaxZipTotalUncompressedBytes {
			return fmt.Errorf("łączny rozmiar multimediów przekracza limit (500 MB)")
		}
		out[clean] = data
		return nil
	}

	if b.CoverFile != "" {
		if err := addFile(b.CoverFile); err != nil {
			return nil, err
		}
	}
	for _, img := range b.ImageFiles {
		if err := addFile(img); err != nil {
			return nil, err
		}
	}
	if b.Narrated {
		for _, aud := range discoverBookAudioFilesForSections(b) {
			if err := addFile(aud); err != nil {
				return nil, err
			}
		}
	}
	for _, sc := range b.Scenes {
		if sc.ImageFile != "" {
			if err := addFile(sc.ImageFile); err != nil {
				return nil, err
			}
		}
		if b.Narrated {
			if sc.AudioFile != "" {
				if err := addFile(sc.AudioFile); err != nil {
					return nil, err
				}
			}
			for _, sec := range ExtractSceneVoiceSections(sc, b) {
				if sec.AudioFile != "" {
					if err := addFile(sec.AudioFile); err != nil {
						return nil, err
					}
				}
			}
		}
	}
	return out, nil
}

func readBookMediaRawBytes(b *Book, cleanRel string) ([]byte, error) {
	if b == nil || b.RootPath == "" {
		return nil, os.ErrNotExist
	}
	if !b.IsPacked {
		full := filepath.Join(b.RootPath, filepath.FromSlash(cleanRel))
		if !IsSafePathInsideRoot(b.RootPath, full) {
			return nil, os.ErrPermission
		}
		f, err := os.Open(full)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		data, err := io.ReadAll(io.LimitReader(f, maxMediaEntryBytes+1))
		if err != nil {
			return nil, err
		}
		if int64(len(data)) > maxMediaEntryBytes || !validateMediaHeader(cleanRel, data) {
			return nil, fmt.Errorf("nieprawidłowy plik multimedialny: %s", cleanRel)
		}
		return data, nil
	}

	zr, err := zip.OpenReader(b.RootPath)
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	if f := findZipEntryByRelPath(zr.File, cleanRel); f != nil {
		var dummy int64
		return readValidatedZipMediaBytes(f, cleanRel, &dummy)
	}
	return nil, os.ErrNotExist
}

// CompileEncryptedSTBC compiles a Book and its media into a polymorphic STBC binary stream,
// encrypts it with a per-build random AES-256-GCM key, and signs it with Ed25519.
func CompileEncryptedSTBC(b *Book, media map[string][]byte) (ciphertext []byte, key [32]byte, nonce [12]byte, permSeed [16]byte, sig [64]byte, err error) {
	if b == nil {
		return nil, key, nonce, permSeed, sig, fmt.Errorf("brak książki do kompilacji STBC")
	}
	if media == nil {
		media, err = CollectBookMediaBytes(b)
		if err != nil {
			return nil, key, nonce, permSeed, sig, err
		}
	}

	if _, err = io.ReadFull(rand.Reader, key[:]); err != nil {
		return nil, key, nonce, permSeed, sig, err
	}
	if _, err = io.ReadFull(rand.Reader, nonce[:]); err != nil {
		return nil, key, nonce, permSeed, sig, err
	}
	if _, err = io.ReadFull(rand.Reader, permSeed[:]); err != nil {
		return nil, key, nonce, permSeed, sig, err
	}

	encOps, _ := stbcOpcodeTable(permSeed)
	var plain bytes.Buffer
	plain.WriteString(stbcHeaderMagic)

	// 1. Book Meta
	plain.WriteByte(encOps[stbcOpBookMeta])
	writeSTBCString(&plain, b.ID)
	writeSTBCString(&plain, b.Title)
	writeSTBCString(&plain, b.Author)
	writeSTBCString(&plain, b.License)
	writeSTBCString(&plain, b.Description)
	writeSTBCString(&plain, b.NormalizedMode())
	writeSTBCString(&plain, b.NormalizedStandard())
	writeSTBCString(&plain, b.StartSceneID)
	writeSTBCString(&plain, b.CoverFile)
	var flags byte
	if b.Narrated {
		flags |= 1
	}
	if b.Imageless {
		flags |= 2
	}
	plain.WriteByte(flags)

	// 2. Enums
	for _, ed := range b.Enums {
		plain.WriteByte(encOps[stbcOpEnumDef])
		writeSTBCString(&plain, ed.Name)
		_ = binary.Write(&plain, binary.LittleEndian, uint16(len(ed.Values)))
		for _, v := range ed.Values {
			writeSTBCString(&plain, v)
		}
	}

	// 3. Variables
	for _, vd := range b.Variables {
		plain.WriteByte(encOps[stbcOpVarDef])
		writeSTBCString(&plain, vd.Name)
		writeSTBCString(&plain, vd.Kind)
		if vd.BoolVal {
			plain.WriteByte(1)
		} else {
			plain.WriteByte(0)
		}
		_ = binary.Write(&plain, binary.LittleEndian, int64(vd.IntVal))
		writeSTBCString(&plain, vd.EnumName)
		writeSTBCString(&plain, vd.EnumVal)
		_ = binary.Write(&plain, binary.LittleEndian, uint16(len(vd.Options)))
		for _, opt := range vd.Options {
			writeSTBCString(&plain, opt)
		}
	}

	// 4. Chapters
	for _, ch := range b.Chapters {
		plain.WriteByte(encOps[stbcOpChapter])
		writeSTBCString(&plain, ch.ID)
		writeSTBCString(&plain, ch.ExplicitID)
		writeSTBCString(&plain, ch.Name)
		writeSTBCString(&plain, ch.StartSceneID)
		_ = binary.Write(&plain, binary.LittleEndian, uint16(len(ch.SceneIDs)))
		for _, sid := range ch.SceneIDs {
			writeSTBCString(&plain, sid)
		}
	}

	// 5. Scenes & Choices
	for _, sc := range b.Scenes {
		plain.WriteByte(encOps[stbcOpScene])
		writeSTBCString(&plain, sc.ID)
		writeSTBCString(&plain, sc.ExplicitID)
		writeSTBCString(&plain, sc.Name)
		writeSTBCString(&plain, sc.Slugline)
		writeSTBCString(&plain, sc.Synopsis)
		writeSTBCString(&plain, sc.Effects)
		writeSTBCString(&plain, sc.Chapter)
		writeSTBCString(&plain, sc.ChapterID)
		writeSTBCString(&plain, SerializeSceneProseWithSectionVoices(sc, b))
		writeSTBCString(&plain, sc.AudioFile)
		writeSTBCString(&plain, sc.ImageFile)
		_ = binary.Write(&plain, binary.LittleEndian, int32(sc.OrderIndex))
		var scFlags byte
		if sc.ExplicitEnd {
			scFlags |= 1
		}
		hasTrim := b.Narrated && sc.HasVoiceTrim()
		if hasTrim {
			scFlags |= 2
		}
		plain.WriteByte(scFlags)
		if hasTrim {
			ts, te := sc.NormalizedVoiceTrim()
			_ = binary.Write(&plain, binary.LittleEndian, float32(ts))
			_ = binary.Write(&plain, binary.LittleEndian, float32(te))
		}
		_ = binary.Write(&plain, binary.LittleEndian, uint16(len(sc.Choices)))
		for _, ch := range sc.Choices {
			plain.WriteByte(encOps[stbcOpChoice])
			writeSTBCString(&plain, ch.Label)
			writeSTBCString(&plain, ch.Target)
			writeSTBCString(&plain, ch.TargetSceneID)
			writeSTBCString(&plain, ch.TargetChapterID)
			writeSTBCString(&plain, ch.Condition)
			writeSTBCString(&plain, ch.Effects)
			var cFlags byte
			if ch.ShowWhenLocked {
				cFlags |= 1
			}
			if ch.IsChapterLink {
				cFlags |= 2
			}
			if ch.IsImplicit {
				cFlags |= 4
			}
			if ch.IsEnding {
				cFlags |= 8
			}
			plain.WriteByte(cFlags)
		}
	}

	// 6. Media files (sorted by path)
	mediaKeys := make([]string, 0, len(media))
	for k := range media {
		mediaKeys = append(mediaKeys, k)
	}
	sort.Strings(mediaKeys)
	for _, rel := range mediaKeys {
		raw := media[rel]
		if len(raw) == 0 {
			continue
		}
		plain.WriteByte(encOps[stbcOpMedia])
		writeSTBCString(&plain, rel)
		_ = binary.Write(&plain, binary.LittleEndian, uint32(len(raw)))
		plain.Write(raw)
	}

	// 7. End marker
	plain.WriteByte(encOps[stbcOpEnd])

	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, key, nonce, permSeed, sig, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, key, nonce, permSeed, sig, err
	}
	ciphertext = gcm.Seal(nil, nonce[:], plain.Bytes(), permSeed[:])

	_, priv := stbcKeypair()
	msgHash := stbcSignatureDigest(ciphertext, key, nonce, permSeed)
	signature := ed25519.Sign(priv, msgHash[:])
	copy(sig[:], signature)
	return ciphertext, key, nonce, permSeed, sig, nil
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

// EmbedSTBCIntoWasm appends a Wasm custom section ("mojra_stbc") containing the encrypted STBC payload
// and the 148-byte cryptographic trailer directly into a compiled .wasm binary.
func EmbedSTBCIntoWasm(baseWasm []byte, b *Book, media map[string][]byte) ([]byte, error) {
	if len(baseWasm) < 8 || !bytes.Equal(baseWasm[:4], []byte("\x00asm")) {
		return nil, fmt.Errorf("nieprawidłowy nagłówek pliku .wasm")
	}
	baseWasm = StripEmbeddedSTBCFromWasm(baseWasm)

	ciphertext, key, nonce, permSeed, sig, err := CompileEncryptedSTBC(b, media)
	if err != nil {
		return nil, err
	}

	secName := "mojra_stbc"
	var secHeader bytes.Buffer
	writeULEB128(&secHeader, uint64(len(secName)))
	secHeader.WriteString(secName)

	secPayloadLen := secHeader.Len() + len(ciphertext) + stbcTrailerSize
	var customSec bytes.Buffer
	customSec.WriteByte(0x00) // Wasm custom section ID = 0
	writeULEB128(&customSec, uint64(secPayloadLen))
	customSec.Write(secHeader.Bytes())

	payloadOffset := uint64(len(baseWasm) + customSec.Len())
	customSec.Write(ciphertext)

	var trailer [stbcTrailerSize]byte
	binary.LittleEndian.PutUint64(trailer[0:8], payloadOffset)
	binary.LittleEndian.PutUint64(trailer[8:16], uint64(len(ciphertext)))
	copy(trailer[16:48], key[:])
	copy(trailer[48:60], nonce[:])
	copy(trailer[60:76], permSeed[:])
	copy(trailer[76:140], sig[:])
	copy(trailer[140:148], stbcTrailerMagic)
	customSec.Write(trailer[:])

	out := make([]byte, 0, len(baseWasm)+customSec.Len())
	out = append(out, baseWasm...)
	out = append(out, customSec.Bytes()...)
	return out, nil
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

// StripEmbeddedSTBCFromWasm removes an existing embedded STBC custom section if present, returning the clean base .wasm.
func StripEmbeddedSTBCFromWasm(wasmBytes []byte) []byte {
	if len(wasmBytes) < 8+stbcTrailerSize {
		return wasmBytes
	}
	trailer := wasmBytes[len(wasmBytes)-stbcTrailerSize:]
	if string(trailer[140:148]) != stbcTrailerMagic {
		return wasmBytes
	}
	payloadOffset := binary.LittleEndian.Uint64(trailer[0:8])
	payloadLen := binary.LittleEndian.Uint64(trailer[8:16])
	endOffset := uint64(len(wasmBytes) - stbcTrailerSize)
	if payloadOffset < 20 || payloadOffset+payloadLen != endOffset {
		return wasmBytes
	}
	// Walk Wasm sections from byte 8 to find where the trailing mojra_stbc custom section starts.
	pos := 8
	for pos < len(wasmBytes) {
		secStart := pos
		secID := wasmBytes[pos]
		pos++
		secLen, n := readULEB128(wasmBytes[pos:])
		if n <= 0 {
			break
		}
		pos += n
		if secID == 0 && pos+int(secLen) == len(wasmBytes) {
			return wasmBytes[:secStart]
		}
		pos += int(secLen)
	}
	return wasmBytes
}

func writeULEB128(buf *bytes.Buffer, v uint64) {
	for step := 0; step < 10; step++ {
		b := byte(v & 0x7f)
		v >>= 7
		if v != 0 {
			b |= 0x80
		}
		buf.WriteByte(b)
		if v == 0 {
			break
		}
	}
}

func readULEB128(b []byte) (uint64, int) {
	var val uint64
	var shift uint
	for i := 0; i < len(b) && i < 10; i++ {
		byt := b[i]
		val |= uint64(byt&0x7f) << shift
		if (byt & 0x80) == 0 {
			return val, i + 1
		}
		shift += 7
	}
	return 0, 0
}

func writeSTBCString(buf *bytes.Buffer, s string) {
	raw := []byte(s)
	_ = binary.Write(buf, binary.LittleEndian, uint32(len(raw)))
	buf.Write(raw)
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
