package book

import (
	"bytes"
	"crypto/md5"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	defaultFLACSampleRate = 24000
	flacMaxBlockSamples   = 1024
	flacMinBlockSamples   = 1024
)

var (
	flacCRC8Table  [256]uint8
	flacCRC16Table [256]uint16
)

func init() {
	for i := 0; i < 256; i++ {
		c8 := uint8(i)
		for b := 0; b < 8; b++ {
			if (c8 & 0x80) != 0 {
				c8 = (c8 << 1) ^ 0x07
			} else {
				c8 <<= 1
			}
		}
		flacCRC8Table[i] = c8

		c16 := uint16(i) << 8
		for b := 0; b < 8; b++ {
			if (c16 & 0x8000) != 0 {
				c16 = (c16 << 1) ^ 0x8005
			} else {
				c16 <<= 1
			}
		}
		flacCRC16Table[i] = c16
	}
}

func computeFLACCRC8(data []byte) uint8 {
	var crc uint8
	for _, b := range data {
		crc = flacCRC8Table[crc^b]
	}
	return crc
}

func computeFLACCRC16(data []byte) uint16 {
	var crc uint16
	for _, b := range data {
		crc = (crc << 8) ^ flacCRC16Table[byte(crc>>8)^b]
	}
	return crc
}

func appendFLACUTF8Uint(dst []byte, v uint32) []byte {
	switch {
	case v < 0x80:
		return append(dst, byte(v))
	case v < 0x800:
		return append(dst,
			byte(0xC0|(v>>6)),
			byte(0x80|(v&0x3F)),
		)
	case v < 0x10000:
		return append(dst,
			byte(0xE0|(v>>12)),
			byte(0x80|((v>>6)&0x3F)),
			byte(0x80|(v&0x3F)),
		)
	default:
		return append(dst,
			byte(0xF0|(v>>18)),
			byte(0x80|((v>>12)&0x3F)),
			byte(0x80|((v>>6)&0x3F)),
			byte(0x80|(v&0x3F)),
		)
	}
}

func readFLACUTF8Uint(data []byte) (uint32, int, error) {
	if len(data) == 0 {
		return 0, 0, fmt.Errorf("unexpected EOF reading FLAC UTF-8 number")
	}
	b0 := data[0]
	if (b0 & 0x80) == 0 {
		return uint32(b0), 1, nil
	}
	var length int
	var mask byte
	switch {
	case (b0 & 0xE0) == 0xC0:
		length = 2
		mask = 0x1F
	case (b0 & 0xF0) == 0xE0:
		length = 3
		mask = 0x0F
	case (b0 & 0xF8) == 0xF0:
		length = 4
		mask = 0x07
	default:
		return 0, 0, fmt.Errorf("invalid FLAC UTF-8 header byte 0x%02x", b0)
	}
	if len(data) < length {
		return 0, 0, fmt.Errorf("truncated FLAC UTF-8 number")
	}
	val := uint32(b0 & mask)
	for i := 1; i < length; i++ {
		if (data[i] & 0xC0) != 0x80 {
			return 0, 0, fmt.Errorf("invalid FLAC UTF-8 continuation byte")
		}
		val = (val << 6) | uint32(data[i]&0x3F)
	}
	return val, length, nil
}

