package gui

import (
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"storyteller"
	"storyteller/internal/tpl"
)

func TestAllActionSpecsHaveHandlers(t *testing.T) {
	if len(actionSpecs) == 0 {
		t.Fatal("expected actionSpecs to be non-empty")
	}
	for name, spec := range actionSpecs {
		if spec.Handler == nil {
			t.Fatalf("action %q in actionSpecs has a nil Handler", name)
		}
	}
}

func TestClientServerActionAndOverlayContracts(t *testing.T) {
	appDir := filepath.Clean(filepath.Join("..", "..", "app"))

	// 1. Verify every action referenced in .tpl templates exists in actionSpecs
	tplFiles, err := filepath.Glob(filepath.Join(appDir, "templates", "*.tpl"))
	if err != nil || len(tplFiles) == 0 {
		t.Fatalf("failed to glob .tpl files: %v", err)
	}
	tplActionRe := regexp.MustCompile(`(?m)^\s*(?:action|data-backdrop-action):\s*([A-Za-z0-9_]+)\s*$`)
	forLoopRe := regexp.MustCompile(`^\s*\{for\s+([A-Za-z0-9_]+)\s+in\s+[A-Za-z0-9_.]+\}\s*$`)
	propRefRe := regexp.MustCompile(`\{([a-z][A-Za-z0-9_]*)\.[A-Za-z0-9_]+\}`)

	knownGlobalObjects := map[string]bool{
		"current_scene":    true,
		"selected_node":    true,
		"selected_chapter": true,
	}

	for _, path := range tplFiles {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		src := string(raw)
		base := filepath.Base(path)
		for _, m := range tplActionRe.FindAllStringSubmatch(src, -1) {
			act := m[1]
			if _, ok := actionSpecs[act]; !ok {
				t.Fatalf("template %s references unknown server action %q", base, act)
			}
		}

		// Verify {for item in list} blocks never reference sibling loop variables out of scope
		lines := strings.Split(src, "\n")
		type loopScope struct {
			indent  int
			varName string
		}
		var stack []loopScope
		for lineNo, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || strings.HasPrefix(trimmed, "//") {
				continue
			}
			indent := len(line) - len(strings.TrimLeft(line, " \t"))
			for len(stack) > 0 && indent <= stack[len(stack)-1].indent {
				stack = stack[:len(stack)-1]
			}
			if fm := forLoopRe.FindStringSubmatch(line); fm != nil {
				stack = append(stack, loopScope{indent: indent, varName: fm[1]})
				continue
			}
			if len(stack) > 0 {
				for _, pm := range propRefRe.FindAllStringSubmatch(line, -1) {
					objName := pm[1]
					if knownGlobalObjects[objName] {
						continue
					}
					inScope := false
					for _, sc := range stack {
						if sc.varName == objName {
							inScope = true
							break
						}
					}
					if !inScope {
						t.Fatalf("template %s:%d references %q outside its {for} loop scope (active scopes: %+v)", base, lineNo+1, pm[0], stack)
					}
				}
			}
		}
	}

	// 2. Verify every literal action dispatched from app/js/*.js exists in actionSpecs
	jsFiles := tpl.JSModuleFiles(appDir)
	jsActionRe := regexp.MustCompile(`(?:sendServerAction|postServerActionSilent)\(\s*"([A-Za-z0-9_]+)"`)
	for _, path := range jsFiles {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		src := string(raw)
		base := filepath.Base(path)
		for _, m := range jsActionRe.FindAllStringSubmatch(src, -1) {
			act := m[1]
			if _, ok := actionSpecs[act]; !ok {
				t.Fatalf("JS module %s dispatches unknown server action %q", base, act)
			}
		}
	}

	// 3. Verify CSS overlay pointer-events and stacking contexts never block UI interaction
	cssProd, err := tpl.BundleFilesToString(tpl.CSSModuleFiles(appDir), true)
	if err != nil {
		t.Fatalf("BundleFilesToString(CSS) failed: %v", err)
	}
	for _, check := range []struct {
		block string
		prop  string
	}{
		{block: ".modals-root {", prop: "pointer-events: none;"},
		{block: ".modals-root {", prop: "grid-row: 2;"},
		{block: ".modal-backdrop {", prop: "pointer-events: auto;"},
		{block: ".modal-backdrop {", prop: "position: absolute;"},
		{block: ".toast {", prop: "pointer-events: none;"},
		{block: ".ui-tooltip {", prop: "pointer-events: none;"},
		{block: ".editor-perf-overlay {", prop: "pointer-events: none;"},
		{block: ".player-bg-stage {", prop: "pointer-events: none;"},
		{block: ".scroll-more-cue {", prop: "pointer-events: none;"},
	} {
		idx := strings.Index(cssProd, check.block)
		if idx < 0 {
			t.Fatalf("expected CSS selector block %s in compiled stylesheet", check.block)
		}
		endIdx := strings.Index(cssProd[idx:], "}")
		if endIdx < 0 || !strings.Contains(cssProd[idx:idx+endIdx], check.prop) {
			t.Fatalf("expected %s to contain %q, got:\n%s", check.block, check.prop, cssProd[idx:idx+endIdx])
		}
	}
}

