"use strict";

const vscode = require("vscode");
const fs = require("fs");
const path = require("path");

const HEADER_KEYS = [
  { label: "chapter:", detail: "Identyfikator rozdziału / aktu" },
  { label: "scene:", detail: "Identyfikator sceny" },
  { label: "title:", detail: "Tytuł książki, rozdziału lub sceny" },
  { label: "exit:", detail: "Wyjście ze sceny (cel: <id>, chapter:<id> lub END)" },
  { label: "text:", detail: "Tekst wyboru gracza pod exit:" },
  { label: "if:", detail: "Warunek wyboru lub bloku tekstu" },
  { label: "else:", detail: "Alternatywna gałąź bloku if:" },
  { label: "endif:", detail: "Domknięcie bloku if:" },
  { label: "set:", detail: "Zmiana zmiennych (np. a = true; b++)" },
  { label: "end: true", detail: "Oznaczenie sceny końcowej" },
  { label: "start:", detail: "Scena początkowa książki lub rozdziału" },
  { label: "image:", detail: "Ścieżka ilustracji (images/...)" },
  { label: "voice:", detail: "Ścieżka nagrania lektorskiego (voice/...)" },
  { label: "slugline:", detail: "Nagłówek sceny scenariuszowej (WN./PL.)" },
  { label: "synopsis:", detail: "Krótkie streszczenie sceny" },
  { label: "var:", detail: "Deklaracja zmiennej w book.playtxt" },
  { label: "enum:", detail: "Deklaracja typu wyliczeniowego w book.playtxt" },
  { label: "author:", detail: "Autor książki w book.playtxt" },
  { label: "description:", detail: "Opis książki w book.playtxt" },
  { label: "license:", detail: "Licencja książki w book.playtxt" },
  { label: "narrated:", detail: "Tryb narracji audio (true / false)" },
  { label: "imageless:", detail: "Tryb bez ilustracji (true / false)" }
];

function indexBookDirectory(dirPath) {
  const scenes = [];
  const chapters = [];
  const variables = [];
  let entries;
  try {
    entries = fs.readdirSync(dirPath, { withFileTypes: true });
  } catch (_) {
    return { scenes, chapters, variables };
  }

  for (const entry of entries) {
    if (!entry.isFile() || !entry.name.toLowerCase().endsWith(".playtxt")) {
      continue;
    }
    const fullPath = path.join(dirPath, entry.name);
    let raw;
    try {
      raw = fs.readFileSync(fullPath, "utf8");
    } catch (_) {
      continue;
    }
    const lines = raw.split(/\r?\n/);
    let pendingSceneLine = -1;
    let pendingSceneName = "";
    let inChapterBlock = false;
    let pendingChapterLine = -1;
    let pendingChapterName = "";

    for (let i = 0; i < lines.length; i++) {
      const trimmed = lines[i].trim();
      if (!trimmed || trimmed.startsWith("#") || trimmed.startsWith("//")) {
        if (!trimmed) {
          inChapterBlock = false;
        }
        continue;
      }
      const colonIdx = trimmed.indexOf(":");
      if (colonIdx <= 0) {
        inChapterBlock = false;
        continue;
      }
      const key = trimmed.slice(0, colonIdx).trim().toLowerCase();
      const val = trimmed.slice(colonIdx + 1).trim();

      if (key === "chapter" || key === "rozdział" || key === "rozdzial" || key === "act" || key === "akt") {
        inChapterBlock = true;
        pendingChapterLine = i;
        pendingChapterName = val;
        chapters.push({ id: val, name: val, file: fullPath, line: i });
      } else if (key === "scene" || key === "scena") {
        inChapterBlock = false;
        pendingSceneLine = i;
        pendingSceneName = val;
        scenes.push({ id: val, name: val, file: fullPath, line: i });
      } else if ((key === "title" || key === "tytuł" || key === "tytul") && val) {
        if (inChapterBlock && chapters.length > 0 && chapters[chapters.length - 1].line === pendingChapterLine) {
          chapters[chapters.length - 1].name = val;
        } else if (!inChapterBlock && scenes.length > 0 && scenes[scenes.length - 1].line === pendingSceneLine) {
          scenes[scenes.length - 1].name = val;
        }
      } else if (key === "id" && val) {
        if (inChapterBlock && pendingChapterLine >= 0) {
          chapters.push({ id: val, name: pendingChapterName || val, file: fullPath, line: pendingChapterLine });
        } else {
          const line = pendingSceneLine >= 0 ? pendingSceneLine : i;
          scenes.push({ id: val, name: pendingSceneName || val, file: fullPath, line });
        }
      } else if ((key === "var" || key === "zmienna") && val) {
        const eqIdx = val.indexOf("=");
        const varName = (eqIdx > 0 ? val.slice(0, eqIdx) : val).trim();
        if (varName) {
          variables.push({ name: varName, detail: val, file: fullPath, line: i });
        }
      }
    }
  }
  return { scenes, chapters, variables };
}