// EncodeFLACMono16 encodes 16-bit signed mono PCM samples into a RFC 9639 compliant .flac stream.
func EncodeFLACMono16(samples []int16, sampleRate int) ([]byte, error) {
	if sampleRate < 8000 || sampleRate > 65535 {
		sampleRate = defaultFLACSampleRate
	}
	work := samples
	if len(work) == 0 {
		work = make([]int16, flacMaxBlockSamples)
	} else if rem := len(work) % flacMaxBlockSamples; rem != 0 {
		padCount := flacMaxBlockSamples - rem
		padded := make([]int16, len(work)+padCount)
		copy(padded, work)
		work = padded
	}

	totalSamples := uint64(len(work))
	minBlock := uint16(flacMaxBlockSamples)
	maxBlock := uint16(flacMaxBlockSamples)

	// Compute MD5 over unencoded 16-bit little-endian signed samples
	md5Hasher := md5.New()
	var leBuf [2]byte
	for _, s := range work {
		binary.LittleEndian.PutUint16(leBuf[:], uint16(s))
		_, _ = md5Hasher.Write(leBuf[:])
	}
	md5Sum := md5Hasher.Sum(nil)

	var out bytes.Buffer
	out.Grow(42 + len(work)*2 + 64)

	// 1. Magic "fLaC" + STREAMINFO metadata block header (is_last=1, type=0, len=34)
	out.WriteString("fLaC")
	out.Write([]byte{0x80, 0x00, 0x00, 0x22})

	var streamInfo [34]byte
	binary.BigEndian.PutUint16(streamInfo[0:2], minBlock)
	binary.BigEndian.PutUint16(streamInfo[2:4], maxBlock)
	// min_frame_size (3 bytes) and max_frame_size (3 bytes) left as 0 (unknown)
	// 20 bits sampleRate | 3 bits (channels-1=0) | 5 bits (bps-1=15) | 36 bits totalSamples
	sr := uint64(sampleRate) & 0xFFFFF
	bpsMinus1 := uint64(15)
	tot := totalSamples & 0xFFFFFFFFF
	packed64 := (sr << 44) | (0 << 41) | (bpsMinus1 << 36) | tot
	binary.BigEndian.PutUint64(streamInfo[10:18], packed64)
	streamInfo[8] = byte(sr >> 12)
	streamInfo[9] = byte(sr >> 4)
	streamInfo[10] = byte((sr&0x0F)<<4) | 0 | byte(bpsMinus1>>4)
	streamInfo[11] = byte((bpsMinus1&0x0F)<<4) | byte((tot>>32)&0x0F)
	binary.BigEndian.PutUint32(streamInfo[12:16], uint32(tot&0xFFFFFFFF))
	// Wait: let's make sure bytes 8..17 (10 bytes = 80 bits) are packed in exact big-endian bit order:
	// Bits 0..19 (20 bits): sample_rate
	// Bits 20..22 (3 bits): channels - 1 (0)
	// Bits 23..27 (5 bits): bits_per_sample - 1 (15 = 0b01111)
	// Bits 28..63 (36 bits): total_samples
	// Total = 64 bits = 8 bytes (bytes 10..17)!
	// Let's check:
	// Bytes 0..1: min_block_size (2 bytes)
	// Bytes 2..3: max_block_size (2 bytes)
	// Bytes 4..6: min_frame_size (3 bytes)
	// Bytes 7..9: max_frame_size (3 bytes)
	// Bytes 10..17: 8 bytes (64 bits) = 20 + 3 + 5 + 36 bits!
	// Bytes 18..33: 16 bytes MD5!
	streamInfo[8] = 0
	streamInfo[9] = 0
	binary.BigEndian.PutUint64(streamInfo[10:18], packed64)
	copy(streamInfo[18:34], md5Sum[:16])
	out.Write(streamInfo[:])

	// 2. Encode audio frames
	var frameBuf bytes.Buffer
	frameIdx := uint32(0)
	minFrameSize := 0
	maxFrameSize := 0
	for offset := 0; offset < len(work); offset += flacMaxBlockSamples {
		end := offset + flacMaxBlockSamples
		if end > len(work) {
			end = len(work)
		}
		block := work[offset:end]
		blockLen := len(block)

		frameBuf.Reset()
		// Sync code 0xFFF8 (fixed-blocksize stream)
		// Byte 2: block_size_code = 0b0111 (16-bit block_size-1 at end of header), sample_rate_code = 0b1101 (16-bit Hz at end of header) -> 0x7D
		// Byte 3: channel_assignment = 0b0000 (mono), sample_size_code = 0b100 (16-bit), reserved = 0 -> 0x08
		hdr := make([]byte, 0, 16)
		hdr = append(hdr, 0xFF, 0xF8, 0x7D, 0x08)
		hdr = appendFLACUTF8Uint(hdr, frameIdx)
		hdr = append(hdr, byte((blockLen-1)>>8), byte(blockLen-1))
		hdr = append(hdr, byte(sampleRate>>8), byte(sampleRate))
		crc8 := computeFLACCRC8(hdr)
		hdr = append(hdr, crc8)
		frameBuf.Write(hdr)

		// Check if block is constant
		isConst := true
		first := block[0]
		for i := 1; i < blockLen; i++ {
			if block[i] != first {
				isConst = false
				break
			}
		}

		if isConst {
			// SUBFRAME_CONSTANT (type=000000, wasted=0 -> 0x00)
			frameBuf.WriteByte(0x00)
			u := uint16(first)
			frameBuf.WriteByte(byte(u >> 8))
			frameBuf.WriteByte(byte(u))
		} else {
			// SUBFRAME_VERBATIM (type=000001, wasted=0 -> 0x02)
			frameBuf.WriteByte(0x02)
			for _, s := range block {
				u := uint16(s)
				frameBuf.WriteByte(byte(u >> 8))
				frameBuf.WriteByte(byte(u))
			}
		}

		crc16 := computeFLACCRC16(frameBuf.Bytes())
		frameBuf.WriteByte(byte(crc16 >> 8))
		frameBuf.WriteByte(byte(crc16))

		fLen := frameBuf.Len()
		if minFrameSize == 0 || fLen < minFrameSize {
			minFrameSize = fLen
		}
		if fLen > maxFrameSize {
			maxFrameSize = fLen
		}

		out.Write(frameBuf.Bytes())
		frameIdx++
	}

	outBytes := out.Bytes()
	if minFrameSize > 0 && maxFrameSize > 0 && len(outBytes) >= 42 {
		outBytes[12] = byte(minFrameSize >> 16)
		outBytes[13] = byte(minFrameSize >> 8)
		outBytes[14] = byte(minFrameSize)
		outBytes[15] = byte(maxFrameSize >> 16)
		outBytes[16] = byte(maxFrameSize >> 8)
		outBytes[17] = byte(maxFrameSize)
	}
	return outBytes, nil
}

