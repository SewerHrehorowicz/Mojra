# Plan refaktoryzacji `internal/actions/actions_modal_scene_text.go` → `internal/actions/actions_editor_scene_text.go`

## 1. Zmiana nazewnictwa i umiejscowienia w stanie (Modal → Panel Edytora)

Panel edycji treści sceny to główny widok roboczy Playbook Studio (`half` / `full`), a nie modal.

| Obecnie | Docelowo |
|---|---|
| `internal/actions/actions_modal_scene_text.go` | `internal/actions/actions_editor_scene_text.go` |
| `internal/state/state_modal_scene_text.go` (`ModalSceneTextState`) | `internal/state/state_editor_scene_text.go` (`EditorSceneTextState`) |
| `internal/views/view_modal_scene_text.go` (`ViewModalSceneText`) | `internal/views/view_editor_scene_text.go` (`ViewEditorSceneText`) |
| `s.ModalSceneText` (w `AppState`) | `s.Editor.SceneText` (wewnątrz `EditorState`) |

Dzięki przeniesieniu `SceneText` do `EditorState`, metody `e.SaveBookUIState(settings)`, `e.RestoreBookUIState(settings, openIfSaved)`, `e.SyncToSettings(settings)` i `e.SetBook(b, settings)` nie muszą już przyjmować dodatkowego wskaźnika `&s.ModalSceneText`.

---

## 2. Analiza problemów linia po linii w `actions_modal_scene_text.go`

### Linie 15–45: `openSceneTextModal`
- **L17–29**: Ręczne szukanie `scene` / `scene_id` / `SelectedSceneID` / `StartSceneID` / `Scenes[0].ID` — warto wydzielić helper `resolveEditorScene(s, r)` lub metodę `b.ResolveSceneOrStart(ids...)`.
- **L37–39**: `if !s.ModalSceneText.IsOpen() { s.ModalSceneText.WidthMode = state.WidthModeHalf }` nadpisuje przywrócony z `settings.ini` tryb szerokości, jeśli panel był zamknięty.
- **L41–42**: Ręczne przypisanie `SceneID = sc.ID` i `InitialID = sc.ID` zamiast wywołania `s.Editor.SceneText.Open(sc.ID)` (przez co `ChoiceRulesOpen`, `SyntaxHelpOpen` i `TestVarsOpen` nie są czyszczone przy otwarciu).