function provideDocumentSymbols(document) {
  const symbols = [];
  const lines = document.getText().split(/\r?\n/);
  let currentChapterSym = null;
  let currentSceneSym = null;

  for (let i = 0; i < lines.length; i++) {
    const trimmed = lines[i].trim();
    if (!trimmed || trimmed.startsWith("#") || trimmed.startsWith("//")) {
      continue;
    }
    const colonIdx = trimmed.indexOf(":");
    if (colonIdx > 0) {
      const key = trimmed.slice(0, colonIdx).trim().toLowerCase();
      const val = trimmed.slice(colonIdx + 1).trim();
      if (key === "chapter" || key === "rozdział" || key === "rozdzial" || key === "act" || key === "akt") {
        const range = new vscode.Range(i, 0, i, lines[i].length);
        currentChapterSym = new vscode.DocumentSymbol(
          val || "Rozdział",
          "chapter",
          vscode.SymbolKind.Module,
          range,
          range
        );
        symbols.push(currentChapterSym);
        currentSceneSym = null;
        continue;
      }
      if (key === "scene" || key === "scena") {
        const range = new vscode.Range(i, 0, i, lines[i].length);
        currentSceneSym = new vscode.DocumentSymbol(
          val || "Scena",
          "",
          vscode.SymbolKind.Function,
          range,
          range
        );
        if (currentChapterSym) {
          currentChapterSym.children.push(currentSceneSym);
        } else {
          symbols.push(currentSceneSym);
        }
        continue;
      }
      if (key === "title" || key === "tytuł" || key === "tytul") {
        if (currentSceneSym && !currentSceneSym.detail) {
          currentSceneSym.detail = currentSceneSym.name;
          currentSceneSym.name = val || currentSceneSym.name;
        } else if (currentChapterSym && currentChapterSym.detail === "chapter") {
          currentChapterSym.detail = currentChapterSym.name;
          currentChapterSym.name = val || currentChapterSym.name;
        }
        continue;
      }
      if (key === "id" && val) {
        if (currentSceneSym && !currentSceneSym.detail) {
          currentSceneSym.detail = "id: " + val;
        } else if (currentChapterSym && currentChapterSym.detail === "chapter") {
          currentChapterSym.detail = "id: " + val;
        }
        continue;
      }
    }
    if (currentSceneSym) {
      currentSceneSym.range = new vscode.Range(
        currentSceneSym.range.start,
        new vscode.Position(i, lines[i].length)
      );
    }
    if (currentChapterSym) {
      currentChapterSym.range = new vscode.Range(
        currentChapterSym.range.start,
        new vscode.Position(i, lines[i].length)
      );
    }
  }
  return symbols;
}

function provideDefinition(document, position) {
  const lineText = document.lineAt(position.line).text;
  let targetRef = "";

  const arrowIdx = lineText.lastIndexOf("->");
  if (arrowIdx >= 0 && !lineText.includes("\\->") && position.character >= arrowIdx) {
    targetRef = lineText.slice(arrowIdx + 2).trim();
  } else {
    const headerMatch = lineText.match(/^\s*(?:exit|next|dalej|start|first_scene|pierwsza_scena|wej[śs]cie)\s*:\s*(.+)$/i);
    if (headerMatch) {
      targetRef = headerMatch[1].trim();
    }
  }
  if (!targetRef || /^(END|KONIEC)$/i.test(targetRef)) {
    return null;
  }

  const dirPath = path.dirname(document.uri.fsPath);
  const { scenes, chapters } = indexBookDirectory(dirPath);

  const chMatch = targetRef.match(/^(?:chapter|rozdzia[łl]|act|akt):(.+)$/i);
  if (chMatch) {
    const chKey = chMatch[1].trim().toLowerCase();
    const foundCh = chapters.find((c) => c.id.toLowerCase() === chKey || c.name.toLowerCase() === chKey);
    if (foundCh) {
      return new vscode.Location(vscode.Uri.file(foundCh.file), new vscode.Position(foundCh.line, 0));
    }
    const chFile = path.join(dirPath, chKey + ".playtxt");
    if (fs.existsSync(chFile)) {
      return new vscode.Location(vscode.Uri.file(chFile), new vscode.Position(0, 0));
    }
    return null;
  }

  const lowerTarget = targetRef.toLowerCase();
  const foundScene = scenes.find((s) => s.id.toLowerCase() === lowerTarget || s.name.toLowerCase() === lowerTarget);
  if (foundScene) {
    return new vscode.Location(vscode.Uri.file(foundScene.file), new vscode.Position(foundScene.line, 0));
  }
  return null;
}

