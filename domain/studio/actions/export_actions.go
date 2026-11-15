//go:build !(js && wasm)

package actions

import (
	"os"
	"path/filepath"
	"strings"

	book "mojra/domain/book/actions"
	"mojra/domain/i18n"
	"mojra/domain/studio/state"
	"mojra/engine/osbridge"
	"mojra/engine/transport"
)

const (
	formKeyExportDir = "export_dir"
	formKeyFormat    = "format"
	formKeyPickedDir = "picked_dir"
	formKeyPromptDir = "prompt_dir"
)

var _ = registerActions(map[string]ActionHandler{
	"OpenExportModal":           openExportModal,
	"CloseExportModal":          closeExportModal,
	"ToggleExportFormat":        toggleExportFormat,
	"ToggleExportModalAdvanced": toggleExportModalAdvanced,
	"SetExportModalStandard":    setExportModalStandard,
	"BrowseExportModalFolder":   browseExportModalFolder,
	"ExportProject":             exportProjectAction,
})

func openExportModal(s *state.StudioState, _ *transport.ActionRequest) []transport.ActionFrame {
	b := s.Editor.ActiveBook
	if b == nil {
		return frameModals(s, tr(s, i18n.TrNoActiveProject))
	}
	s.Overlays.QR = false
	s.Overlays.AddBook = false
	s.Export.Open = true
	s.Export.AdvancedOpen = false
	s.Export.Standard = b.NormalizedStandard()
	s.Export.Formats = map[string]bool{
		book.ExportFmtPlaybook: true,
		book.ExportFmtWASM:     true,
		book.ExportFmtPDF:      true,
	}
	if b.IsScreenplay() {
		s.Export.Format = book.ExportFmtPDF
	} else {
		s.Export.Format = book.ExportFmtPlaybook
	}
	s.Export.Dir = b.ResolveExportDir(s.Settings)
	return frameModals(s, "")
}

func closeExportModal(s *state.StudioState, _ *transport.ActionRequest) []transport.ActionFrame {
	s.Export.Reset()
	return frameModals(s, "")
}

func toggleExportFormat(s *state.StudioState, r *transport.ActionRequest) []transport.ActionFrame {
	if dir := strings.TrimSpace(r.FormValue(formKeyExportDir)); dir != "" {
		s.Export.Dir = dir
	}
	if std := strings.TrimSpace(r.FormValue(newBookFormStandard)); std != "" {
		s.Export.Standard = book.NormalizeScriptStandard(std)
	}
	fmtID := strings.ToLower(strings.TrimSpace(r.FormValue(formKeyFormat)))
	if book.IsValidExportFormat(fmtID) {
		if s.Export.Formats == nil {
			s.Export.Formats = make(map[string]bool)
		}
		if s.Export.Formats[fmtID] {
			delete(s.Export.Formats, fmtID)
		} else {
			s.Export.Formats[fmtID] = true
		}
	}
	return frameModals(s, "")
}

func toggleExportModalAdvanced(s *state.StudioState, r *transport.ActionRequest) []transport.ActionFrame {
	if dir := strings.TrimSpace(r.FormValue(formKeyExportDir)); dir != "" {
		s.Export.Dir = dir
	}
	if std := strings.TrimSpace(r.FormValue(newBookFormStandard)); std != "" {
		s.Export.Standard = book.NormalizeScriptStandard(std)
	}
	s.Export.AdvancedOpen = !s.Export.AdvancedOpen
	return frameModals(s, "")
}

func setExportModalStandard(s *state.StudioState, r *transport.ActionRequest) []transport.ActionFrame {
	if dir := strings.TrimSpace(r.FormValue(formKeyExportDir)); dir != "" {
		s.Export.Dir = dir
	}
	if std := strings.TrimSpace(r.FormValue(newBookFormStandard)); std != "" {
		s.Export.Standard = book.NormalizeScriptStandard(std)
	}
	return frameModals(s, "")
}

