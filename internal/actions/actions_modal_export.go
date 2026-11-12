//go:build !(js && wasm)

package actions

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"mojra/internal/book"
	"mojra/internal/i18n"
	"mojra/internal/osbridge"
	"mojra/internal/state"
)

func openExportModal(s *state.AppState, _ *http.Request) string {
	b := s.Editor.ActiveBook
	if b == nil {
		return tr(s, i18n.TrNoActiveProject)
	}
	s.Modals.QR = false
	s.ModalNewBook.Open = false
	s.Modals.AddBook = false
	s.ModalExport.Open = true
	s.ModalExport.AdvancedOpen = false
	s.ModalExport.Standard = b.NormalizedStandard()
	s.ModalExport.Formats = map[string]bool{
		book.ExportFmtPlaybook: true,
		book.ExportFmtWASM:     true,
		book.ExportFmtPDF:      true,
	}
	if b.IsScreenplay() {
		s.ModalExport.Format = book.ExportFmtPDF
	} else {
		s.ModalExport.Format = book.ExportFmtPlaybook
	}
	s.ModalExport.Dir = b.ResolveExportDir(s.Settings)
	return ""
}

func closeExportModal(s *state.AppState, _ *http.Request) string {
	s.ModalExport.Reset()
	return ""
}

func toggleExportFormat(s *state.AppState, r *http.Request) string {
	if dir := strings.TrimSpace(r.FormValue("export_dir")); dir != "" {
		s.ModalExport.Dir = dir
	}
	if std := strings.TrimSpace(r.FormValue("standard")); std != "" {
		s.ModalExport.Standard = book.NormalizeScriptStandard(std)
	}
	fmtID := strings.ToLower(strings.TrimSpace(r.FormValue("format")))
	if book.IsValidExportFormat(fmtID) {
		if s.ModalExport.Formats == nil {
			s.ModalExport.Formats = make(map[string]bool)
		}
		if s.ModalExport.Formats[fmtID] {
			delete(s.ModalExport.Formats, fmtID)
		} else {
			s.ModalExport.Formats[fmtID] = true
		}
	}
	return ""
}

func toggleExportModalAdvanced(s *state.AppState, r *http.Request) string {
	if dir := strings.TrimSpace(r.FormValue("export_dir")); dir != "" {
		s.ModalExport.Dir = dir
	}
	if std := strings.TrimSpace(r.FormValue("standard")); std != "" {
		s.ModalExport.Standard = book.NormalizeScriptStandard(std)
	}
	s.ModalExport.AdvancedOpen = !s.ModalExport.AdvancedOpen
	return ""
}

func setExportModalStandard(s *state.AppState, r *http.Request) string {
	if dir := strings.TrimSpace(r.FormValue("export_dir")); dir != "" {
		s.ModalExport.Dir = dir
	}
	if std := strings.TrimSpace(r.FormValue("standard")); std != "" {
		s.ModalExport.Standard = book.NormalizeScriptStandard(std)
	}
	return ""
}

func browseExportModalFolder(s *state.AppState, r *http.Request) string {
	if fmtVal := strings.ToLower(strings.TrimSpace(r.FormValue("format"))); fmtVal != "" {
		s.ModalExport.Format = fmtVal
	}
	if stdVal := strings.TrimSpace(r.FormValue("standard")); stdVal != "" {
		s.ModalExport.Standard = book.NormalizeScriptStandard(stdVal)
	}
	if curDir := strings.TrimSpace(r.FormValue("export_dir")); curDir != "" {
		if expanded, err := book.ExpandLocalPath(curDir); err == nil && expanded != "" {
			s.ModalExport.Dir = expanded
		}
	}
	picked := strings.TrimSpace(r.FormValue("picked_dir"))
	if picked == "" {
		picked = pickNativeBookPathUnlocked(s, true)
	}
	if picked != "" {
		if expanded, err := book.ExpandLocalPath(picked); err == nil && expanded != "" {
			picked = expanded
		}
		s.ModalExport.Dir = picked
		s.Editor.ActiveBook.PersistExportDir(s.Config.SavesDir, s.Settings, picked)
	}
	return ""
}

