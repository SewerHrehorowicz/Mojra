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

func openStudioProjectModal(s *state.AppState, _ *http.Request) string {
	s.Modals.QR = false
	s.ModalNewBook.Open = false
	s.Modals.AddBook = false
	s.ModalExport.Open = false
	s.Modals.StudioProjects = true
	return ""
}

func closeStudioProjectModal(s *state.AppState, _ *http.Request) string {
	s.Modals.StudioProjects = false
	return ""
}

func openStudioProjectAction(s *state.AppState, r *http.Request) string {
	bookID := book.SanitizeBookID(r.FormValue("book"))
	if b := s.Library.Get(bookID); b != nil {
		return openBookInStudio(s, b)
	}
	return "Nie znaleziono projektu"
}

func browseOpenStudioProject(s *state.AppState, r *http.Request) string {
	if picked := pickDiskBookPath(s, r); picked != "" {
		return openStudioProjectFromDiskPath(s, picked)
	}
	return ""
}

func openStudioProjectFromDisk(s *state.AppState, r *http.Request) string {
	rawPath := strings.TrimSpace(r.FormValue("path"))
	return openStudioProjectFromDiskPath(s, rawPath)
}

func openStudioProjectFromDiskPath(s *state.AppState, rawPath string) string {
	if isScreenplayImportFile(rawPath) {
		return importScreenplayFromDiskFile(s, rawPath, true)
	}
	resolvedPaths, err := book.ResolveDiskBookPaths(rawPath)
	if err != nil {
		return err.Error()
	}
	firstPath := filepath.Clean(resolvedPaths[0])
	loaded, err := book.LoadBookFromPath(firstPath)
	if err != nil || loaded == nil {
		if err != nil {
			return err.Error()
		}
		return "Nie udało się otworzyć projektu"
	}
	if loaded.IsPacked {
		return promptUnpackPlaybookForStudio(s, loaded)
	}
	if filepath.Clean(filepath.Dir(firstPath)) != filepath.Clean(s.Config.BooksDir) {
		s.Settings.AddExternalBook(firstPath)
		loaded.IsExternal = true
	}
	s.Library.Put(loaded)
	return openBookInStudio(s, loaded)
}

func promptUnpackPlaybookForStudio(s *state.AppState, b *book.Book) string {
	if b == nil {
		return ""
	}
	s.CloseAllModals()
	s.Settings.ActiveView = state.ViewEditor.String()
	_ = s.SaveSettings()
	author := strings.TrimSpace(b.Author)
	if author == "" {
		author = tr(s, i18n.TrUnknownAuthor)
	}
	s.ModalConfirm.Prompt(state.ModalConfirmState{
		Title:         tr(s, i18n.TrConfirmPlaybookReadonlyTitle),
		Message:       trf(s, i18n.TrConfirmPlaybookUnpackMsgFmt, b.Title, author, b.EffectiveLicense()),
		PendingAction: UnpackPlaybookForStudio,
		PendingParam:  b.RootPath,
		Danger:        false,
	})
	return ""
}

func unpackPlaybookWorkingCopyForStudio(s *state.AppState, archivePath string) string {
	cleanArchive := filepath.Clean(strings.TrimSpace(archivePath))
	if cleanArchive == "" || cleanArchive == "." {
		return "Brak ścieżki archiwum .playbook"
	}
	packedBook, err := book.LoadBookFromPath(cleanArchive)
	if err != nil || packedBook == nil {
		return "Nie udało się odczytać archiwum .playbook"
	}
	projectsDir := book.DefaultProjectsDir()
	if osbridge.IsRunningUnderGoTestBinary() && book.IsTempDir(s.Config.RootDir) && !book.IsInsideRepoDir(s.Config.RootDir) {
		projectsDir = filepath.Join(s.Config.RootDir, "projects")
	}
	if projectsDir == "" {
		projectsDir = s.Config.BooksDir
	}
	_ = os.MkdirAll(projectsDir, 0755)
	baseID := book.SanitizeBookID(packedBook.ID)
	if baseID == "" {
		baseID = "projekt"
	}
	targetDir := filepath.Join(projectsDir, baseID)
	if !book.HasBookFilesInDir(targetDir) {
		if err := book.UnpackBookZip(cleanArchive, targetDir); err != nil {
			return "Błąd wypakowywania kopii roboczej: " + err.Error()
		}
	}
	unpacked, err := book.LoadBookFromPath(targetDir)
	if err != nil || unpacked == nil {
		return "Błąd wczytywania wypakowanego projektu"
	}
	if filepath.Clean(filepath.Dir(unpacked.RootPath)) != filepath.Clean(s.Config.BooksDir) {
		s.Settings.AddExternalBook(unpacked.RootPath)
		unpacked.IsExternal = true
	}
	s.Library.Put(unpacked)
	s.Editor.SetBook(unpacked, s.Settings)
	if s.Player.Book == nil {
		s.Player.SetBook(s.Config.SavesDir, s.Settings, unpacked)
	}
	s.Settings.ActiveView = state.ViewEditor.String()
	s.CloseAllModals()
	_ = s.SaveSettings()
	return fmt.Sprintf("Wypakowano kopię roboczą do Studio: %s (%s)", unpacked.Title, unpacked.EffectiveLicense())
}