// NormalizeFLACForPlayback repairs legacy fixed-blocksize FLAC streams whose STREAMINFO
// min_block_size differs from max_block_size (which causes Apple CoreAudio / WebKit to fail with -50).
func NormalizeFLACForPlayback(raw []byte) ([]byte, bool) {
	if len(raw) < 42 || string(raw[:4]) != "fLaC" || (raw[4]&0x7F) != 0 {
		return raw, false
	}
	minBlock := binary.BigEndian.Uint16(raw[8:10])
	maxBlock := binary.BigEndian.Uint16(raw[10:12])
	if minBlock == maxBlock && minBlock > 0 {
		return raw, false
	}
	if samples, sr, err := DecodeFLACMono16(raw); err == nil && len(samples) > 0 {
		if reencoded, encErr := EncodeFLACMono16(samples, sr); encErr == nil && len(reencoded) >= 42 {
			return reencoded, true
		}
	}
	patched := make([]byte, len(raw))
	copy(patched, raw)
	binary.BigEndian.PutUint16(patched[8:10], maxBlock)
	return patched, true
}

// DecodeFLACMono16 decodes a mono 16-bit FLAC stream produced by EncodeFLACMono16 (or standard verbatim/constant FLAC) into samples.
func DecodeFLACMono16(data []byte) ([]int16, int, error) {
	if len(data) < 42 || string(data[:4]) != "fLaC" {
		return nil, 0, fmt.Errorf("nieprawidłowy nagłówek pliku FLAC")
	}
	pos := 4
	sampleRate := defaultFLACSampleRate
	var totalSamples uint64
	for pos+4 <= len(data) {
		hdr := data[pos]
		isLast := (hdr & 0x80) != 0
		blockType := hdr & 0x7F
		blockLen := int(data[pos+1])<<16 | int(data[pos+2])<<8 | int(data[pos+3])
		pos += 4
		if pos+blockLen > len(data) {
			return nil, 0, fmt.Errorf("uszkodzony blok metadanych FLAC")
		}
		if blockType == 0 && blockLen >= 34 {
			packed64 := binary.BigEndian.Uint64(data[pos+10 : pos+18])
			sr := int((packed64 >> 44) & 0xFFFFF)
			if sr > 0 {
				sampleRate = sr
			}
			totalSamples = packed64 & 0xFFFFFFFFF
		}
		pos += blockLen
		if isLast {
			break
		}
	}

	var samples []int16
	if totalSamples > 0 && totalSamples < 100_000_000 {
		samples = make([]int16, 0, int(totalSamples))
	}

	for pos+4 < len(data) {
		if data[pos] != 0xFF || (data[pos+1]&0xFE) != 0xF8 {
			break
		}
		blockSizeCode := (data[pos+2] >> 4) & 0x0F
		sampleRateCode := data[pos+2] & 0x0F
		_, utf8Len, err := readFLACUTF8Uint(data[pos+4:])
		if err != nil {
			break
		}
		cur := pos + 4 + utf8Len
		blockLen := flacMaxBlockSamples
		switch blockSizeCode {
		case 0x01:
			blockLen = 192
		case 0x02, 0x03, 0x04, 0x05:
			blockLen = 576 << (blockSizeCode - 2)
		case 0x06:
			if cur+1 > len(data) {
				return samples, sampleRate, nil
			}
			blockLen = int(data[cur]) + 1
			cur++
		case 0x07:
			if cur+2 > len(data) {
				return samples, sampleRate, nil
			}
			blockLen = (int(data[cur])<<8 | int(data[cur+1])) + 1
			cur += 2
		case 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F:
			blockLen = 256 << (blockSizeCode - 8)
		}
		switch sampleRateCode {
		case 0x0C:
			cur++
		case 0x0D, 0x0E:
			if cur+2 <= len(data) && sampleRateCode == 0x0D {
				sr := int(data[cur])<<8 | int(data[cur+1])
				if sr > 0 {
					sampleRate = sr
				}
			}
			cur += 2
		}
		// Skip CRC-8 byte
		cur++
		if cur >= len(data) {
			break
		}
		subHdr := data[cur]
		cur++
		subType := (subHdr >> 1) & 0x3F
		switch subType {
		case 0x00: // SUBFRAME_CONSTANT
			if cur+2 > len(data) {
				return samples, sampleRate, nil
			}
			val := int16(uint16(data[cur])<<8 | uint16(data[cur+1]))
			cur += 2
			for i := 0; i < blockLen; i++ {
				samples = append(samples, val)
			}
		case 0x01: // SUBFRAME_VERBATIM
			need := blockLen * 2
			if cur+need > len(data) {
				return samples, sampleRate, nil
			}
			for i := 0; i < blockLen; i++ {
				val := int16(uint16(data[cur])<<8 | uint16(data[cur+1]))
				samples = append(samples, val)
				cur += 2
			}
		default:
			return samples, sampleRate, fmt.Errorf("nieobsługiwany podtyp ramki FLAC (%d) przy bezpośrednim przycinaniu", subType)
		}
		// Skip 2-byte CRC-16 footer
		cur += 2
		pos = cur
	}

	if len(samples) == 0 {
		return nil, sampleRate, fmt.Errorf("brak próbek audio w strumieniu FLAC")
	}
	return samples, sampleRate, nil
}