func resolveOrPickBookExportDir(s *state.AppState, r *http.Request) (string, string, bool) {
	b := s.Editor.ActiveBook
	if b == nil {
		return "", "Brak aktywnego projektu", false
	}
	if r != nil {
		if rawDir := strings.TrimSpace(r.FormValue("export_dir")); rawDir != "" {
			expanded, err := book.ExpandLocalPath(rawDir)
			if err != nil {
				return "", err.Error(), false
			}
			b.PersistExportDir(s.Config.SavesDir, s.Settings, expanded)
			return expanded, "", true
		}
	}
	forcePrompt := r != nil && r.FormValue("prompt_dir") == "true"
	if !forcePrompt && strings.TrimSpace(b.ExportDir) != "" {
		expanded, err := book.ExpandLocalPath(b.ExportDir)
		if err != nil {
			return "", err.Error(), false
		}
		return expanded, "", true
	}
	if !osbridge.IsRunningUnderGoTestBinary() {
		picked := pickNativeBookPathUnlocked(s, true)
		if picked == "" {
			if r != nil {
				r.Form.Set("silent", "true")
			}
			return "", "", false
		}
		if expanded, err := book.ExpandLocalPath(picked); err == nil && expanded != "" {
			picked = expanded
		}
		b.PersistExportDir(s.Config.SavesDir, s.Settings, picked)
		return picked, "", true
	}
	defDir := b.ResolveExportDir(s.Settings)
	b.PersistExportDir(s.Config.SavesDir, s.Settings, defDir)
	return defDir, "", true
}

