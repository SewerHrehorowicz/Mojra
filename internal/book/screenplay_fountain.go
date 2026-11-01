package book

import (
	"fmt"
	"strings"
	"time"
)

// LinearizeScreenplayScenes returns scenes in deterministic primary-path order starting from b.StartSceneID,
// followed by any remaining scenes in graph/chapter order so no scene is omitted.
func LinearizeScreenplayScenes(b *Book) []*Scene {
	if b == nil || len(b.Scenes) == 0 {
		return nil
	}
	byID := make(map[string]*Scene, len(b.Scenes))
	for _, sc := range b.Scenes {
		byID[strings.ToLower(sc.ID)] = sc
	}

	visited := make(map[string]bool, len(b.Scenes))
	var out []*Scene

	curr := byID[strings.ToLower(b.StartSceneID)]
	if curr == nil {
		curr = b.Scenes[0]
	}
	for curr != nil && !visited[strings.ToLower(curr.ID)] {
		visited[strings.ToLower(curr.ID)] = true
		out = append(out, curr)
		var nextSc *Scene
		for _, ch := range curr.Choices {
			if ch.IsEnding || ch.TargetSceneID == "" || strings.EqualFold(ch.TargetSceneID, "END") {
				continue
			}
			if cand := byID[strings.ToLower(ch.TargetSceneID)]; cand != nil && !visited[strings.ToLower(cand.ID)] {
				nextSc = cand
				break
			}
		}
		curr = nextSc
	}

	for _, sc := range b.Scenes {
		if !visited[strings.ToLower(sc.ID)] {
			visited[strings.ToLower(sc.ID)] = true
			out = append(out, sc)
		}
	}
	return out
}

// ExportScreenplayFountain exports a Book into a standard UTF-8 .fountain screenplay document.
func ExportScreenplayFountain(b *Book) string {
	if b == nil {
		return ""
	}
	normStd := b.NormalizedStandard()
	numbered := normStd == "pl" || normStd == "eu"

	title := strings.TrimSpace(b.Title)
	if title == "" {
		title = "Bez tytułu"
	}
	author := strings.TrimSpace(b.Author)
	if author == "" {
		author = "Autor"
	}

	branchName := "main"
	if h, err := LoadBookHistory(b); err == nil && h != nil {
		branchName = h.NormalizedActiveBranch()
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Title: %s\n", title)
	if normStd == "pl" {
		sb.WriteString("Credit: scenariusz\n")
	} else {
		sb.WriteString("Credit: Written by\n")
	}
	fmt.Fprintf(&sb, "Author: %s\n", author)
	if desc := strings.TrimSpace(b.Description); desc != "" {
		fmt.Fprintf(&sb, "Notes: %s\n", desc)
	}
	fmt.Fprintf(&sb, "Draft date: %s (%s)\n\n", time.Now().Format("2006-01-02"), branchName)

	scenes := LinearizeScreenplayScenes(b)
	lastAct := ""

	for i, sc := range scenes {
		sceneNum := i + 1
		actName := strings.TrimSpace(sc.Chapter)
		if actName != "" && !strings.EqualFold(actName, lastAct) {
			fmt.Fprintf(&sb, "# %s\n\n", actName)
			lastAct = actName
		}

		blocks := ParseScreenplaySceneBlocks(sc, normStd)
		hasSlug := len(blocks) > 0 && blocks[0].Kind == BlockSlugline
		if !hasSlug {
			slug := strings.ToUpper(strings.TrimSpace(sc.Slugline))
			if slug == "" {
				slug = DefaultSluglineForStandard(normStd)
			}
			blocks = append([]ScreenplayBlock{{Kind: BlockSlugline, Text: slug}}, blocks...)
		}

		renderedPrimarySlug := false
		for j, blk := range blocks {
			switch blk.Kind {
			case BlockSlugline:
				slugTxt := strings.ToUpper(strings.TrimSpace(blk.Text))
				if !renderedPrimarySlug {
					renderedPrimarySlug = true
					if numbered {
						if IsSluglineLine(slugTxt) && !strings.HasPrefix(slugTxt, ".") {
							fmt.Fprintf(&sb, "%s #%d#\n", slugTxt, sceneNum)
						} else {
							fmt.Fprintf(&sb, ".%s #%d#\n", slugTxt, sceneNum)
						}
					} else {
						if IsSluglineLine(slugTxt) && !strings.HasPrefix(slugTxt, ".") {
							fmt.Fprintf(&sb, "%s\n", slugTxt)
						} else {
							fmt.Fprintf(&sb, ".%s\n", slugTxt)
						}
					}
					if syn := strings.TrimSpace(sc.Synopsis); syn != "" {
						fmt.Fprintf(&sb, "= %s\n", syn)
					}
					sb.WriteString("\n")
				} else {
					fmt.Fprintf(&sb, "%s\n\n", slugTxt)
				}
			case BlockAction:
				fmt.Fprintf(&sb, "%s\n\n", strings.TrimSpace(blk.Text))
			case BlockCharacter:
				fmt.Fprintf(&sb, "%s\n", strings.TrimSpace(blk.Text))
			case BlockParenthetical:
				fmt.Fprintf(&sb, "%s\n", strings.TrimSpace(blk.Text))
			case BlockDialogue:
				fmt.Fprintf(&sb, "%s\n", strings.TrimSpace(blk.Text))
				// Add trailing blank line unless next block is a parenthetical continuing the same speech
				if j+1 >= len(blocks) || blocks[j+1].Kind != BlockParenthetical {
					sb.WriteString("\n")
				}
			case BlockTransition:
				trans := strings.TrimSpace(blk.Text)
				if !strings.HasSuffix(trans, "TO:") && !strings.HasSuffix(trans, "DO:") {
					fmt.Fprintf(&sb, "> %s\n\n", trans)
				} else {
					fmt.Fprintf(&sb, "%s\n\n", trans)
				}
			}
		}
	}

	return strings.TrimSpace(sb.String()) + "\n"
}
