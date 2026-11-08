package book

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
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

func matchBookMedia(b *Book, audioFiles []string, imageFiles []string) {
	sort.Strings(audioFiles)
	sort.Strings(imageFiles)
	if b.Narrated {
		b.AudioFiles = audioFiles
	} else {
		b.AudioFiles = nil
	}
	b.ImageFiles = imageFiles

	// Match book cover (raster image only)
	if b.CoverFile != "" {
		if matched, ok := findMediaFile(imageFiles, b.CoverFile, "", ""); ok {
			b.CoverFile = matched
		} else {
			b.Warnings = append(b.Warnings, fmt.Sprintf("book.playtxt: nie znaleziono pliku okładki „%s”", b.CoverFile))
			b.CoverFile = ""
		}
	}
	if b.CoverFile == "" {
		for _, img := range imageFiles {
			stem := NormalizeLookupKey(strings.TrimSuffix(filepath.Base(img), filepath.Ext(img)))
			if stem == "cover" || stem == "okladka" || stem == "okładka" {
				b.CoverFile = filepath.ToSlash(img)
				break
			}
		}
	}
	if b.CoverFile != "" {
		b.CoverURL = encodeMediaURL(b.ID, b.CoverFile)
	}

	for _, sc := range b.Scenes {
		sc.AudioFile = ""
		sc.AudioURL = ""
		sc.ImageFile = ""
		sc.ImageURL = ""

		// 1. Match Voice / Audio (all-or-nothing policy: voice is forbidden when !b.Narrated)
		if !b.Narrated {
			sc.ExplicitVoice = ""
			sc.VoiceTrimStart = 0
			sc.VoiceTrimEnd = 0
			sc.SectionVoices = nil
			ExtractSceneVoiceSections(sc, b)
		} else if HasConditionalVoiceDirectives(sc.Text) {
			secs := ExtractSceneVoiceSections(sc, b)
			for _, sec := range secs {
				if sec.HasAudio && sc.AudioFile == "" {
					sc.AudioFile = sec.AudioFile
					sc.AudioURL = encodeMediaURL(b.ID, sec.AudioFile)
				}
			}
			appendSceneVoiceSectionWarnings(b, sc)
		} else {
			if sc.ExplicitVoice != "" {
				if matched, ok := findMediaFile(audioFiles, sc.ExplicitVoice, "", ""); ok {
					if strings.EqualFold(filepath.Ext(sc.ExplicitVoice), ".wav") {
						sc.ExplicitVoice = matched
					}
					sc.AudioFile = matched
					sc.AudioURL = encodeMediaURL(b.ID, matched)
				} else {
					warn := fmt.Sprintf("Scena „%s”: nie znaleziono pliku voice: „%s”", sc.DisplayName(), sc.ExplicitVoice)
					sc.Warnings = append(sc.Warnings, warn)
					b.Warnings = append(b.Warnings, warn)
				}
			}

			if sc.AudioFile == "" && sc.ExplicitVoice == "" {
				expectedName := sc.DisplayName()
				warn := fmt.Sprintf("Scena „%s”: brak nagrania audio (książka ma włączoną narrację — ustaw voice: ścieżka.ogg/.flac/.mp3 lub dodaj plik „%s.ogg”)", sc.DisplayName(), expectedName)
				sc.Warnings = append(sc.Warnings, warn)
				b.Warnings = append(b.Warnings, warn)
			}
			ExtractSceneVoiceSections(sc, b)
		}

		// 2. Match Image (optional for scenes!)
		if sc.ExplicitImage != "" {
			if matched, ok := findMediaFile(imageFiles, sc.ExplicitImage, "", ""); ok {
				sc.ImageFile = matched
				sc.ImageURL = encodeMediaURL(b.ID, matched)
			} else {
				warn := fmt.Sprintf("Scena „%s”: nie znaleziono pliku image: „%s”", sc.DisplayName(), sc.ExplicitImage)
				sc.Warnings = append(sc.Warnings, warn)
				b.Warnings = append(b.Warnings, warn)
			}
		}

		if b.CoverURL == "" && sc.ImageURL != "" {
			b.CoverFile = sc.ImageFile
			b.CoverURL = sc.ImageURL
		}
	}
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

// ServeBookMedia streams an audio or raster image file from either an unpacked directory or a .stbook archive,
// supporting safe subdirectory paths (e.g. "images/scene.png" or "voice/scene.ogg").
func ServeBookMedia(w http.ResponseWriter, r *http.Request, b *Book, rawRelPath string) {
	decoded, err := url.PathUnescape(rawRelPath)
	if err != nil {
		decoded = rawRelPath
	}
	cleanRel := SanitizeRelativeMediaPath(decoded)
	if cleanRel == "" {
		http.NotFound(w, r)
		return
	}

	contentType := mediaContentType(cleanRel)
	if contentType == "" || (!isAudioFilename(cleanRel) && !isRasterImageFilename(cleanRel)) {
		http.Error(w, "unsupported media type (SVG is forbidden in book media)", http.StatusForbidden)
		return
	}
	if !b.Narrated && isAudioFilename(cleanRel) {
		http.Error(w, "voice media is disabled for narrationless books", http.StatusForbidden)
		return
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "private, max-age=3600")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; media-src 'self'; img-src 'self'; sandbox")

	if !b.IsPacked {
		fullPath := filepath.Join(b.RootPath, filepath.FromSlash(cleanRel))
		if !IsSafePathInsideRoot(b.RootPath, fullPath) {
			http.NotFound(w, r)
			return
		}
		f, err := os.Open(fullPath)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer f.Close()
		info, err := f.Stat()
		if err != nil || info.IsDir() {
			http.NotFound(w, r)
			return
		}
		var hdr [512]byte
		n, _ := io.ReadFull(f, hdr[:])
		if !validateMediaHeader(cleanRel, hdr[:n]) {
			http.Error(w, "invalid or disguised media header", http.StatusForbidden)
			return
		}
		if n >= 42 && string(hdr[:4]) == "fLaC" && (hdr[4]&0x7F) == 0 &&
			binary.BigEndian.Uint16(hdr[8:10]) != binary.BigEndian.Uint16(hdr[10:12]) &&
			info.Size() <= maxMediaEntryBytes {
			if _, err := f.Seek(0, io.SeekStart); err == nil {
				if raw, readErr := io.ReadAll(io.LimitReader(f, maxMediaEntryBytes+1)); readErr == nil {
					fixed, changed := NormalizeFLACForPlayback(raw)
					if changed {
						tmpPath := fullPath + ".tmp"
						if writeErr := os.WriteFile(tmpPath, fixed, 0644); writeErr == nil {
							if err := os.Rename(tmpPath, fullPath); err != nil {
								_ = os.Remove(tmpPath)
							}
						}
					}
					http.ServeContent(w, r, filepath.Base(cleanRel), info.ModTime(), bytes.NewReader(fixed))
					return
				}
			}
		}
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			http.NotFound(w, r)
			return
		}
		http.ServeContent(w, r, filepath.Base(cleanRel), info.ModTime(), f)
		return
	}

	if b.PackedMedia != nil {
		entry, ok := b.PackedMedia[strings.ToLower(cleanRel)]
		if !ok {
			http.NotFound(w, r)
			return
		}
		if entry.Size > maxMediaEntryBytes {
			http.Error(w, "media entry exceeds size limit", http.StatusRequestEntityTooLarge)
			return
		}
		if !entry.ValidHeader {
			http.Error(w, "invalid or disguised media header", http.StatusForbidden)
			return
		}
		if entry.Method == zip.Store && entry.DataOffset >= 30 {
			osFile, err := os.Open(b.RootPath)
			if err != nil {
				http.NotFound(w, r)
				return
			}
			defer osFile.Close()
			section := io.NewSectionReader(osFile, entry.DataOffset, entry.Size)
			http.ServeContent(w, r, filepath.Base(cleanRel), entry.ModTime, section)
			return
		}
	}

	zr, err := zip.OpenReader(b.RootPath)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer zr.Close()

	if f := findZipEntryByRelPath(zr.File, cleanRel); f != nil {
		if f.UncompressedSize64 > maxMediaEntryBytes {
			http.Error(w, "media entry exceeds size limit", http.StatusRequestEntityTooLarge)
			return
		}
		rc, err := f.Open()
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer rc.Close()
		raw, err := io.ReadAll(io.LimitReader(rc, maxMediaEntryBytes+1))
		if err != nil {
			http.NotFound(w, r)
			return
		}
		if len(raw) > maxMediaEntryBytes {
			http.Error(w, "media entry exceeds size limit", http.StatusRequestEntityTooLarge)
			return
		}
		if !validateMediaHeader(cleanRel, raw) {
			http.Error(w, "invalid or disguised media header", http.StatusForbidden)
			return
		}
		http.ServeContent(w, r, filepath.Base(cleanRel), f.Modified, bytes.NewReader(raw))
		return
	}
	http.NotFound(w, r)
}