### Linie 47–58: `setSceneTextWidthModeAction`
- **L48–56**: Ręczne `strings.ToLower(strings.TrimSpace(...))` i `if mode == "full"` duplikuje gotowy parser `state.ParseSceneWidthMode(raw)` z `state_modal_scene_text.go:19`.
- **Powinno być**:
  ```go
  raw := r.FormValue("mode")
  if raw == "" {
      raw = r.FormValue("width")
  }
  s.Editor.SceneText.WidthMode = state.ParseSceneWidthMode(raw)
Linie 60–84: closeSceneTextModal
L63–78: Logika warunkowego zapisu niezapisanych zmian formularza przy zamykaniu/nawigacji/usuwaniu wyboru jest zduplikowana w 3 miejscach:
closeSceneTextModal (L63–78)
navigateSceneTextModal (L135–153)
promptRemoveChoiceFromScene w actions_modal_confirm.go (L245–260) Należy wydzielić jeden wspólny helper flushSceneTextChanges(s, r) (saved bool, errToast string).
L70: Zahardkodowany polski string "Edycja treści sceny: " + curScLabel zamiast klucza i18n trf(s, i18n.TrHistSceneTextEditedFmt, curScLabel).
L70–74: Kruche wykrywanie błędu zapisu przez strings.HasPrefix(mutMsg, "Edycja treści sceny: ") — po zmianie języka na EN ten warunek przestaje działać i wyświetla zwykły komunikat historii jako toast błędu.
L79–81: Ręczne zerowanie SceneID = "", InitialID = "", TestVarsOpen = false zamiast s.Editor.SceneText.Close(), przez co w pamięci zostają stare PreviewMode, TestVars, ChoiceRulesOpen i SyntaxHelpOpen.
Linie 91–125: saveSceneTextModal
L96–98: Parsowanie scene_settings_open powtarza się w wielu akcjach (actions_modal_confirm.go:230, actions_library.go:144, actions_editor_scene.go:714,831) — wydzielić syncSceneTextSettingsOpen(s, r).
L107–122: Ręczne wywoływanie book.FinalizeBook(b), book.SaveBookScenesToDisk(b), reloadActiveBook(s) i s.Editor.History.RecordStep(...) duplikuje funkcję commitScenesMutation(s, toast, "text", "textmodal:"+targetID).
L111–112: Ręczne SceneID = target.ID i InitialID = target.ID zamiast metody stanu.
L120: Zahardkodowany polski string "Zapisano treść sceny: " + targetLabel zamiast trf(s, i18n.TrSceneTextSavedFmt, targetLabel).
Linie 127–198: navigateSceneTextModal
L135–153: Ręczna duplikacja auto-zapisu zmian przed przejściem do kolejnej sceny (z pominięciem commitScenesMutation i z zahardkodowanym "Edycja treści sceny: "). Zastąpić wywołaniem wspólnego flushSceneTextChanges(s, r).
L159–176: Wyliczanie docelowej sceny (targetID) i efektów wyboru (chosenEffects) na podstawie choice_idx warto wydzielić do czystej metody modelu curSc.ResolveChoiceNavigation(choiceIdx, b.StartSceneID).
L178–186: Aplikowanie efektów zmiennych testowych przenieść do metody s.Editor.SceneText.ApplyNavigationEffects(b, chosenEffects, nextSc.Effects).
L187–195: Zduplikowane ręczne ustawianie SelectedSceneID, SceneID, InitialID, SelectedVoiceSectionKey = "01" w if i else if.
Linie 200–237: toggleChoiceRuleAction
L202–205: Ręczny reset ChoiceRulesOpen = nil i SyntaxHelpOpen = false gdy książka nie ma zmiennych.
L207–213: Ponowne ręczne szukanie scene / scene_id / SelectedSceneID.
L218–235: Logika przełączania mapy ChoiceRulesOpen[key] z fallbackiem na hasRule powinna być metodą stanu s.Editor.SceneText.ToggleChoiceRule(scID, idxStr, hasRule).
Linie 248–273: setSceneTextPreviewModeAction
L254–271: Ręczny switch mode po "highlight", "output", "normal" duplikuje state.ParseScenePreviewMode(raw) z state_modal_scene_text.go:50.
Powinno być:
raw := strings.TrimSpace(r.FormValue("mode"))
if raw == "" {
    s.Editor.SceneText.TogglePreviewMode()
} else {
    s.Editor.SceneText.PreviewMode = state.ParseScenePreviewMode(raw)
}
if s.Editor.SceneText.PreviewMode != state.PreviewModeNormal {
    s.Editor.SceneText.TestVars = book.EnsureSaveVariables(b, s.Editor.SceneText.TestVars)
}
Linie 288–334: updateSceneTextTestVarAction
L295–332: Cała walidacja i mutacja zmiennych testowych (toggle, delta, VarKindBool z zahardkodowanym "tak", VarKindInt, VarKindEnum) leży bezpośrednio w handlerze HTTP zamiast w metodzie domenowej book.UpdateTestVariable(vars, b, varName, val, deltaStr, isToggle).
3. Nowe klucze w internal/i18n/
Dodać w internal/i18n/i18n.go oraz internal/i18n/dict.go:

TrSceneTextSavedFmt:
PL: "Zapisano treść sceny: %s"
EN: "Saved scene text: %s"
TrHistSceneTextEditedFmt:
PL: "Edycja treści sceny: %s"
EN: "Edited scene text: %s"
4. Docelowy kształt internal/actions/actions_editor_scene_text.go
//go:build !(js && wasm)

package actions

import (
	"net/http"
	"strconv"
	"strings"

	"mojra/internal/book"
	"mojra/internal/i18n"
	"mojra/internal/state"
)

func syncSceneTextSettingsOpen(s *state.AppState, r *http.Request) {
	if r != nil {
		if _, ok := r.Form["scene_settings_open"]; ok {
			s.Editor.SceneText.SettingsOpen = r.FormValue("scene_settings_open") == "true"
		}
	}
}

func resolveSceneTextTarget(s *state.AppState, r *http.Request) *book.Scene {
	b := s.Editor.ActiveBook
	if b == nil {
		return nil
	}
	for _, cand := range []string{
		r.FormValue("scene"),
		r.FormValue("scene_id"),
		s.Editor.SceneText.SceneID,
		s.Editor.SelectedSceneID,
		b.StartSceneID,
	} {
		if id := strings.TrimSpace(cand); id != "" {
			if sc := b.FindScene(id); sc != nil {
				return sc
			}
		}
	}
	if len(b.Scenes) > 0 {
		return b.Scenes[0]
	}
	return nil
}

func flushSceneTextChanges(s *state.AppState, r *http.Request) (saved bool, errToast string) {
	if isHistoryPreview(s) || !s.Editor.SceneText.IsOpen() {
		return false, ""
	}
	b := s.Editor.ActiveBook
	curID := strings.TrimSpace(r.FormValue("scene_id"))
	if curID == "" {
		curID = s.Editor.SceneText.SceneID
	}
	curSc := b.FindScene(curID)
	if curSc == nil || b == nil || b.IsPacked || !hasSceneTextModalChanges(s, curSc, r) {
		return false, ""
	}
	s.Editor.History.EnsureBaseline(b, s.Editor.SelectedSceneID)
	applySceneFormFields(s, curSc, r)
	curScID := curSc.ID
	histLabel := trf(s, i18n.TrHistSceneTextEditedFmt, curSc.DisplayName())
	mutMsg := commitScenesMutation(s, histLabel, "text", "textmodal:"+curScID)
	s.Editor.SelectedSceneID = curScID
	s.Editor.SceneText.SyncSceneID(curScID)
	if mutMsg != histLabel {
		return false, mutMsg
	}
	return true, ""
}

func openSceneTextModal(s *state.AppState, r *http.Request) string {
	sc := resolveSceneTextTarget(s, r)
	if sc == nil {
		return ""
	}
	if s.Editor.SelectedSceneID != sc.ID || s.Editor.SceneText.SceneID != sc.ID {
		s.Editor.SelectedVoiceSectionKey = "01"
	}
	if s.Editor.SelectedSceneID != sc.ID {
		s.Editor.SceneText.SettingsOpen = false
	}
	s.Editor.SelectedSceneID = sc.ID
	s.Editor.SceneText.Open(sc.ID)
	return ""
}

func setSceneTextWidthModeAction(s *state.AppState, r *http.Request) string {
	raw := r.FormValue("mode")
	if raw == "" {
		raw = r.FormValue("width")
	}
	s.Editor.SceneText.WidthMode = state.ParseSceneWidthMode(raw)
	return ""
}

func closeSceneTextModal(s *state.AppState, r *http.Request) string {
	_, errToast := flushSceneTextChanges(s, r)
	s.Editor.SceneText.Close()
	s.Editor.SelectedVoiceSectionKey = "01"
	return errToast
}

func toggleSceneTextSettings(s *state.AppState, _ *http.Request) string {
	s.Editor.SceneText.SettingsOpen = !s.Editor.SceneText.SettingsOpen
	return ""
}

func saveSceneTextModal(s *state.AppState, r *http.Request) string {
	if isHistoryPreview(s) {
		return tr(s, i18n.TrHistoryPreviewLocked)
	}
	syncSceneTextSettingsOpen(s, r)
	b := s.Editor.ActiveBook
	target := resolveSceneTextTarget(s, r)
	if target == nil || b == nil || b.IsPacked {
		return ""
	}
	prevID := book.SanitizeSceneID(r.FormValue("scene_id"))
	s.Editor.History.EnsureBaseline(b, s.Editor.SelectedSceneID)
	applySceneFormFields(s, target, r)
	if prevID != "" && !strings.EqualFold(prevID, target.ID) {
		s.Editor.LastRenamedFromSceneID = prevID
	}
	s.Editor.SelectedSceneID = target.ID
	s.Editor.SceneText.SyncSceneID(target.ID)
	toast := trf(s, i18n.TrSceneTextSavedFmt, target.DisplayName())
	return commitScenesMutation(s, toast, "text", "textmodal:"+target.ID)
}

func navigateSceneTextModal(s *state.AppState, r *http.Request) string {
	s.Editor.SceneText.SettingsOpen = false
	_, errToast := flushSceneTextChanges(s, r)
	b := s.Editor.ActiveBook
	if b == nil {
		return errToast
	}
	curSc := b.FindScene(s.Editor.SceneText.SceneID)
	targetID := strings.TrimSpace(r.FormValue("scene"))
	if targetID == "" {
		targetID = strings.TrimSpace(r.FormValue("target_scene"))
	}
	var chosenEffects string
	if choiceIdx, err := strconv.Atoi(strings.TrimSpace(r.FormValue("choice_idx"))); err == nil && curSc != nil {
		targetID, chosenEffects = curSc.ResolveChoiceNavigation(choiceIdx, targetID, b.StartSceneID)
	}
	if strings.EqualFold(targetID, "END") {
		targetID = b.StartSceneID
	}
	nextSc := b.FindScene(targetID)
	if nextSc == nil && len(b.Scenes) > 0 && targetID == "" {
		nextSc = b.Scenes[0]
	}
	if nextSc != nil {
		s.Editor.SceneText.ApplyNavigationEffects(b, chosenEffects, nextSc.Effects)
		s.Editor.SelectedSceneID = nextSc.ID
		s.Editor.SceneText.SyncSceneID(nextSc.ID)
		s.Editor.SelectedVoiceSectionKey = "01"
	}
	return errToast
}

func toggleChoiceRuleAction(s *state.AppState, r *http.Request) string {
	b := s.Editor.ActiveBook
	if b == nil || len(b.Variables) == 0 {
		s.Editor.SceneText.ResetRulesUI()
		return ""
	}
	sc := resolveSceneTextTarget(s, r)
	idxStr := strings.TrimSpace(r.FormValue("choice_idx"))
	if sc == nil || idxStr == "" {
		return ""
	}
	hasRule := false
	if chIdx, err := strconv.Atoi(idxStr); err == nil && chIdx >= 0 && chIdx < len(sc.Choices) {
		ch := sc.Choices[chIdx]
		hasRule = strings.TrimSpace(ch.Condition) != "" || strings.TrimSpace(ch.Effects) != ""
	}
	s.Editor.SceneText.ToggleChoiceRule(sc.ID, idxStr, hasRule)
	return ""
}

func toggleSyntaxHelpAction(s *state.AppState, _ *http.Request) string {
	if s.Editor.ActiveBook == nil || len(s.Editor.ActiveBook.Variables) == 0 {
		s.Editor.SceneText.SyntaxHelpOpen = false
		return ""
	}
	s.Editor.SceneText.SyntaxHelpOpen = !s.Editor.SceneText.SyntaxHelpOpen
	return ""
}

func setSceneTextPreviewModeAction(s *state.AppState, r *http.Request) string {
	b := s.Editor.ActiveBook
	if b == nil || len(b.Variables) == 0 {
		s.Editor.SceneText.PreviewMode = state.PreviewModeNormal
		return ""
	}
	if raw := strings.TrimSpace(r.FormValue("mode")); raw != "" {
		s.Editor.SceneText.PreviewMode = state.ParseScenePreviewMode(raw)
	} else {
		s.Editor.SceneText.TogglePreviewMode()
	}
	if s.Editor.SceneText.PreviewMode != state.PreviewModeNormal {
		s.Editor.SceneText.TestVars = book.EnsureSaveVariables(b, s.Editor.SceneText.TestVars)
	}
	return ""
}

func toggleSceneTestVarsPopoverAction(s *state.AppState, _ *http.Request) string {
	b := s.Editor.ActiveBook
	if b == nil || len(b.Variables) == 0 {
		s.Editor.SceneText.TestVarsOpen = false
		return ""
	}
	s.Editor.SceneText.TestVarsOpen = !s.Editor.SceneText.TestVarsOpen
	if s.Editor.SceneText.TestVarsOpen {
		s.Editor.SceneText.TestVars = book.EnsureSaveVariables(b, s.Editor.SceneText.TestVars)
	}
	return ""
}

func updateSceneTextTestVarAction(s *state.AppState, r *http.Request) string {
	b := s.Editor.ActiveBook
	varName := book.SanitizeVarName(r.FormValue("var_name"))
	if varName == "" || b == nil {
		return ""
	}
	val := strings.TrimSpace(r.FormValue("var_value"))
	if val == "" {
		val = strings.TrimSpace(r.FormValue("var_val"))
	}
	s.Editor.SceneText.TestVars = book.UpdateTestVariable(
		s.Editor.SceneText.TestVars,
		b,
		varName,
		val,
		r.FormValue("delta"),
		r.FormValue("toggle") == "true",
	)
	return ""
}