func slicePCMByRatio(samples []int16, sampleRate int, startRatio, endRatio float64) []int16 {
	if len(samples) == 0 {
		return samples
	}
	if startRatio < 0 {
		startRatio = 0
	}
	if endRatio <= 0 || endRatio > 1 {
		endRatio = 1
	}
	if endRatio <= startRatio {
		endRatio = 1
		startRatio = 0
	}
	startIdx := int( float64(len(samples)) * startRatio )
	endIdx := int( float64(len(samples)) * endRatio )
	if startIdx < 0 {
		startIdx = 0
	}
	if endIdx > len(samples) {
		endIdx = len(samples)
	}
	minSamples := sampleRate / 10
	if minSamples < flacMinBlockSamples {
		minSamples = flacMinBlockSamples
	}
	if endIdx-startIdx < minSamples {
		if startIdx+minSamples <= len(samples) {
			endIdx = startIdx + minSamples
		} else if endIdx-minSamples >= 0 {
			startIdx = endIdx - minSamples
		} else {
			startIdx = 0
			endIdx = len(samples)
		}
	}
	return samples[startIdx:endIdx]
}

func writeVoiceFLACFileAtomically(b *Book, sceneID string, flacBytes []byte) (string, error) {
	if b == nil {
		return "", fmt.Errorf("brak aktywnej książki")
	}
	if b.IsPacked {
		return "", fmt.Errorf("nie można edytować spakowanego archiwum .stbook")
	}
	if !b.Narrated {
		return "", fmt.Errorf("książka nie posiada włączonej narracji (narrated: false)")
	}
	if strings.TrimSpace(b.RootPath) == "" {
		return "", fmt.Errorf("brak ścieżki katalogu książki na dysku")
	}
	stem := SanitizeBookID(sceneID)
	if stem == "" {
		stem = "scena"
	}
	relPath := filepath.ToSlash(filepath.Join("voice", stem+".flac"))
	destDir := filepath.Join(b.RootPath, "voice")
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return "", fmt.Errorf("nie można utworzyć katalogu voice/: %w", err)
	}
	destPath := filepath.Join(destDir, stem+".flac")
	if !IsSafePathInsideRoot(b.RootPath, destPath) {
		return "", fmt.Errorf("niedozwolona ścieżka pliku audio")
	}
	tmpPath := destPath + ".tmp"
	if err := os.WriteFile(tmpPath, flacBytes, 0644); err != nil {
		return "", fmt.Errorf("błąd zapisu pliku audio: %w", err)
	}
	if err := os.Rename(tmpPath, destPath); err != nil {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("błąd podmiany pliku audio: %w", err)
	}
	return relPath, nil
}