func TestClientJSRuntimeAndInteractiveResponsiveness(t *testing.T) {
	nodeBin, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not found in PATH; skipping live JS DOM responsiveness test")
	}

	appDir := filepath.Clean(filepath.Join("..", "..", "app"))

	// Verify syntax & clean compilation of all 3 JS bundles (prod, dev, standalone player)
	prodJS, err := tpl.BundleFilesToString(tpl.JSModuleFiles(appDir), true)
	if err != nil {
		t.Fatalf("BundleFilesToString(prod) failed: %v", err)
	}
	devJS, err := tpl.BundleFilesToString(tpl.JSModuleFiles(appDir), false)
	if err != nil {
		t.Fatalf("BundleFilesToString(dev) failed: %v", err)
	}
	playerJS, err := tpl.BundleFilesToString(tpl.JSPlayerModuleFiles(appDir), true)
	if err != nil {
		t.Fatalf("BundleFilesToString(player) failed: %v", err)
	}

	// Run live end-to-end DOM + event-delegation + WebGL canvas + form submission harness in Node
	harnessScript := `
"use strict";
const vm = require("vm");

function parseSimpleStyle(raw) {
  const s = {
    left: "",
    top: "",
    width: "",
    height: "",
    cursor: "",
    transform: "",
    minHeight: "",
    setProperty(k, v) { this[k] = String(v); },
    getPropertyValue(k) { return this[k] || ""; }
  };
  if (raw) {
    raw.split(";").forEach((pair) => {
      const idx = pair.indexOf(":");
      if (idx > 0) {
        const k = pair.slice(0, idx).trim();
        const v = pair.slice(idx + 1).trim();
        s[k] = v;
      }
    });
  }
  return s;
}

function splitTopLevelCommas(sel) {
  const out = [];
  let depth = 0;
  let cur = "";
  for (let i = 0; i < sel.length; i++) {
    const ch = sel[i];
    if (ch === "[") depth++;
    else if (ch === "]") depth = Math.max(0, depth - 1);
    if (ch === "," && depth === 0) {
      if (cur.trim()) out.push(cur.trim());
      cur = "";
    } else {
      cur += ch;
    }
  }
  if (cur.trim()) out.push(cur.trim());
  return out;
}

function splitDescendantTokens(sel) {
  const out = [];
  let depth = 0;
  let cur = "";
  for (let i = 0; i < sel.length; i++) {
    const ch = sel[i];
    if (ch === "[") depth++;
    else if (ch === "]") depth = Math.max(0, depth - 1);
    if (/\s/.test(ch) && depth === 0) {
      if (cur.trim()) out.push(cur.trim());
      cur = "";
    } else {
      cur += ch;
    }
  }
  if (cur.trim()) out.push(cur.trim());
  return out;
}

function matchesCompoundSelector(el, compound) {
  if (!el || el.nodeType !== 1) return false;
  let rem = compound.trim();
  if (!rem) return false;

  const attrMatches = [];
  rem = rem.replace(/\[([^\]]+)\]/g, (_, inner) => {
    attrMatches.push(inner.trim());
    return "";
  });

  const notClassMatches = [];
  rem = rem.replace(/:not\(\.([A-Za-z0-9_-]+)\)/g, (_, cls) => {
    notClassMatches.push(cls);
    return "";
  });

  const classMatches = [];
  rem = rem.replace(/\.([A-Za-z0-9_-]+)/g, (_, cls) => {
    classMatches.push(cls);
    return "";
  });

  let idMatch = "";
  rem = rem.replace(/#([A-Za-z0-9_-]+)/g, (_, id) => {
    idMatch = id;
    return "";
  });

  const tagMatch = rem.trim().toLowerCase();
  if (tagMatch && tagMatch !== "*" && el.tagName.toLowerCase() !== tagMatch) {
    return false;
  }
  if (idMatch && el.id !== idMatch) {
    return false;
  }
  for (let i = 0; i < classMatches.length; i++) {
    if (!el.classList.contains(classMatches[i])) return false;
  }
  for (let i = 0; i < notClassMatches.length; i++) {
    if (el.classList.contains(notClassMatches[i])) return false;
  }
  for (let i = 0; i < attrMatches.length; i++) {
    const expr = attrMatches[i];
    const caretEq = expr.indexOf("^=");
    const plainEq = expr.indexOf("=");
    if (caretEq > 0) {
      const k = expr.slice(0, caretEq).trim();
      const v = expr.slice(caretEq + 2).trim().replace(/^["']|["']$/g, "");
      const actual = el.getAttribute(k);
      if (actual === null || !String(actual).startsWith(v)) return false;
    } else if (plainEq > 0) {
      const k = expr.slice(0, plainEq).trim();
      const v = expr.slice(plainEq + 1).trim().replace(/^["']|["']$/g, "");
      const actual = el.getAttribute(k);
      if (actual === null || String(actual) !== v) return false;
    } else {
      if (!el.hasAttribute(expr)) return false;
    }
  }
  return true;
}

function matchesSelector(el, selector) {
  if (!el || el.nodeType !== 1 || !selector) return false;
  const groups = splitTopLevelCommas(selector);
  for (let g = 0; g < groups.length; g++) {
    const chain = splitDescendantTokens(groups[g]);
    if (chain.length === 0) continue;
    if (!matchesCompoundSelector(el, chain[chain.length - 1])) continue;
    let cur = el.parentElement;
    let idx = chain.length - 2;
    while (idx >= 0 && cur) {
      if (matchesCompoundSelector(cur, chain[idx])) {
        idx--;
      }
      cur = cur.parentElement;
    }
    if (idx < 0) return true;
  }
  return false;
}

function decodeEntities(str) {
  if (!str) return "";
  return String(str)
    .replace(/&quot;/g, '"')
    .replace(/&#39;/g, "'")
    .replace(/&lt;/g, "<")
    .replace(/&gt;/g, ">")
    .replace(/&amp;/g, "&");
}

class DOMElement {
  constructor(tagName, ownerDocument) {
    this.nodeType = 1;
    this.tagName = String(tagName || "DIV").toUpperCase();
    this.ownerDocument = ownerDocument;
    this.parentElement = null;
    this.children = [];
    this.childNodes = [];
    this._attrs = new Map();
    this._classes = new Set();
    this._listeners = new Map();
    this.style = parseSimpleStyle("");
    this.scrollTop = 0;
    this.scrollLeft = 0;
    this.scrollHeight = 600;
    this.clientWidth = 1200;
    this.clientHeight = 700;
    this.offsetWidth = 180;
    this.offsetHeight = 36;
    this.offsetTop = 0;
    this.selectionStart = 0;
    this.selectionEnd = 0;
    this.complete = true;
    this.readyState = 4;
    this.paused = true;
    this.ended = false;
    this.currentTime = 0;
    this.duration = 30;
    this.playbackRate = 1.0;
    this.width = 1200;
    this.height = 700;
    this._text = "";
    this._value = undefined;

    const self = this;
    this.classList = {
      add(...names) {
        names.forEach((n) => { if (n) self._classes.add(n); });
        self._syncClassAttr();
      },
      remove(...names) {
        names.forEach((n) => self._classes.delete(n));
        self._syncClassAttr();
      },
      toggle(name, force) {
        const shouldHave = force !== undefined ? Boolean(force) : !self._classes.has(name);
        if (shouldHave) self._classes.add(name);
        else self._classes.delete(name);
        self._syncClassAttr();
        return shouldHave;
      },
      contains(name) {
        return self._classes.has(name);
      }
    };

    this.dataset = new Proxy({}, {
      get(_, prop) {
        if (typeof prop !== "string") return undefined;
        const attrName = "data-" + prop.replace(/[A-Z]/g, (m) => "-" + m.toLowerCase());
        const val = self.getAttribute(attrName);
        return val === null ? undefined : val;
      },
      set(_, prop, val) {
        const attrName = "data-" + prop.replace(/[A-Z]/g, (m) => "-" + m.toLowerCase());
        self.setAttribute(attrName, String(val));
        return true;
      },
      ownKeys() {
        const keys = [];
        self._attrs.forEach((_, k) => {
          if (k.startsWith("data-")) {
            const camel = k.slice(5).replace(/-([a-z])/g, (_, c) => c.toUpperCase());
            keys.push(camel);
          }
        });
        return keys;
      },
      getOwnPropertyDescriptor(_, prop) {
        const attrName = "data-" + String(prop).replace(/[A-Z]/g, (m) => "-" + m.toLowerCase());
        if (self._attrs.has(attrName)) {
          return { enumerable: true, configurable: true, value: self._attrs.get(attrName) };
        }
        return undefined;
      }
    });
  }

  _syncClassAttr() {
    if (this._classes.size > 0) {
      this._attrs.set("class", Array.from(this._classes).join(" "));
    } else {
      this._attrs.delete("class");
    }
  }

  get isConnected() {
    let cur = this;
    while (cur) {
      if (cur === this.ownerDocument.documentElement || cur === this.ownerDocument.body) return true;
      cur = cur.parentElement;
    }
    return false;
  }

  get offsetParent() {
    return this.parentElement;
  }

  get id() {
    return this._attrs.get("id") || "";
  }
  set id(v) {
    this.setAttribute("id", v);
  }

  get name() {
    return this._attrs.get("name") || "";
  }
  set name(v) {
    this.setAttribute("name", v);
  }

  get type() {
    return this._attrs.get("type") || (this.tagName === "BUTTON" ? "submit" : "");
  }
  set type(v) {
    this.setAttribute("type", v);
  }

  get disabled() {
    return this._attrs.has("disabled");
  }
  set disabled(v) {
    if (v) this._attrs.set("disabled", "disabled");
    else this._attrs.delete("disabled");
  }

  get value() {
    if (this._value !== undefined) return this._value;
    if (this.tagName === "TEXTAREA") return this.textContent;
    return this._attrs.get("value") || "";
  }
  set value(v) {
    this._value = String(v);
    this._attrs.set("value", String(v));
  }

  hasAttribute(k) {
    return this._attrs.has(String(k).toLowerCase());
  }

  getAttribute(k) {
    const key = String(k).toLowerCase();
    return this._attrs.has(key) ? this._attrs.get(key) : null;
  }

  setAttribute(k, v) {
    const key = String(k).toLowerCase();
    const val = String(v);
    this._attrs.set(key, val);
    if (key === "class") {
      this._classes.clear();
      val.split(/\s+/).forEach((c) => { if (c) this._classes.add(c); });
    } else if (key === "value" && this._value === undefined) {
      this._value = val;
    }
  }

  removeAttribute(k) {
    const key = String(k).toLowerCase();
    this._attrs.delete(key);
    if (key === "class") this._classes.clear();
  }

  appendChild(child) {
    if (!child) return null;
    if (child.parentElement) {
      const idx = child.parentElement.children.indexOf(child);
      if (idx >= 0) child.parentElement.children.splice(idx, 1);
    }
    child.parentElement = this;
    this.children.push(child);
    this.childNodes = this.children;
    return child;
  }

  get firstElementChild() {
    return this.children.length > 0 ? this.children[0] : null;
  }

  cloneNode(deep) {
    const copy = new DOMElement(this.tagName, this.ownerDocument);
    this._attrs.forEach((v, k) => copy.setAttribute(k, v));
    if (deep) {
      copy._text = this._text;
      this.children.forEach((ch) => copy.appendChild(ch.cloneNode(true)));
    }
    return copy;
  }

  get textContent() {
    let out = this._text || "";
    for (let i = 0; i < this.children.length; i++) {
      out += this.children[i].textContent;
    }
    return out;
  }
  set textContent(v) {
    this.children = [];
    this.childNodes = [];
    this._text = String(v || "");
    if (this.tagName === "TEXTAREA") {
      this._value = this._text;
    }
  }

  get innerHTML() {
    if (this.children.length === 0 && !this._text) return "";
    return this._text + this.children.map((c) => c.outerHTML).join("");
  }
  set innerHTML(html) {
    for (let i = 0; i < this.children.length; i++) {
      this.children[i].parentElement = null;
    }
    this.children = [];
    this.childNodes = [];
    this._text = "";
    if (!html) return;
    const nodes = parseHTMLFragment(String(html), this.ownerDocument);
    nodes.forEach((n) => this.appendChild(n));
  }

  get outerHTML() {
    const tag = this.tagName.toLowerCase();
    let attrs = "";
    this._attrs.forEach((v, k) => {
      attrs += " " + k + '="' + String(v).replace(/"/g, "&quot;") + '"';
    });
    return "<" + tag + attrs + ">" + this.innerHTML + "</" + tag + ">";
  }
  set outerHTML(html) {
    const parent = this.parentElement;
    if (!parent) return;
    const idx = parent.children.indexOf(this);
    if (idx < 0) return;
    const nodes = parseHTMLFragment(String(html), this.ownerDocument);
    this.parentElement = null;
    parent.children.splice(idx, 1, ...nodes);
    nodes.forEach((n) => { n.parentElement = parent; });
    parent.childNodes = parent.children;
  }

  insertAdjacentHTML(pos, html) {
    const nodes = parseHTMLFragment(String(html), this.ownerDocument);
    if (pos === "afterbegin") {
      nodes.forEach((n) => { n.parentElement = this; });
      this.children.unshift(...nodes);
      this.childNodes = this.children;
    } else {
      nodes.forEach((n) => this.appendChild(n));
    }
  }

  matches(sel) {
    return matchesSelector(this, sel);
  }

  closest(sel) {
    let cur = this;
    while (cur && cur.nodeType === 1) {
      if (matchesSelector(cur, sel)) return cur;
      cur = cur.parentElement;
    }
    return null;
  }

  contains(other) {
    let cur = other;
    while (cur) {
      if (cur === this) return true;
      cur = cur.parentElement;
    }
    return false;
  }

  querySelector(sel) {
    const all = this.querySelectorAll(sel);
    return all.length > 0 ? all[0] : null;
  }

  querySelectorAll(sel) {
    const res = [];
    const walk = (node) => {
      for (let i = 0; i < node.children.length; i++) {
        const ch = node.children[i];
        if (matchesSelector(ch, sel)) res.push(ch);
        walk(ch);
      }
    };
    walk(this);
    return res;
  }

  getBoundingClientRect() {
    return { left: 0, top: 0, width: this.clientWidth || 1200, height: this.clientHeight || 700, right: this.clientWidth || 1200, bottom: this.clientHeight || 700 };
  }

  focus() {
    this.ownerDocument.activeElement = this;
  }
  blur() {
    if (this.ownerDocument.activeElement === this) {
      this.ownerDocument.activeElement = this.ownerDocument.body;
    }
  }
  select() {}
  setSelectionRange(s, e) {
    this.selectionStart = s;
    this.selectionEnd = e;
  }
  load() {}
  play() {
    this.paused = false;
    return Promise.resolve();
  }
  pause() {
    this.paused = true;
  }

  getContext(kind) {
    if (kind !== "webgl" && kind !== "experimental-webgl") return null;
    if (!this._gl) {
      const self = this;
      this._glDrawCalls = 0;
      this._glLastInstCount = 0;
      const instExt = {
        vertexAttribDivisorANGLE() {},
        drawArraysInstancedANGLE(_mode, _first, _count, primcount) {
          self._glDrawCalls++;
          self._glLastInstCount = primcount;
        }
      };
      this._gl = {
        VERTEX_SHADER: 35633,
        FRAGMENT_SHADER: 35632,
        COMPILE_STATUS: 35713,
        LINK_STATUS: 35714,
        ARRAY_BUFFER: 34962,
        STATIC_DRAW: 35044,
        DYNAMIC_DRAW: 35048,
        FLOAT: 5126,
        UNSIGNED_BYTE: 5121,
        TEXTURE_2D: 3553,
        TEXTURE0: 33984,
        TEXTURE1: 33985,
        RGBA: 6408,
        LINEAR: 9729,
        CLAMP_TO_EDGE: 33071,
        TEXTURE_MIN_FILTER: 10241,
        TEXTURE_MAG_FILTER: 10240,
        TEXTURE_WRAP_S: 10242,
        TEXTURE_WRAP_T: 10243,
        BLEND: 3042,
        SRC_ALPHA: 770,
        ONE_MINUS_SRC_ALPHA: 771,
        TRIANGLES: 4,
        getExtension(name) { return name === "ANGLE_instanced_arrays" ? instExt : null; },
        createShader() { return {}; },
        shaderSource() {},
        compileShader() {},
        getShaderParameter() { return true; },
        createProgram() { return {}; },
        attachShader() {},
        linkProgram() {},
        getProgramParameter() { return true; },
        useProgram() {},
        createBuffer() { return {}; },
        bindBuffer() {},
        bufferData() {},
        bufferSubData() {},
        getAttribLocation() { return 0; },
        enableVertexAttribArray() {},
        vertexAttribPointer() {},
        getUniformLocation() { return {}; },
        uniform1i() {},
        uniform1f() {},
        uniform2f() {},
        uniform3f() {},
        createTexture() { return {}; },
        activeTexture() {},
        bindTexture() {},
        texParameteri() {},
        texImage2D() {},
        enable() {},
        blendFunc() {},
        viewport() {}
      };
    }
    return this._gl;
  }

  addEventListener(type, fn) {
    if (!this._listeners.has(type)) this._listeners.set(type, []);
    this._listeners.get(type).push(fn);
  }
  removeEventListener(type, fn) {
    const arr = this._listeners.get(type);
    if (!arr) return;
    const idx = arr.indexOf(fn);
    if (idx >= 0) arr.splice(idx, 1);
  }
  dispatchEvent(ev) {
    ev.target = ev.target || this;
    const path = [];
    let cur = this;
    while (cur) {
      path.push(cur);
      cur = cur.parentElement;
    }
    path.push(this.ownerDocument);
    for (let i = 0; i < path.length; i++) {
      if (ev._stopped) break;
      const node = path[i];
      ev.currentTarget = node;
      const list = node._listeners ? node._listeners.get(ev.type) : null;
      if (list) {
        const copy = list.slice();
        for (let j = 0; j < copy.length; j++) {
          copy[j].call(node, ev);
          if (ev._ImmediateStopped) break;
        }
      }
    }
    return !ev.defaultPrevented;
  }
}

const VOID_TAGS = new Set(["area", "base", "br", "col", "embed", "hr", "img", "input", "link", "meta", "param", "source", "track", "wbr", "path", "rect", "circle", "line", "polyline", "polygon"]);

function parseHTMLFragment(html, doc) {
  const root = new DOMElement("fragment", doc);
  const stack = [root];
  const tagRe = /<!--[\s\S]*?-->|<!DOCTYPE[^>]*>|<\/([A-Za-z0-9_-]+)\s*>|<([A-Za-z0-9_-]+)\b((?:"[^"]*"|'[^']*'|[^>"'])*)>|([^<]+)/gi;
  const attrRe = /([^\s/>=]+)(?:=(?:"([^"]*)"|'([^']*)'|([^\s"'>]+)))?/g;

  let m;
  while ((m = tagRe.exec(html)) !== null) {
    const closeTag = m[1];
    const openTag = m[2];
    const rawAttr = m[3] || "";
    const selfClose = rawAttr.trim().endsWith("/");
    const attrStr = selfClose ? rawAttr.replace(/\/\s*$/, "") : rawAttr;
    const textChunk = m[4];

    if (closeTag) {
      const upper = closeTag.toUpperCase();
      for (let i = stack.length - 1; i >= 1; i--) {
        if (stack[i].tagName === upper) {
          stack.length = i;
          break;
        }
      }
    } else if (openTag) {
      const lower = openTag.toLowerCase();
      if (lower === "doctype") continue;
      const el = new DOMElement(openTag, doc);
      let am;
      attrRe.lastIndex = 0;
      while ((am = attrRe.exec(attrStr)) !== null) {
        const ak = am[1];
        const av = am[2] !== undefined ? am[2] : am[3] !== undefined ? am[3] : am[4] !== undefined ? am[4] : "";
        el.setAttribute(ak, decodeEntities(av));
      }
      stack[stack.length - 1].appendChild(el);
      if (!selfClose && !VOID_TAGS.has(lower)) {
        stack.push(el);
      }
    } else if (textChunk !== undefined) {
      const decoded = decodeEntities(textChunk);
      const top = stack[stack.length - 1];
      if (top.children.length === 0) {
        top._text += decoded;
        if (top.tagName === "TEXTAREA") {
          top._value = top._text;
        }
      } else {
        const span = new DOMElement("#text", doc);
        span._text = decoded;
        top._text += decoded;
      }
    }
  }
  return root.children.slice().map((c) => {
    c.parentElement = null;
    return c;
  });
}

class CustomFormData {
  constructor(formEl) {
    this._pairs = [];
    if (formEl && formEl.querySelectorAll) {
      const fields = formEl.querySelectorAll("input, textarea, select");
      for (let i = 0; i < fields.length; i++) {
        const f = fields[i];
        if (f.disabled) continue;
        const name = f.getAttribute("name");
        if (!name) continue;
        const type = (f.getAttribute("type") || "").toLowerCase();
        if (type === "checkbox" || type === "radio") {
          if (!f.checked && !f.hasAttribute("checked")) continue;
        }
        this._pairs.push([name, f.value || ""]);
      }
    }
  }
  set(k, v) {
    const key = String(k);
    const val = String(v);
    for (let i = 0; i < this._pairs.length; i++) {
      if (this._pairs[i][0] === key) {
        this._pairs[i][1] = val;
        return;
      }
    }
    this._pairs.push([key, val]);
  }
  forEach(cb) {
    for (let i = 0; i < this._pairs.length; i++) {
      cb(this._pairs[i][1], this._pairs[i][0]);
    }
  }
}

function makeEvent(type, props) {
  return Object.assign({
    type: type,
    button: 0,
    clientX: 0,
    clientY: 0,
    key: "",
    code: "",
    ctrlKey: false,
    metaKey: false,
    shiftKey: false,
    altKey: false,
    cancelable: true,
    defaultPrevented: false,
    _stopped: false,
    _ImmediateStopped: false,
    preventDefault() { this.defaultPrevented = true; },
    stopPropagation() { this._stopped = true; },
    stopImmediatePropagation() { this._stopped = true; this._ImmediateStopped = true; }
  }, props || {});
}

(async () => {
  const baseURL = process.env.ST_TEST_BASE_URL;
  const appJSSrc = process.env.ST_TEST_APP_JS;
  const actionLog = [];
  const inFlight = new Set();
  process.on("unhandledRejection", (reason) => {
    console.error("UNHANDLED_REJECTION=" + (reason && reason.stack ? reason.stack : String(reason)));
    process.exit(1);
  });

  const initResp = await fetch(baseURL + "/");
  if (!initResp.ok) {
    throw new Error("GET / failed: " + initResp.status);
  }
  const initHTML = await initResp.text();

  const doc = {
    nodeType: 9,
    _listeners: new Map(),
    documentElement: null,
    body: null,
    activeElement: null,
    fullscreenElement: null,
    addEventListener(type, fn) {
      if (!this._listeners.has(type)) this._listeners.set(type, []);
      this._listeners.get(type).push(fn);
    },
    removeEventListener() {},
    dispatchEvent(ev) {
      ev.target = ev.target || this.body || this;
      const list = this._listeners.get(ev.type) || [];
      list.slice().forEach((fn) => fn.call(this, ev));
      return !ev.defaultPrevented;
    },
    querySelector(sel) {
      return this.documentElement ? this.documentElement.querySelector(sel) : null;
    },
    querySelectorAll(sel) {
      return this.documentElement ? this.documentElement.querySelectorAll(sel) : [];
    },
    getElementById(id) {
      return this.querySelector("#" + id);
    },
    elementFromPoint() {
      return this.body;
    }
  };
  doc.ownerDocument = doc;

  const parsedRoot = parseHTMLFragment(initHTML, doc);
  const htmlEl = parsedRoot.find((el) => el.tagName === "HTML") || parsedRoot[0];
  doc.documentElement = htmlEl;
  doc.body = htmlEl.querySelector("body") || htmlEl;
  doc.activeElement = doc.body;

  const win = {
    document: doc,
    navigator: {
      userAgent: "Mozilla/5.0 StoryTellerTest/1.0",
      mediaDevices: {
        getUserMedia: async () => ({ getTracks: () => [{ stop() {} }] })
      }
    },
    innerWidth: 1280,
    innerHeight: 800,
    devicePixelRatio: 2,
    __STUDIO_MSDF_DATA: "",
    CSS: { escape: (s) => String(s) },
    FormData: CustomFormData,
    URLSearchParams: URLSearchParams,
    AbortController: AbortController,
    TextDecoder: TextDecoder,
    Uint8Array: Uint8Array,
    Int16Array: Int16Array,
    Float32Array: Float32Array,
    Uint32Array: Uint32Array,
    DataView: DataView,
    ArrayBuffer: ArrayBuffer,
    Map: Map,
    Set: Set,
    WeakMap: WeakMap,
    WeakSet: WeakSet,
    Promise: Promise,
    Math: Math,
    Number: Number,
    String: String,
    Boolean: Boolean,
    Object: Object,
    Array: Array,
    RegExp: RegExp,
    Error: Error,
    Date: Date,
    Infinity: Infinity,
    NaN: NaN,
    parseInt: parseInt,
    parseFloat: parseFloat,
    isNaN: isNaN,
    console: console,
    performance: { now: () => Date.now() },
    requestAnimationFrame: (cb) => { cb(Date.now()); return 1; },
    cancelAnimationFrame: () => {},
    setTimeout: setTimeout,
    clearTimeout: clearTimeout,
    btoa: (str) => Buffer.from(str, "binary").toString("base64"),
    atob: (b64) => Buffer.from(b64, "base64").toString("binary"),
    Audio: class {
      constructor() { this.src = ""; this.paused = true; this.currentTime = 0; this.duration = 2.0; }
      play() { this.paused = false; return Promise.resolve(); }
      pause() { this.paused = true; }
      getAttribute(k) { return this["_" + k] || ""; }
      setAttribute(k, v) { this["_" + k] = String(v); }
    },
    AudioContext: class {
      constructor() { this.sampleRate = 24000; this.destination = {}; }
      createMediaStreamSource() { return { connect() {}, disconnect() {} }; }
      createScriptProcessor() {
        const node = {
          onaudioprocess: null,
          connect() {
            if (typeof this.onaudioprocess === "function") {
              const fakeBuf = new Float32Array(24000);
              for (let i = 0; i < fakeBuf.length; i++) fakeBuf[i] = Math.sin(i * 0.1) * 0.4;
              this.onaudioprocess({ inputBuffer: { getChannelData: () => fakeBuf } });
            }
          },
          disconnect() {}
        };
        return node;
      }
      close() { return Promise.resolve(); }
    },
    ResizeObserver: class { observe() {} disconnect() {} },
    Image: class { set src(v) { this._src = v; if (this.onload) this.onload(); } },
    addEventListener(type, fn) { doc.addEventListener("win:" + type, fn); },
    close() {},
    __trackAction: (p) => {
      inFlight.add(p);
      p.finally(() => inFlight.delete(p));
      return p;
    },
    fetch: async (urlStr, opts) => {
      const fullURL = urlStr.startsWith("http") ? urlStr : baseURL + urlStr;
      let bodyStr = "";
      if (opts && opts.body) {
        bodyStr = String(opts.body);
        const ct = opts.headers && opts.headers["Content-Type"] ? String(opts.headers["Content-Type"]) : "";
        if (!ct.includes("text/x-wfp")) {
          throw new Error("Expected WFP Content-Type text/x-wfp, got: " + ct);
        }
        const parsed = typeof win.decodeWFP === "function" ? win.decodeWFP(bodyStr) : {};
        const act = parsed.action;
        if (act) actionLog.push(act + "(" + bodyStr.replace(/\n/g, "|") + ")");
      }
      const p = fetch(fullURL, opts);
      inFlight.add(p);
      try {
        const res = await p;
        return res;
      } finally {
        inFlight.delete(p);
      }
    }
  };
  win.window = win;
  win.globalThis = win;

  vm.createContext(win);
  vm.runInContext(appJSSrc, win, { filename: "app.js" });
  win.__inFlight = inFlight;
  vm.runInContext(
    "const _origSend = sendServerAction; sendServerAction = async function(act, params) { const p = (async () => { try { return await _origSend(act, params); } catch (e) { console.error('SEND_ACTION_ERR=' + act + ':' + (e && e.stack ? e.stack : String(e))); throw e; } })(); __inFlight.add(p); try { return await p; } finally { __inFlight.delete(p); } };",
    win
  );

  // Fire DOMContentLoaded to bind declarative action bridge & canvas gestures
  doc.dispatchEvent(makeEvent("DOMContentLoaded", { target: doc }));

  const waitForActions = async () => {
    while (inFlight.size > 0) {
      await Promise.all(Array.from(inFlight));
    }
    await new Promise((r) => setTimeout(r, 5));
    while (inFlight.size > 0) {
      await Promise.all(Array.from(inFlight));
    }
  };
  const clickEl = async (selectorOrEl) => {
    const el = typeof selectorOrEl === "string" ? doc.querySelector(selectorOrEl) : selectorOrEl;
    if (!el) throw new Error("Missing element to click: " + selectorOrEl);
    el.dispatchEvent(makeEvent("click", { target: el }));
    await waitForActions();
  };

  // 1. Switch to Node Studio (#view-editor) via Topbar tab click
  await clickEl('button.topbar-tab[data-view="editor"]');
  if (!doc.querySelector("#view-editor")) {
    const appChildren = doc.querySelector("#app") ? doc.querySelector("#app").children.map((c) => c.tagName + "#" + c.id).join(",") : "NONE";
    const allIds = doc.querySelectorAll("*").map((c) => c.tagName + "#" + c.id).filter((s) => !s.endsWith("#")).join(",");
    throw new Error("Expected #view-editor; actionLog=" + actionLog.join(" | ") + " appChildren=" + appChildren + " allIds=" + allIds);
  }

  // 2. Click OpenNewBookModal button in Node Studio
  await clickEl('button[action="OpenNewBookModal"]');
  const newBookTitle = doc.querySelector("#new-book-title");
  if (!newBookTitle) {
    throw new Error("Expected #new-book-title in #modals-root after OpenNewBookModal");
  }

  // 3. Test Escape key closing modal
  doc.body.dispatchEvent(makeEvent("keydown", { key: "Escape", code: "Escape", target: doc.body }));
  await waitForActions();
  if (doc.querySelector("#new-book-title")) {
    throw new Error("Expected Escape key to close NewBookModal");
  }

  // 4. Re-open NewBookModal and submit form via button[type="submit"] click
  await clickEl('button[action="OpenNewBookModal"]');
  const titleInp = doc.querySelector("#new-book-title");
  titleInp.value = "Książka Testowa JS";
  await clickEl('form[action="CreateNewBook"] button[type="submit"]');

  const viewport = doc.querySelector("#timeline-scroll-viewport");
  const canvas = doc.querySelector("#studio-canvas");
  if (!viewport || !canvas) {
    throw new Error("Expected #timeline-scroll-viewport and #studio-canvas after CreateNewBook");
  }
  const nodesCount = vm.runInContext("studioGraphState.nodes.length", win);
  if (nodesCount < 1 || !canvas._gl || canvas._glDrawCalls < 1) {
    throw new Error("Expected WebGL canvas to initialize and draw nodes, got nodes=" + nodesCount + " actionLog=" + actionLog.join(" | ") + " toast=" + (doc.querySelector("#toast-banner") ? doc.querySelector("#toast-banner").textContent : ""));
  }

  // 5. Click empty canvas to clear selection, then click node card on WebGL canvas
  canvas.dispatchEvent(makeEvent("mousedown", { button: 0, clientX: 10, clientY: 10, target: canvas }));
  canvas.dispatchEvent(makeEvent("mouseup", { button: 0, clientX: 10, clientY: 10, target: canvas }));
  await waitForActions();
  if (!doc.querySelector("#editor-context-form").classList.contains("hidden")) {
    throw new Error("Expected #editor-context-form to hide after clicking empty canvas");
  }

  const nodeScreenPt = vm.runInContext("(() => { const n = studioGraphState.nodes[0]; const z = editorCanvasState.zoom; return { x: Math.round((n.x + 60) * z + editorCanvasState.panX), y: Math.round((n.y + 42) * z + editorCanvasState.panY), titleX: Math.round((n.x + 60) * z + editorCanvasState.panX), titleY: Math.round((n.y + 16) * z + editorCanvasState.panY), addX: Math.round((n.x + 20) * z + editorCanvasState.panX), addY: Math.round((n.y + n.h - 20) * z + editorCanvasState.panY), editX: Math.round((n.x + n.w - 20) * z + editorCanvasState.panX), editY: Math.round((n.y + n.h - 20) * z + editorCanvasState.panY), gripX: Math.round((n.x + 12) * z + editorCanvasState.panX), gripY: Math.round((n.y + 16) * z + editorCanvasState.panY) }; })()", win);

  canvas.dispatchEvent(makeEvent("mousedown", { button: 0, clientX: nodeScreenPt.x, clientY: nodeScreenPt.y, target: canvas }));
  canvas.dispatchEvent(makeEvent("mouseup", { button: 0, clientX: nodeScreenPt.x, clientY: nodeScreenPt.y, target: canvas }));
  await waitForActions();
  if (!doc.querySelector("#editor-context-form").classList.contains("hidden") || !doc.querySelector("#editor-context-panel").classList.contains("hidden")) {
    throw new Error("Expected #editor-context-panel and #editor-context-form to remain hidden when clicking a node card on canvas");
  }
  if (!vm.runInContext("selectedNodeIds.has(studioGraphState.nodes[0].id)", win)) {
    throw new Error("Expected clicking node card on canvas to select the node");
  }

  // 5b. Click node title on WebGL canvas to edit title inline via #canvas-node-title-input
  canvas.dispatchEvent(makeEvent("mousedown", { button: 0, clientX: nodeScreenPt.titleX, clientY: nodeScreenPt.titleY, target: canvas }));
  canvas.dispatchEvent(makeEvent("mouseup", { button: 0, clientX: nodeScreenPt.titleX, clientY: nodeScreenPt.titleY, target: canvas }));
  const nodeTitleInp = doc.querySelector("#canvas-node-title-input");
  if (!nodeTitleInp || nodeTitleInp.classList.contains("hidden")) {
    throw new Error("Expected clicking node title on canvas to open #canvas-node-title-input");
  }
  const expectedTitleFont = vm.runInContext("(13 * (editorCanvasState.zoom || 1)).toFixed(2) + 'px'", win);
  if (nodeTitleInp.style.fontSize !== expectedTitleFont) {
    throw new Error("Expected #canvas-node-title-input fontSize " + expectedTitleFont + ", got " + nodeTitleInp.style.fontSize);
  }
  const expectedTitleLeft = vm.runInContext("((studioGraphState.nodes[0].x + 28) * (editorCanvasState.zoom || 1) + (editorCanvasState.panX || 0)).toFixed(2) + 'px'", win);
  if (nodeTitleInp.style.left !== expectedTitleLeft) {
    throw new Error("Expected #canvas-node-title-input left " + expectedTitleLeft + ", got " + nodeTitleInp.style.left);
  }
  nodeTitleInp.value = "Scena Startowa Inline";
  nodeTitleInp.dispatchEvent(makeEvent("keydown", { key: "Enter", target: nodeTitleInp }));
  await waitForActions();
  if (vm.runInContext("studioGraphState.nodes[0].name", win) !== "Scena Startowa Inline") {
    throw new Error("Expected Enter on #canvas-node-title-input to update node name on canvas");
  }

  // 6. Click add-choice icon button (branch-plus) directly on WebGL canvas node card footer
  canvas.dispatchEvent(makeEvent("mousedown", { button: 0, clientX: nodeScreenPt.addX, clientY: nodeScreenPt.addY, target: canvas }));
  canvas.dispatchEvent(makeEvent("mouseup", { button: 0, clientX: nodeScreenPt.addX, clientY: nodeScreenPt.addY, target: canvas }));
  await waitForActions();
  const portCountAfterAdd = vm.runInContext("studioGraphState.nodes[0].ports.length", win);
  if (portCountAfterAdd < 2) {
    throw new Error("Expected canvas add-choice icon click to add choices (ports >= 2), got " + portCountAfterAdd);
  }

  // 6b. Click choice row label on WebGL canvas node card to edit choice inline via #canvas-choice-input
  const choiceLabelPt = vm.runInContext("(() => { const n = studioGraphState.nodes[0]; const z = editorCanvasState.zoom; return { x: Math.round((n.x + 50) * z + editorCanvasState.panX), y: Math.round((n.y + 36 + 14) * z + editorCanvasState.panY) }; })()", win);
  canvas.dispatchEvent(makeEvent("mousedown", { button: 0, clientX: choiceLabelPt.x, clientY: choiceLabelPt.y, target: canvas }));
  canvas.dispatchEvent(makeEvent("mouseup", { button: 0, clientX: choiceLabelPt.x, clientY: choiceLabelPt.y, target: canvas }));
  const choiceInp = doc.querySelector("#canvas-choice-input");
  if (!choiceInp || choiceInp.classList.contains("hidden")) {
    throw new Error("Expected clicking choice label on canvas node to open #canvas-choice-input");
  }
  const expectedChoiceFont = vm.runInContext("(11 * (editorCanvasState.zoom || 1)).toFixed(2) + 'px'", win);
  if (choiceInp.style.fontSize !== expectedChoiceFont) {
    throw new Error("Expected #canvas-choice-input fontSize " + expectedChoiceFont + ", got " + choiceInp.style.fontSize);
  }
  choiceInp.value = "Wybór Inline 1";
  choiceInp.dispatchEvent(makeEvent("keydown", { key: "Enter", target: choiceInp }));
  await waitForActions();
  if (vm.runInContext("studioGraphState.nodes[0].ports[0].label", win) !== "Wybór Inline 1") {
    throw new Error("Expected Enter on #canvas-choice-input to update choice label on canvas");
  }

  // 6c. Connect choice ports to two parallel child scenes, then click gray 8x8 branch-radio square on secondary parallel node to promote it to primary
  const rootId = vm.runInContext("studioGraphState.nodes[0].id", win);
  await vm.runInContext("sendServerAction('BranchChoiceScene', { scene: '" + rootId + "', choice_idx: '0' })", win);
  await waitForActions();
  await vm.runInContext("sendServerAction('BranchChoiceScene', { scene: '" + rootId + "', choice_idx: '1' })", win);
  await waitForActions();
  const secRadioPt = vm.runInContext("(() => { const sec = studioGraphState.nodes.find((n) => n.branchRole === 'secondary'); if (!sec) return null; const z = editorCanvasState.zoom; return { id: sec.id, x: Math.round((sec.branchRadioX + 4) * z + editorCanvasState.panX), y: Math.round((sec.branchRadioY + 4) * z + editorCanvasState.panY) }; })()", win);
  if (!secRadioPt) {
    throw new Error("Expected parallel child node with branchRole=secondary on canvas");
  }
  canvas.dispatchEvent(makeEvent("mousedown", { button: 0, clientX: secRadioPt.x, clientY: secRadioPt.y, target: canvas }));
  canvas.dispatchEvent(makeEvent("mouseup", { button: 0, clientX: secRadioPt.x, clientY: secRadioPt.y, target: canvas }));
  await waitForActions();
  const promotedRole = vm.runInContext("(() => { const n = studioGraphState.nodeById.get('" + secRadioPt.id + "'); return n ? n.branchRole : ''; })()", win);
  if (promotedRole !== "primary") {
    throw new Error("Expected clicking gray branch-radio square to promote node to primary, got " + promotedRole);
  }

  // 6d. Drag choice-grip handle on WebGL canvas node card from row 0 to row 1 to reorder choices live + persist via ReorderChoice
  const beforeDragZeroLabel = vm.runInContext("studioGraphState.nodes[0].ports[0].label", win);
  const choiceGripPt = vm.runInContext("(() => { const n = studioGraphState.nodes[0]; const hit = hitTestStudioCanvas(n.x + 10, n.y + 36 + 14); if (!hit || hit.type !== 'choice-grip') return null; const z = editorCanvasState.zoom; return { x: Math.round((n.x + 10) * z + editorCanvasState.panX), y0: Math.round((n.y + 36 + 14) * z + editorCanvasState.panY), y1: Math.round((n.y + 36 + 42) * z + editorCanvasState.panY) }; })()", win);
  if (!choiceGripPt) {
    throw new Error("Expected hitTestStudioCanvas to return choice-grip on left side of choice row");
  }
  canvas.dispatchEvent(makeEvent("mousedown", { button: 0, clientX: choiceGripPt.x, clientY: choiceGripPt.y0, target: canvas }));
  canvas.dispatchEvent(makeEvent("mousemove", { button: 0, clientX: choiceGripPt.x, clientY: choiceGripPt.y1, target: canvas }));
  canvas.dispatchEvent(makeEvent("mouseup", { button: 0, clientX: choiceGripPt.x, clientY: choiceGripPt.y1, target: canvas }));
  await waitForActions();
  const reorderedSecondLabel = vm.runInContext("studioGraphState.nodes[0].ports[1].label", win);
  if (reorderedSecondLabel !== beforeDragZeroLabel) {
    throw new Error("Expected choice-grip drag to reorder '" + beforeDragZeroLabel + "' from idx 0 to idx 1, got " + reorderedSecondLabel);
  }
  const directRightForkPtsLen = vm.runInContext("(() => { const n = studioGraphState.nodes[0]; const rightWire = studioGraphState.wires.find((w) => w.fromNode === n && w.toNode && w.x2 >= n.x + n.w + 16); return rightWire && rightWire.pts ? rightWire.pts.length : 0; })()", win);
  if (directRightForkPtsLen !== 3) {
    throw new Error("Expected right-branch choice wire to route as a clean 3-point L-wire without extra kinks, got pts.length=" + directRightForkPtsLen);
  }

  // 7. Drag node grip on WebGL canvas (+48px X, -24px Y) and verify grid snap + SaveEditorState
  const startNodeX = vm.runInContext("studioGraphState.nodes[0].x", win);
  const updatedGripPt = vm.runInContext("(() => { const n = studioGraphState.nodes[0]; const z = editorCanvasState.zoom; return { x: Math.round((n.x + 12) * z + editorCanvasState.panX), y: Math.round((n.y + 16) * z + editorCanvasState.panY) }; })()", win);
  canvas.dispatchEvent(makeEvent("mousedown", { button: 0, clientX: updatedGripPt.x, clientY: updatedGripPt.y, target: canvas }));
  canvas.dispatchEvent(makeEvent("mousemove", { button: 0, clientX: updatedGripPt.x + 48, clientY: updatedGripPt.y - 24, target: canvas }));
  canvas.dispatchEvent(makeEvent("mouseup", { button: 0, clientX: updatedGripPt.x + 48, clientY: updatedGripPt.y - 24, target: canvas }));
  await waitForActions();
  const movedNodeX = vm.runInContext("studioGraphState.nodes[0].x", win);
  if (movedNodeX !== startNodeX + 48) {
    throw new Error("Expected node grip drag to snap +48px on 24px grid, got " + startNodeX + " -> " + movedNodeX);
  }

  // 7a. Verify node-on-node collision prevention (48px = 2 grid jumps), single-input socket color inheritance, and frustum culling
  const collisionPrevented = vm.runInContext("(() => { const n0 = studioGraphState.nodes[0]; const n1 = studioGraphState.nodes[1]; const dx = n1.x - n0.x; const dy = n1.y - n0.y; const res = resolveValidNodeDragDelta([{ node: n0, sceneId: n0.id, startX: n0.x, startY: n0.y }], dx, dy, 0, 0); const nx = snapToEditorGrid(n0.x + res.dx); const ny = snapToEditorGrid(n0.y + res.dy); return !(nx < n1.right + 48 && nx + n0.w + 48 > n1.left && ny < n1.bottom + 48 && ny + n0.h + 48 > n1.top); })()", win);
  if (!collisionPrevented) {
    throw new Error("Expected resolveValidNodeDragDelta to enforce 48px (2 grid jumps) spacing between nodes");
  }
  const singleInColoredOk = vm.runInContext("(() => { const w = studioGraphState.wires.find((it) => it.fromNode && it.fromNode.hasChoices && it.fromNode.ports && it.fromNode.ports.length >= 2 && it.toNode && studioGraphState.wires.filter((x) => x.toNode === it.toNode).length === 1); return Boolean(w && w.colorIdx >= 0 && w.toNode.inColorIdx === w.colorIdx); })()", win);
  if (!singleInColoredOk) {
    throw new Error("Expected single-input target node to inherit its incoming choice wire colorIdx on its input socket");
  }
  const multiInDistinctColorsOk = vm.runInContext("(() => { const byT = new Map(); for (const w of studioGraphState.wires) { if (!w.targetGroupKey) continue; const g = byT.get(w.targetGroupKey) || []; g.push(w); byT.set(w.targetGroupKey, g); } for (const g of byT.values()) { if (g.length >= 2 && g.length <= 8) { const s = new Set(g.map((w) => w.colorIdx)); if (s.size !== g.length) return false; } } return true; })()", win);
  if (!multiInDistinctColorsOk) {
    throw new Error("Expected all incoming wires to any multi-input target to have distinct colorIdx values");
  }
  const culledQuadCount = vm.runInContext("(() => { const savedPanX = editorCanvasState.panX; const savedPanY = editorCanvasState.panY; editorCanvasState.panX = 200000; editorCanvasState.panY = 200000; renderStudioCanvas(); const c = studioGL.staticInstCount; editorCanvasState.panX = savedPanX; editorCanvasState.panY = savedPanY; renderStudioCanvas(); return c; })()", win);
  if (culledQuadCount !== 1) {
    throw new Error("Expected frustum culling to emit only 1 background grid quad when all nodes are off-screen, got " + culledQuadCount);
  }

  // 7b. Drag a wire segment on WebGL canvas to insert a control pin (wire-pin), then double-click pin to remove it
  const wireMidScreenPt = vm.runInContext("(() => { const w = studioGraphState.wires[0]; const p0 = w.pts[0]; const p1 = w.pts[1]; const wx = (p0.x + p1.x) * 0.5; const wy = (p0.y + p1.y) * 0.5; const z = editorCanvasState.zoom; return { x: Math.round(wx * z + editorCanvasState.panX), y: Math.round(wy * z + editorCanvasState.panY) }; })()", win);
  canvas.dispatchEvent(makeEvent("mousedown", { button: 0, clientX: wireMidScreenPt.x, clientY: wireMidScreenPt.y, target: canvas }));
  canvas.dispatchEvent(makeEvent("mousemove", { button: 0, clientX: wireMidScreenPt.x + 36, clientY: wireMidScreenPt.y + 24, target: canvas }));
  canvas.dispatchEvent(makeEvent("mouseup", { button: 0, clientX: wireMidScreenPt.x + 36, clientY: wireMidScreenPt.y + 24, target: canvas }));
  await waitForActions();
  const pinCountAfterDrag = vm.runInContext("studioGraphState.wires[0].pins ? studioGraphState.wires[0].pins.length : 0", win);
  if (pinCountAfterDrag !== 1) {
    throw new Error("Expected dragging wire segment to create 1 control pin, got " + pinCountAfterDrag);
  }
	const pinBarsOk = vm.runInContext("(() => { const mockWire = { pts: [{ x: 200, y: 200 }, { x: 200, y: 300 }, { x: 50, y: 300 }, { x: 50, y: 80 }, { x: 320, y: 80 }, { x: 320, y: 160 }] }; const b0 = computeWirePinSegmentBar(mockWire, { x: 200, y: 250 }); const b1 = computeWirePinSegmentBar(mockWire, { x: 120, y: 300 }); const b2 = computeWirePinSegmentBar(mockWire, { x: 50, y: 180 }); const b3 = computeWirePinSegmentBar(mockWire, { x: 180, y: 80 }); return Boolean(b0 && !b0.isHoriz && b1 && b1.isHoriz && b2 && !b2.isHoriz && b3 && b3.isHoriz); })()", win);
	if (!pinBarsOk) {
		throw new Error("Expected computeWirePinSegmentBar to resolve vertical and horizontal bars along wire segments");
	}
  const pinScreenPt = vm.runInContext("(() => { const p = studioGraphState.wires[0].pins[0]; const z = editorCanvasState.zoom; return { x: Math.round(p.x * z + editorCanvasState.panX), y: Math.round(p.y * z + editorCanvasState.panY) }; })()", win);
  canvas.dispatchEvent(makeEvent("dblclick", { button: 0, clientX: pinScreenPt.x, clientY: pinScreenPt.y, target: canvas }));
  await waitForActions();
  const pinCountAfterDblClick = vm.runInContext("studioGraphState.wires[0].pins ? studioGraphState.wires[0].pins.length : 0", win);
  if (pinCountAfterDblClick !== 0) {
    throw new Error("Expected double-clicking wire-pin to remove it, got " + pinCountAfterDblClick);
  }

  // 8. Add 2 bool variables in Variables panel, then double-click node card on WebGL canvas to open #modal-scene-text
  await clickEl("#btn-editor-variables");
  await clickEl('button.var-section-add-btn[data-kind="bool"]');
  const firstVarInp = doc.querySelector("input.var-name-input");
  firstVarInp.value = "cwiczenia_z_wioslem";
  firstVarInp.dispatchEvent(makeEvent("change", { target: firstVarInp }));
  await waitForActions();
  await clickEl('button.var-section-add-btn[data-kind="bool"]');
  const allVarInps = doc.querySelectorAll("input.var-name-input");
  allVarInps[1].value = "wypil_gorzalke";
  allVarInps[1].dispatchEvent(makeEvent("change", { target: allVarInps[1] }));
  await waitForActions();

  const canvasForDbl = doc.querySelector("#studio-canvas");
  const beforeModalPan = vm.runInContext("({ panX: editorCanvasState.panX, panY: editorCanvasState.panY })", win);
  const afterMovePt = vm.runInContext("(() => { const n = studioGraphState.nodes[0]; const z = editorCanvasState.zoom; return { x: Math.round((n.x + 60) * z + editorCanvasState.panX), y: Math.round((n.y + 20) * z + editorCanvasState.panY) }; })()", win);
  canvasForDbl.dispatchEvent(makeEvent("dblclick", { button: 0, clientX: afterMovePt.x, clientY: afterMovePt.y, target: canvasForDbl }));
  await waitForActions();
  const afterModalPan = vm.runInContext("({ panX: editorCanvasState.panX, panY: editorCanvasState.panY })", win);
  if (afterModalPan.panX !== beforeModalPan.panX || afterModalPan.panY !== beforeModalPan.panY) {
    throw new Error("Expected opening #modal-scene-text to keep canvas panX/panY unchanged, got before=" + beforeModalPan.panX + "," + beforeModalPan.panY + " after=" + afterModalPan.panX + "," + afterModalPan.panY);
  }
  const modalTa = doc.querySelector("#modal-scene-textarea");
  if (!modalTa) {
    throw new Error("Expected double-clicking node card on canvas to open #modal-scene-text");
  }

  // 9. Toggle scene settings inside #modal-scene-text, edit textarea, and click back button
  await clickEl("[data-scene-settings-toggle]");
  const settingsBlock = doc.querySelector("#modal-scene-settings-block");
  if (!settingsBlock || settingsBlock.classList.contains("hidden")) {
    throw new Error("Expected [data-scene-settings-toggle] to unhide #modal-scene-settings-block");
  }
  if (modalTa.getAttribute("autocorrect") !== "off" || modalTa.getAttribute("spellcheck") !== "false") {
    throw new Error("Expected #modal-scene-textarea to disable autocorrect and spellcheck");
  }
  modalTa.value = "Akapit wstępu.\nif: wypil_gorzalke\nKakakakaka\nelse:\nAktywna gałąź\nendif:";
  modalTa.dispatchEvent(makeEvent("input", { target: modalTa }));
  const synSegs = doc.querySelectorAll("#modal-scene-prose-hl .prose-hl-seg.is-syntax:not(.hidden)");
  const activeSegs = doc.querySelectorAll("#modal-scene-prose-hl .prose-hl-seg:not(.is-syntax):not(.hidden)");
  if (synSegs.length !== 2 || !synSegs[0].textContent.includes("if: wypil_gorzalke\nKakakakaka\nelse:") || !synSegs[1].textContent.includes("endif:")) {
    throw new Error("Expected #modal-scene-prose-hl to gray out if:/else:/endif: and inactive branch text, got count=" + synSegs.length);
  }
  if (activeSegs.length !== 2 || !activeSegs[1].textContent.includes("Aktywna gałąź")) {
    throw new Error("Expected #modal-scene-prose-hl to keep active branch text bright, got count=" + activeSegs.length);
  }
  modalTa.value = "Treść zapisana przez mostek JS.";
  modalTa.dispatchEvent(makeEvent("input", { target: modalTa }));

  // 9b. Test .syn-autocomplete-dropdown ArrowDown/ArrowUp + Enter, excluding already-used variables, and automatic space insertion before next variable in set:
  const effInp = doc.querySelector('#modal-scene-text input.choice-rule-input[name="choice_effects_0"]');
  if (!effInp) {
    throw new Error("Expected #modal-scene-text input.choice-rule-input[name=choice_effects_0]");
  }
  effInp.value = "cwiczenia_z_wioslem = false?";
  effInp.selectionStart = effInp.value.length;
  effInp.selectionEnd = effInp.value.length;
  effInp.dispatchEvent(makeEvent("input", { target: effInp }));
  const synDrop = doc.querySelector("#modal-syn-autocomplete-dropdown");
  if (!synDrop || synDrop.classList.contains("hidden")) {
    throw new Error("Expected #modal-syn-autocomplete-dropdown to open when typing '?' in set:");
  }
  const visibleVarBtns = synDrop.querySelectorAll(".syn-dropdown-item:not(.hidden)");
  if (visibleVarBtns.length !== 1 || visibleVarBtns[0].getAttribute("data-syn-var-name") !== "wypil_gorzalke") {
    throw new Error("Expected already-used variable 'cwiczenia_z_wioslem' to be hidden from suggestions, leaving 'wypil_gorzalke'");
  }
  effInp.dispatchEvent(makeEvent("keydown", { key: "Enter", target: effInp }));
  if (effInp.value !== "cwiczenia_z_wioslem = false wypil_gorzalke") {
    throw new Error("Expected Enter on 'cwiczenia_z_wioslem = false?' to auto-insert space and select 'wypil_gorzalke', got: " + effInp.value);
  }
  // Also verify ArrowDown/ArrowUp navigation when both variables are available on empty input
  effInp.value = "?";
  effInp.selectionStart = 1;
  effInp.selectionEnd = 1;
  effInp.dispatchEvent(makeEvent("input", { target: effInp }));
  effInp.dispatchEvent(makeEvent("keydown", { key: "ArrowDown", target: effInp }));
  effInp.dispatchEvent(makeEvent("keydown", { key: "Enter", target: effInp }));
  if (effInp.value !== "wypil_gorzalke") {
    throw new Error("Expected ArrowDown + Enter on '?' to select 2nd variable 'wypil_gorzalke', got: " + effInp.value);
  }

  const openModalScId = doc.querySelector('#modal-scene-text input[name="scene_id"]').value;
  if (doc.querySelector("#editor-context-form").getAttribute("data-context-scene-id") !== openModalScId) {
    throw new Error("Expected canvas context selection to be synced with open #modal-scene-text scene_id: " + openModalScId);
  }
  const navNextBtn = doc.querySelector('#modal-scene-text button[action="NavigateSceneTextModal"]');
  if (navNextBtn) {
    await clickEl('#modal-scene-text button[action="NavigateSceneTextModal"]');
    const nextModalScId = doc.querySelector('#modal-scene-text input[name="scene_id"]').value;
    if (doc.querySelector("#editor-context-form").getAttribute("data-context-scene-id") !== nextModalScId) {
      throw new Error("Expected NavigateSceneTextModal to sync canvas selection to " + nextModalScId);
    }
  }

  await clickEl('button.scene-text-back-btn[action="CloseSceneTextModal"]');
  if (doc.querySelector("#modal-scene-text")) {
    throw new Error("Expected CloseSceneTextModal to close #modal-scene-text");
  }

  // 10. Built-in Voice Recorder & Trimmer (ToggleBookNarrated -> ToggleSceneVoiceRecorder -> Record -> Playhead -> Trim)
  await clickEl("#btn-book-settings");
  const narratedBtn = doc.querySelector('#editor-book-form button[action="ToggleBookNarrated"]');
  if (narratedBtn && !narratedBtn.classList.contains("active")) {
    await clickEl(narratedBtn);
  }
  await clickEl("#btn-book-settings");
  const selectNodePt = vm.runInContext("(() => { const n = studioGraphState.nodes[0]; const z = editorCanvasState.zoom; return { x: Math.round((n.x + 60) * z + editorCanvasState.panX), y: Math.round((n.y + 20) * z + editorCanvasState.panY) }; })()", win);
  const edCanvasForVoice = doc.querySelector("#studio-canvas");
  edCanvasForVoice.dispatchEvent(makeEvent("mousedown", { button: 0, clientX: selectNodePt.x, clientY: selectNodePt.y, target: edCanvasForVoice }));
  edCanvasForVoice.dispatchEvent(makeEvent("mouseup", { button: 0, clientX: selectNodePt.x, clientY: selectNodePt.y, target: edCanvasForVoice }));
  await waitForActions();

  await clickEl("#btn-inspector-voice-recorder");
  const voiceStrip = doc.querySelector("#inspector-voice-rec-strip");
  if (!voiceStrip || voiceStrip.classList.contains("hidden")) {
    throw new Error("Expected #btn-inspector-voice-recorder to reveal #inspector-voice-rec-strip");
  }
  if (!doc.querySelector("#inspector-voice-rec-strip").classList.contains("is-recording")) {
    throw new Error("Expected clicking #btn-inspector-voice-recorder on scene without audio to auto-start recording");
  }
  // Click Stop ([data-voice-transport=true]) to finish recording & auto-save voice/<scene_id>.flac
  await clickEl('#inspector-voice-rec-strip [data-voice-transport="true"]');
  const savedVoiceAttr = doc.querySelector("#inspector-voice-rec-strip").getAttribute("data-voice-file") || "";
  if (!savedVoiceAttr.endsWith(".flac")) {
    throw new Error("Expected stopping recording to auto-save .flac file and set data-voice-file, got: " + savedVoiceAttr);
  }
  // Drag right 4-corner trim handle (.trim-end) and release to auto-save trimmed .flac
  const trimEndEl = doc.querySelector('#inspector-voice-rec-strip .voice-rec-trim-handle.trim-end');
  const waveWrapEl = doc.querySelector('#inspector-voice-rec-strip [data-voice-waveform-wrap="true"]');
  waveWrapEl.getBoundingClientRect = () => ({ left: 0, top: 0, width: 200, height: 28, right: 200, bottom: 28 });
  trimEndEl.dispatchEvent(makeEvent("pointerdown", { button: 0, clientX: 200, clientY: 14, target: trimEndEl }));
  doc.dispatchEvent(makeEvent("pointermove", { button: 0, clientX: 150, clientY: 14, target: doc }));
  doc.dispatchEvent(makeEvent("pointerup", { button: 0, clientX: 150, clientY: 14, target: doc }));
  await waitForActions();

  // 10b. Topbar toggles (Theme, UI Scale, Hamburger Menu)
  await clickEl("#btn-topbar-menu");
  if (!doc.querySelector("#topbar-actions-menu").classList.contains("open")) {
    throw new Error("Expected #btn-topbar-menu to open #topbar-actions-menu");
  }
  await clickEl("#btn-toggle-ui-scale");
  if (doc.body.getAttribute("data-ui-scale") !== "large" || doc.querySelector("#icon-zoom-zen").classList.contains("hidden")) {
    throw new Error("Expected #btn-toggle-ui-scale to set data-ui-scale=large and show #icon-zoom-zen for next step");
  }
  await clickEl("#btn-toggle-ui-scale");
  if (doc.body.getAttribute("data-ui-scale") !== "zen" || doc.querySelector("#icon-zoom-out").classList.contains("hidden")) {
    throw new Error("Expected second click on #btn-toggle-ui-scale to set data-ui-scale=zen and show #icon-zoom-out");
  }
  await clickEl("#btn-toggle-ui-scale");
  if (doc.body.getAttribute("data-ui-scale") !== "small" || doc.querySelector("#icon-zoom-in").classList.contains("hidden")) {
    throw new Error("Expected third click on #btn-toggle-ui-scale to return to data-ui-scale=small");
  }
  await clickEl('button[action="ToggleTheme"]');
  if (doc.body.getAttribute("data-theme") !== "light") {
    throw new Error("Expected ToggleTheme to set data-theme=light");
  }

  // 11. Switch to Player view via Topbar and click a ChoiceBranch button
  await clickEl('#tab-player');
  if (!doc.querySelector("#view-player")) {
    throw new Error("Expected #view-player after clicking #tab-player");
  }
  const proseText = doc.querySelector("#active-scene-stage .scene-prose").textContent;
  if (!proseText.includes("Treść zapisana przez mostek JS.")) {
    throw new Error("Expected dirty textarea auto-flush on CloseSceneTextModal to persist prose in player view");
  }
  await clickEl('#active-scene-stage button.choice-btn[action="ChooseBranch"]');
  if (!doc.querySelector("#history-stack")) {
    throw new Error("Expected #history-stack in player view after clicking ChooseBranch");
  }

  // 12. Switch to Editor, create a Screenplay project via custom-select mode picker, test Breakdown Panel forms & jump
  await clickEl('button.topbar-tab[data-view="editor"]');
  await clickEl('button[action="OpenNewBookModal"]');
  await clickEl('button[data-form-toggle="narrated"]');
  if (doc.querySelector('#new-book-narrated-field input[name="narrated"]').value !== "true") {
    throw new Error("Expected clicking [data-form-toggle=narrated] to toggle hidden input value to true");
  }
  await clickEl('.new-book-mode-select [data-select-toggle]');
  await clickEl('.new-book-mode-select .custom-select-option[data-select-value="screenplay"]');
  if (doc.querySelector("#new-book-standard-field").classList.contains("hidden")) {
    throw new Error("Expected selecting screenplay mode in custom-select to reveal #new-book-standard-field");
  }
  await clickEl('#new-book-standard-field label.form-radio-option input[value="eu"]');
  if (!doc.querySelector('#new-book-standard-field input[value="eu"]').checked) {
    throw new Error("Expected clicking .form-radio-option to check EU standard radio input");
  }
  doc.querySelector("#new-book-title").value = "Scenariusz Responsywny";
  await clickEl('form[action="CreateNewBook"] button[type="submit"]');

  // Open Scene Text Modal in screenplay mode via edit-scene-btn icon on node card footer and click [data-insert-script-block="dialogue"] pill
  const spEditBtnPt = vm.runInContext("(() => { const n = studioGraphState.nodes[0]; const z = editorCanvasState.zoom; return { x: Math.round((n.x + n.w - 50) * z + editorCanvasState.panX), y: Math.round((n.y + n.h - 20) * z + editorCanvasState.panY) }; })()", win);
  const spDelHitType = vm.runInContext("(() => { const n = studioGraphState.nodes[0]; const h = hitTestStudioCanvas(n.x + n.w - 20, n.y + n.h - 20); return h ? h.type : ''; })()", win);
  if (spDelHitType !== "delete-scene-btn") {
    throw new Error("Expected delete-scene-btn after edit-scene-btn on node card footer, got " + spDelHitType);
  }
  const spCanvas0 = doc.querySelector("#studio-canvas");
  spCanvas0.dispatchEvent(makeEvent("mousedown", { button: 0, clientX: spEditBtnPt.x, clientY: spEditBtnPt.y, target: spCanvas0 }));
  spCanvas0.dispatchEvent(makeEvent("mouseup", { button: 0, clientX: spEditBtnPt.x, clientY: spEditBtnPt.y, target: spCanvas0 }));
  await waitForActions();
  await clickEl('button[data-insert-script-block="dialogue"]');
  const spTaVal = doc.querySelector("#modal-scene-textarea").value;
  if (!spTaVal.includes("POSTAĆ")) {
    throw new Error("Expected [data-insert-script-block=dialogue] to insert POSTAĆ block into #modal-scene-textarea");
  }
  await clickEl('button.scene-text-back-btn[action="CloseSceneTextModal"]');

  await clickEl("#btn-editor-breakdown");
  if (!doc.querySelector("#editor-breakdown-panel") || doc.querySelector("#editor-breakdown-panel").classList.contains("hidden")) {
    throw new Error("Expected #btn-editor-breakdown to open #editor-breakdown-panel");
  }

  // Clear canvas selection first, then click .breakdown-jump-btn[action="SelectStudioScene"]
  const spCanvas = doc.querySelector("#studio-canvas");
  spCanvas.dispatchEvent(makeEvent("mousedown", { button: 0, clientX: 5, clientY: 5, target: spCanvas }));
  spCanvas.dispatchEvent(makeEvent("mouseup", { button: 0, clientX: 5, clientY: 5, target: spCanvas }));
  await waitForActions();
  await clickEl('.breakdown-jump-btn[action="SelectStudioScene"]');
  if (vm.runInContext("selectedNodeIds.size", win) !== 1) {
    throw new Error("Expected clicking .breakdown-jump-btn after canvas deselect to select the target scene on canvas");
  }

  // Auto-save Character Rename and Location Rename inputs in Breakdown Panel on change/blur
  const charRenameForm = doc.querySelector('form.breakdown-rename-form[action="RenameCharacter"]');
  if (!charRenameForm) {
    throw new Error("Expected form.breakdown-rename-form[action=RenameCharacter] in #editor-breakdown-panel");
  }
  const newCharInp = charRenameForm.querySelector('input[name="new_name"]');
  newCharInp.value = "DETEKTYW";
  newCharInp.dispatchEvent(makeEvent("change", { target: newCharInp }));
  await waitForActions();

  const locRenameForm = doc.querySelector('form.breakdown-rename-form[action="RenameLocation"]');
  if (!locRenameForm) {
    throw new Error("Expected form.breakdown-rename-form[action=RenameLocation] in #editor-breakdown-panel");
  }
  const newLocInp = locRenameForm.querySelector('input[name="new_location"]');
  newLocInp.value = "PORT RZECZNY";
  newLocInp.dispatchEvent(makeEvent("blur", { target: newLocInp }));
  await waitForActions();

  // Verify Export Project Modal multi-select, collapsible ZAAWANSOWANE, and norma dropdown in live DOM
  await clickEl('button[action="OpenExportModal"]');
  if (!doc.querySelector("#modal-export-project")) {
    throw new Error("Expected #modal-export-project to open on OpenExportModal");
  }
  if (!doc.querySelector(".export-selection-count").textContent.includes("3 WYBRANE")) {
    throw new Error("Expected initial export modal counter '3 WYBRANE'");
  }
  await clickEl('button.export-format-item[data-format="wasm"]');
  if (!doc.querySelector(".export-selection-count").textContent.includes("2 WYBRANE")) {
    throw new Error("Expected export modal counter '2 WYBRANE' after deselecting wasm");
  }
  await clickEl('button.export-advanced-toggle[action="ToggleExportModalAdvanced"]');
  const advBody = doc.querySelector(".export-advanced-body");
  if (!advBody || advBody.classList.contains("hidden")) {
    throw new Error("Expected .export-advanced-body to expand after clicking ToggleExportModalAdvanced");
  }
  await clickEl('button.export-format-item[data-format="fountain"]');
  if (!doc.querySelector(".export-selection-count").textContent.includes("3 WYBRANE")) {
    throw new Error("Expected export modal counter '3 WYBRANE' after selecting fountain");
  }
  await clickEl(".export-standard-select [data-select-toggle]");
  await clickEl('.export-standard-select .custom-select-option[data-select-value="us"]');
  await waitForActions();
  await clickEl('button[action="CloseExportModal"]');
  if (doc.querySelector("#modal-export-project")) {
    throw new Error("Expected #modal-export-project to close on CloseExportModal");
  }

  const f12Evt = makeEvent("keydown", { key: "F12", code: "F12", target: doc.body });
  doc.dispatchEvent(f12Evt);
  if (process.env.ST_TEST_BUNDLE_NAME === "prod_app_js" && !f12Evt.defaultPrevented) {
    throw new Error("Expected F12 keydown to be blocked (defaultPrevented) in production bundle");
  }
  if (process.env.ST_TEST_BUNDLE_NAME === "dev_app_js" && f12Evt.defaultPrevented) {
    throw new Error("Expected F12 keydown NOT to be blocked in dev bundle");
  }

  console.log("STATUS=OK");
  console.log("ACTIONS=" + actionLog.join(","));
})().catch((err) => {
  console.error("FATAL=" + (err && err.stack ? err.stack : String(err)));
  process.exit(1);
});
`

	for _, bundleCase := range []struct {
		name string
		code string
	}{
		{name: "prod_app_js", code: prodJS},
		{name: "dev_app_js", code: devJS},
	} {
		caseDir := t.TempDir()
		srv := NewServer(caseDir, storyteller.AppFS)
		ts := httptest.NewServer(srv.Handler())

		cmd := exec.Command(nodeBin, "-e", harnessScript)
		cmd.Env = append(os.Environ(),
			"ST_TEST_BASE_URL="+ts.URL,
			"ST_TEST_APP_JS="+bundleCase.code,
			"ST_TEST_BUNDLE_NAME="+bundleCase.name,
		)
		out, err := cmd.CombinedOutput()
		ts.Close()
		if err != nil {
			t.Fatalf("%s live DOM responsiveness test failed: %v\nOutput:\n%s", bundleCase.name, err, string(out))
		}
		if !strings.Contains(string(out), "STATUS=OK") {
			t.Fatalf("expected STATUS=OK from %s harness, got:\n%s", bundleCase.name, string(out))
		}
		if srv.ActiveBook == nil || !srv.ActiveBook.IsScreenplay() {
			t.Fatalf("%s: expected active screenplay book after JS test", bundleCase.name)
		}
		if !strings.Contains(srv.ActiveBook.Scenes[0].Text, "DETEKTYW") || !strings.Contains(srv.ActiveBook.Scenes[0].Slugline, "PORT RZECZNY") {
			t.Fatalf("%s: expected DOM-submitted RenameCharacter and RenameLocation forms to update scene on server, got slugline=%q text=%q",
				bundleCase.name, srv.ActiveBook.Scenes[0].Slugline, srv.ActiveBook.Scenes[0].Text)
		}
	}

	// Verify standalone player.js bundle syntax & clean initialization in Node VM
	playerCheckCmd := exec.Command(nodeBin, "--check")
	playerCheckCmd.Stdin = strings.NewReader(playerJS)
	if out, err := playerCheckCmd.CombinedOutput(); err != nil {
		t.Fatalf("standalone player.js syntax check failed: %v\n%s", err, string(out))
	}
}