func browseExportModalFolder(s *state.StudioState, r *transport.ActionRequest) []transport.ActionFrame {
	if fmtVal := strings.ToLower(strings.TrimSpace(r.FormValue(formKeyFormat))); fmtVal != "" {
		s.Export.Format = fmtVal
	}
	if stdVal := strings.TrimSpace(r.FormValue(newBookFormStandard)); stdVal != "" {
		s.Export.Standard = book.NormalizeScriptStandard(stdVal)
	}
	if curDir := strings.TrimSpace(r.FormValue(formKeyExportDir)); curDir != "" {
		if expanded, err := book.ExpandLocalPath(curDir); err == nil && expanded != "" {
			s.Export.Dir = expanded
		}
	}
	picked := strings.TrimSpace(r.FormValue(formKeyPickedDir))
	if picked == "" {
		picked = pickNativeBookPathUnlocked(s, true)
	}
	if picked != "" {
		if expanded, err := book.ExpandLocalPath(picked); err == nil && expanded != "" {
			picked = expanded
		}
		s.Export.Dir = picked
		s.Editor.ActiveBook.PersistExportDir(s.Config.SavesDir, s.Settings, picked)
	}
	return frameModals(s, "")
}

func resolveOrPickBookExportDir(s *state.StudioState, r *transport.ActionRequest) (string, string, bool) {
	b := s.Editor.ActiveBook
	if b == nil {
		return "", tr(s, i18n.TrNoActiveProject), false
	}
	if r != nil {
		if rawDir := strings.TrimSpace(r.FormValue(formKeyExportDir)); rawDir != "" {
			expanded, err := book.ExpandLocalPath(rawDir)
			if err != nil {
				return "", err.Error(), false
			}
			b.PersistExportDir(s.Config.SavesDir, s.Settings, expanded)
			return expanded, "", true
		}
	}
	forcePrompt := r != nil && r.FormValue(formKeyPromptDir) == boolValTrue
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
				r.Form.Set(formFieldSilent, boolValTrue)
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

func exportProjectAction(s *state.StudioState, r *transport.ActionRequest) []transport.ActionFrame {
	return frameModals(s, exportProjectToast(s, r))
}

func exportProjectToast(s *state.StudioState, r *transport.ActionRequest) string {
	b := s.Editor.ActiveBook
	if b == nil {
		return tr(s, i18n.TrNoActiveProject)
	}
	rawDir := strings.TrimSpace(r.FormValue(formKeyExportDir))
	if rawDir == "" {
		rawDir = strings.TrimSpace(s.Export.Dir)
	}
	if rawDir != "" && r != nil && r.Form != nil {
		r.Form.Set(formKeyExportDir, rawDir)
	}
	exportDir, errMsg, ok := resolveOrPickBookExportDir(s, r)
	if !ok {
		return errMsg
	}
	if err := os.MkdirAll(exportDir, 0755); err != nil {
		return err.Error()
	}

	var selectedOrder []string
	explicitFormat := strings.ToLower(strings.TrimSpace(r.FormValue(formKeyFormat)))
	if explicitFormat != "" {
		selectedOrder = []string{explicitFormat}
	} else if len(s.Export.Formats) > 0 {
		for _, id := range book.ExportFormats {
			if s.Export.Formats[id] {
				selectedOrder = append(selectedOrder, id)
			}
		}
	} else {
		fallback := strings.ToLower(strings.TrimSpace(s.Export.Format))
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
		return tr(s, i18n.TrSelectExportFormat)
	}

	expStd := strings.TrimSpace(r.FormValue(newBookFormStandard))
	if expStd == "" {
		expStd = s.Export.Standard
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
		return tr(s, i18n.TrNoExportedFile)
	}

	osbridge.RevealExportedFileInOS(exportedPaths[0])
	s.Export.Reset()

	if len(exportedPaths) == 1 {
		return tr(s, i18n.TrExportedSinglePrefix) + exportedPaths[0]
	}
	return trf(s, i18n.TrExportedMultiFmt, len(exportedPaths), exportDir)
}