// GenerateAtmosphericPNG renders a rich raster PNG illustration with warm lantern/moon light, mist, and river/forest silhouettes.
func GenerateAtmosphericPNG(outPath string, width, height int, baseCol, glowCol color.RGBA, variant int) error {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	cx := float64(width) * (0.32 + float64(variant%5)*0.09)
	cy := float64(height) * (0.32 + float64(variant%3)*0.06)
	maxDist := float64(width+height) * 0.52
	orbRadius := float64(height) * (0.09 + float64(variant%3)*0.015)
	phase := float64(variant) * 1.15

	for y := 0; y < height; y++ {
		fy := float64(y) / float64(height)
		for x := 0; x < width; x++ {
			fx := float64(x) / float64(width)
			dx := float64(x) - cx
			dy := float64(y) - cy
			distSq := dx*dx + dy*dy
			dist := math.Sqrt(distSq)

			glow := 1.0 - (dist / maxDist)
			if glow < 0 {
				glow = 0
			}
			glow = math.Pow(glow, 1.35)

			// Bright moon / lantern core
			orb := 0.0
			if dist < orbRadius*2.2 {
				t := 1.0 - (dist / (orbRadius * 2.2))
				orb = math.Pow(t, 2.2) * 0.85
				if dist < orbRadius {
					orb = 0.88 + 0.12*(1.0-dist/orbRadius)
				}
			}

			// Mist waves
			mist := 0.18 * (0.5 + 0.5*math.Sin(fx*10.0+phase+fy*4.0)) * math.Exp(-math.Pow((fy-0.54)/0.16, 2))

			// Distant hills & treeline silhouette around horizon (fy ~ 0.58..0.72)
			hill1 := 0.58 + 0.045*math.Sin(fx*6.2+phase) + 0.025*math.Cos(fx*15.0-phase)
			hill2 := 0.68 + 0.035*math.Cos(fx*8.5+phase*1.4) + 0.02*math.Sin(fx*21.0)

			r := float64(baseCol.R)*0.65 + float64(glowCol.R)*(glow*0.95+mist*0.7) + 235.0*orb
			g := float64(baseCol.G)*0.65 + float64(glowCol.G)*(glow*0.95+mist*0.7) + 225.0*orb
			bl := float64(baseCol.B)*0.65 + float64(glowCol.B)*(glow*0.95+mist*0.7) + 210.0*orb

			if fy > hill2 {
				// River water reflection at bottom
				waterDepth := (fy - hill2) / (1.0 - hill2)
				xDist := math.Abs(float64(x)-cx) / (float64(width) * 0.28)
				ripple := 0.5 + 0.5*math.Sin(fy*95.0+fx*18.0+phase)
				refl := math.Max(0, 1.0-xDist) * ripple * 0.65 * (1.0 - waterDepth*0.4)
				r = float64(baseCol.R)*0.35 + float64(glowCol.R)*(0.22+refl)
				g = float64(baseCol.G)*0.35 + float64(glowCol.G)*(0.22+refl)
				bl = float64(baseCol.B)*0.35 + float64(glowCol.B)*(0.22+refl)
			} else if fy > hill1 {
				// Far forest / riverbank silhouette
				r *= 0.32
				g *= 0.34
				bl *= 0.38
			}

			img.SetRGBA(x, y, color.RGBA{
				R: uint8(clampFloat(r)),
				G: uint8(clampFloat(g)),
				B: uint8(clampFloat(bl)),
				A: 255,
			})
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return err
	}
	return os.WriteFile(outPath, buf.Bytes(), 0644)
}

func clampFloat(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return v
}
