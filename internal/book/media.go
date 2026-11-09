package book

import (
	"bytes"
	"encoding/binary"
	"net/url"
	"path/filepath"
	"strings"
)

func isAudioFilename(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".ogg", ".flac", ".mp3":
		return true
	default:
		return false
	}
}

// IsAudioPath reports whether a filename or path has a supported audio extension (.ogg, .flac, .mp3).
func IsAudioPath(name string) bool {
	return isAudioFilename(name)
}

// isRasterImageFilename allows only raster formats (.png, .jpg, .jpeg, .webp) and strictly excludes .svg.
func isRasterImageFilename(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".png", ".jpg", ".jpeg", ".webp":
		return true
	default:
		return false
	}
}

// IsRasterImagePath reports whether a filename or path has a supported raster image extension (.png, .jpg, .jpeg, .webp).
func IsRasterImagePath(name string) bool {
	return isRasterImageFilename(name)
}

func encodeMediaURL(bookID, relPath string) string {
	parts := strings.Split(filepath.ToSlash(relPath), "/")
	for i, p := range parts {
		parts[i] = url.PathEscape(p)
	}
	return "/api/books/" + url.PathEscape(bookID) + "/media/" + strings.Join(parts, "/")
}

func findMediaFile(candidates []string, explicitPath string, idKey string, nameKey string) (string, bool) {
	if explicitPath != "" {
		expNorm := NormalizeLookupKey(filepath.ToSlash(explicitPath))
		expNoExt := NormalizeLookupKey(strings.TrimSuffix(filepath.ToSlash(explicitPath), filepath.Ext(explicitPath)))
		for _, f := range candidates {
			fSlash := filepath.ToSlash(f)
			fNorm := NormalizeLookupKey(fSlash)
			fNoExt := NormalizeLookupKey(strings.TrimSuffix(fSlash, filepath.Ext(fSlash)))
			baseNorm := NormalizeLookupKey(filepath.Base(fSlash))
			baseNoExt := NormalizeLookupKey(strings.TrimSuffix(filepath.Base(fSlash), filepath.Ext(fSlash)))
			if fNorm == expNorm || fNoExt == expNoExt || baseNorm == expNorm || baseNoExt == expNoExt {
				return fSlash, true
			}
		}
		return "", false
	}

	// Direct matching by scene ID or scene Name (either in root or in a subdirectory)
	idNorm := NormalizeLookupKey(idKey)
	nameNorm := NormalizeLookupKey(nameKey)
	for _, f := range candidates {
		fSlash := filepath.ToSlash(f)
		base := filepath.Base(fSlash)
		stem := NormalizeLookupKey(strings.TrimSuffix(base, filepath.Ext(base)))
		if (idNorm != "" && stem == idNorm) || (nameNorm != "" && stem == nameNorm) {
			return fSlash, true
		}
	}
	return "", false
}

const MaxSingleMediaBytes = 100 * 1024 * 1024
const maxMediaEntryBytes = MaxSingleMediaBytes

func mediaContentType(relPath string) string {
	switch strings.ToLower(filepath.Ext(relPath)) {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".webp":
		return "image/webp"
	case ".ogg":
		return "audio/ogg"
	case ".flac":
		return "audio/flac"
	case ".mp3":
		return "audio/mpeg"
	default:
		return ""
	}
}

func validateMediaHeader(relPath string, header []byte) bool {
	if len(header) == 0 {
		return false
	}
	probe := header
	if len(probe) > 512 {
		probe = probe[:512]
	}
	lowerProbe := strings.ToLower(string(probe))
	if hasDisallowedMediaSignature(lowerProbe) {
		return false
	}

	switch strings.ToLower(filepath.Ext(relPath)) {
	case ".png":
		return len(header) >= 8 && bytes.Equal(header[:8], []byte("\x89PNG\r\n\x1a\n"))
	case ".jpg", ".jpeg":
		return len(header) >= 3 && header[0] == 0xff && header[1] == 0xd8 && header[2] == 0xff
	case ".webp":
		return len(header) >= 12 && string(header[:4]) == "RIFF" && string(header[8:12]) == "WEBP"
	case ".ogg":
		return len(header) >= 4 && string(header[:4]) == "OggS"
	case ".flac":
		if len(header) >= 4 && (string(header[:4]) == "fLaC" || string(header[:4]) == "OggS") {
			return true
		}
		return len(header) >= 3 && string(header[:3]) == "ID3"
	case ".mp3":
		if len(header) >= 3 && string(header[:3]) == "ID3" {
			return true
		}
		return len(header) >= 2 && header[0] == 0xff && (header[1]&0xe0) == 0xe0
	default:
		return false
	}
}

var readUnpackedAudioDurationHook func(b *Book, cleanRel string) (float64, int64)

// ReadAudioDurationSecondsAndStamp returns the raw duration in seconds and modtime stamp for a scene's audio file.
func ReadAudioDurationSecondsAndStamp(b *Book, relVoice string) (float64, int64) {
	if b == nil || strings.TrimSpace(relVoice) == "" {
		return 0, 0
	}
	cleanRel := SanitizeRelativeMediaPath(relVoice)
	if cleanRel == "" {
		return 0, 0
	}
	if b.IsPacked {
		if b.PackedMedia != nil {
			if entry, ok := b.PackedMedia[strings.ToLower(cleanRel)]; ok && entry.HeaderLen >= 26 {
				hdr := entry.Header512[:entry.HeaderLen]
				if string(hdr[:4]) == "fLaC" && (hdr[4]&0x7F) == 0 {
					packed64 := binary.BigEndian.Uint64(hdr[18:26])
					sr := (packed64 >> 44) & 0xFFFFF
					tot := packed64 & 0xFFFFFFFFF
					if sr > 0 && tot > 0 {
						return float64(tot) / float64(sr), entry.ModTime.UnixNano()
					}
				}
			}
		}
		return 0, 0
	}
	if readUnpackedAudioDurationHook != nil {
		return readUnpackedAudioDurationHook(b, cleanRel)
	}
	return 0, 0
}
