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
    return trf("err_missing_val_after_op", op, varName);
  }
  if (vDef.kind === "int") {
    if (/^-?\d+$/.test(cleanRhs)) return "";
    const rhsVar = varsMap.get(cleanRhs);
    if (rhsVar && rhsVar.kind === "int") return "";
    return trf("err_int_type", varName, cleanRhs);
  }
  if (vDef.kind === "enum") {
    if (op !== "==" && op !== "!=" && op !== "=") {
      return trf("err_enum_op", varName);
    }
    if (vDef.opts.includes(cleanRhs)) return "";
    const rhsVar = varsMap.get(cleanRhs);
    if (rhsVar && rhsVar.kind === "enum") return "";
    return trf("err_enum_val", cleanRhs, varName, vDef.opts.join(", "));
  }
  if (op !== "==" && op !== "!=" && op !== "=") {
    return trf("err_bool_op", varName);
  }
  if (isClientBoolLiteral(cleanRhs)) return "";
  if (cleanRhs.startsWith("!")) {
    const negVar = varsMap.get(cleanRhs.slice(1).trim());
    if (negVar && negVar.kind === "bool") return "";
  }
  const rhsVar = varsMap.get(cleanRhs);
  if (rhsVar && rhsVar.kind === "bool") return "";
  return trf("err_bool_type", varName);
}