// SaveSceneVoicePCM encodes little-endian signed 16-bit mono PCM samples into voice/<scene_id>.flac,
// optionally trimming to [startRatio, endRatio], and assigns the file to the target scene.
func SaveSceneVoicePCM(b *Book, sceneID string, pcmLE []byte, sampleRate int, startRatio, endRatio float64) (string, error) {
	if len(pcmLE) < 2 {
		return "", fmt.Errorf("puste nagranie audio")
	}
	numSamples := len(pcmLE) / 2
	samples := make([]int16, numSamples)
	for i := 0; i < numSamples; i++ {
		samples[i] = int16(binary.LittleEndian.Uint16(pcmLE[i*2 : i*2+2]))
	}
	samples = slicePCMByRatio(samples, sampleRate, startRatio, endRatio)
	flacBytes, err := EncodeFLACMono16(samples, sampleRate)
	if err != nil {
		return "", err
	}
	return writeVoiceFLACFileAtomically(b, sceneID, flacBytes)
}

// TrimSceneVoiceFile trims an existing .flac voice file for a scene to [startRatio, endRatio] in-place.
func TrimSceneVoiceFile(b *Book, sceneID string, currentRelVoice string, startRatio, endRatio float64) (string, error) {
	if b == nil || b.IsPacked {
		return "", fmt.Errorf("brak edytowalnej książki")
	}
	cleanRel := SanitizeRelativeMediaPath(currentRelVoice)
	if cleanRel == "" {
		stem := SanitizeBookID(sceneID)
		if stem != "" {
			cleanRel = filepath.ToSlash(filepath.Join("voice", stem+".flac"))
		}
	}
	if cleanRel == "" {
		return "", fmt.Errorf("brak przypisanego pliku audio do przycięcia")
	}
	absPath := filepath.Join(b.RootPath, filepath.FromSlash(cleanRel))
	if !IsSafePathInsideRoot(b.RootPath, absPath) {
		return "", fmt.Errorf("niedozwolona ścieżka pliku audio")
	}
	raw, err := os.ReadFile(absPath)
	if err != nil {
		return "", fmt.Errorf("nie można odczytać pliku audio: %w", err)
	}
	samples, sampleRate, err := DecodeFLACMono16(raw)
	if err != nil {
		return "", err
	}
	trimmed := slicePCMByRatio(samples, sampleRate, startRatio, endRatio)
	flacBytes, err := EncodeFLACMono16(trimmed, sampleRate)
	if err != nil {
		return "", err
	}
	return writeVoiceFLACFileAtomically(b, sceneID, flacBytes)
}

