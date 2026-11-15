//go:build !(js && wasm)

package actions

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"mojra/domain/book"
	"mojra/domain/i18n"
	"mojra/domain/state"
	"mojra/engine"
	"mojra/engine/osbridge"
)

const (
	formKeyBook          = "book"
	projectsSubDir       = "projects"
	defaultProjectBaseID = "projekt"
)

var _ = registerActions(map[string]ActionHandler{
	"OpenStudioProjectModal":    openStudioProjectModal,
	"CloseStudioProjectModal":   closeStudioProjectModal,
	"OpenStudioProject":         openStudioProjectAction,
	"BrowseOpenStudioProject":   browseOpenStudioProject,
	"OpenStudioProjectFromDisk": openStudioProjectFromDisk,
})

func openStudioProjectModal(s *state.AppState, _ *http.Request) []engine.ActionFrame {
	s.Overlays.QR = false
	s.NewBook.Open = false
	s.Overlays.AddBook = false
	s.Export.Open = false
	s.Overlays.StudioProjects = true
	return frameModals(s, "")
}

func closeStudioProjectModal(s *state.AppState, _ *http.Request) []engine.ActionFrame {
	s.Overlays.StudioProjects = false
	return frameModals(s, "")
}

func openStudioProjectAction(s *state.AppState, r *http.Request) []engine.ActionFrame {
	bookID := book.SanitizeBookID(r.FormValue(formKeyBook))
	if b := s.Library.Get(bookID); b != nil {
		return frameAppResetCanvas(s, openBookInStudio(s, b))
	}
	return frameAppResetCanvas(s, tr(s, i18n.TrProjectNotFound))
}

func browseOpenStudioProject(s *state.AppState, r *http.Request) []engine.ActionFrame {
	toast := ""
	if picked := pickDiskBookPath(s, r); picked != "" {
		toast = openStudioProjectFromDiskPath(s, picked)
	}
	return frameAppResetCanvas(s, toast)
}

func openStudioProjectFromDisk(s *state.AppState, r *http.Request) []engine.ActionFrame {
	rawPath := strings.TrimSpace(r.FormValue(formKeyPath))
	return frameAppResetCanvas(s, openStudioProjectFromDiskPath(s, rawPath))
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
		return tr(s, i18n.TrFailedToOpenProject)
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
	s.Confirm.Prompt(state.ConfirmState{
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
		return tr(s, i18n.TrMissingPlaybookArchive)
	}
	packedBook, err := book.LoadBookFromPath(cleanArchive)
	if err != nil || packedBook == nil {
		return tr(s, i18n.TrFailedToReadPlaybook)
	}
	projectsDir := book.DefaultProjectsDir()
	if osbridge.IsRunningUnderGoTestBinary() && book.IsTempDir(s.Config.RootDir) && !book.IsInsideRepoDir(s.Config.RootDir) {
		projectsDir = filepath.Join(s.Config.RootDir, projectsSubDir)
	}
	if projectsDir == "" {
		projectsDir = s.Config.BooksDir
	}
	_ = os.MkdirAll(projectsDir, 0755)
	baseID := book.SanitizeBookID(packedBook.ID)
	if baseID == "" {
		baseID = defaultProjectBaseID
	}
	targetDir := filepath.Join(projectsDir, baseID)
	if !book.HasBookFilesInDir(targetDir) {
		if err := book.UnpackBookZip(cleanArchive, targetDir); err != nil {
			return tr(s, i18n.TrUnpackWorkingCopyErrPrefix) + err.Error()
		}
	}
	unpacked, err := book.LoadBookFromPath(targetDir)
	if err != nil || unpacked == nil {
		return tr(s, i18n.TrLoadUnpackedErr)
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
	return trf(s, i18n.TrUnpackedToStudioFmt, unpacked.Title, unpacked.EffectiveLicense())
}
