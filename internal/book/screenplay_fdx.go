//go:build !(js && wasm)

package book

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

type fdxDocument struct {
	XMLName   xml.Name      `xml:"FinalDraft"`
	Content   fdxContent    `xml:"Content"`
	TitlePage *fdxTitlePage `xml:"TitlePage"`
}

type fdxContent struct {
	Paragraphs []fdxParagraph `xml:"Paragraph"`
}

type fdxTitlePage struct {
	Content fdxContent `xml:"Content"`
}

type fdxParagraph struct {
	Type            string              `xml:"Type,attr"`
	Number          string              `xml:"Number,attr"`
	SceneProperties *fdxSceneProperties `xml:"SceneProperties"`
	Texts           []string            `xml:"Text"`
}

type fdxSceneProperties struct {
	Title   string     `xml:"Title,attr"`
	Summary fdxSummary `xml:"Summary"`
}

type fdxSummary struct {
	Paragraphs []fdxParagraph `xml:"Paragraph"`
}

func (p fdxParagraph) plainText() string {
	return strings.TrimSpace(strings.Join(p.Texts, ""))
}

func xmlEscapeText(s string) string {
	var buf bytes.Buffer
	_ = xml.EscapeText(&buf, []byte(s))
	return buf.String()
}

// ExportScreenplayFDX exports the primary path of a Book into a Final Draft XML (.fdx) document.
func ExportScreenplayFDX(b *Book) string {
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
	credit := "scenariusz"
	if normStd != "pl" {
		credit = "Written by"
	}

	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="no" ?>` + "\n")
	sb.WriteString(`<FinalDraft DocumentType="Script" Template="No" Version="5">` + "\n")
	sb.WriteString("  <Content>\n")

	scenes := LinearizeScreenplayScenes(b)
	lastAct := ""

	for i, sc := range scenes {
		sceneNum := i + 1
		actName := strings.TrimSpace(sc.Chapter)
		if actName != "" && !strings.EqualFold(actName, lastAct) {
			fmt.Fprintf(&sb, "    <Paragraph Type=\"New Act\">\n      <Text>%s</Text>\n    </Paragraph>\n", xmlEscapeText(actName))
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
		for _, blk := range blocks {
			txt := strings.TrimSpace(blk.Text)
			if txt == "" {
				continue
			}
			switch blk.Kind {
			case BlockSlugline:
				slugTxt := strings.ToUpper(txt)
				if !renderedPrimarySlug {
					renderedPrimarySlug = true
					numAttr := ""
					if numbered {
						numAttr = fmt.Sprintf(` Number="%d"`, sceneNum)
					}
					fmt.Fprintf(&sb, "    <Paragraph Type=\"Scene Heading\"%s>\n", numAttr)
					scTitle := strings.TrimSpace(sc.Name)
					syn := strings.TrimSpace(sc.Synopsis)
					if scTitle != "" || syn != "" {
						fmt.Fprintf(&sb, "      <SceneProperties Title=\"%s\">\n", xmlEscapeText(scTitle))
						if syn != "" {
							fmt.Fprintf(&sb, "        <Summary>\n          <Paragraph>\n            <Text>%s</Text>\n          </Paragraph>\n        </Summary>\n", xmlEscapeText(syn))
						}
						sb.WriteString("      </SceneProperties>\n")
					}
					fmt.Fprintf(&sb, "      <Text>%s</Text>\n    </Paragraph>\n", xmlEscapeText(slugTxt))
				} else {
					fmt.Fprintf(&sb, "    <Paragraph Type=\"Scene Heading\">\n      <Text>%s</Text>\n    </Paragraph>\n", xmlEscapeText(slugTxt))
				}
			case BlockAction:
				fmt.Fprintf(&sb, "    <Paragraph Type=\"Action\">\n      <Text>%s</Text>\n    </Paragraph>\n", xmlEscapeText(txt))
			case BlockCharacter:
				fmt.Fprintf(&sb, "    <Paragraph Type=\"Character\">\n      <Text>%s</Text>\n    </Paragraph>\n", xmlEscapeText(txt))
			case BlockParenthetical:
				fmt.Fprintf(&sb, "    <Paragraph Type=\"Parenthetical\">\n      <Text>%s</Text>\n    </Paragraph>\n", xmlEscapeText(txt))
			case BlockDialogue:
				fmt.Fprintf(&sb, "    <Paragraph Type=\"Dialogue\">\n      <Text>%s</Text>\n    </Paragraph>\n", xmlEscapeText(txt))
			case BlockTransition:
				fmt.Fprintf(&sb, "    <Paragraph Type=\"Transition\">\n      <Text>%s</Text>\n    </Paragraph>\n", xmlEscapeText(txt))
			}
		}
	}

	sb.WriteString("  </Content>\n")
	sb.WriteString("  <TitlePage>\n    <Content>\n")
	fmt.Fprintf(&sb, "      <Paragraph Type=\"Title\">\n        <Text>%s</Text>\n      </Paragraph>\n", xmlEscapeText(title))
	fmt.Fprintf(&sb, "      <Paragraph Type=\"Credit\">\n        <Text>%s</Text>\n      </Paragraph>\n", xmlEscapeText(credit))
	fmt.Fprintf(&sb, "      <Paragraph Type=\"Author\">\n        <Text>%s</Text>\n      </Paragraph>\n", xmlEscapeText(author))
	if desc := strings.TrimSpace(b.Description); desc != "" {
		fmt.Fprintf(&sb, "      <Paragraph Type=\"Source\">\n        <Text>%s</Text>\n      </Paragraph>\n", xmlEscapeText(desc))
	}
	sb.WriteString("    </Content>\n  </TitlePage>\n")
	sb.WriteString("</FinalDraft>\n")
	return sb.String()
}

// ImportScreenplayFDX parses a Final Draft XML (.fdx) screenplay into a Playbook Studio node-graph Book saved under destParentDir.
func ImportScreenplayFDX(raw, fallbackTitle, destParentDir string) (*Book, error) {
	cleaned := strings.TrimPrefix(strings.TrimSpace(raw), "\xef\xbb\xbf")
	dec := xml.NewDecoder(strings.NewReader(cleaned))
	dec.Strict = false

	var doc fdxDocument
	if err := dec.Decode(&doc); err != nil && err != io.EOF {
		return nil, fmt.Errorf("nieprawidłowy plik Final Draft (.fdx): %w", err)
	}

	title := strings.TrimSpace(fallbackTitle)
	author := "Autor"
	description := ""

	if doc.TitlePage != nil {
		var titleCandidate, authorCandidate, descCandidate string
		for i, p := range doc.TitlePage.Content.Paragraphs {
			txt := p.plainText()
			if txt == "" {
				continue
			}
			pType := strings.ToLower(strings.TrimSpace(p.Type))
			switch {
			case strings.Contains(pType, "title") || (i == 0 && titleCandidate == ""):
				if titleCandidate == "" {
					titleCandidate = txt
				}
			case strings.Contains(pType, "author"):
				if authorCandidate == "" {
					authorCandidate = txt
				} else {
					authorCandidate += ", " + txt
				}
			case strings.Contains(pType, "source") || strings.Contains(pType, "note"):
				if descCandidate == "" {
					descCandidate = txt
				}
			}
		}
		if titleCandidate != "" {
			title = titleCandidate
		}
		if authorCandidate != "" {
			author = authorCandidate
		}
		if descCandidate != "" {
			description = descCandidate
		}
	}

	curAct := ""
	var drafts []ImportedSceneDraft
	var curScene *ImportedSceneDraft
	var bodyBlocks []string

	flushScene := func() {
		if curScene == nil {
			return
		}
		curScene.Body = strings.TrimSpace(strings.Join(bodyBlocks, "\n"))
		drafts = append(drafts, *curScene)
		curScene = nil
		bodyBlocks = nil
	}

	for _, p := range doc.Content.Paragraphs {
		pType := strings.ToLower(strings.TrimSpace(p.Type))
		txt := p.plainText()

		if pType == "new act" || pType == "section" || pType == "act" {
			if txt != "" {
				flushScene()
				curAct = txt
			}
			continue
		}

		if pType == "scene heading" || (pType == "" && IsSluglineLine(txt)) {
			flushScene()
			slug := strings.ToUpper(txt)
			scTitle := slug
			var synParts []string
			if p.SceneProperties != nil {
				if t := strings.TrimSpace(p.SceneProperties.Title); t != "" {
					scTitle = t
				}
				for _, sp := range p.SceneProperties.Summary.Paragraphs {
					if st := sp.plainText(); st != "" {
						synParts = append(synParts, st)
					}
				}
			}
			curScene = &ImportedSceneDraft{
				ActName:  curAct,
				Slugline: slug,
				Title:    scTitle,
				Synopsis: strings.Join(synParts, " "),
			}
			continue
		}

		if txt == "" {
			continue
		}
		if curScene == nil {
			curScene = &ImportedSceneDraft{
				ActName:  curAct,
				Slugline: "",
			}
		}

		switch pType {
		case "character":
			if len(bodyBlocks) > 0 {
				bodyBlocks = append(bodyBlocks, "")
			}
			bodyBlocks = append(bodyBlocks, strings.ToUpper(txt))
		case "parenthetical":
			if !strings.HasPrefix(txt, "(") {
				txt = "(" + txt + ")"
			}
			bodyBlocks = append(bodyBlocks, txt)
		case "dialogue":
			bodyBlocks = append(bodyBlocks, txt)
		case "transition":
			if len(bodyBlocks) > 0 {
				bodyBlocks = append(bodyBlocks, "")
			}
			bodyBlocks = append(bodyBlocks, strings.ToUpper(txt))
		default:
			if len(bodyBlocks) > 0 {
				bodyBlocks = append(bodyBlocks, "")
			}
			bodyBlocks = append(bodyBlocks, txt)
		}
	}
	flushScene()

	return buildImportedScreenplayBook(title, author, description, drafts, destParentDir)
}