// ReadAudioDurationAndStamp returns a formatted M:SS duration label and a cache-busting modtime stamp for a scene's audio file.
func ReadAudioDurationAndStamp(b *Book, relVoice string) (string, int64) {
	if b == nil || b.IsPacked || strings.TrimSpace(relVoice) == "" || strings.TrimSpace(b.RootPath) == "" {
		return "0:00", 0
	}
	cleanRel := SanitizeRelativeMediaPath(relVoice)
	if cleanRel == "" {
		return "0:00", 0
	}
	absPath := filepath.Join(b.RootPath, filepath.FromSlash(cleanRel))
	if !IsSafePathInsideRoot(b.RootPath, absPath) {
		return "0:00", 0
	}
	f, err := os.Open(absPath)
	if err != nil {
		return "0:00", 0
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || info.IsDir() {
		return "0:00", 0
	}
	stamp := info.ModTime().UnixNano()
	var hdr [42]byte
	n, _ := f.Read(hdr[:])
	if n >= 26 && string(hdr[:4]) == "fLaC" && (hdr[4]&0x7F) == 0 {
		if n >= 42 && binary.BigEndian.Uint16(hdr[8:10]) != binary.BigEndian.Uint16(hdr[10:12]) && info.Size() <= maxMediaEntryBytes {
			if raw, readErr := os.ReadFile(absPath); readErr == nil {
				if fixed, changed := NormalizeFLACForPlayback(raw); changed {
					tmpPath := absPath + ".tmp"
					if writeErr := os.WriteFile(tmpPath, fixed, 0644); writeErr == nil {
						if renameErr := os.Rename(tmpPath, absPath); renameErr == nil {
							copy(hdr[:], fixed[:42])
							if st, stErr := os.Stat(absPath); stErr == nil {
								stamp = st.ModTime().UnixNano()
							}
						}
					}
				}
			}
		}
		packed64 := binary.BigEndian.Uint64(hdr[18:26])
		sr := (packed64 >> 44) & 0xFFFFF
		tot := packed64 & 0xFFFFFFFFF
		if sr > 0 && tot > 0 {
			secs := int((tot + sr/2) / sr)
			if secs < 0 {
				secs = 0
			}
			return fmt.Sprintf("%d:%02d", secs/60, secs%60), stamp
		}
	}
	return "0:00", stamp
}

// DeleteSceneVoiceFile removes the voice assignment from a scene and deletes its voice/<scene_id>.flac file if unused by other scenes.
func DeleteSceneVoiceFile(b *Book, sc *Scene) error {
	if b == nil || sc == nil {
		return fmt.Errorf("brak aktywnej sceny")
	}
	if b.IsPacked {
		return fmt.Errorf("nie można edytować spakowanego archiwum .stbook")
	}
	prevRel := SanitizeRelativeMediaPath(sc.ExplicitVoice)
	if prevRel == "" {
		prevRel = SanitizeRelativeMediaPath(sc.AudioFile)
	}
	sc.ExplicitVoice = ""
	sc.AudioFile = ""
	sc.AudioURL = ""
	if prevRel != "" && strings.TrimSpace(b.RootPath) != "" {
		stillUsed := false
		for _, other := range b.Scenes {
			if other == nil || other.ID == sc.ID {
				continue
			}
			if strings.EqualFold(SanitizeRelativeMediaPath(other.ExplicitVoice), prevRel) ||
				strings.EqualFold(SanitizeRelativeMediaPath(other.AudioFile), prevRel) {
				stillUsed = true
				break
			}
		}
		if !stillUsed {
			absPath := filepath.Join(b.RootPath, filepath.FromSlash(prevRel))
			if IsSafePathInsideRoot(b.RootPath, absPath) {
				_ = os.Remove(absPath)
			}
		}
	}
	return nil
}