function provideCompletionItems(document, position) {
  const linePrefix = document.lineAt(position.line).text.slice(0, position.character);
  const dirPath = path.dirname(document.uri.fsPath);

  if (/->\s*[\p{L}\p{N}_\-:]*$/u.test(linePrefix) || /^\s*(?:exit|next|dalej|start)\s*:\s*[\p{L}\p{N}_\-:]*$/iu.test(linePrefix)) {
    const { scenes, chapters } = indexBookDirectory(dirPath);
    const seen = new Set();
    const items = [];

    const endItem = new vscode.CompletionItem("END", vscode.CompletionItemKind.Keyword);
    endItem.detail = "Zakończenie ścieżki";
    items.push(endItem);

    for (const sc of scenes) {
      const key = sc.id.toLowerCase();
      if (seen.has(key)) {
        continue;
      }
      seen.add(key);
      const item = new vscode.CompletionItem(sc.id, vscode.CompletionItemKind.Reference);
      item.detail = sc.name !== sc.id ? `${sc.name} (${path.basename(sc.file)})` : path.basename(sc.file);
      items.push(item);
    }
    for (const ch of chapters) {
      const ref = "chapter:" + ch.id;
      if (seen.has(ref.toLowerCase())) {
        continue;
      }
      seen.add(ref.toLowerCase());
      const item = new vscode.CompletionItem(ref, vscode.CompletionItemKind.Module);
      item.detail = ch.name;
      items.push(item);
    }
    return items;
  }

  const lastOpen = linePrefix.lastIndexOf("{{");
  const lastClose = linePrefix.lastIndexOf("}}");
  if ((lastOpen >= 0 && lastOpen > lastClose) || /^\s*(?:if|set|then)\s*:/iu.test(linePrefix)) {
    const { variables } = indexBookDirectory(dirPath);
    return variables.map((v) => {
      const item = new vscode.CompletionItem(v.name, vscode.CompletionItemKind.Variable);
      item.detail = v.detail;
      return item;
    });
  }

  if (/^\s*[\p{L}_]*$/u.test(linePrefix)) {
    return HEADER_KEYS.map((hk) => {
      const item = new vscode.CompletionItem(hk.label, vscode.CompletionItemKind.Keyword);
      item.detail = hk.detail;
      return item;
    });
  }

  return [];
}

function activate(context) {
  const selector = { language: "playtxt", scheme: "file" };
  const whiteTitleDecoration = vscode.window.createTextEditorDecorationType({
    color: "#ffffff"
  });
  const blueIdDecoration = vscode.window.createTextEditorDecorationType({
    color: "#58a6ff"
  });

  function updateWhiteTitleDecorations(editor) {
    if (!editor || !editor.document || editor.document.languageId !== "playtxt") {
      return;
    }
    const whiteRanges = [];
    const blueRanges = [];
    const doc = editor.document;
    for (let i = 0; i < doc.lineCount; i++) {
      const line = doc.lineAt(i).text;
      const mWhite = line.match(/^\s*(?:title|tytu[łl]|text|tekst|label|etykieta|author|autor|description|opis|summary|license|licencja|slugline|nag[łl][óo]wek|heading|synopsis|streszczenie)\s*:\s*(.+)$/i);
      if (mWhite && mWhite[1]) {
        const valStart = line.indexOf(mWhite[1], line.indexOf(":") + 1);
        if (valStart >= 0) {
          whiteRanges.push(new vscode.Range(i, valStart, i, line.length));
        }
        continue;
      }
      const mBlue = line.match(/^\s*(?:chapter|rozdzia[łl]|act|akt|scene|scena|id|exit|next|dalej|start)\s*:\s*(?:chapter:\s*)?(.+)$/i);
      if (mBlue && mBlue[1]) {
        const valStart = line.lastIndexOf(mBlue[1]);
        if (valStart >= 0) {
          blueRanges.push(new vscode.Range(i, valStart, i, line.length));
        }
      }
    }
    editor.setDecorations(whiteTitleDecoration, whiteRanges);
    editor.setDecorations(blueIdDecoration, blueRanges);
  }

  vscode.window.visibleTextEditors.forEach(updateWhiteTitleDecorations);
  context.subscriptions.push(
    whiteTitleDecoration,
    blueIdDecoration,
    vscode.window.onDidChangeActiveTextEditor(updateWhiteTitleDecorations),
    vscode.workspace.onDidChangeTextDocument((e) => {
      vscode.window.visibleTextEditors.forEach((ed) => {
        if (ed.document === e.document) {
          updateWhiteTitleDecorations(ed);
        }
      });
    }),
    vscode.languages.registerDocumentSymbolProvider(selector, { provideDocumentSymbols }),
    vscode.languages.registerDefinitionProvider(selector, { provideDefinition }),
    vscode.languages.registerCompletionItemProvider(selector, { provideCompletionItems }, ":", ">", "{", " ")
  );
}

function deactivate() {}

module.exports = { activate, deactivate };