func exportProjectAction(s *state.AppState, r *http.Request) string {
	b := s.Editor.ActiveBook
	if b == nil {
		return "Brak aktywnego projektu"
	}
	rawDir := strings.TrimSpace(r.FormValue("export_dir"))
	if rawDir == "" {
		rawDir = strings.TrimSpace(s.ModalExport.Dir)
	}
	if rawDir != "" && r != nil && r.Form != nil {
		r.Form.Set("export_dir", rawDir)
	}
	exportDir, errMsg, ok := resolveOrPickBookExportDir(s, r)
	if !ok {
		return errMsg
	}
	if err := os.MkdirAll(exportDir, 0755); err != nil {
		return err.Error()
	}

	var selectedOrder []string
	explicitFormat := strings.ToLower(strings.TrimSpace(r.FormValue("format")))
	if explicitFormat != "" {
		selectedOrder = []string{explicitFormat}
	} else if len(s.ModalExport.Formats) > 0 {
		for _, id := range book.ExportFormats {
			if s.ModalExport.Formats[id] {
				selectedOrder = append(selectedOrder, id)
			}
		}
	} else {
		fallback := strings.ToLower(strings.TrimSpace(s.ModalExport.Format))
		if fallback == "" {
			if b.IsScreenplay() {
				fallback = book.ExportFmtPDF
			} else {
				fallback = book.ExportFmtPlaybook
			}
		}
		selectedOrder = []string{fallback}
	}

	if len(selectedOrder) == 0 {
		return "Wybierz przynajmniej jeden format eksportu"
	}

	expStd := strings.TrimSpace(r.FormValue("standard"))
	if expStd == "" {
		expStd = s.ModalExport.Standard
	}
	if expStd == "" {
		expStd = b.NormalizedStandard()
	} else {
		expStd = book.NormalizeScriptStandard(expStd)
	}

	var exportedPaths []string
	for _, fmtID := range selectedOrder {
		switch fmtID {
		case book.ExportFmtPlaybook:
			if !b.IsPacked {
				outZip := filepath.Join(exportDir, b.ID+book.ExportSuffixPlaybook)
				if err := book.PackBookToZip(b.RootPath, outZip); err != nil {
					return err.Error()
				}
				exportedPaths = append(exportedPaths, outZip)
			} else if b.RootPath != "" {
				outZip := filepath.Join(exportDir, b.ID+book.ExportSuffixPlaybook)
				if raw, err := os.ReadFile(b.RootPath); err == nil {
					if err := os.WriteFile(outZip, raw, 0644); err != nil {
						return err.Error()
					}
					exportedPaths = append(exportedPaths, outZip)
				}
			}
		case book.ExportFmtWASM:
			outWebDir, err := book.ExportWebWASMApp(b, exportDir, s.Config.RootDir, s.Config.EmbeddedFS)
			if err != nil {
				return err.Error()
			}
			exportedPaths = append(exportedPaths, outWebDir)
		case book.ExportFmtPDF:
			bookClone := *b
			bookClone.ScriptStandard = expStd
			pdfBytes, err := book.ExportScreenplayPDF(&bookClone)
			if err != nil {
				return err.Error()
			}
			outPDF := filepath.Join(exportDir, b.ID+book.ExportSuffixPDF)
			if err := os.WriteFile(outPDF, pdfBytes, 0644); err != nil {
				return err.Error()
			}
			exportedPaths = append(exportedPaths, outPDF)
		case book.ExportFmtFountain:
			bookClone := *b
			bookClone.ScriptStandard = expStd
			fountainText := book.ExportScreenplayFountain(&bookClone)
			outFountain := filepath.Join(exportDir, b.ID+book.ExportSuffixFountain)
			if err := os.WriteFile(outFountain, []byte(fountainText), 0644); err != nil {
				return err.Error()
			}
			exportedPaths = append(exportedPaths, outFountain)
		case book.ExportFmtFDX:
			bookClone := *b
			bookClone.ScriptStandard = expStd
			fdxText := book.ExportScreenplayFDX(&bookClone)
			outFDX := filepath.Join(exportDir, b.ID+book.ExportSuffixFDX)
			if err := os.WriteFile(outFDX, []byte(fdxText), 0644); err != nil {
				return err.Error()
			}
			exportedPaths = append(exportedPaths, outFDX)
		case book.ExportFmtTreatment:
			bookClone := *b
			bookClone.ScriptStandard = expStd
			pdfBytes, err := book.ExportScreenplayTreatmentPDF(&bookClone)
			if err != nil {
				return err.Error()
			}
			outPDF := filepath.Join(exportDir, b.ID+book.ExportSuffixTreatment)
			if err := os.WriteFile(outPDF, pdfBytes, 0644); err != nil {
				return err.Error()
			}
			exportedPaths = append(exportedPaths, outPDF)
		case book.ExportFmtBookPDF:
			pdfBytes, err := book.ExportBookPDF(b)
			if err != nil {
				return err.Error()
			}
			outPDF := filepath.Join(exportDir, b.ID+book.ExportSuffixBookPDF)
			if err := os.WriteFile(outPDF, pdfBytes, 0644); err != nil {
				return err.Error()
			}
			exportedPaths = append(exportedPaths, outPDF)
		case book.ExportFmtVOCSV:
			csvText := book.ExportVOTableCSV(b)
			outCSV := filepath.Join(exportDir, b.ID+book.ExportSuffixVOCSV)
			if err := os.WriteFile(outCSV, []byte(csvText), 0644); err != nil {
				return err.Error()
			}
			exportedPaths = append(exportedPaths, outCSV)
		case book.ExportFmtActorPDF:
			pdfBytes, err := book.ExportActorScriptsPDF(b)
			if err != nil {
				return err.Error()
			}
			outPDF := filepath.Join(exportDir, b.ID+book.ExportSuffixActorPDF)
			if err := os.WriteFile(outPDF, pdfBytes, 0644); err != nil {
				return err.Error()
			}
			exportedPaths = append(exportedPaths, outPDF)
		case book.ExportFmtPlaytxt:
			outPlaytxtDir, err := book.ExportPlaytxtProject(b, exportDir)
			if err != nil {
				return err.Error()
			}
			exportedPaths = append(exportedPaths, outPlaytxtDir)
		}
	}

	if len(exportedPaths) == 0 {
		return "Nie wyeksportowano żadnego pliku"
	}

	osbridge.RevealExportedFileInOS(exportedPaths[0])
	s.ModalExport.Reset()

	if len(exportedPaths) == 1 {
		return "Wyeksportowano: " + exportedPaths[0]
	}
	return fmt.Sprintf("Wyeksportowano %d formaty do: %s", len(exportedPaths), exportDir)
}
