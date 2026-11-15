package transport

import (
	"mojra/engine"
	"mojra/engine/templates"
)

// RenderActionFrames converts domain ActionFrames and global script chunks into wire-ready FDPFrames.
func RenderActionFrames(
	lang engine.Lang,
	actionFrames []ActionFrame,
	globalScriptChunks []templates.TemplateChunk,
	loadedTemplates map[string]*templates.ParsedTemplate,
) ([]FDPFrame, error) {
	fdpFrames := make([]FDPFrame, 0, len(actionFrames)+len(globalScriptChunks))
	for _, actionFrame := range actionFrames {
		if actionFrame.Target == fdpTargetNone {
			for _, chunk := range actionFrame.Chunks {
				fdpFrames = append(fdpFrames, FDPFrame{
					Target: fdpTargetNone,
					Chunk:  chunk,
				})
			}
			continue
		}
		if len(actionFrame.Chunks) == 0 {
			fdpFrames = append(fdpFrames, FDPFrame{
				Target: actionFrame.Target,
			})
			continue
		}
		htmlBuf := templates.AcquireBuffer()
		for _, chunk := range actionFrame.Chunks {
			tplName, tplData := chunk.Resolve()
			if tplName != "" && loadedTemplates != nil {
				ctx := engine.NewViewContext(lang, tplData)
				if renderErr := templates.RenderTemplateTo(htmlBuf, tplName, loadedTemplates, ctx); renderErr != nil {
					templates.ReleaseBuffer(htmlBuf)
					return nil, renderErr
				}
			}
		}
		bodyCopy := append([]byte(nil), htmlBuf.Bytes()...)
		templates.ReleaseBuffer(htmlBuf)

		fdpFrames = append(fdpFrames, FDPFrame{
			Target:   actionFrame.Target,
			HTMLBody: bodyCopy,
			Chunk:    actionFrame.Chunks[0],
		})
		for idx := 1; idx < len(actionFrame.Chunks); idx++ {
			if actionFrame.Chunks[idx].ScriptData != nil {
				fdpFrames = append(fdpFrames, FDPFrame{
					Target: fdpTargetNone,
					Chunk:  templates.TemplateChunk{ScriptData: actionFrame.Chunks[idx].ScriptData},
				})
			}
		}
	}

	for _, scriptChunk := range globalScriptChunks {
		fdpFrames = append(fdpFrames, FDPFrame{
			Target: fdpTargetNone,
			Chunk:  scriptChunk,
		})
	}
	return fdpFrames, nil
}

// RenderActionFramesToFDP encodes domain ActionFrames into a multi-frame FDP wire string.
func RenderActionFramesToFDP(
	lang engine.Lang,
	actionFrames []ActionFrame,
	globalScriptChunks []templates.TemplateChunk,
	loadedTemplates map[string]*templates.ParsedTemplate,
) string {
	fdpFrames, renderErr := RenderActionFrames(lang, actionFrames, globalScriptChunks, loadedTemplates)
	if renderErr != nil {
		return ""
	}
	buf := templates.AcquireBuffer()
	defer templates.ReleaseBuffer(buf)
	_ = WriteFDPFrames(buf, fdpFrames...)
	return buf.String()
}
