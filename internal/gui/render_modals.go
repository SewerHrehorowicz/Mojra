package gui

import (
	"fmt"
	"strings"
)

func (s *Server) populateModalsContext(vars map[string]any) {
	if s.QRModalOpen && s.DevMode && !s.StandaloneMode {
		lanIP := DetectLANIP()
		port := s.Port
		if port == 0 {
			port = 8484
		}
		vars["lan_url"] = fmt.Sprintf("http://%s:%d", lanIP, port)
	}
	vars["show_qr_modal"] = s.QRModalOpen
	vars["show_confirm_modal"] = s.ConfirmPendingAction != ""
	vars["confirm_title"] = s.ConfirmTitle
	vars["confirm_message"] = s.ConfirmMessage
	confirmAcceptClass := "btn-primary"
	if s.ConfirmPendingAction == "RemoveBook" || s.ConfirmPendingAction == "DeleteScene" || s.ConfirmPendingAction == "DeleteScenes" || s.ConfirmPendingAction == "RemoveChoice" {
		confirmAcceptClass = "btn-danger"
	}
	vars["confirm_accept_class"] = confirmAcceptClass
	vars["show_new_book_modal"] = s.NewBookModalOpen
	vars["show_add_book_modal"] = s.AddBookModalOpen
	vars["show_editor_help_modal"] = s.EditorHelpModalOpen

	if s.SceneTextModalID != "" {
		if modalSc := s.findScene(s.SceneTextModalID); modalSc != nil {
			vars["show_scene_text_modal"] = true
			vars["scene_text_modal_id"] = modalSc.ID
			vars["scene_text_modal_name"] = modalSc.DisplayName()
			vars["scene_text_modal_raw_name"] = modalSc.Name
			vars["scene_text_modal_explicit_id"] = modalSc.ExplicitID
			vars["scene_text_modal_text"] = modalSc.Text

			isStart := false
			if s.ActiveBook != nil {
				if s.ActiveBook.StartSceneID != "" {
					isStart = strings.EqualFold(modalSc.ID, s.ActiveBook.StartSceneID)
				} else if len(s.ActiveBook.Scenes) > 0 {
					isStart = modalSc.ID == s.ActiveBook.Scenes[0].ID
				}
			}
			vars["scene_text_modal_is_start"] = isStart

			curImg, curImgLabel, imgOptions, curVoice, curVoiceLabel, voiceOptions := s.buildSceneMediaOptions(modalSc, true)
			vars["scene_text_modal_current_image"] = curImg
			vars["scene_text_modal_current_image_label"] = curImgLabel
			vars["scene_text_modal_image_options"] = imgOptions
			vars["scene_text_modal_book_narrated"] = s.ActiveBook != nil && s.ActiveBook.Narrated
			vars["scene_text_modal_current_voice"] = curVoice
			vars["scene_text_modal_current_voice_label"] = curVoiceLabel
			vars["scene_text_modal_voice_options"] = voiceOptions

			settingsOpenVal := "false"
			settingsActiveClass := ""
			settingsHiddenClass := "hidden"
			if s.SceneTextModalSettingsOpen {
				settingsOpenVal = "true"
				settingsActiveClass = "active"
				settingsHiddenClass = ""
			}
			vars["scene_text_modal_settings_open_val"] = settingsOpenVal
			vars["scene_text_modal_settings_active_class"] = settingsActiveClass
			vars["scene_text_modal_settings_hidden_class"] = settingsHiddenClass

			modalHeaderMeta := ""
			if s.ActiveBook != nil {
				modalHeaderMeta = s.ActiveBook.Title
			}
			if modalSc.Chapter != "" {
				if modalHeaderMeta != "" {
					modalHeaderMeta += " • " + modalSc.Chapter
				} else {
					modalHeaderMeta = modalSc.Chapter
				}
			}
			vars["scene_text_modal_header_meta"] = modalHeaderMeta

			imageless := s.isEffectiveImageless()
			modalImgURL := ""
			modalImgClass := ""
			modalImagelessClass := "is-imageless"
			if modalSc.ImageURL != "" {
				modalImgURL = modalSc.ImageURL
				modalImgClass = "has-illustration"
				if !imageless {
					modalImagelessClass = ""
				}
			}
			vars["scene_text_modal_has_image"] = modalImgURL != ""
			vars["scene_text_modal_image_url"] = modalImgURL
			vars["scene_text_modal_has_image_class"] = modalImgClass
			vars["scene_text_modal_imageless_class"] = modalImagelessClass

			var incomingScenes []map[string]any
			if s.ActiveBook != nil {
				seenIncoming := make(map[string]bool)
				for _, sc := range s.ActiveBook.Scenes {
					if sc == nil || sc.ID == modalSc.ID {
						continue
					}
					scHasMulti := len(sc.Choices) > 1
					for _, ch := range sc.Choices {
						if ch.IsEnding || !strings.EqualFold(ch.TargetSceneID, modalSc.ID) {
							continue
						}
						choiceLbl := strings.TrimSpace(ch.Label)
						if !scHasMulti && (ch.IsImplicit || strings.EqualFold(choiceLbl, "Dalej")) {
							choiceLbl = ""
						}
						dedupKey := sc.ID + "\x00" + choiceLbl
						if seenIncoming[dedupKey] {
							continue
						}
						seenIncoming[dedupKey] = true
						incomingScenes = append(incomingScenes, map[string]any{
							"scene_id":         sc.ID,
							"scene_name":       sc.DisplayName(),
							"choice_label":     choiceLbl,
							"has_choice_label": choiceLbl != "",
						})
					}
				}
			}
			vars["scene_text_modal_incoming"] = incomingScenes
			vars["scene_text_modal_has_incoming"] = len(incomingScenes) > 0

			hasMulti := len(modalSc.Choices) > 1
			modalChoices := make([]map[string]any, 0, len(modalSc.Choices))
			for idx, ch := range modalSc.Choices {
				lbl := ch.Label
				if !hasMulti {
					lbl = "Dalej"
				}
				arr := "»"
				if ch.IsEnding {
					arr = "↺"
				}
				selectedVal, selectedLabel, targetOptions := s.buildChoiceTargetOptions(modalSc, ch)
				portClass := ""
				if !ch.IsEnding && ch.TargetSceneID != "" && !strings.EqualFold(ch.TargetSceneID, "END") && !strings.EqualFold(selectedVal, "END") {
					portClass = "port-filled"
				}
				modalChoices = append(modalChoices, map[string]any{
					"idx":             idx,
					"label":           lbl,
					"arrow":           arr,
					"is_branch":       hasMulti && !ch.IsEnding,
					"is_editable":     hasMulti,
					"target_scene_id": ch.TargetSceneID,
					"is_ending":       ch.IsEnding,
					"selected_value":  selectedVal,
					"selected_label":  selectedLabel,
					"port_class":      portClass,
					"target_options":  targetOptions,
				})
			}
			vars["scene_text_modal_choices"] = modalChoices
			isTerminal := len(modalSc.Choices) == 0 || modalSc.ExplicitEnd
			vars["scene_text_modal_is_terminal"] = isTerminal
			termVal, termLabel, termOpts := s.buildChoiceTargetOptions(
				modalSc,
				s.terminalChoiceDescriptor(),
			)
			termPortClass := ""
			if termVal != "" && !strings.EqualFold(termVal, "END") {
				termPortClass = "port-filled"
			}
			vars["scene_text_modal_terminal_selected_value"] = termVal
			vars["scene_text_modal_terminal_selected_label"] = termLabel
			vars["scene_text_modal_terminal_port_class"] = termPortClass
			vars["scene_text_modal_terminal_target_options"] = termOpts
			startID := ""
			if s.ActiveBook != nil {
				startID = s.ActiveBook.StartSceneID
				if startID == "" && len(s.ActiveBook.Scenes) > 0 {
					startID = s.ActiveBook.Scenes[0].ID
				}
			}
			vars["scene_text_modal_start_id"] = startID
		} else {
			s.SceneTextModalID = ""
		}
	}
}
