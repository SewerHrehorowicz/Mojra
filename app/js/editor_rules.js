"use strict";

function parseBookVarsSpec(specStr) {
  const map = new Map();
  if (!specStr) return map;
  specStr.split(";").forEach((entry) => {
    if (!entry) return;
    const parts = entry.split(":");
    const name = (parts[0] || "").trim().toLowerCase();
    if (!name) return;
    const kind = (parts[1] || "bool").trim().toLowerCase();
    const opts = (parts[2] || "")
      .split(",")
      .map((o) => o.trim().toLowerCase())
      .filter(Boolean);
    map.set(name, { kind, opts });
  });
  return map;
}

const PLAYTXT_MUT_OPS = ["++", "--", "+=", "-="];

function isClientBoolLiteral(s) {
  return isPlaytxtBoolTrue(s) || isPlaytxtBoolFalse(s);
}

function explainClientVarValueError(varName, vDef, op, rhs, varsMap) {
  const cleanRhs = (rhs || "").replace(/^["']|["']$/g, "").trim().toLowerCase();
  if (!cleanRhs) {
    return "Błąd składni: brak wartości po operatorze '" + op + "' dla zmiennej '" + varName + "'";
  }
  if (vDef.kind === "int") {
    if (/^-?\d+$/.test(cleanRhs)) return "";
    const rhsVar = varsMap.get(cleanRhs);
    if (rhsVar && rhsVar.kind === "int") return "";
    return "Błąd typu: zmienna '" + varName + "' (int) wymaga liczby całkowitej, otrzymano '" + cleanRhs + "'";
  }
  if (vDef.kind === "enum") {
    if (op !== "==" && op !== "!=" && op !== "=") {
      return "Błąd operatora: zmienna '" + varName + "' (enum) obsługuje tylko '=' / '==' lub '!='";
    }
    if (vDef.opts.includes(cleanRhs)) return "";
    const rhsVar = varsMap.get(cleanRhs);
    if (rhsVar && rhsVar.kind === "enum") return "";
    return "Nieznana wartość '" + cleanRhs + "' dla zmiennej '" + varName + "' (dozwolone: " + vDef.opts.join(", ") + ")";
  }
  if (op !== "==" && op !== "!=" && op !== "=") {
    return "Błąd operatora: zmienna '" + varName + "' (bool) obsługuje tylko '=' / '==' lub '!='";
  }
  if (isClientBoolLiteral(cleanRhs)) return "";
  if (cleanRhs.startsWith("!")) {
    const negVar = varsMap.get(cleanRhs.slice(1).trim());
    if (negVar && negVar.kind === "bool") return "";
  }
  const rhsVar = varsMap.get(cleanRhs);
  if (rhsVar && rhsVar.kind === "bool") return "";
  return "Błąd typu: zmienna '" + varName + "' (bool) wymaga wartości true/false lub samej nazwy zmiennej";
}

function explainClientCondExprError(raw, varsMap) {
  let s = (raw || "").trim();
  if (s.startsWith("{{") && s.endsWith("}}") && s.length >= 4) {
    s = s.slice(2, -2).trim();
  }
  if (s.startsWith("?")) s = s.slice(1).trim();
  if (!s) return "";
  if (varsMap.size === 0) return "Brak zadeklarowanych zmiennych w książce";
  let depth = 0;
  for (let i = 0; i < s.length; i++) {
    if (s[i] === "(") depth++;
    else if (s[i] === ")") {
      depth--;
      if (depth < 0) return "Błąd składni: niedomknięty nawias w warunku";
    }
  }
  if (depth !== 0) return "Błąd składni: niedomknięty nawias w warunku";
  const clauses = s.split(/\|\||&&/);
  for (let i = 0; i < clauses.length; i++) {
    let c = clauses[i].replace(/^[()\s]+|[()\s]+$/g, "");
    while ((c.startsWith("!") && !c.startsWith("!=")) || c.startsWith("?")) {
      c = c.slice(1).replace(/^[()\s]+|[()\s]+$/g, "");
    }
    if (!c) return "Błąd składni: brak wyrażenia po operatorze logicznym";
    for (let m = 0; m < PLAYTXT_MUT_OPS.length; m++) {
      if (c.indexOf(PLAYTXT_MUT_OPS[m]) >= 0) {
        return "Błąd w if: operator '" + PLAYTXT_MUT_OPS[m] + "' służy do zmiany wartości w polu set:, a w if: podaj porównanie";
      }
    }
    if (c.indexOf(",") >= 0 || c.indexOf(";") >= 0) {
      return "Błąd składni w if: użyj '&&' (oraz) lub '||' (lub) zamiast przecinka/średnika";
    }
    let matchedOp = "";
    let opIdx = -1;
    for (let j = 0; j < PLAYTXT_CMP_OPS.length; j++) {
      const idx = c.indexOf(PLAYTXT_CMP_OPS[j]);
      if (idx >= 0) {
        matchedOp = PLAYTXT_CMP_OPS[j];
        opIdx = idx;
        break;
      }
    }
    if (opIdx >= 0) {
      const lhs = c.slice(0, opIdx).trim().toLowerCase();
      const rhs = c.slice(opIdx + matchedOp.length).trim();
      if (!lhs) return "Błąd składni: brak nazwy zmiennej przed operatorem '" + matchedOp + "'";
      if (!rhs) return "Błąd składni: brak wartości po operatorze '" + matchedOp + "' dla zmiennej '" + lhs + "'";
      const vDef = varsMap.get(lhs);
      if (!vDef) return "Nieznana zmienna '" + lhs + "' — zadeklaruj ją w panelu Zmienne {}";
      const errMsg = explainClientVarValueError(lhs, vDef, matchedOp, rhs, varsMap);
      if (errMsg) return errMsg;
    } else {
      const varName = c.trim().toLowerCase();
      if (/[\s=!<>+\-(),:|&]/.test(varName)) {
        return "Błąd składni w warunku: '" + c.trim() + "'";
      }
      if (!varsMap.has(varName)) {
        return "Nieznana zmienna '" + varName + "' — zadeklaruj ją w panelu Zmienne {}";
      }
    }
  }
  return "";
}

function splitClientSpaceEffectChunk(chunk) {
  if (
    chunk.includes("==") ||
    chunk.includes("!=") ||
    chunk.includes(">=") ||
    chunk.includes("\x3c=") ||
    chunk.includes(">") ||
    chunk.includes("\x3c") ||
    chunk.includes("&&") ||
    chunk.includes("||")
  ) {
    return [chunk];
  }
  let norm = "";
  for (let i = 0; i < chunk.length; i++) {
    const ch = chunk.charAt(i);
    if ((ch === "+" || ch === "-") && i + 1 < chunk.length && chunk.charAt(i + 1) === "=") {
      norm += " " + ch + "= ";
      i++;
      continue;
    }
    if (ch === "=") {
      norm += " = ";
      continue;
    }
    norm += ch;
  }
  const tokens = norm.trim().split(/\s+/).filter(Boolean);
  if (tokens.length <= 1) return [chunk];
  const clauses = [];
  for (let i = 0; i < tokens.length; ) {
    const tok = tokens[i];
    if (i + 1 < tokens.length && (tokens[i + 1] === "=" || tokens[i + 1] === "+=" || tokens[i + 1] === "-=")) {
      const op = tokens[i + 1];
      if (i + 2 >= tokens.length) {
        clauses.push(tok + " " + op);
        break;
      }
      let clause = tok + " " + op + " " + tokens[i + 2];
      i += 3;
      while (i < tokens.length && /^-?\d+$/.test(tokens[i])) {
        clause += " " + tokens[i];
        i++;
      }
      clauses.push(clause);
      continue;
    }
    if (tok === "!" && i + 1 < tokens.length) {
      clauses.push("!" + tokens[i + 1]);
      i += 2;
      continue;
    }
    if ((tok.startsWith("+") || tok.startsWith("-")) && !tok.endsWith("++") && !tok.endsWith("--")) {
      let clause = tok;
      i++;
      if ((tok === "+" || tok === "-") && i < tokens.length) {
        clause += tokens[i];
        i++;
      }
      if (i < tokens.length && /^-?\d+$/.test(tokens[i])) {
        clause += " " + tokens[i];
        i++;
      }
      clauses.push(clause);
      continue;
    }
    clauses.push(tok);
    i++;
  }
  return clauses;
}

function explainClientEffectsExprError(raw, varsMap) {
  let s = (raw || "").trim();
  if (s.startsWith("{{") && s.endsWith("}}") && s.length >= 4) {
    s = s.slice(2, -2).trim();
  }
  if (!s) return "";
  if (varsMap.size === 0) return "Brak zadeklarowanych zmiennych w książce";
  const rawParts = s.split(/[;,]/).map((p) => p.trim());
  if (rawParts.length === 0) return "Błąd składni w polu set:";
  const parts = [];
  for (let r = 0; r < rawParts.length; r++) {
    const rp = rawParts[r];
    if (!rp) {
      if (r === rawParts.length - 1 && s.endsWith(";")) continue;
      return "Błąd składni w set: puste wyrażenie po przecinku";
    }
    const sub = splitClientSpaceEffectChunk(rp);
    for (let k = 0; k < sub.length; k++) {
      parts.push(sub[k]);
    }
  }
  for (let i = 0; i < parts.length; i++) {
    const c = parts[i];
    if (!c) {
      return "Błąd składni w set: puste wyrażenie po przecinku";
    }
    if ((c.endsWith("++") || c.endsWith("--")) && !c.includes("=")) {
      const op = c.slice(-2);
      const varName = c.slice(0, -2).trim().toLowerCase();
      const vDef = varsMap.get(varName);
      if (!vDef) return "Nieznana zmienna '" + varName + "' — zadeklaruj ją w panelu Zmienne {}";
      if (vDef.kind !== "int") {
        return "Błąd typu: operator '" + op + "' wymaga zmiennej typu int (zmienna '" + varName + "' jest typu " + vDef.kind + ")";
      }
      continue;
    }
    if (c.startsWith("!") && !c.includes("=")) {
      const varName = c.slice(1).trim().toLowerCase();
      const vDef = varsMap.get(varName);
      if (!vDef) return "Nieznana zmienna '" + varName + "' — zadeklaruj ją w panelu Zmienne {}";
      if (vDef.kind !== "bool") {
        return "Błąd typu: negacja '!" + varName + "' wymaga zmiennej typu bool (jest typu " + vDef.kind + ")";
      }
      continue;
    }
    if ((c.startsWith("+") || c.startsWith("-")) && !c.includes("=")) {
      const tokens = c.slice(1).trim().split(/\s+/).filter(Boolean);
      if (tokens.length === 0 || tokens.length > 2) return "Błąd składni w set: '" + c + "'";
      const varName = tokens[0].toLowerCase();
      const vDef = varsMap.get(varName);
      if (!vDef) return "Nieznana zmienna '" + varName + "' — zadeklaruj ją w panelu Zmienne {}";
      if (vDef.kind !== "bool" && vDef.kind !== "int") {
        return "Błąd typu: dla zmiennej '" + varName + "' (enum) użyj przypisania '='";
      }
      if (tokens.length === 2 && (vDef.kind !== "int" || !/^-?\d+$/.test(tokens[1]))) {
        return "Błąd typu: zmienna '" + varName + "' wymaga liczby całkowitej";
      }
      continue;
    }
    if (c.includes("==") || c.includes("!=") || c.includes(">=") || c.includes("\x3c=") || c.includes(">") || c.includes("\x3c") || c.includes("&&") || c.includes("||")) {
      return "Błąd w set: użyj przypisania ('=', '+=', '-=', '++', '--'), a nie operatora warunkowego";
    }
    const plusEq = c.indexOf("+=");
    const minusEq = c.indexOf("-=");
    if (plusEq > 0 || minusEq > 0) {
      const idx = plusEq > 0 ? plusEq : minusEq;
      const op = plusEq > 0 ? "+=" : "-=";
      const lhs = c.slice(0, idx).trim().toLowerCase();
      const rhs = c.slice(idx + 2).trim().toLowerCase();
      const vDef = varsMap.get(lhs);
      if (!vDef) return "Nieznana zmienna '" + lhs + "' — zadeklaruj ją w panelu Zmienne {}";
      if (vDef.kind !== "int") {
        return "Błąd typu: operator '" + op + "' wymaga zmiennej typu int (zmienna '" + lhs + "' jest typu " + vDef.kind + ")";
      }
      if (!rhs) return "Błąd składni: brak wartości po '" + op + "' dla zmiennej '" + lhs + "'";
      if (!/^-?\d+$/.test(rhs)) {
        const rhsVar = varsMap.get(rhs);
        if (!rhsVar || rhsVar.kind !== "int") {
          return "Błąd typu: zmienna '" + lhs + "' (int) wymaga liczby całkowitej po '" + op + "', otrzymano '" + rhs + "'";
        }
      }
      continue;
    }
    const eqIdx = c.indexOf("=");
    if (eqIdx > 0) {
      const lhs = c.slice(0, eqIdx).trim().toLowerCase();
      const rhs = c.slice(eqIdx + 1).trim();
      const vDef = varsMap.get(lhs);
      if (!vDef) return "Nieznana zmienna '" + lhs + "' — zadeklaruj ją w panelu Zmienne {}";
      if (!rhs) return "Błąd składni: brak przypisywanej wartości po '=' dla zmiennej '" + lhs + "'";
      const errMsg = explainClientVarValueError(lhs, vDef, "=", rhs, varsMap);
      if (errMsg) return errMsg;
      continue;
    }
    const bareName = c.toLowerCase();
    const bareDef = varsMap.get(bareName);
    if (bareDef) {
      if (bareDef.kind === "bool") continue;
      return "Błąd składni: dla zmiennej '" + bareName + "' (" + bareDef.kind + ") podaj przypisanie (np. " + bareName + " = wartość)";
    }
    if (!/[\s+\-*/=<>!&|,;:]/.test(bareName)) {
      return "Nieznana zmienna '" + bareName + "' — zadeklaruj ją w panelu Zmienne {}";
    }
    return "Błąd składni w set: '" + c + "'";
  }
  return "";
}

function validateRuleInputElement(inputEl, form) {
  if (!inputEl || !form) return;
  const name = inputEl.name || "";
  const isCond = name.indexOf("choice_cond_") === 0;
  const isEff = name.indexOf("choice_effects_") === 0 || name === "scene_effects";
  if (!isCond && !isEff) return;
  const isEmpty = !(inputEl.value || "").trim();
  const emptyRow = inputEl.closest(".choice-triple-row, .node-meta-row, .scene-settings-row");
  if (emptyRow) {
    emptyRow.classList.toggle("is-empty-rule", isEmpty);
  }
  const varsMap = parseBookVarsSpec(form.getAttribute("data-book-vars") || "");
  const errMsg = isCond
    ? explainClientCondExprError(inputEl.value, varsMap)
    : explainClientEffectsExprError(inputEl.value, varsMap);
  const valid = !errMsg;
  const wrap = inputEl.closest(".choice-triple-row, .syn-input-wrap");
  if (wrap) {
    wrap.classList.toggle("is-invalid", !valid);
  }
  if (errMsg) {
    inputEl.setAttribute("data-hint", errMsg);
    if (wrap) wrap.setAttribute("data-hint", errMsg);
    if (typeof activeHintEl !== "undefined" && (activeHintEl === inputEl || activeHintEl === wrap || document.activeElement === inputEl)) {
      activeHintEl = inputEl;
      if (typeof positionAndShowUITooltip === "function") {
        positionAndShowUITooltip(inputEl);
      }
    }
  } else {
    inputEl.removeAttribute("data-hint");
    if (wrap) wrap.removeAttribute("data-hint");
    if (typeof activeHintEl !== "undefined" && (activeHintEl === inputEl || activeHintEl === wrap)) {
      if (typeof hideUITooltip === "function") {
        hideUITooltip();
      }
    }
  }
}

let activeSynRuleInput = null;
let lastSynRuleInputName = "";
let lastRulePointerWasTouch = false;

function stripOuterBracesClient(s) {
  let v = (s || "").trim();
  if (v.startsWith("{{") && v.endsWith("}}") && v.length >= 4) {
    v = v.slice(2, -2).trim();
  }
  return v;
}

function extractCurrentTypingToken(val, cursorPos) {
  const before = val.slice(0, cursorPos);
  const m = before.match(/([a-z0-9_ąćęłńóśźż]+)$/i);
  return m ? m[1].toLowerCase() : "";
}

function isAtFreshOperandSlot(val, cursorPos) {
  const before = val.slice(0, cursorPos);
  return /^\s*[!?+\-]?\s*$/.test(before) || /(?:&&|\|\||;|,)\s*[!?+\-]?\s*$/.test(before);
}

function replaceCurrentTypingToken(inputEl, varName) {
  let val = inputEl.value || "";
  if (!val.trim()) {
    inputEl.value = varName;
    const p = varName.length;
    if (typeof inputEl.setSelectionRange === "function") inputEl.setSelectionRange(p, p);
    return;
  }
  const pos = typeof inputEl.selectionStart === "number" ? inputEl.selectionStart : val.length;
  const before = val.slice(0, pos);
  const after = val.slice(pos).replace(/^[a-z0-9_ąćęłńóśźż]*/i, "");
  const prefix = before.replace(/\?\s*[a-z0-9_ąćęłńóśźż]*$|[a-z0-9_ąćęłńóśźż]+$/i, "");
  let sep = "";
  if (prefix.length > 0 && !/\s$/.test(prefix) && !/[(!]$/.test(prefix) && !/(?:^|[\s;(,])[-+]$/.test(prefix)) {
    sep = " ";
  }
  const replacedBefore = prefix + sep + varName;
  const needTrailingSpace = after.length > 0 && /^[a-z0-9_ąćęłńóśźż!?]/i.test(after);
  inputEl.value = replacedBefore + (needTrailingSpace ? " " : "") + after;
  const newPos = replacedBefore.length;
  if (typeof inputEl.setSelectionRange === "function") {
    inputEl.setSelectionRange(newPos, newPos);
  }
}

function getVisibleSynDropdownItems(dropdown) {
  if (!dropdown) return [];
  return qsa(".syn-dropdown-item", dropdown).filter((btn) => !btn.classList.contains("hidden"));
}

function showAndAnchorSynDropdown(dropdown, inputEl, anchorSel, matchCount) {
  if (matchCount === 0) {
    dropdown.classList.add("hidden");
    return;
  }
  const anchor = inputEl.closest(anchorSel);
  if (anchor && dropdown.parentNode !== anchor) {
    anchor.appendChild(dropdown);
  }
  dropdown.classList.remove("hidden");
  setSynDropdownActiveIndex(dropdown, 0);
}

function commitChosenSynItem(itemBtn, inputEl, form, dropdown) {
  if (!itemBtn || !inputEl) return;
  const kind = itemBtn.getAttribute("data-syn-kind") || "var";
  if (kind === "target") {
    const targetVal = itemBtn.getAttribute("data-syn-target-value") || "";
    if (!targetVal) return;
    replaceChoiceTargetToken(inputEl, targetVal);
  } else {
    const varName = itemBtn.getAttribute("data-syn-var-name") || "";
    if (!varName) return;
    replaceCurrentTypingToken(inputEl, varName);
    if (form) validateRuleInputElement(inputEl, form);
  }
  if (dropdown) dropdown.classList.add("hidden");
  if (form && typeof scheduleNodeLiveSave === "function") scheduleNodeLiveSave(form);
}

function setSynDropdownActiveIndex(dropdown, nextIdx) {
  if (!dropdown) return;
  const visibleItems = getVisibleSynDropdownItems(dropdown);
  qsa(".syn-dropdown-item", dropdown).forEach((btn) => btn.classList.remove("active"));
  if (visibleItems.length === 0) return;
  const clamped = ((nextIdx % visibleItems.length) + visibleItems.length) % visibleItems.length;
  const activeBtn = visibleItems[clamped];
  if (activeBtn) {
    activeBtn.classList.add("active");
    if (typeof activeBtn.scrollIntoView === "function") {
      activeBtn.scrollIntoView({ block: "nearest" });
    }
  }
}

function moveSynDropdownSelection(dropdown, delta) {
  if (!dropdown) return;
  const visibleItems = getVisibleSynDropdownItems(dropdown);
  if (visibleItems.length === 0) return;
  let curIdx = -1;
  for (let i = 0; i < visibleItems.length; i++) {
    if (visibleItems[i].classList.contains("active")) {
      curIdx = i;
      break;
    }
  }
  if (curIdx < 0) {
    setSynDropdownActiveIndex(dropdown, delta >= 0 ? 0 : visibleItems.length - 1);
  } else {
    setSynDropdownActiveIndex(dropdown, curIdx + delta);
  }
}

function syncChoiceInlineHighlight(inputEl) {
  if (!inputEl) return;
  const val = inputEl.value || "";
  const arrowIdx = val.lastIndexOf("->");
  const hasArrow = arrowIdx >= 0 && (arrowIdx === 0 || val.charAt(arrowIdx - 1) !== "\\");
  const wrap = inputEl.closest ? inputEl.closest(".choice-inline-wrap") : null;
  if (wrap) {
    const hlLabel = qs(".choice-hl-label", wrap);
    const hlArrow = qs(".choice-hl-arrow", wrap);
    if (hasArrow) {
      if (hlLabel) hlLabel.textContent = val.slice(0, arrowIdx);
      if (hlArrow) hlArrow.textContent = val.slice(arrowIdx);
    } else {
      if (hlLabel) hlLabel.textContent = val;
      if (hlArrow) hlArrow.textContent = "";
    }
  }
  const choiceIdx = inputEl.getAttribute ? inputEl.getAttribute("data-choice-idx") : null;
  const form = inputEl.closest ? inputEl.closest("form.editor-context-form, form#form-scene-text") : null;
  if (form && choiceIdx !== null && choiceIdx !== "" && hasArrow) {
    const tgtInput = qs('input[name="choice_target_' + choiceIdx + '"]', form);
    if (tgtInput) {
      const parsedTgt = val.slice(arrowIdx + 2).trim();
      tgtInput.value = parsedTgt || "END";
    }
  }
}

function replaceChoiceTargetToken(inputEl, targetVal) {
  if (!inputEl) return;
  if (inputEl.classList && inputEl.classList.contains("choice-exit-input")) {
    inputEl.value = targetVal;
    const newPos = targetVal.length;
    if (typeof inputEl.setSelectionRange === "function") {
      inputEl.setSelectionRange(newPos, newPos);
    }
    return;
  }
  const val = inputEl.value || "";
  const pos = typeof inputEl.selectionStart === "number" ? inputEl.selectionStart : val.length;
  const before = val.slice(0, pos);
  const after = val.slice(pos);
  const replacedBefore = before.replace(/->\s*([a-z0-9_ąćęłńóśźż:\-]*)$/i, "-> " + targetVal);
  inputEl.value = replacedBefore + after;
  const newPos = replacedBefore.length;
  if (typeof inputEl.setSelectionRange === "function") {
    inputEl.setSelectionRange(newPos, newPos);
  }
  syncChoiceInlineHighlight(inputEl);
}

function updateChoiceTargetAutocompleteOnInput(inputEl, forceAll) {
  if (!inputEl || typeof document === "undefined") return;
  const isExitInput = Boolean(inputEl.classList && inputEl.classList.contains("choice-exit-input"));
  if (!isExitInput) {
    syncChoiceInlineHighlight(inputEl);
  }
  const form = inputEl.closest("form.editor-context-form, form#form-scene-text");
  if (!form) return;
  const dropdown = qs(".syn-autocomplete-dropdown", form);
  if (!dropdown) return;
  activeSynRuleInput = inputEl;
  lastSynRuleInputName = inputEl.name || "";

  const val = inputEl.value || "";
  let prefix = "";
  if (isExitInput) {
    prefix = val.trim().toLowerCase();
    if (!forceAll && prefix === "") {
      dropdown.classList.add("hidden");
      return;
    }
  } else {
    const pos = typeof inputEl.selectionStart === "number" ? inputEl.selectionStart : val.length;
    const before = val.slice(0, pos);
    const m = before.match(/->\s*([a-z0-9_ąćęłńóśźż:\-]*)$/i);
    if (!m) {
      dropdown.classList.add("hidden");
      return;
    }
    prefix = (m[1] || "").toLowerCase();
  }
  const items = qsa(".syn-dropdown-item", dropdown);
  let matchCount = 0;
  items.forEach((btn) => {
    const kind = btn.getAttribute("data-syn-kind") || "var";
    if (kind !== "target") {
      btn.classList.add("hidden");
      return;
    }
    const tVal = (btn.getAttribute("data-syn-target-value") || "").toLowerCase();
    const show = prefix === "" || (tVal.startsWith(prefix) && tVal !== prefix);
    btn.classList.toggle("hidden", !show);
    if (show) matchCount++;
  });
  showAndAnchorSynDropdown(dropdown, inputEl, ".choice-triple-row, .choice-inline-wrap", matchCount);
}

function updateSynAutocompleteOnInput(inputEl, forceAll) {
  if (!inputEl || typeof document === "undefined" || inputEl.id === "modal-scene-textarea") return;
  const form = inputEl.closest("form.editor-context-form, form#form-scene-text");
  const dropdown = form ? qs(".syn-autocomplete-dropdown", form) : null;
  if (!dropdown) return;
  activeSynRuleInput = inputEl;
  lastSynRuleInputName = inputEl.name || "";

  const val = inputEl.value || "";
  const pos = typeof inputEl.selectionStart === "number" ? inputEl.selectionStart : val.length;
  const beforeCursor = val.slice(0, pos);
  const lastNewline = beforeCursor.lastIndexOf("\n");
  const currentLineBefore = lastNewline >= 0 ? beforeCursor.slice(lastNewline + 1) : beforeCursor;

  const varsMap = parseBookVarsSpec(form.getAttribute("data-book-vars") || "");
  const enumSlotMatch = currentLineBefore.match(/([a-z0-9_ąćęłńóśźż]+)\s*(?:==|!=|=)\s*(\??\s*[a-z0-9_ąćęłńóśźż]*)$/i);
  const items = qsa(".syn-dropdown-item", dropdown);

  if (enumSlotMatch) {
    const enumVarName = (enumSlotMatch[1] || "").toLowerCase();
    const vDef = varsMap.get(enumVarName);
    if (vDef && vDef.kind === "enum") {
      const rhsRaw = (enumSlotMatch[2] || "").trim();
      const hasQuestion = rhsRaw.startsWith("?");
      const enumPrefix = rhsRaw.replace(/^\?\s*/, "").toLowerCase();
      if (!forceAll && !hasQuestion && !enumPrefix && !lastRulePointerWasTouch) {
        dropdown.classList.add("hidden");
        return;
      }
      let matchCount = 0;
      items.forEach((btn) => {
        const kind = btn.getAttribute("data-syn-kind") || "var";
        const btnEnumVar = (btn.getAttribute("data-syn-enum-var") || "").toLowerCase();
        const optVal = (btn.getAttribute("data-syn-var-name") || "").toLowerCase();
        const show =
          kind === "enum_val" &&
          btnEnumVar === enumVarName &&
          (forceAll || hasQuestion || !enumPrefix || (optVal.startsWith(enumPrefix) && optVal !== enumPrefix));
        btn.classList.toggle("hidden", !show);
        if (show) matchCount++;
      });
      showAndAnchorSynDropdown(dropdown, inputEl, ".choice-triple-row, .syn-input-wrap, .scene-body-block", matchCount);
      return;
    }
  }

  const triggeredByQuestionMark = /\?\s*$/.test(currentLineBefore);
  const token = extractCurrentTypingToken(currentLineBefore, currentLineBefore.length);
  const touchSlot = lastRulePointerWasTouch && isAtFreshOperandSlot(currentLineBefore, currentLineBefore.length);

  if (!forceAll && !triggeredByQuestionMark && !touchSlot && !token) {
    dropdown.classList.add("hidden");
    return;
  }

  const usedVars = new Set();
  const prefixBeforeToken = currentLineBefore.replace(/\?\s*[a-z0-9_ąćęłńóśźż]*$|[a-z0-9_ąćęłńóśźż]+$/i, "");
  const afterToken = val.slice(pos).replace(/^[a-z0-9_ąćęłńóśźż]*/i, "");
  const restTokens = (prefixBeforeToken + " " + afterToken).toLowerCase().match(/[a-z0-9_ąćęłńóśźż]+/gi) || [];
  restTokens.forEach((tok) => {
    if (varsMap.has(tok)) usedVars.add(tok);
  });

  let matchCount = 0;
  items.forEach((btn) => {
    const kind = btn.getAttribute("data-syn-kind") || "var";
    const vName = (btn.getAttribute("data-syn-var-name") || "").toLowerCase();
    const show =
      kind === "var" &&
      !usedVars.has(vName) &&
      (forceAll || triggeredByQuestionMark || touchSlot || (token.length > 0 && vName.startsWith(token) && vName !== token));
    btn.classList.toggle("hidden", !show);
    if (show) matchCount++;
  });

  showAndAnchorSynDropdown(dropdown, inputEl, ".choice-triple-row, .syn-input-wrap, .scene-body-block", matchCount);
}

function bindSynAutocompleteDropdown() {
  if (typeof document === "undefined") return;

  document.addEventListener("focusin", (e) => {
    const t = e.target;
    if (!t || !t.matches) return;
    if (t.matches("input.choice-rule-input, input.syn-inner-input")) {
      if (lastRulePointerWasTouch) {
        updateSynAutocompleteOnInput(t, !(t.value || "").trim());
      }
    } else if (t.matches("input.choice-exit-input")) {
      if (lastRulePointerWasTouch) {
        updateChoiceTargetAutocompleteOnInput(t, true);
      }
    }
  });

  document.addEventListener("focusout", (e) => {
    const t = e.target;
    if (!t || !t.matches) return;
    if (t.matches("input.choice-rule-input, input.syn-inner-input")) {
      const inner = stripOuterBracesClient(t.value);
      t.value = inner;
      const emptyRow = t.closest(".choice-triple-row, .node-meta-row, .scene-settings-row");
      if (emptyRow) {
        emptyRow.classList.toggle("is-empty-rule", !inner);
      }
    } else if (t.matches("input.node-choice-label, textarea.node-choice-label, input.choice-label-text, textarea.choice-label-text")) {
      syncChoiceInlineHighlight(t);
    }
  });

  document.addEventListener("input", (e) => {
    const t = e.target;
    if (!t || !t.matches) return;
    if (t.matches("input.choice-rule-input, input.syn-inner-input")) {
      updateSynAutocompleteOnInput(t, false);
    } else if (t.matches("input.choice-exit-input, input.node-choice-label, textarea.node-choice-label, input.choice-label-text, textarea.choice-label-text")) {
      updateChoiceTargetAutocompleteOnInput(t, false);
    }
  });

  document.addEventListener("keydown", (e) => {
    const t = e.target;
    if (!t || !t.matches || !t.matches("input.choice-exit-input, input.choice-rule-input, input.syn-inner-input, input.node-choice-label, textarea.node-choice-label, input.choice-label-text, textarea.choice-label-text")) return;
    const form = t.closest("form.editor-context-form, form#form-scene-text");
    const dropdown = form ? qs(".syn-autocomplete-dropdown", form) : null;
    if (!dropdown || dropdown.classList.contains("hidden")) {
      if (e.key === "ArrowDown") {
        if (t.matches("input.choice-rule-input, input.syn-inner-input")) {
          e.preventDefault();
          e.stopPropagation();
          updateSynAutocompleteOnInput(t, true);
          return;
        }
        if (t.matches("input.choice-exit-input")) {
          e.preventDefault();
          e.stopPropagation();
          updateChoiceTargetAutocompleteOnInput(t, true);
          return;
        }
      }
      if (e.key === "Enter" && t.matches("textarea.node-choice-label, textarea.choice-label-text")) {
        e.preventDefault();
        t.blur();
      }
      return;
    }
    const visibleItems = getVisibleSynDropdownItems(dropdown);
    if (visibleItems.length === 0) return;
    if (e.key === "ArrowDown") {
      e.preventDefault();
      e.stopPropagation();
      moveSynDropdownSelection(dropdown, 1);
      return;
    }
    if (e.key === "ArrowUp") {
      e.preventDefault();
      e.stopPropagation();
      moveSynDropdownSelection(dropdown, -1);
      return;
    }
    if (e.key === "Tab" || e.key === "Enter") {
      e.preventDefault();
      e.stopPropagation();
      const chosenBtn = visibleItems.find((btn) => btn.classList.contains("active")) || visibleItems[0];
      commitChosenSynItem(chosenBtn, t, form, dropdown);
    } else if (e.key === "Escape") {
      e.preventDefault();
      e.stopPropagation();
      dropdown.classList.add("hidden");
    }
  });

  document.addEventListener("pointerover", (e) => {
    const t = e.target;
    if (!t || !t.closest) return;
    const itemBtn = t.closest(".syn-dropdown-item");
    if (!itemBtn || itemBtn.classList.contains("hidden")) return;
    const dropdown = itemBtn.closest(".syn-autocomplete-dropdown");
    if (!dropdown || dropdown.classList.contains("hidden")) return;
    const idx = getVisibleSynDropdownItems(dropdown).indexOf(itemBtn);
    if (idx >= 0) setSynDropdownActiveIndex(dropdown, idx);
  });

  document.addEventListener("pointerdown", (e) => {
    lastRulePointerWasTouch = e.pointerType === "touch";
    const t = e.target;
    if (!t || !t.closest) return;

    const itemBtn = t.closest(".syn-dropdown-item");
    if (itemBtn && activeSynRuleInput) {
      e.preventDefault();
      e.stopPropagation();
      const form = activeSynRuleInput.closest("form.editor-context-form, form#form-scene-text");
      commitChosenSynItem(itemBtn, activeSynRuleInput, form, itemBtn.closest(".syn-autocomplete-dropdown"));
      activeSynRuleInput.focus();
      return;
    }

    if (!t.closest(".syn-autocomplete-dropdown")) {
      qsa(".syn-autocomplete-dropdown").forEach((d) => d.classList.add("hidden"));
    }
  });
}

bindSynAutocompleteDropdown();