function explainClientCondExprError(raw, varsMap) {
  let s = (raw || "").trim();
  if (s.startsWith("{{") && s.endsWith("}}") && s.length >= 4) {
    s = s.slice(2, -2).trim();
  }
  if (s.startsWith("?")) s = s.slice(1).trim();
  if (!s) return "";
  if (varsMap.size === 0) return tr("err_no_vars");
  let depth = 0;
  for (let i = 0; i < s.length; i++) {
    if (s[i] === "(") depth++;
    else if (s[i] === ")") {
      depth--;
      if (depth < 0) return tr("err_unclosed_paren");
    }
  }
  if (depth !== 0) return tr("err_unclosed_paren");
  const clauses = s.split(/\|\||&&/);
  for (let i = 0; i < clauses.length; i++) {
    let c = clauses[i].replace(/^[()\s]+|[()\s]+$/g, "");
    while ((c.startsWith("!") && !c.startsWith("!=")) || c.startsWith("?")) {
      c = c.slice(1).replace(/^[()\s]+|[()\s]+$/g, "");
    }
    if (!c) return tr("err_empty_after_logic_op");
    for (let m = 0; m < PLAYTXT_MUT_OPS.length; m++) {
      if (c.indexOf(PLAYTXT_MUT_OPS[m]) >= 0) {
        return trf("err_mut_op_in_if", PLAYTXT_MUT_OPS[m]);
      }
    }
    if (c.indexOf(",") >= 0 || c.indexOf(";") >= 0) {
      return tr("err_comma_in_if");
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
      if (!lhs) return trf("err_missing_var_before_op", matchedOp);
      if (!rhs) return trf("err_missing_val_after_op", matchedOp, lhs);
      const vDef = varsMap.get(lhs);
      if (!vDef) return trf("err_unknown_var", lhs);
      const errMsg = explainClientVarValueError(lhs, vDef, matchedOp, rhs, varsMap);
      if (errMsg) return errMsg;
    } else {
      const varName = c.trim().toLowerCase();
      if (/[\s=!<>+\-(),:|&]/.test(varName)) {
        return trf("err_cond_syntax", c.trim());
      }
      if (!varsMap.has(varName)) {
        return trf("err_unknown_var", varName);
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
  if (varsMap.size === 0) return tr("err_no_vars");
  const rawParts = s.split(/[;,]/).map((p) => p.trim());
  if (rawParts.length === 0) return tr("err_set_syntax_empty");
  const parts = [];
  for (let r = 0; r < rawParts.length; r++) {
    const rp = rawParts[r];
    if (!rp) {
      if (r === rawParts.length - 1 && s.endsWith(";")) continue;
      return tr("err_set_empty_after_comma");
    }
    const sub = splitClientSpaceEffectChunk(rp);
    for (let k = 0; k < sub.length; k++) {
      parts.push(sub[k]);
    }
  }
  for (let i = 0; i < parts.length; i++) {
    const c = parts[i];
    if (!c) {
      return tr("err_set_empty_after_comma");
    }
    if ((c.endsWith("++") || c.endsWith("--")) && !c.includes("=")) {
      const op = c.slice(-2);
      const varName = c.slice(0, -2).trim().toLowerCase();
      const vDef = varsMap.get(varName);
      if (!vDef) return trf("err_unknown_var", varName);
      if (vDef.kind !== "int") {
        return trf("err_op_requires_int", op, varName, vDef.kind);
      }
      continue;
    }
    if (c.startsWith("!") && !c.includes("=")) {
      const varName = c.slice(1).trim().toLowerCase();
      const vDef = varsMap.get(varName);
      if (!vDef) return trf("err_unknown_var", varName);
      if (vDef.kind !== "bool") {
        return trf("err_neg_requires_bool", varName, vDef.kind);
      }
      continue;
    }
    if ((c.startsWith("+") || c.startsWith("-")) && !c.includes("=")) {
      const tokens = c.slice(1).trim().split(/\s+/).filter(Boolean);
      if (tokens.length === 0 || tokens.length > 2) return trf("err_set_clause_syntax", c);
      const varName = tokens[0].toLowerCase();
      const vDef = varsMap.get(varName);
      if (!vDef) return trf("err_unknown_var", varName);
      if (vDef.kind !== "bool" && vDef.kind !== "int") {
        return trf("err_enum_needs_assign", varName);
      }
      if (tokens.length === 2 && (vDef.kind !== "int" || !/^-?\d+$/.test(tokens[1]))) {
        return trf("err_var_requires_int", varName);
      }
      continue;
    }
    if (c.includes("==") || c.includes("!=") || c.includes(">=") || c.includes("\x3c=") || c.includes(">") || c.includes("\x3c") || c.includes("&&") || c.includes("||")) {
      return tr("err_cond_op_in_set");
    }
    const plusEq = c.indexOf("+=");
    const minusEq = c.indexOf("-=");
    if (plusEq > 0 || minusEq > 0) {
      const idx = plusEq > 0 ? plusEq : minusEq;
      const op = plusEq > 0 ? "+=" : "-=";
      const lhs = c.slice(0, idx).trim().toLowerCase();
      const rhs = c.slice(idx + 2).trim().toLowerCase();
      const vDef = varsMap.get(lhs);
      if (!vDef) return trf("err_unknown_var", lhs);
      if (vDef.kind !== "int") {
        return trf("err_op_requires_int", op, lhs, vDef.kind);
      }
      if (!rhs) return trf("err_missing_val_after_compound", op, lhs);
      if (!/^-?\d+$/.test(rhs)) {
        const rhsVar = varsMap.get(rhs);
        if (!rhsVar || rhsVar.kind !== "int") {
          return trf("err_int_after_compound", lhs, op, rhs);
        }
      }
      continue;
    }
    const eqIdx = c.indexOf("=");
    if (eqIdx > 0) {
      const lhs = c.slice(0, eqIdx).trim().toLowerCase();
      const rhs = c.slice(eqIdx + 1).trim();
      const vDef = varsMap.get(lhs);
      if (!vDef) return trf("err_unknown_var", lhs);
      if (!rhs) return trf("err_missing_val_after_eq", lhs);
      const errMsg = explainClientVarValueError(lhs, vDef, "=", rhs, varsMap);
      if (errMsg) return errMsg;
      continue;
    }
    const bareName = c.toLowerCase();
    const bareDef = varsMap.get(bareName);
    if (bareDef) {
      if (bareDef.kind === "bool") continue;
      return trf("err_bare_var_needs_assign", bareName, bareDef.kind, bareName);
    }
    if (!/[\s+\-*/=<>!&|,;:]/.test(bareName)) {
      return trf("err_unknown_var", bareName);
    }
    return trf("err_set_clause_syntax", c);
  }
  return "";
}

function explainClientExitTargetError(raw, form) {
  const s = (raw || "").trim();
  if (!s) return tr("err_exit_empty");
  if (s.indexOf("?") >= 0) return tr("err_exit_question");
  const lower = s.toLowerCase();
  if (lower === "end" || lower === "koniec" || lower === "next" || lower === "dalej") return "";
  if (!form) return "";

  const sceneIdInput = qs('input[name="scene_id"]', form);
  const expIdInput = qs('input[name="explicit_id"]', form);
  const curSceneId = ((sceneIdInput ? sceneIdInput.value : "") || form.getAttribute("data-context-scene-id") || "").trim().toLowerCase();
  const curExpId = (expIdInput ? expIdInput.value : "").trim().toLowerCase();
  if ((curSceneId && lower === curSceneId) || (curExpId && lower === curExpId)) {
    return tr("err_exit_self_scene");
  }

  const chInput = qs('input[name="chapter"]', form);
  const curChapter = (chInput ? chInput.value : "").trim().toLowerCase();
  const targetItems = qsa('.syn-dropdown-item[data-syn-kind="target"]', form);

  const chPrefixMatch = lower.match(/^(chapter|rozdzial|rozdział|act|akt):(.*)$/);
  if (chPrefixMatch) {
    const chRefRaw = s.slice(s.indexOf(":") + 1).trim();
    const chRef = chRefRaw.toLowerCase();
    if (!chRef) return trf("err_exit_unknown_chapter", s);
    if (curChapter && (chRef === curChapter || chRef === curChapter.replace(/\s+/g, "_"))) {
      return tr("err_exit_self_chapter");
    }
    const canonCh = "chapter:" + chRef;
    for (let i = 0; i < targetItems.length; i++) {
      const tVal = (targetItems[i].getAttribute("data-syn-var-name") || "").toLowerCase();
      const tMetaEl = qs(".syn-dropdown-var-meta", targetItems[i]);
      const tMeta = (tMetaEl ? tMetaEl.textContent : "").trim().toLowerCase();
      if (tVal === canonCh || (tVal.indexOf("chapter:") === 0 && tMeta === chRef)) {
        return "";
      }
    }
    return trf("err_exit_unknown_chapter", chRefRaw);
  }

  if (curChapter && (lower === curChapter || lower === curChapter.replace(/\s+/g, "_"))) {
    return tr("err_exit_self_chapter");
  }

  for (let i = 0; i < targetItems.length; i++) {
    const tVal = (targetItems[i].getAttribute("data-syn-var-name") || "").toLowerCase();
    const tMetaEl = qs(".syn-dropdown-var-meta", targetItems[i]);
    const tMeta = (tMetaEl ? tMetaEl.textContent : "").trim().toLowerCase();
    if (tVal === lower || (tMeta && tMeta === lower && tVal.indexOf("chapter:") !== 0)) {
      return "";
    }
  }
  return trf("err_exit_unknown_target", s);
}

function validateRuleInputElement(inputEl, form) {
  if (!inputEl || !form) return;
  const name = inputEl.name || "";
  const isExit = name.indexOf("choice_target_") === 0;
  const isCond = name.indexOf("choice_cond_") === 0;
  const isEff = name.indexOf("choice_effects_") === 0 || name === "scene_effects";
  if (!isExit && !isCond && !isEff) return;
  const isEmpty = !(inputEl.value || "").trim();
  const emptyRow = inputEl.closest(".choice-triple-row, .node-meta-row, .scene-settings-row");
  if (emptyRow && !isExit) {
    emptyRow.classList.toggle("is-empty-rule", isEmpty);
  }
  let errMsg = "";
  if (isExit) {
    errMsg = explainClientExitTargetError(inputEl.value, form);
  } else {
    const varsMap = parseBookVarsSpec(form.getAttribute("data-book-vars") || "");
    errMsg = isCond
      ? explainClientCondExprError(inputEl.value, varsMap)
      : explainClientEffectsExprError(inputEl.value, varsMap);
  }
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
  const m = before.match(/([a-z0-9_ąćęłńóśźż:\-]+)$/i);
  return m ? m[1].toLowerCase() : "";
}

function isAtFreshOperandSlot(val, cursorPos) {
  const before = val.slice(0, cursorPos);
  return /^\s*[!?+\-]?\s*$/.test(before) || /(?:&&|\|\||;|,)\s*[!?+\-]?\s*$/.test(before);
}

function replaceCurrentTypingToken(inputEl, varName) {
  let val = inputEl.value || "";
  if (!val.trim() || (inputEl.classList && inputEl.classList.contains("choice-exit-input"))) {
    inputEl.value = varName;
    const p = varName.length;
    if (typeof inputEl.setSelectionRange === "function") inputEl.setSelectionRange(p, p);
    return;
  }
  const pos = typeof inputEl.selectionStart === "number" ? inputEl.selectionStart : val.length;
  const before = val.slice(0, pos);
  const after = val.slice(pos).replace(/^[a-z0-9_ąćęłńóśźż:\-]*/i, "");
  const prefix = before.replace(/\?\s*[a-z0-9_ąćęłńóśźż:\-]*$|[a-z0-9_ąćęłńóśźż:\-]+$/i, "");
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
  const varName = itemBtn.getAttribute("data-syn-var-name") || "";
  if (!varName) return;
  replaceCurrentTypingToken(inputEl, varName);
  if (form) {
    validateRuleInputElement(inputEl, form);
    if (typeof syncChoiceTargetToCanvas === "function" && inputEl.name && inputEl.name.indexOf("choice_target_") === 0) {
      syncChoiceTargetToCanvas(form, inputEl);
    }
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

function updateSynAutocompleteOnInput(inputEl, forceAll) {
  if (!inputEl || typeof document === "undefined" || inputEl.id === "modal-scene-textarea") return;
  const form = inputEl.closest("form.editor-context-form, form#form-scene-text");
  const dropdown = form ? qs(".syn-autocomplete-dropdown", form) : null;
  if (!dropdown) return;
  activeSynRuleInput = inputEl;
  lastSynRuleInputName = inputEl.name || "";
  const isExitInput = Boolean(inputEl.classList && inputEl.classList.contains("choice-exit-input"));
  const wantKind = isExitInput ? "target" : "var";

  const val = inputEl.value || "";
  const pos = typeof inputEl.selectionStart === "number" ? inputEl.selectionStart : val.length;
  const beforeCursor = val.slice(0, pos);
  const lastNewline = beforeCursor.lastIndexOf("\n");
  const currentLineBefore = lastNewline >= 0 ? beforeCursor.slice(lastNewline + 1) : beforeCursor;

  const varsMap = parseBookVarsSpec(form.getAttribute("data-book-vars") || "");
  const enumSlotMatch = !isExitInput ? currentLineBefore.match(/([a-z0-9_ąćęłńóśźż]+)\s*(?:==|!=|=)\s*(\??\s*[a-z0-9_ąćęłńóśźż]*)$/i) : null;
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
  const prefixBeforeToken = currentLineBefore.replace(/\?\s*[a-z0-9_ąćęłńóśźż:\-]*$|[a-z0-9_ąćęłńóśźż:\-]+$/i, "");
  const afterToken = val.slice(pos).replace(/^[a-z0-9_ąćęłńóśźż:\-]*/i, "");
  if (isExitInput) {
    const exitRest = (prefixBeforeToken + " " + val.slice(pos)).replace(/\?/g, "").trim().toLowerCase();
    if (exitRest) usedVars.add(exitRest);
  }
  const restTokens = (prefixBeforeToken + " " + (isExitInput ? val.slice(pos) : afterToken)).toLowerCase().match(/[a-z0-9_ąćęłńóśźż:\-]+/gi) || [];
  restTokens.forEach((tok) => {
    if (isExitInput || varsMap.has(tok)) usedVars.add(tok);
  });

  let matchCount = 0;
  items.forEach((btn) => {
    const kind = btn.getAttribute("data-syn-kind") || "var";
    const vName = (btn.getAttribute("data-syn-var-name") || "").toLowerCase();
    const matchesToken =
      token.length > 0 &&
      (vName.startsWith(token) || (isExitInput && vName.startsWith("chapter:") && vName.slice(8).startsWith(token))) &&
      vName !== token;
    const show =
      kind === wantKind &&
      !usedVars.has(vName) &&
      (forceAll || triggeredByQuestionMark || touchSlot || matchesToken);
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
    if (t.matches("input.choice-exit-input, input.choice-rule-input, input.syn-inner-input") && lastRulePointerWasTouch) {
      updateSynAutocompleteOnInput(t, t.matches("input.choice-exit-input") || !(t.value || "").trim());
    }
  });

  document.addEventListener("focusout", (e) => {
    const t = e.target;
    if (!t || !t.matches) return;
    if (t.matches("input.choice-exit-input, input.choice-rule-input, input.syn-inner-input")) {
      let inner = stripOuterBracesClient(t.value);
      if (t.matches("input.choice-exit-input")) {
        inner = inner.replace(/\?/g, "").trim();
        t.value = inner;
        const form = t.closest("form.editor-context-form, form#form-scene-text");
        if (form) {
          validateRuleInputElement(t, form);
          if (typeof syncChoiceTargetToCanvas === "function") syncChoiceTargetToCanvas(form, t);
        }
        return;
      }
      t.value = inner;
      const emptyRow = t.closest(".choice-triple-row, .node-meta-row, .scene-settings-row");
      if (emptyRow) {
        emptyRow.classList.toggle("is-empty-rule", !inner);
      }
    }
  });

  document.addEventListener("input", (e) => {
    const t = e.target;
    if (!t || !t.matches) return;
    if (t.matches("input.choice-exit-input, input.choice-rule-input, input.syn-inner-input")) {
      updateSynAutocompleteOnInput(t, false);
    }
  });

  document.addEventListener("keydown", (e) => {
    const t = e.target;
    if (!t || !t.matches || !t.matches("input.choice-exit-input, input.choice-rule-input, input.syn-inner-input, textarea.node-choice-label, textarea.choice-label-text")) return;
    const form = t.closest("form.editor-context-form, form#form-scene-text");
    const dropdown = form ? qs(".syn-autocomplete-dropdown", form) : null;
    if (!dropdown || dropdown.classList.contains("hidden")) {
      if (e.key === "ArrowDown" && t.matches("input.choice-exit-input, input.choice-rule-input, input.syn-inner-input")) {
        e.preventDefault();
        e.stopPropagation();
        updateSynAutocompleteOnInput(t, true);
        return;
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
