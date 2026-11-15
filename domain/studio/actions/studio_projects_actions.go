//go:build !(js && wasm)

package actions

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	book "mojra/domain/book/actions"
	commonstate "mojra/domain/common/state"
	"mojra/domain/i18n"
	"mojra/domain/studio/state"
	"mojra/engine/osbridge"
	"mojra/engine/transport"
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

func pickDiskBookPath(s *state.StudioState, r *http.Request) string {
	mode := strings.ToLower(strings.TrimSpace(r.FormValue(formKeyMode)))
	return pickNativeBookPathUnlocked(s, mode != modeValFile)
}

func isScreenplayImportFile(rawPath string) bool {
	ext := strings.ToLower(filepath.Ext(strings.TrimSpace(rawPath)))
	return ext == extFountain || ext == extFDX
}

func OpenBookInStudio(s *state.StudioState, b *book.Book) string {
	return openBookInStudio(s, b)
}

func openBookInStudio(s *state.StudioState, b *book.Book) string {
	if b == nil {
		return ""
	}
	if b.IsPacked {
		return PromptUnpackPlaybookForStudio(s, b)
	}
	s.Editor.SetBook(b, s.Settings)
	if s.Hooks.OnBookMutated != nil {
		s.Hooks.OnBookMutated(b)
	}
	s.Settings.ActiveView = string(commonstate.ViewEditor)
	s.CloseTransientViews()
	s.Editor.RestoreBookUIState(s.Settings, true)
	s.Editor.SyncToSettings(s.Settings)
	s.SaveSettings()
	if strings.TrimSpace(b.License) != "" {
		return trf(s, i18n.TrOpenedInStudioLicenseFmt, b.Title, b.License)
	}
	return tr(s, i18n.TrOpenedInStudioPrefix) + b.Title
}

func importScreenplayFromDiskFile(s *state.StudioState, rawPath string, openInStudioFlag bool) string {
	expanded, err := book.ExpandLocalPath(rawPath)
	if err != nil {
		return err.Error()
	}
	destDir := book.ResolveNewBookDir(s.Settings)
	if destDir == "" {
		destDir = s.Config.BooksDir
	}
	imported, err := book.ImportScreenplayFileToDir(expanded, destDir)
	if err != nil {
		return err.Error()
	}
	if filepath.Clean(filepath.Dir(imported.RootPath)) != filepath.Clean(s.Config.BooksDir) {
		s.Settings.AddExternalBook(imported.RootPath)
		imported.IsExternal = true
	}
	if s.Hooks.OnBookMutated != nil {
		s.Hooks.OnBookMutated(imported)
	}
	s.Overlays.AddBook = false
	s.Overlays.StudioProjects = false
	if openInStudioFlag {
		return openBookInStudio(s, imported)
	}
	s.SaveSettings()
	return tr(s, i18n.TrImportedScreenplayPrefix) + imported.Title
}

func openStudioProjectModal(s *state.StudioState, _ *http.Request) []transport.ActionFrame {
	s.Overlays.QR = false
	s.Overlays.AddBook = false
	s.Export.Open = false
	s.Overlays.StudioProjects = true
	return frameModals(s, "")
}

func closeStudioProjectModal(s *state.StudioState, _ *http.Request) []transport.ActionFrame {
	s.Overlays.StudioProjects = false
	return frameModals(s, "")
}

func openStudioProjectAction(s *state.StudioState, r *http.Request) []transport.ActionFrame {
	bookID := book.SanitizeBookID(r.FormValue(formKeyBook))
	if s.Hooks.ListAllBooks != nil {
		for _, b := range s.Hooks.ListAllBooks() {
			if b != nil && b.ID == bookID {
				return frameAppResetCanvas(s, openBookInStudio(s, b))
			}
		}
	}
	return frameAppResetCanvas(s, tr(s, i18n.TrProjectNotFound))
}

func browseOpenStudioProject(s *state.StudioState, r *http.Request) []transport.ActionFrame {
	toast := ""
	if picked := pickDiskBookPath(s, r); picked != "" {
		toast = openStudioProjectFromDiskPath(s, picked)
	}
	return frameAppResetCanvas(s, toast)
}

func openStudioProjectFromDisk(s *state.StudioState, r *http.Request) []transport.ActionFrame {
	rawPath := strings.TrimSpace(r.FormValue(formKeyPath))
	return frameAppResetCanvas(s, openStudioProjectFromDiskPath(s, rawPath))
}

func openStudioProjectFromDiskPath(s *state.StudioState, rawPath string) string {
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
		return PromptUnpackPlaybookForStudio(s, loaded)
	}
	if filepath.Clean(filepath.Dir(firstPath)) != filepath.Clean(s.Config.BooksDir) {
		s.Settings.AddExternalBook(firstPath)
		loaded.IsExternal = true
	}
	if s.Hooks.OnBookMutated != nil {
		s.Hooks.OnBookMutated(loaded)
	}
	return openBookInStudio(s, loaded)
}

func PromptUnpackPlaybookForStudio(s *state.StudioState, b *book.Book) string {
	if b == nil {
		return ""
	}
	s.CloseTransientViews()
	s.Editor.SceneText.Close()
	s.Export.Reset()
	s.Settings.ActiveView = string(commonstate.ViewEditor)
	s.SaveSettings()
	author := strings.TrimSpace(b.Author)
	if author == "" {
		author = tr(s, i18n.TrUnknownAuthor)
	}
	s.Confirm.Prompt(commonstate.ConfirmState{
		Title:         tr(s, i18n.TrConfirmPlaybookReadonlyTitle),
		Message:       trf(s, i18n.TrConfirmPlaybookUnpackMsgFmt, b.Title, author, b.EffectiveLicense()),
		PendingAction: UnpackPlaybookForStudio,
		PendingParam:  b.RootPath,
		Danger:        false,
		OnAccept: func() []transport.ActionFrame {
			toast := UnpackPlaybookWorkingCopyForStudio(s, b.RootPath)
			return frameAppResetCanvas(s, toast)
		},
	})
	return ""
}

func UnpackPlaybookWorkingCopyForStudio(s *state.StudioState, archivePath string) string {
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
	if s.Hooks.OnBookMutated != nil {
		s.Hooks.OnBookMutated(unpacked)
	}
	s.Editor.SetBook(unpacked, s.Settings)
	s.Settings.ActiveView = string(commonstate.ViewEditor)
	s.CloseTransientViews()
	s.Editor.SceneText.Close()
	s.Export.Reset()
	s.SaveSettings()
	return trf(s, i18n.TrUnpackedToStudioFmt, unpacked.Title, unpacked.EffectiveLicense())
}
