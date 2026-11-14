package server

import (
	"io/fs"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"mojra"
	"mojra/internal/actions"
	"mojra/internal/book"
	"mojra/internal/tpl"
)

func TestAllActionSpecsHaveHandlers(t *testing.T) {
	if len(actions.ActionSpecs) == 0 {
		t.Fatal("expected actions.ActionSpecs to be non-empty")
	}
	for name, spec := range actions.ActionSpecs {
		if spec.Handler == nil {
			t.Fatalf("action %q in actions.ActionSpecs has a nil Handler", name)
		}
	}
}

func TestClientServerActionAndOverlayContracts(t *testing.T) {
	appDir := filepath.Clean(filepath.Join("..", "..", "app"))

	// 1. Verify every action referenced in .tpl templates exists in actions.ActionSpecs
	tplFiles, err := filepath.Glob(filepath.Join(appDir, "templates", "*.tpl"))
	if err != nil || len(tplFiles) == 0 {
		t.Fatalf("failed to glob .tpl files: %v", err)
	}
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
		parsedTpl, err := tpl.ParseTemplateString(src, base)
		if err != nil {
			t.Fatalf("parse %s: %v", base, err)
		}
		var checkNodes func(nodes []*tpl.Node)
		checkNodes = func(nodes []*tpl.Node) {
			for _, n := range nodes {
				if n == nil {
					continue
				}
				attrMap := make(map[string]string, len(n.Attrs))
				for _, a := range n.Attrs {
					attrMap[a.Key] = a.Val
				}
				if act := attrMap["action"]; act != "" && !strings.Contains(act, "{") {
					kind := attrMap["kind"]
					if kind == "" || kind == "backend" {
						if _, ok := actions.ActionSpecs[act]; !ok {
							t.Fatalf("template %s references unknown server action %q", base, act)
						}
					}
				}
				for k, v := range attrMap {
					if strings.HasPrefix(k, "event-") && strings.HasSuffix(k, "-action") && v != "" && !strings.Contains(v, "{") {
						evtPrefix := strings.TrimSuffix(k, "action")
						kind := attrMap[evtPrefix+"kind"]
						if kind == "" {
							kind = attrMap["kind"]
						}
						if kind == "" || kind == "backend" {
							if _, ok := actions.ActionSpecs[v]; !ok {
								t.Fatalf("template %s references unknown server action %q in %s", base, v, k)
							}
						}
					}
				}
				checkNodes(n.Children)
			}
		}
		checkNodes(parsedTpl.Nodes)

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

	// 2. Verify every literal action dispatched from app/js/*.js exists in actions.ActionSpecs
	// and every app/js/*.js file defines a single 1:1 object literal without loose function declarations.
	jsFiles := tpl.JSModuleFiles(appDir)
	jsActionRe := regexp.MustCompile(`Dispatcher\.(?:post|action)\(\s*"([A-Za-z0-9_]+)"`)
	looseFuncRe := regexp.MustCompile(`(?m)^\s*(?:async\s+)?function\s+[A-Za-z0-9_]+\s*\(`)
	objConstRe := regexp.MustCompile(`(?m)^const\s+([A-Z][A-Za-z0-9]*)\s*=\s*\{`)
	for _, path := range jsFiles {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		src := string(raw)
		base := filepath.Base(path)
		if looseFuncRe.MatchString(src) {
			t.Fatalf("JS module %s contains forbidden top-level function declaration", base)
		}
		matches := objConstRe.FindAllStringSubmatch(src, -1)
		if len(matches) != 1 {
			t.Fatalf("JS module %s must define exactly 1 top-level object literal, found %d", base, len(matches))
		}
		for _, m := range jsActionRe.FindAllStringSubmatch(src, -1) {
			act := m[1]
			if _, ok := actions.ActionSpecs[act]; !ok {
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

  replaceChild(newChild, oldChild) {
    const idx = this.children.indexOf(oldChild);
    if (idx < 0) return oldChild;
    if (newChild.parentElement) {
      const pIdx = newChild.parentElement.children.indexOf(newChild);
      if (pIdx >= 0) newChild.parentElement.children.splice(pIdx, 1);
    }
    oldChild.parentElement = null;
    newChild.parentElement = this;
    this.children[idx] = newChild;
    this.childNodes = this.children;
    return oldChild;
  }

  get firstElementChild() {
    return this.children.length > 0 ? this.children[0] : null;
  }

  get localName() {
    return (this.tagName || "").toLowerCase();
  }

  get attributes() {
    const out = [];
    this._attrs.forEach((value, name) => out.push({ name: name, value: value }));
    return out;
  }

  get elements() {
    return this.querySelectorAll("input, textarea, select");
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

  remove() {
    const parent = this.parentElement;
    if (!parent) return;
    const idx = parent.children.indexOf(this);
    if (idx >= 0) {
      parent.children.splice(idx, 1);
      parent.childNodes = parent.children;
    }
    this.parentElement = null;
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
    if (ev.type === "click" && !ev.defaultPrevented && (this.tagName === "BUTTON" || this.tagName === "INPUT") && this.type === "submit") {
      const form = this.closest("form");
      if (form) {
        form.dispatchEvent(makeEvent("submit", { target: form }));
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
      userAgent: "Mozilla/5.0 MojraTest/1.0",
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
    requestAnimationFrame: (cb) => setTimeout(() => cb(Date.now()), 16),
    cancelAnimationFrame: (id) => clearTimeout(id),
    setTimeout: setTimeout,
    clearTimeout: clearTimeout,
    setInterval: setInterval,
    clearInterval: clearInterval,
    Event: function(type, opts) { return makeEvent(type, opts || {}); },
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
        if (!ct.includes("text/x-fdp")) {
          throw new Error("Expected FDP Content-Type text/x-fdp, got: " + ct);
        }
        const parsed = vm.runInContext("typeof Fdp !== 'undefined' ? Fdp.decode(" + JSON.stringify(bodyStr) + ") : {}", win);
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
    "const _origPost = Dispatcher.post.bind(Dispatcher); Dispatcher.post = async function(act, params, target) { const p = (async () => { try { return await _origPost(act, params, target); } catch (e) { console.error('SEND_ACTION_ERR=' + act + ':' + (e && e.stack ? e.stack : String(e))); throw e; } })(); __inFlight.add(p); try { return await p; } finally { __inFlight.delete(p); } };",
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

  // 1. Switch to Studio Home (#view-editor) via bottom mode switch in Reader Home
  await clickEl('#btn-switch-to-studio');
  if (!doc.querySelector("#view-editor") || !doc.querySelector("#editor-empty-state")) {
    const appChildren = doc.querySelector("#app") ? doc.querySelector("#app").children.map((c) => c.tagName + "#" + c.id).join(",") : "NONE";
    const allIds = doc.querySelectorAll("*").map((c) => c.tagName + "#" + c.id).filter((s) => !s.endsWith("#")).join(",");
    throw new Error("Expected #view-editor and #editor-empty-state; actionLog=" + actionLog.join(" | ") + " appChildren=" + appChildren + " allIds=" + allIds);
  }

  // 2. Click OpenNewBookModal button in Node Studio
  await clickEl('#btn-new-book');
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
  await clickEl('#btn-new-book');
  const titleInp = doc.querySelector("#new-book-title");
  titleInp.value = "Książka Testowa JS";
  await clickEl('#form-new-book button[type="submit"]');

  const viewport = doc.querySelector("#timeline-scroll-viewport");
  const canvas = doc.querySelector("#studio-canvas");
  if (!viewport || !canvas) {
    throw new Error("Expected #timeline-scroll-viewport and #studio-canvas after CreateNewBook");
  }
  const nodesCount = vm.runInContext("CanvasModel.studioGraphState.nodes.length", win);
  if (nodesCount < 1 || !canvas._gl || canvas._glDrawCalls < 1) {
    throw new Error("Expected WebGL canvas to initialize and draw nodes, got nodes=" + nodesCount + " actionLog=" + actionLog.join(" | ") + " toast=" + (doc.querySelector("#toast-banner") ? doc.querySelector("#toast-banner").textContent : ""));
  }

  // 5. Click empty canvas to clear selection, then click node card on WebGL canvas
  canvas.dispatchEvent(makeEvent("mousedown", { button: 0, clientX: 10, clientY: 10, target: canvas }));
  canvas.dispatchEvent(makeEvent("mouseup", { button: 0, clientX: 10, clientY: 10, target: canvas }));
  await waitForActions();

  const nodeScreenPt = vm.runInContext("(() => { const n = CanvasModel.studioGraphState.nodes[0]; const z = CanvasModel.editorCanvasState.zoom; return { x: Math.round((n.x + 60) * z + CanvasModel.editorCanvasState.panX), y: Math.round((n.y + 42) * z + CanvasModel.editorCanvasState.panY), titleX: Math.round((n.x + 60) * z + CanvasModel.editorCanvasState.panX), titleY: Math.round((n.y + 16) * z + CanvasModel.editorCanvasState.panY), addX: Math.round((n.x + 20) * z + CanvasModel.editorCanvasState.panX), addY: Math.round((n.y + n.h - 20) * z + CanvasModel.editorCanvasState.panY), editX: Math.round((n.x + n.w - 20) * z + CanvasModel.editorCanvasState.panX), editY: Math.round((n.y + n.h - 20) * z + CanvasModel.editorCanvasState.panY), gripX: Math.round((n.x + 12) * z + CanvasModel.editorCanvasState.panX), gripY: Math.round((n.y + 16) * z + CanvasModel.editorCanvasState.panY) }; })()", win);

  canvas.dispatchEvent(makeEvent("mousedown", { button: 0, clientX: nodeScreenPt.x, clientY: nodeScreenPt.y, target: canvas }));
  canvas.dispatchEvent(makeEvent("mouseup", { button: 0, clientX: nodeScreenPt.x, clientY: nodeScreenPt.y, target: canvas }));
  await waitForActions();
  if (!doc.querySelector("#editor-context-form").classList.contains("hidden") || !doc.querySelector("#editor-context-panel").classList.contains("hidden")) {
    throw new Error("Expected #editor-context-panel and #editor-context-form to remain hidden when clicking a node card on canvas");
  }
  if (!vm.runInContext("CanvasModel.selectedNodeIds.has(CanvasModel.studioGraphState.nodes[0].id)", win)) {
    throw new Error("Expected clicking node card on canvas to select the node");
  }

  // 5b. Click node title on WebGL canvas to edit title inline via #canvas-node-title-input
  canvas.dispatchEvent(makeEvent("mousedown", { button: 0, clientX: nodeScreenPt.titleX, clientY: nodeScreenPt.titleY, target: canvas }));
  canvas.dispatchEvent(makeEvent("mouseup", { button: 0, clientX: nodeScreenPt.titleX, clientY: nodeScreenPt.titleY, target: canvas }));
  const nodeTitleInp = doc.querySelector("#canvas-node-title-input");
  if (!nodeTitleInp || nodeTitleInp.classList.contains("hidden")) {
    throw new Error("Expected clicking node title on canvas to open #canvas-node-title-input");
  }
  const expectedTitleFont = vm.runInContext("(13 * (CanvasModel.editorCanvasState.zoom || 1)).toFixed(2) + 'px'", win);
  if (nodeTitleInp.style.fontSize !== expectedTitleFont) {
    throw new Error("Expected #canvas-node-title-input fontSize " + expectedTitleFont + ", got " + nodeTitleInp.style.fontSize);
  }
  const expectedTitleLeft = vm.runInContext("((CanvasModel.studioGraphState.nodes[0].x + 28) * (CanvasModel.editorCanvasState.zoom || 1) + (CanvasModel.editorCanvasState.panX || 0)).toFixed(2) + 'px'", win);
  if (nodeTitleInp.style.left !== expectedTitleLeft) {
    throw new Error("Expected #canvas-node-title-input left " + expectedTitleLeft + ", got " + nodeTitleInp.style.left);
  }
  nodeTitleInp.value = "Scena Startowa Inline";
  nodeTitleInp.dispatchEvent(makeEvent("keydown", { key: "Enter", target: nodeTitleInp }));
  await waitForActions();
  if (vm.runInContext("CanvasModel.studioGraphState.nodes[0].name", win) !== "Scena Startowa Inline") {
    throw new Error("Expected Enter on #canvas-node-title-input to update node name on canvas");
  }

  // 6. Touch-tap add-choice icon button (branch-plus) directly on WebGL canvas node card footer (with synthesized mousedown/mouseup/click)
  canvas.dispatchEvent(makeEvent("mousedown", { button: 0, clientX: nodeScreenPt.addX, clientY: nodeScreenPt.addY, target: canvas }));
  canvas.dispatchEvent(makeEvent("mouseup", { button: 0, clientX: nodeScreenPt.addX, clientY: nodeScreenPt.addY, target: canvas }));
  await waitForActions();
  const portCountAfterAdd = vm.runInContext("CanvasModel.studioGraphState.nodes[0].ports.length", win);
  if (portCountAfterAdd !== 2) {
    throw new Error("Expected tap on canvas add-choice icon to result in 2 choices, got " + portCountAfterAdd + " graph=" + (doc.querySelector("#studio-graph-data") ? doc.querySelector("#studio-graph-data").textContent : "NONE") + " workspace=" + (doc.querySelector("#workspace") ? doc.querySelector("#workspace").textContent.slice(0, 200) : "NONE"));
  }

  // 6b. Click choice row label on WebGL canvas node card to edit choice inline via #canvas-choice-input
  const choiceLabelPt = vm.runInContext("(() => { const n = CanvasModel.studioGraphState.nodes[0]; const z = CanvasModel.editorCanvasState.zoom; return { x: Math.round((n.x + 50) * z + CanvasModel.editorCanvasState.panX), y: Math.round((n.y + 36 + 14) * z + CanvasModel.editorCanvasState.panY) }; })()", win);
  canvas.dispatchEvent(makeEvent("mousedown", { button: 0, clientX: choiceLabelPt.x, clientY: choiceLabelPt.y, target: canvas }));
  canvas.dispatchEvent(makeEvent("mouseup", { button: 0, clientX: choiceLabelPt.x, clientY: choiceLabelPt.y, target: canvas }));
  const choiceInp = doc.querySelector("#canvas-choice-input");
  if (!choiceInp || choiceInp.classList.contains("hidden")) {
    throw new Error("Expected clicking choice label on canvas node to open #canvas-choice-input");
  }
  const expectedChoiceFont = vm.runInContext("(11 * (CanvasModel.editorCanvasState.zoom || 1)).toFixed(2) + 'px'", win);
  if (choiceInp.style.fontSize !== expectedChoiceFont) {
    throw new Error("Expected #canvas-choice-input fontSize " + expectedChoiceFont + ", got " + choiceInp.style.fontSize);
  }
  choiceInp.value = "Wybór Inline 1";
  choiceInp.dispatchEvent(makeEvent("keydown", { key: "Enter", target: choiceInp }));
  await waitForActions();
  if (vm.runInContext("CanvasModel.studioGraphState.nodes[0].ports[0].label", win) !== "Wybór Inline 1") {
    throw new Error("Expected Enter on #canvas-choice-input to update choice label on canvas");
  }

  // 6c. Connect choice ports to two parallel child scenes, then click gray 8x8 branch-radio square on secondary parallel node to promote it to primary
  const rootId = vm.runInContext("CanvasModel.studioGraphState.nodes[0].id", win);
  await vm.runInContext("Dispatcher.post('BranchChoiceScene', { scene: '" + rootId + "', choice_idx: '0' })", win);
  await waitForActions();
  await vm.runInContext("Dispatcher.post('BranchChoiceScene', { scene: '" + rootId + "', choice_idx: '1' })", win);
  await waitForActions();
  const secRadioPt = vm.runInContext("(() => { const sec = CanvasModel.studioGraphState.nodes.find((n) => n.branchRole === 'secondary'); if (!sec) return null; const z = CanvasModel.editorCanvasState.zoom; return { id: sec.id, x: Math.round((sec.branchRadioX + 4) * z + CanvasModel.editorCanvasState.panX), y: Math.round((sec.branchRadioY + 4) * z + CanvasModel.editorCanvasState.panY) }; })()", win);
  if (!secRadioPt) {
    throw new Error("Expected parallel child node with branchRole=secondary on canvas");
  }
  canvas.dispatchEvent(makeEvent("mousedown", { button: 0, clientX: secRadioPt.x, clientY: secRadioPt.y, target: canvas }));
  canvas.dispatchEvent(makeEvent("mouseup", { button: 0, clientX: secRadioPt.x, clientY: secRadioPt.y, target: canvas }));
  await waitForActions();
  const promotedRole = vm.runInContext("(() => { const n = CanvasModel.studioGraphState.nodeById.get('" + secRadioPt.id + "'); return n ? n.branchRole : ''; })()", win);
  if (promotedRole !== "primary") {
    throw new Error("Expected clicking gray branch-radio square to promote node to primary, got " + promotedRole);
  }

  // 6c2. Open #modal-scene-text on rootId, then click add-choice-btn on another canvas node (secRadioPt.id) and verify the target node refreshes on canvas
  await vm.runInContext("Dispatcher.post('OpenSceneTextModal', { scene: '" + rootId + "' })", win);
  await waitForActions();
  const childAddPt = vm.runInContext("(() => { const n = CanvasModel.studioGraphState.nodeById.get('" + secRadioPt.id + "'); const z = CanvasModel.editorCanvasState.zoom; return { x: Math.round((CanvasModel.getNodeFooterBtnX(n, 'add', false) + 12) * z + CanvasModel.editorCanvasState.panX), y: Math.round((n.y + n.h - 19) * z + CanvasModel.editorCanvasState.panY) }; })()", win);
  const activeCanvas = doc.querySelector("#studio-canvas");
  activeCanvas.dispatchEvent(makeEvent("mousedown", { button: 0, clientX: childAddPt.x, clientY: childAddPt.y, target: activeCanvas }));
  activeCanvas.dispatchEvent(makeEvent("mouseup", { button: 0, clientX: childAddPt.x, clientY: childAddPt.y, target: activeCanvas }));
  await waitForActions();
  const childPortsAfterCanvasAdd = vm.runInContext("(() => { const n = CanvasModel.studioGraphState.nodeById.get('" + secRadioPt.id + "'); return n && n.hasChoices && n.ports ? n.ports.length : 0; })()", win);
  if (childPortsAfterCanvasAdd !== 2) {
    throw new Error("Expected clicking add-choice-btn on another canvas node while #modal-scene-text is open to refresh that node to 2 choices, got " + childPortsAfterCanvasAdd);
  }
  await clickEl('#btn-collapse-scene-text');

  // 6d. Drag choice-grip handle on WebGL canvas node card from row 0 to row 1 to reorder choices live + persist via ReorderChoice
  const beforeDragZeroLabel = vm.runInContext("CanvasModel.studioGraphState.nodes[0].ports[0].label", win);
  const choiceGripPt = vm.runInContext("(() => { const n = CanvasModel.studioGraphState.nodes[0]; const hit = CanvasHittest.hitTest(n.x + 10, n.y + 36 + 14); if (!hit || hit.type !== 'choice-grip') return null; const z = CanvasModel.editorCanvasState.zoom; return { x: Math.round((n.x + 10) * z + CanvasModel.editorCanvasState.panX), y0: Math.round((n.y + 36 + 14) * z + CanvasModel.editorCanvasState.panY), y1: Math.round((n.y + 36 + 42) * z + CanvasModel.editorCanvasState.panY) }; })()", win);
  if (!choiceGripPt) {
    throw new Error("Expected CanvasHittest.hitTest to return choice-grip on left side of choice row");
  }
  canvas.dispatchEvent(makeEvent("mousedown", { button: 0, clientX: choiceGripPt.x, clientY: choiceGripPt.y0, target: canvas }));
  canvas.dispatchEvent(makeEvent("mousemove", { button: 0, clientX: choiceGripPt.x, clientY: choiceGripPt.y1, target: canvas }));
  canvas.dispatchEvent(makeEvent("mouseup", { button: 0, clientX: choiceGripPt.x, clientY: choiceGripPt.y1, target: canvas }));
  await waitForActions();
  const reorderedSecondLabel = vm.runInContext("CanvasModel.studioGraphState.nodes[0].ports[1].label", win);
  if (reorderedSecondLabel !== beforeDragZeroLabel) {
    throw new Error("Expected choice-grip drag to reorder '" + beforeDragZeroLabel + "' from idx 0 to idx 1, got " + reorderedSecondLabel);
  }
  const directRightForkPtsLen = vm.runInContext("(() => { const n = CanvasModel.studioGraphState.nodes[0]; const rightWire = CanvasModel.studioGraphState.wires.find((w) => w.fromNode === n && w.toNode && w.x2 >= n.x + n.w + 16); return rightWire && rightWire.pts ? rightWire.pts.length : 0; })()", win);
  if (directRightForkPtsLen !== 3) {
    throw new Error("Expected right-branch choice wire to route as a clean 3-point L-wire without extra kinks, got pts.length=" + directRightForkPtsLen);
  }

  // 7. Drag node grip on WebGL canvas (+48px X, -24px Y) and verify RTS Ghost Drag (no live move during mousemove, apply on mouseup) + SaveEditorState
  const startNodeX = vm.runInContext("CanvasModel.studioGraphState.nodes[0].x", win);
  const updatedGripPt = vm.runInContext("(() => { const n = CanvasModel.studioGraphState.nodes[0]; const z = CanvasModel.editorCanvasState.zoom; return { x: Math.round((n.x + 12) * z + CanvasModel.editorCanvasState.panX), y: Math.round((n.y + 16) * z + CanvasModel.editorCanvasState.panY) }; })()", win);
  canvas.dispatchEvent(makeEvent("mousedown", { button: 0, clientX: updatedGripPt.x, clientY: updatedGripPt.y, target: canvas }));
  canvas.dispatchEvent(makeEvent("mousemove", { button: 0, clientX: updatedGripPt.x + 48, clientY: updatedGripPt.y - 24, target: canvas }));
  const midDragCheck = vm.runInContext("(() => ({ nodeX: CanvasModel.studioGraphState.nodes[0].x, hasGhost: Boolean(CanvasModel.activeCanvasGesture && CanvasModel.activeCanvasGesture.ghostState && CanvasModel.activeCanvasGesture.ghostState.valid && CanvasModel.activeCanvasGesture.ghostState.rects.length === 1 && CanvasModel.activeCanvasGesture.ghostState.rects[0].x === CanvasModel.studioGraphState.nodes[0].x + 48) }))()", win);
  if (midDragCheck.nodeX !== startNodeX || !midDragCheck.hasGhost) {
    throw new Error("Expected RTS Ghost Drag to keep node static during mousemove while projecting ghost rect, got nodeX=" + midDragCheck.nodeX + " hasGhost=" + midDragCheck.hasGhost);
  }
  canvas.dispatchEvent(makeEvent("mouseup", { button: 0, clientX: updatedGripPt.x + 48, clientY: updatedGripPt.y - 24, target: canvas }));
  await waitForActions();
  const movedNodeX = vm.runInContext("CanvasModel.studioGraphState.nodes[0].x", win);
  if (movedNodeX !== startNodeX + 48) {
    throw new Error("Expected node grip drag to snap +48px on 24px grid on mouseup, got " + startNodeX + " -> " + movedNodeX);
  }

  // 7a. Verify node-on-node collision prevention (48px = 2 grid jumps), RTS red collision cells, single-input socket color inheritance, and frustum culling
  const collisionPrevented = vm.runInContext("(() => { const n0 = CanvasModel.studioGraphState.nodes[0]; const n1 = CanvasModel.studioGraphState.nodes[1]; const dx = n1.x - n0.x; const dy = n1.y - n0.y; const ghostCollide = CanvasLayout.evaluateNodeDragGhost([{ node: n0, sceneId: n0.id, startX: n0.x, startY: n0.y }], dx, dy); const ghostTwoGridY = CanvasLayout.evaluateNodeDragGhost([{ node: n1, sceneId: n1.id, startX: n1.x, startY: n1.y }], n0.x - n1.x, (n0.y - n1.h - 48) - n1.y); const ghostOneGridY = CanvasLayout.evaluateNodeDragGhost([{ node: n1, sceneId: n1.id, startX: n1.x, startY: n1.y }], n0.x - n1.x, (n0.y - n1.h - 24) - n1.y); return !ghostCollide.valid && ghostCollide.collisionCells.length > 0 && ghostTwoGridY.valid && ghostTwoGridY.collisionCells.length === 0 && !ghostOneGridY.valid && ghostOneGridY.collisionCells.length === 8; })()", win);
  if (!collisionPrevented) {
    throw new Error("Expected 48px (2 grid jumps) spacing to be valid and 24px (1 grid jump) to light up 8 red collision cells");
  }
  const culledQuadCount = vm.runInContext("(() => { const savedPanX = CanvasModel.editorCanvasState.panX; const savedPanY = CanvasModel.editorCanvasState.panY; CanvasModel.editorCanvasState.panX = 200000; CanvasModel.editorCanvasState.panY = 200000; EditorWebglScene.render(); const c = EditorWebgl.studioGL.staticInstCount; CanvasModel.editorCanvasState.panX = savedPanX; CanvasModel.editorCanvasState.panY = savedPanY; EditorWebglScene.render(); return c; })()", win);
  if (culledQuadCount !== 1) {
    throw new Error("Expected frustum culling to emit only 1 background grid quad when all nodes are off-screen, got " + culledQuadCount);
  }

  // 7b. Drag a wire segment on WebGL canvas to insert a control pin (wire-pin), then double-click pin to remove it
  const wireMidScreenPt = vm.runInContext("(() => { const w = CanvasModel.studioGraphState.wires[0]; const p0 = w.pts[0]; const p1 = w.pts[1]; const wx = (p0.x + p1.x) * 0.5; const wy = (p0.y + p1.y) * 0.5; const z = CanvasModel.editorCanvasState.zoom; return { x: Math.round(wx * z + CanvasModel.editorCanvasState.panX), y: Math.round(wy * z + CanvasModel.editorCanvasState.panY) }; })()", win);
  canvas.dispatchEvent(makeEvent("mousedown", { button: 0, clientX: wireMidScreenPt.x, clientY: wireMidScreenPt.y, target: canvas }));
  canvas.dispatchEvent(makeEvent("mousemove", { button: 0, clientX: wireMidScreenPt.x + 36, clientY: wireMidScreenPt.y + 24, target: canvas }));
  canvas.dispatchEvent(makeEvent("mouseup", { button: 0, clientX: wireMidScreenPt.x + 36, clientY: wireMidScreenPt.y + 24, target: canvas }));
  await waitForActions();
  const pinCountAfterDrag = vm.runInContext("CanvasModel.studioGraphState.wires[0].pins ? CanvasModel.studioGraphState.wires[0].pins.length : 0", win);
  if (pinCountAfterDrag !== 1) {
    throw new Error("Expected dragging wire segment to create 1 control pin, got " + pinCountAfterDrag);
  }
	const pinBarsOk = vm.runInContext("(() => { const mockWire = { pts: [{ x: 200, y: 200 }, { x: 200, y: 300 }, { x: 50, y: 300 }, { x: 50, y: 80 }, { x: 320, y: 80 }, { x: 320, y: 160 }] }; const b0 = EditorWebgl.computeWirePinSegmentBar(mockWire, { x: 200, y: 250 }); const b1 = EditorWebgl.computeWirePinSegmentBar(mockWire, { x: 120, y: 300 }); const b2 = EditorWebgl.computeWirePinSegmentBar(mockWire, { x: 50, y: 180 }); const b3 = EditorWebgl.computeWirePinSegmentBar(mockWire, { x: 180, y: 80 }); return Boolean(b0 && !b0.isHoriz && b1 && b1.isHoriz && b2 && !b2.isHoriz && b3 && b3.isHoriz); })()", win);
	if (!pinBarsOk) {
		throw new Error("Expected computeWirePinSegmentBar to resolve vertical and horizontal bars along wire segments");
	}
  const pinScreenPt = vm.runInContext("(() => { const p = CanvasModel.studioGraphState.wires[0].pins[0]; const z = CanvasModel.editorCanvasState.zoom; return { x: Math.round(p.x * z + CanvasModel.editorCanvasState.panX), y: Math.round(p.y * z + CanvasModel.editorCanvasState.panY) }; })()", win);
  canvas.dispatchEvent(makeEvent("dblclick", { button: 0, clientX: pinScreenPt.x, clientY: pinScreenPt.y, target: canvas }));
  await waitForActions();
  const pinCountAfterDblClick = vm.runInContext("CanvasModel.studioGraphState.wires[0].pins ? CanvasModel.studioGraphState.wires[0].pins.length : 0", win);
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
  const beforeModalPan = vm.runInContext("({ panX: CanvasModel.editorCanvasState.panX, panY: CanvasModel.editorCanvasState.panY })", win);
  const afterMovePt = vm.runInContext("(() => { const n = CanvasModel.studioGraphState.nodes[0]; const z = CanvasModel.editorCanvasState.zoom; return { x: Math.round((n.x + 60) * z + CanvasModel.editorCanvasState.panX), y: Math.round((n.y + 20) * z + CanvasModel.editorCanvasState.panY) }; })()", win);
  canvasForDbl.dispatchEvent(makeEvent("dblclick", { button: 0, clientX: afterMovePt.x, clientY: afterMovePt.y, target: canvasForDbl }));
  await waitForActions();
  const afterModalPan = vm.runInContext("({ panX: CanvasModel.editorCanvasState.panX, panY: CanvasModel.editorCanvasState.panY })", win);
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
  modalTa.dispatchEvent(makeEvent("change", { target: modalTa }));
  await waitForActions();
  await vm.runInContext("Dispatcher.post('OpenSceneTextModal', { scene: document.querySelector('#form-scene-text input[name=\"scene_id\"]').value })", win);
  await waitForActions();
  const synSegs = doc.querySelectorAll("#modal-scene-prose-hl .prose-hl-seg.is-syntax:not(.hidden)");
  const activeSegs = doc.querySelectorAll("#modal-scene-prose-hl .prose-hl-seg:not(.is-syntax):not(.hidden)");
  if (synSegs.length < 2 || !synSegs[0].textContent.includes("if: wypil_gorzalke")) {
    throw new Error("Expected #modal-scene-prose-hl to gray out if:/else:/endif: and inactive branch text, got count=" + synSegs.length);
  }
  if (activeSegs.length < 2 || !activeSegs[1].textContent.includes("Aktywna gałąź")) {
    throw new Error("Expected #modal-scene-prose-hl to keep active branch text bright, got count=" + activeSegs.length);
  }
  const refreshedModalTa = doc.querySelector("#modal-scene-textarea");
  refreshedModalTa.value = "Treść zapisana przez mostek JS.";
  refreshedModalTa.dispatchEvent(makeEvent("change", { target: refreshedModalTa }));
  await waitForActions();

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

  // 9c. Test exit: input '?' replacement dropdown, excluding current scene/chapter/target
  const exitInp = doc.querySelector('#modal-scene-text input.choice-exit-input[name="choice_target_0"]');
  if (!exitInp) {
    throw new Error("Expected #modal-scene-text input.choice-exit-input[name=choice_target_0]");
  }
  const origExitVal = exitInp.value;
  exitInp.value = "?" + origExitVal;
  exitInp.selectionStart = 1;
  exitInp.selectionEnd = 1;
  exitInp.dispatchEvent(makeEvent("input", { target: exitInp }));
  if (synDrop.classList.contains("hidden")) {
    throw new Error("Expected #modal-syn-autocomplete-dropdown to open when typing '?' before existing exit target");
  }
  const visibleTargetBtns = synDrop.querySelectorAll('.syn-dropdown-item[data-syn-kind="target"]:not(.hidden)');
  if (visibleTargetBtns.length === 0) {
    throw new Error("Expected target suggestions in dropdown for exit: '?...'");
  }
  for (let i = 0; i < visibleTargetBtns.length; i++) {
    if ((visibleTargetBtns[i].getAttribute("data-syn-var-name") || "").toLowerCase() === origExitVal.toLowerCase()) {
      throw new Error("Expected current exit target '" + origExitVal + "' to be excluded when typing '?'");
    }
  }
  exitInp.dispatchEvent(makeEvent("keydown", { key: "Enter", target: exitInp }));
  await waitForActions();
  const refreshedExitInp = doc.querySelector('#modal-scene-text input.choice-exit-input[name="choice_target_0"]') || exitInp;
  const refreshedExitRow = refreshedExitInp.closest(".choice-triple-row");
  if (refreshedExitInp.value.includes("?") || (refreshedExitRow && refreshedExitRow.classList.contains("is-invalid"))) {
    throw new Error("Expected selecting target from dropdown to replace full exit: value and clear .is-invalid, got: " + refreshedExitInp.value);
  }
  refreshedExitInp.value = origExitVal;
  refreshedExitInp.dispatchEvent(makeEvent("change", { target: refreshedExitInp }));
  await waitForActions();

  const modalNameInp = doc.querySelector('#modal-scene-text input[name="name"]');
  if (modalNameInp) {
    modalNameInp.value = "Scena Zsynchronizowana";
    modalNameInp.dispatchEvent(makeEvent("change", { target: modalNameInp }));
    await waitForActions();
  }
  const navNextBtn = doc.querySelector('#modal-scene-text .modal-choice-nav-btn');
  if (navNextBtn) {
    await clickEl('#modal-scene-text .modal-choice-nav-btn');
  }

  await clickEl('#btn-collapse-scene-text');
  if (doc.querySelector("#modal-scene-text")) {
    throw new Error("Expected CloseSceneTextModal to close #modal-scene-text");
  }

  // 10. Built-in Voice Recorder & Trimmer (ToggleBookNarrated -> ToggleSceneVoiceRecorder -> Record -> Playhead -> Trim)
  await clickEl("#btn-book-settings");
  const narratedBtn = doc.querySelector('#editor-book-form .btn-toggle-book-narrated');
  if (narratedBtn && !narratedBtn.classList.contains("active")) {
    await clickEl(narratedBtn);
  }
  await clickEl("#btn-book-settings");
  const selectNodePt = vm.runInContext("(() => { const n = CanvasModel.studioGraphState.nodes[0]; const z = CanvasModel.editorCanvasState.zoom; return { x: Math.round((n.x + 60) * z + CanvasModel.editorCanvasState.panX), y: Math.round((n.y + 20) * z + CanvasModel.editorCanvasState.panY) }; })()", win);
  const edCanvasForVoice = doc.querySelector("#studio-canvas");
  edCanvasForVoice.dispatchEvent(makeEvent("mousedown", { button: 0, clientX: selectNodePt.x, clientY: selectNodePt.y, target: edCanvasForVoice }));
  edCanvasForVoice.dispatchEvent(makeEvent("mouseup", { button: 0, clientX: selectNodePt.x, clientY: selectNodePt.y, target: edCanvasForVoice }));
  await waitForActions();

  await clickEl("#btn-inspector-voice-recorder");
  const voiceStrip = doc.querySelector("#inspector-voice-rec-strip");
  if (!voiceStrip || voiceStrip.classList.contains("hidden")) {
    throw new Error("Expected #btn-inspector-voice-recorder to reveal #inspector-voice-rec-strip");
  }
  if (doc.querySelector("#inspector-voice-rec-strip").getAttribute("data-voice-state") !== "recording") {
    throw new Error("Expected clicking #btn-inspector-voice-recorder on scene without audio to auto-start recording");
  }
  const cutBtnWhileRec = doc.querySelector('#inspector-voice-rec-strip [data-voice-trim-destructive="true"]');
  if (!cutBtnWhileRec || cutBtnWhileRec.classList.contains("hidden") || !cutBtnWhileRec.disabled) {
    throw new Error("Expected destructive trim scissors button to always be visible and disabled/grayed out while recording");
  }
  // Click Stop ([data-voice-transport=true]) to finish recording & auto-save voice/<scene_id>.flac
  await clickEl('#inspector-voice-rec-strip [data-voice-transport="true"]');
  const savedVoiceAttr = doc.querySelector("#inspector-voice-rec-strip").getAttribute("data-voice-file") || "";
  if (!savedVoiceAttr.endsWith(".flac")) {
    throw new Error("Expected stopping recording to auto-save .flac file and set data-voice-file, got: " + savedVoiceAttr);
  }
  const cutBtnUntrimmed = doc.querySelector('#inspector-voice-rec-strip [data-voice-trim-destructive="true"]');
  if (!cutBtnUntrimmed || cutBtnUntrimmed.classList.contains("hidden") || !cutBtnUntrimmed.disabled) {
    throw new Error("Expected destructive trim scissors button to remain visible and disabled/grayed out when untrimmed [0, 1]");
  }
  // Drag right 4-corner trim handle (.trim-end) and release to auto-save non-destructive trim by default
  const trimEndEl = doc.querySelector('#inspector-voice-rec-strip [data-voice-trim="end"]');
  const waveWrapEl = doc.querySelector('#inspector-voice-rec-strip [data-voice-waveform-wrap="true"]');
  waveWrapEl.getBoundingClientRect = () => ({ left: 0, top: 0, width: 200, height: 28, right: 200, bottom: 28 });
  trimEndEl.dispatchEvent(makeEvent("pointerdown", { button: 0, clientX: 200, clientY: 14, target: trimEndEl }));
  waveWrapEl.dispatchEvent(makeEvent("pointermove", { button: 0, clientX: 150, clientY: 14, target: waveWrapEl }));
  waveWrapEl.dispatchEvent(makeEvent("pointerup", { button: 0, clientX: 150, clientY: 14, target: waveWrapEl }));
  await waitForActions();
  const nonDestructiveEnd = doc.querySelector("#inspector-voice-rec-strip").getAttribute("data-trim-end") || "";
  if (nonDestructiveEnd !== "0.7500") {
    throw new Error("Expected default non-destructive trim to persist data-trim-end=0.7500, got: " + nonDestructiveEnd);
  }
  const cutBtn = doc.querySelector('#inspector-voice-rec-strip [data-voice-trim-destructive="true"]');
  if (!cutBtn || cutBtn.classList.contains("hidden") || cutBtn.disabled) {
    throw new Error("Expected destructive trim button [data-voice-trim-destructive=true] to be visible and enabled when non-destructive trim is active");
  }
  await clickEl('#inspector-voice-rec-strip [data-voice-trim-destructive="true"]');
  const afterDestructiveEnd = doc.querySelector("#inspector-voice-rec-strip").getAttribute("data-trim-end") || "";
  if (afterDestructiveEnd !== "1.0000") {
    throw new Error("Expected destructive trim to crop file and reset data-trim-end=1.0000, got: " + afterDestructiveEnd);
  }
  const cutBtnAfter = doc.querySelector('#inspector-voice-rec-strip [data-voice-trim-destructive="true"]');
  if (!cutBtnAfter || cutBtnAfter.classList.contains("hidden") || !cutBtnAfter.disabled) {
    throw new Error("Expected destructive trim button to remain visible and become disabled/grayed out after destructive trim resets bounds to [0, 1]");
  }

  // Verify Android native host bridge (window.mojraAndroid) recording lifecycle (pending permission -> live peak -> stop -> SaveSceneVoiceRecording)
  let androidCaptureCalls = 0;
  let androidStopCalls = 0;
  win.mojraAndroid = {
    startAudioCapture: () => {
      androidCaptureCalls++;
      return 2; // permission pending initially
    },
    pollAudioPeak: () => "0.450",
    getAudioSampleRate: () => 24000,
    stopAudioCaptureBase64: () => {
      androidStopCalls++;
      const fakeSamples = new Int16Array(2400);
      for (let i = 0; i < fakeSamples.length; i++) fakeSamples[i] = (i % 64) * 120;
      const bytes = new Uint8Array(fakeSamples.buffer);
      let bin = "";
      for (let i = 0; i < bytes.length; i++) bin += String.fromCharCode(bytes[i]);
      return win.btoa(bin);
    }
  };
  await clickEl('#inspector-voice-rec-strip [data-voice-record-start="true"]');
  if (androidCaptureCalls !== 1 || doc.querySelector("#inspector-voice-rec-strip").getAttribute("data-voice-state") !== "recording") {
    throw new Error("Expected clicking re-record with window.mojraAndroid to invoke startAudioCapture() and enter recording state");
  }
  await clickEl('#inspector-voice-rec-strip [data-voice-transport="true"]');
  if (androidStopCalls !== 1) {
    throw new Error("Expected stopping recording with window.mojraAndroid to invoke stopAudioCaptureBase64()");
  }
  delete win.mojraAndroid;

  // 10b. Settings modal toggles (Theme, UI Scale) & Studio Home <-> Reader Home navigation
  if (doc.querySelector("#btn-toggle-zen")) {
    throw new Error("Expected #btn-toggle-zen to NOT exist in Studio view");
  }
  await clickEl("#btn-open-settings");
  if (!doc.querySelector('.ui-scale-radio-btn[data-scale="large"]')) {
    throw new Error("Expected #btn-open-settings to open settings modal with UI scale and theme controls");
  }
  await clickEl('.ui-scale-radio-btn[data-scale="large"]');
  if (doc.body.getAttribute("data-ui-scale") !== "large") {
    throw new Error("Expected SetUIScale(large) to set data-ui-scale=large");
  }
  await clickEl('.ui-scale-radio-btn[data-scale="small"]');
  if (doc.body.getAttribute("data-ui-scale") !== "small") {
    throw new Error("Expected SetUIScale(small) to return directly to data-ui-scale=small");
  }
  await clickEl('#btn-toggle-theme');
  if (doc.body.getAttribute("data-theme") !== "light") {
    throw new Error("Expected ToggleTheme to set data-theme=light");
  }
  await clickEl('#modal-backdrop-qr .btn-close-qr');

  // 11. Navigate Node Editor -> Studio Home -> Reader Home -> Book Reader (#view-player)
  await clickEl("#tab-studio-home");
  if (!doc.querySelector("#editor-empty-state") || !doc.querySelector("#studio-projects-grid")) {
    throw new Error("Expected clicking #tab-studio-home to return to Studio Home with #studio-projects-grid");
  }
  await clickEl("#btn-switch-to-reader");
  if (!doc.querySelector("#view-library")) {
    throw new Error("Expected #view-library after clicking #btn-switch-to-reader");
  }
  await clickEl('button.book-cover-wrap');
  if (!doc.querySelector("#view-player")) {
    throw new Error("Expected #view-player after clicking book cover in Reader Home");
  }
  if (!doc.querySelector("#btn-toggle-zen") || doc.querySelector("#btn-open-settings")) {
    throw new Error("Expected #btn-toggle-zen and no #btn-open-settings in book reader (#view-player)");
  }
  await clickEl("#btn-toggle-zen");
  if (doc.body.getAttribute("data-zen") !== "true" || !doc.querySelector("#btn-toggle-zen").classList.contains("active")) {
    throw new Error("Expected #btn-toggle-zen to enable data-zen=true in player view");
  }
  await clickEl("#btn-toggle-zen");
  if (doc.body.getAttribute("data-zen") !== "false" || doc.querySelector("#btn-toggle-zen").classList.contains("active")) {
    throw new Error("Expected second click on #btn-toggle-zen to disable data-zen=false");
  }
  const proseText = doc.querySelector("#active-scene-stage .scene-prose").textContent;
  if (!proseText.includes("Treść zapisana przez mostek JS.")) {
    throw new Error("Expected dirty textarea auto-flush on CloseSceneTextModal to persist prose in player view");
  }
  if (!vm.runInContext("AppShellController.data && PlayerController.data && SceneController.data && typeof AppShellController.bindings === 'function' && typeof PlayerController.bindings === 'function' && typeof SceneController.bindings === 'function'", win)) {
    throw new Error("Expected AppShellController, PlayerController, and SceneController with bindings() to be registered");
  }
  await clickEl("#active-scene-stage button.choice-btn");
  if (!doc.querySelector("#history-stack")) {
    throw new Error("Expected #history-stack in player view after clicking .choice-btn");
  }
  const railTicks = doc.querySelectorAll("#story-history-rail .history-rail-tick");
  if (railTicks.length >= 2) {
    await clickEl(railTicks[0]);
    const cardsInFeed = doc.querySelectorAll("#story-scroll-feed .scene-card");
    if (!cardsInFeed[0].classList.contains("scene-focused")) {
      throw new Error("Expected clicking first .history-rail-tick via binding() to focus first .scene-card");
    }
    await clickEl(railTicks[railTicks.length - 1]);
  }
  const storyAudio = doc.querySelector("#story-audio");
  if (!storyAudio || storyAudio.paused || doc.querySelector("#icon-audio-pause").classList.contains("hidden")) {
    throw new Error("Expected narrated scene entry in #view-player to auto-play #story-audio with pause button visible");
  }
  if (vm.runInContext("AudioDeckController.data.autoChoiceDelayMs", win) !== 2000) {
    throw new Error("Expected default AudioDeckController.data.autoChoiceDelayMs=2000, got: " + vm.runInContext("AudioDeckController.data.autoChoiceDelayMs", win));
  }
  await clickEl("#btn-audio-play");
  if (!storyAudio.paused || doc.querySelector("#icon-audio-play").classList.contains("hidden")) {
    throw new Error("Expected clicking #btn-audio-play while playing to pause #story-audio and show play icon");
  }
  await clickEl("#btn-auto-choice-toggle");
  const rewindBtn = doc.querySelector("#history-stack button.contextual-rewind-btn");
  if (rewindBtn) {
    await clickEl(rewindBtn);
    await clickEl('#modal-confirm .btn-confirm-accept');
    const rewoundAudio = doc.querySelector("#story-audio");
    if (rewoundAudio) {
      vm.runInContext("AudioDeckController.data.autoChoiceDelayMs = 20", win);
      rewoundAudio.dispatchEvent(makeEvent("ended", { target: rewoundAudio }));
      await new Promise((r) => setTimeout(r, 40));
      await waitForActions();
      if (!doc.querySelector("#history-stack")) {
        throw new Error("Expected AutoChoice + ended event on #story-audio to advance to next scene after autoChoiceDelay");
      }
    }
  }

  // 12. Return from Player to Reader Home (#tab-library), switch to Studio Home (#btn-switch-to-studio), create a Screenplay project, test Breakdown Panel forms & jump
  await clickEl("#tab-library");
  if (!doc.querySelector("#view-library")) {
    throw new Error("Expected clicking #tab-library in player topbar to return to Reader Home (#view-library)");
  }
  await clickEl("#btn-switch-to-studio");
  await clickEl('#btn-new-book, .canvas-toolbar .btn-open-new-book');
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
  await clickEl('#form-new-book button[type="submit"]');

  // Open Scene Text Modal in screenplay mode via edit-scene-btn icon on node card footer and click [data-insert-script-block="dialogue"] pill
  const spEditBtnPt = vm.runInContext("(() => { const n = CanvasModel.studioGraphState.nodes[0]; const z = CanvasModel.editorCanvasState.zoom; return { x: Math.round((n.x + n.w - 50) * z + CanvasModel.editorCanvasState.panX), y: Math.round((n.y + n.h - 20) * z + CanvasModel.editorCanvasState.panY) }; })()", win);
  const spDelHitType = vm.runInContext("(() => { const n = CanvasModel.studioGraphState.nodes[0]; const h = CanvasHittest.hitTest(n.x + n.w - 20, n.y + n.h - 20); return h ? h.type : ''; })()", win);
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
  await clickEl('#btn-collapse-scene-text');

  await clickEl("#btn-editor-breakdown");
  if (!doc.querySelector("#editor-breakdown-panel") || doc.querySelector("#editor-breakdown-panel").classList.contains("hidden")) {
    throw new Error("Expected #btn-editor-breakdown to open #editor-breakdown-panel");
  }

  // Clear canvas selection first, then click .breakdown-jump-btn
  const spCanvas = doc.querySelector("#studio-canvas");
  spCanvas.dispatchEvent(makeEvent("mousedown", { button: 0, clientX: 5, clientY: 5, target: spCanvas }));
  spCanvas.dispatchEvent(makeEvent("mouseup", { button: 0, clientX: 5, clientY: 5, target: spCanvas }));
  await waitForActions();
  await clickEl('.breakdown-jump-btn');
  if (vm.runInContext("CanvasModel.selectedNodeIds.size", win) !== 1) {
    throw new Error("Expected clicking .breakdown-jump-btn after canvas deselect to select the target scene on canvas");
  }

  // Auto-save Character Rename and Location Rename inputs in Breakdown Panel on change/blur
  const charRenameForm = doc.querySelector('form.breakdown-rename-char-form');
  if (!charRenameForm) {
    throw new Error("Expected form.breakdown-rename-char-form in #editor-breakdown-panel");
  }
  const newCharInp = charRenameForm.querySelector('input[name="new_name"]');
  newCharInp.value = "DETEKTYW";
  newCharInp.dispatchEvent(makeEvent("change", { target: newCharInp }));
  await waitForActions();

  const locRenameForm = doc.querySelector('form.breakdown-rename-loc-form');
  if (!locRenameForm) {
    throw new Error("Expected form.breakdown-rename-loc-form in #editor-breakdown-panel");
  }
  const newLocInp = locRenameForm.querySelector('input[name="new_location"]');
  newLocInp.value = "PORT RZECZNY";
  newLocInp.dispatchEvent(makeEvent("blur", { target: newLocInp }));
  await waitForActions();

  // Verify Studio Full-Text Search (#btn-studio-search, #studio-search-palette, live filter, and #modal-scene-text jump)
  await clickEl("#btn-studio-search");
  const searchPalette = doc.querySelector("#studio-search-palette");
  const searchBtn = doc.querySelector("#btn-studio-search");
  const searchInput = doc.querySelector("#studio-search-input");
  if (!searchPalette || searchPalette.classList.contains("hidden") || !searchBtn.classList.contains("active")) {
    throw new Error("Expected #studio-search-palette to open and #btn-studio-search.active on click");
  }
  searchInput.value = "DETEKTYW";
  searchInput.dispatchEvent(makeEvent("change", { target: searchInput }));
  await waitForActions();
  if (doc.querySelector("#studio-search-count").textContent !== "1") {
    throw new Error("Expected #studio-search-count to equal '1' when filtering for DETEKTYW, got: " + doc.querySelector("#studio-search-count").textContent);
  }
  await clickEl('#studio-search-results .studio-search-row:not(.hidden) [data-search-open-text]');
  if (!doc.querySelector("#modal-scene-text") || !doc.querySelector("#topbar")) {
    throw new Error("Expected clicking search result edit button to open #modal-scene-text while keeping #topbar visible");
  }
  await clickEl('#btn-collapse-scene-text');
  await clickEl("#btn-close-studio-search");
  if (!doc.querySelector("#studio-search-palette").classList.contains("hidden")) {
    throw new Error("Expected #studio-search-palette to close on #btn-close-studio-search click");
  }

  // Verify live two-way synchronization of node title & choice label between canvas floating inputs and #form-scene-text
  const expandTitleSpan = doc.querySelector("#scene-text-expand-title");
  if (!expandTitleSpan || !expandTitleSpan.textContent.trim()) {
    throw new Error("Expected non-empty #scene-text-expand-title inside collapsed #btn-expand-scene-text");
  }
  await clickEl("#btn-expand-scene-text");
  if (!doc.querySelector("#modal-scene-text")) {
    throw new Error("Expected clicking #btn-expand-scene-text to open #modal-scene-text");
  }
  await vm.runInContext("(() => { const scId = document.querySelector('#form-scene-text input[name=\"scene_id\"]').value; const nd = CanvasModel.studioGraphState.nodeById.get(scId); CanvasOverlays.openFloatingNodeTitleInput(nd); })()", win);
  const canvasTitleInp = doc.querySelector("#canvas-node-title-input");
  const modalTitleInp = doc.querySelector('#form-scene-text input[name="name"]');
  if (!canvasTitleInp || canvasTitleInp.classList.contains("hidden") || !modalTitleInp) {
    throw new Error("Expected #canvas-node-title-input and #form-scene-text input[name=name] to be present");
  }
  canvasTitleInp.value = "Costam2";
  canvasTitleInp.dispatchEvent(makeEvent("input", { target: canvasTitleInp }));
  if (modalTitleInp.value !== "Costam2") {
    throw new Error("Expected typing in #canvas-node-title-input to live-sync #form-scene-text input[name=name] to 'Costam2', got: " + modalTitleInp.value);
  }
  if (doc.querySelector("#scene-text-expand-title").textContent !== "Costam2") {
    throw new Error("Expected #scene-text-expand-title to live-sync to 'Costam2', got: " + doc.querySelector("#scene-text-expand-title").textContent);
  }
  canvasTitleInp.dispatchEvent(makeEvent("blur", { target: canvasTitleInp }));
  await waitForActions();
  await clickEl('#btn-collapse-scene-text');

  // Verify Export Project Modal multi-select, collapsible ZAAWANSOWANE, and norma dropdown in live DOM
  await clickEl('#btn-open-export-modal');
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
  await clickEl('button.export-advanced-toggle');
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
  await clickEl('.btn-close-export');
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

  // Verify sequential conditional voice queue (AudioDeckController.data.audioQueue) in AudioDeckController:
  // 01 -> 02_if -> 04 progression, nextAudioPreload buffering, and O(K) seek slider mapping across total duration
  const queueSpec = "/books/test/media/voice/s__01.flac|0.000|10.000|10.000;/books/test/media/voice/s__02_if.flac|1.000|9.000|8.000;/books/test/media/voice/s__04.flac|0.000|12.000|12.000";
  const parsedQueue = vm.runInContext("AudioDeckController.parseSceneAudioQueue(" + JSON.stringify(queueSpec) + ")", win);
  if (!parsedQueue || parsedQueue.length !== 3 || Math.abs(parsedQueue[1].startSec - 1.0) > 0.05 || Math.abs(parsedQueue[1].endSec - 9.0) > 0.05) {
    throw new Error("Expected parseSceneAudioQueue to parse 3 segments with 02_if [1.0..9.0], got: " + String(parsedQueue && parsedQueue.length));
  }
  const mockAudioEl = new DOMElement("audio", doc);
  mockAudioEl.setAttribute("id", "story-audio");
  mockAudioEl.duration = 10;
  const mockSliderEl = new DOMElement("input", doc);
  mockSliderEl.setAttribute("id", "audio-seek-slider");
  mockSliderEl.value = "0";
  const mockCurTimeEl = new DOMElement("span", doc);
  mockCurTimeEl.setAttribute("id", "audio-time-current");
  const mockTotTimeEl = new DOMElement("span", doc);
  mockTotTimeEl.setAttribute("id", "audio-time-total");
  mockTotTimeEl.textContent = "0:30";
  const mockPlayBtnEl = new DOMElement("button", doc);
  mockPlayBtnEl.setAttribute("id", "btn-audio-play");
  doc.body.appendChild(mockAudioEl);
  doc.body.appendChild(mockSliderEl);
  doc.body.appendChild(mockCurTimeEl);
  doc.body.appendChild(mockTotTimeEl);
  doc.body.appendChild(mockPlayBtnEl);

  vm.runInContext("AudioDeckController.data.audioQueue = " + JSON.stringify(queueSpec) + "; AudioDeckController.data.audioTotalDuration = 30; AudioDeckController.data.resumeIndex = 0; AudioDeckController.data.resumeOffsetSec = 0; AudioDeckController.data.audioSpeed = 1; AudioDeckController.initController();", win);
  if (mockTotTimeEl.textContent !== "0:30") {
    throw new Error("Expected total queue duration 0:30 (10+8+12), got: " + mockTotTimeEl.textContent);
  }
  if (vm.runInContext("!AudioDeckController.data.nextAudioPreload || AudioDeckController.data.nextAudioPreload.src !== '/books/test/media/voice/s__02_if.flac'", win)) {
    throw new Error("Expected nextAudioPreload to buffer segment 02_if while on segment 01");
  }

  // Trigger ended on segment 01 -> must automatically switch to segment 02_if at trimStart (1.0s) and preload segment 04
  mockAudioEl.dispatchEvent(makeEvent("ended", { target: mockAudioEl }));
  if (vm.runInContext("AudioDeckController.data.currentSegmentIndex !== 1", win) || mockAudioEl.src !== "/books/test/media/voice/s__02_if.flac") {
    throw new Error("Expected ended on segment 01 to advance to segment 02_if, got idx=" + vm.runInContext("AudioDeckController.data.currentSegmentIndex", win));
  }
  if (Math.abs(mockAudioEl.currentTime - 1.0) > 0.05) {
    throw new Error("Expected segment 02_if to start at trimStart=1.0s, got currentTime=" + mockAudioEl.currentTime);
  }
  if (vm.runInContext("!AudioDeckController.data.nextAudioPreload || AudioDeckController.data.nextAudioPreload.src !== '/books/test/media/voice/s__04.flac'", win)) {
    throw new Error("Expected nextAudioPreload to buffer segment 04 while on segment 02_if");
  }

  // Advance segment 02_if past trimEnd (9.0s) via timeupdate -> must switch to segment 04
  mockAudioEl.currentTime = 9.05;
  mockAudioEl.dispatchEvent(makeEvent("timeupdate", { target: mockAudioEl }));
  if (vm.runInContext("AudioDeckController.data.currentSegmentIndex !== 2", win) || mockAudioEl.src !== "/books/test/media/voice/s__04.flac") {
    throw new Error("Expected reaching trimEnd of 02_if to advance to segment 04, got idx=" + vm.runInContext("AudioDeckController.data.currentSegmentIndex", win));
  }

  // Seek slider to 50% of 30s (= 15.0s global -> 5.0s into segment 02_if -> rawAudioTime = 1.0 + 5.0 = 6.0s)
  mockSliderEl.value = "500";
  mockSliderEl.dispatchEvent(makeEvent("input", { target: mockSliderEl }));
  if (vm.runInContext("AudioDeckController.data.currentSegmentIndex !== 1", win) || Math.abs(mockAudioEl.currentTime - 6.0) > 0.05 || mockCurTimeEl.textContent !== "0:15") {
    throw new Error("Expected seeking slider to 500 (15s) to map to segment 02_if at rawTime=6.0s and display 0:15, got idx=" + vm.runInContext("AudioDeckController.data.currentSegmentIndex", win) + " time=" + mockAudioEl.currentTime + " cur=" + mockCurTimeEl.textContent);
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
		srv := NewServer(caseDir, mojra.AppFS)
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
		if srv.State.Editor.ActiveBook == nil || !srv.State.Editor.ActiveBook.IsScreenplay() {
			t.Fatalf("%s: expected active screenplay book after JS test", bundleCase.name)
		}
		if !strings.Contains(srv.State.Editor.ActiveBook.Scenes[0].Text, "DETEKTYW") || !strings.Contains(srv.State.Editor.ActiveBook.Scenes[0].Slugline, "PORT RZECZNY") {
			t.Fatalf("%s: expected DOM-submitted RenameCharacter and RenameLocation forms to update scene on server, got slugline=%q text=%q",
				bundleCase.name, srv.State.Editor.ActiveBook.Scenes[0].Slugline, srv.State.Editor.ActiveBook.Scenes[0].Text)
		}
	}

	// Verify standalone player.js bundle syntax & clean initialization in Node VM
	playerCheckCmd := exec.Command(nodeBin, "--check")
	playerCheckCmd.Stdin = strings.NewReader(playerJS)
	if out, err := playerCheckCmd.CombinedOutput(); err != nil {
		t.Fatalf("standalone player.js syntax check failed: %v\n%s", err, string(out))
	}
}

type filteredEmbedFS struct {
	base    fs.FS
	allowed map[string]bool
}

func (f *filteredEmbedFS) Open(name string) (fs.File, error) {
	clean := filepath.ToSlash(strings.TrimPrefix(name, "./"))
	if clean == "app/templates" || clean == "app/icons" || f.allowed[clean] {
		return f.base.Open(clean)
	}
	return nil, fs.ErrNotExist
}

func (f *filteredEmbedFS) ReadDir(name string) ([]fs.DirEntry, error) {
	clean := filepath.ToSlash(strings.TrimPrefix(name, "./"))
	entries, err := fs.ReadDir(f.base, clean)
	if err != nil {
		return nil, err
	}
	var out []fs.DirEntry
	for _, e := range entries {
		full := clean + "/" + e.Name()
		if f.allowed[full] {
			out = append(out, e)
		}
	}
	return out, nil
}

func (f *filteredEmbedFS) ReadFile(name string) ([]byte, error) {
	clean := filepath.ToSlash(strings.TrimPrefix(name, "./"))
	if !f.allowed[clean] {
		return nil, fs.ErrNotExist
	}
	return fs.ReadFile(f.base, clean)
}

func TestWasmPlayerStaticAnalysisAndRuntimeExecution(t *testing.T) {
	nodeBin, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not found in PATH")
	}
	rootDir := filepath.Clean(filepath.Join("..", ".."))

	// 1. Static Analysis of embed_wasm.go and cmd/wasmplayer/main.go
	embedWasmRaw, err := os.ReadFile(filepath.Join(rootDir, "embed_wasm.go"))
	if err != nil {
		t.Fatalf("read embed_wasm.go failed: %v", err)
	}
	var embedPatterns []string
	for _, line := range strings.Split(string(embedWasmRaw), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//go:embed ") {
			embedPatterns = append(embedPatterns, strings.Fields(strings.TrimPrefix(trimmed, "//go:embed "))...)
		}
	}
	if len(embedPatterns) == 0 {
		t.Fatalf("expected //go:embed patterns in embed_wasm.go")
	}
	allowedWasmFiles := make(map[string]bool)
	for _, pat := range embedPatterns {
		matches, gErr := fs.Glob(mojra.AppFS, pat)
		if gErr != nil || len(matches) == 0 {
			t.Fatalf("embed_wasm.go pattern %q matched 0 files in AppFS (err=%v)", pat, gErr)
		}
		for _, m := range matches {
			allowedWasmFiles[filepath.ToSlash(m)] = true
		}
	}

	wasmSimFS := &filteredEmbedFS{base: mojra.AppFS, allowed: allowedWasmFiles}
	wasmTemplates, err := tpl.LoadTemplatesFromDir("", wasmSimFS)
	if err != nil {
		t.Fatalf("LoadTemplatesFromDir(\"\", wasmSimFS) failed (missing template or SVG in embed_wasm.go): %v", err)
	}
	for _, reqTpl := range []string{"player_view", "scene_choice_arrow", "confirm_modal"} {
		if _, ok := wasmTemplates[reqTpl]; !ok {
			t.Fatalf("expected template %q to be loaded from WasmAppFS", reqTpl)
		}
	}
	if allowedWasmFiles["app/templates/modals.tpl"] || allowedWasmFiles["app/icons/studio.svg"] {
		t.Fatalf("expected embed_wasm.go to exclude studio templates and icons, got: %v", embedPatterns)
	}

	wasmMainRaw, err := os.ReadFile(filepath.Join(rootDir, "cmd", "wasmplayer", "main.go"))
	if err != nil {
		t.Fatalf("read cmd/wasmplayer/main.go failed: %v", err)
	}
	wasmMainSrc := string(wasmMainRaw)
	for _, forbiddenOSCall := range []string{"os.Open", "os.ReadFile", "os.ReadDir", "os.WriteFile", "os.Stat", "os.Mkdir"} {
		if strings.Contains(wasmMainSrc, forbiddenOSCall) {
			t.Fatalf("cmd/wasmplayer/main.go must not contain host filesystem call %q", forbiddenOSCall)
		}
	}
	for _, reqAction := range []string{
		"SaveAudioTime",
		"ChooseBranch",
		"AutoAdvanceImplicit",
		"AudioEnded",
		"SetAutoChoiceDelay",
		"CycleAudioSpeed",
		"ToggleAutoChoice",
		"PromptRewindStep",
		"PromptResetProgress",
		"ConfirmModalClose",
		"ConfirmModalAccept",
		"RevealHistory",
		"FocusHistoryStep",
		"FocusPresent",
	} {
		if !strings.Contains(wasmMainSrc, `case "`+reqAction+`":`) {
			t.Fatalf("cmd/wasmplayer/main.go is missing handler for player action %q", reqAction)
		}
	}

	// 2. Verify WASM dependency graph excludes server/archive/image/exec/base64 bloat, then build exported Web/WASM bundle
	depsCmd := exec.Command("go", "list", "-deps", "./cmd/wasmplayer")
	depsCmd.Dir = rootDir
	depsCmd.Env = append(os.Environ(), "GOOS=js", "GOARCH=wasm")
	if depsOut, dErr := depsCmd.CombinedOutput(); dErr != nil {
		t.Fatalf("go list -deps ./cmd/wasmplayer failed: %v\n%s", dErr, string(depsOut))
	} else {
		depsSet := make(map[string]bool)
		for _, ln := range strings.Split(string(depsOut), "\n") {
			depsSet[strings.TrimSpace(ln)] = true
		}
		for _, forbiddenPkg := range []string{
			"net/http",
			"crypto/tls",
			"crypto/x509",
			"archive/zip",
			"compress/gzip",
			"image/png",
			"encoding/base64",
			"encoding/xml",
			"os/exec",
			"mojra/internal/qr",
		} {
			if depsSet[forbiddenPkg] {
				t.Fatalf("cmd/wasmplayer dependency graph must not include %q", forbiddenPkg)
			}
		}
	}

	sampleBook, err := book.LoadBookFromDir(filepath.Join(rootDir, "books", "nie-boj-sie-policji-rzecznej"))
	if err != nil || sampleBook == nil {
		t.Fatalf("LoadBookFromDir failed: %v", err)
	}
	exportRoot := t.TempDir()
	webOutDir, err := book.ExportWebWASMApp(sampleBook, exportRoot, rootDir, mojra.AppFS)
	if err != nil {
		t.Fatalf("ExportWebWASMApp failed: %v", err)
	}

	wasmBytes, err := os.ReadFile(filepath.Join(webOutDir, "mojra.wasm"))
	if err != nil {
		t.Fatalf("read exported mojra.wasm failed: %v", err)
	}
	if len(wasmBytes) < 64 || string(wasmBytes[:4]) != "\x00asm" {
		t.Fatalf("invalid Wasm magic header in exported mojra.wasm")
	}
	// Static binary section walk of mojra.wasm: verify every Wasm section header is well-formed up to EOF
	pos := 8
	foundSTBCCustomSection := false
	for pos < len(wasmBytes) {
		secID := wasmBytes[pos]
		pos++
		var secLen uint64
		var shift uint
		for {
			if pos >= len(wasmBytes) {
				t.Fatalf("truncated ULEB128 section length in mojra.wasm at byte %d", pos)
			}
			b := wasmBytes[pos]
			pos++
			secLen |= uint64(b&0x7f) << shift
			if (b & 0x80) == 0 {
				break
			}
			shift += 7
		}
		if pos+int(secLen) > len(wasmBytes) {
			t.Fatalf("corrupted Wasm section (id=%d) overflows file size: pos=%d len=%d total=%d", secID, pos, secLen, len(wasmBytes))
		}
		if secID == 0 && pos+int(secLen) == len(wasmBytes) && strings.Contains(string(wasmBytes[pos:pos+32]), "mojra_stbc") {
			foundSTBCCustomSection = true
		}
		pos += int(secLen)
	}
	if pos != len(wasmBytes) || !foundSTBCCustomSection {
		t.Fatalf("expected mojra.wasm sections to walk cleanly to EOF (%d==%d) and end with mojra_stbc custom section (%v)",
			pos, len(wasmBytes), foundSTBCCustomSection)
	}

	// Verify exported app.js has I18N_TABLE and zero __wasmBase64 bloat
	exportedAppJS, err := os.ReadFile(filepath.Join(webOutDir, "app.js"))
	if err != nil {
		t.Fatalf("read exported app.js failed: %v", err)
	}
	if !strings.Contains(string(exportedAppJS), "const I18N_TABLE =") || strings.Contains(string(exportedAppJS), "__wasmBase64") {
		t.Fatalf("expected exported app.js to contain I18N_TABLE and no __wasmBase64 bloat")
	}

	// 3. End-to-End WebAssembly Execution in Node VM (testing both file:// fallback and interactive __wasmBoot / __wasmAction)
	wasmE2EScript := `
const fs = require("fs");
const path = require("path");
const vm = require("vm");

const webDir = process.env.ST_WASM_WEB_DIR;
const wwwDir = process.env.ST_WASM_WWW_DIR;

function createDOMElement(tag, ownerDoc) {
  const el = {
    tagName: String(tag || "div").toUpperCase(),
    id: "",
    className: "",
    innerHTML: "",
    textContent: "",
    value: "",
    disabled: false,
    paused: true,
    ended: false,
    currentTime: 0,
    duration: 12,
    scrollTop: 0,
    scrollHeight: 600,
    clientHeight: 600,
    offsetWidth: 200,
    offsetHeight: 40,
    style: {},
    dataset: {},
    _attrs: new Map(),
    get attributes() {
      const out = [];
      this._attrs.forEach((value, name) => out.push({ name, value }));
      return out;
    },
    localName: String(tag || "div").toLowerCase(),
    children: [],
    parentNode: null,
    isConnected: true,
    ownerDocument: ownerDoc,
    classList: null,
    setAttribute(k, v) {
      const sv = String(v);
      this._attrs.set(k, sv);
      if (k === "id") this.id = sv;
      if (k === "class") this.className = sv;
      if (k.startsWith("data-")) {
        const camel = k.slice(5).replace(/-([a-z])/g, (_, c) => c.toUpperCase());
        this.dataset[camel] = sv;
      }
    },
    getAttribute(k) {
      if (k === "id" && this.id) return this.id;
      if (k === "class" && this.className) return this.className;
      return this._attrs.has(k) ? this._attrs.get(k) : null;
    },
    removeAttribute(k) {
      this._attrs.delete(k);
    },
    hasAttribute(k) {
      return this._attrs.has(k);
    },
    appendChild(c) {
      c.parentNode = this;
      this.children.push(c);
      return c;
    },
    contains(other) {
      let cur = other;
      while (cur) {
        if (cur === this) return true;
        cur = cur.parentNode;
      }
      return false;
    },
    closest(sel) {
      let cur = this;
      while (cur) {
        if (matchesSimple(cur, sel)) return cur;
        cur = cur.parentNode;
      }
      return null;
    },
    querySelector(sel) {
      return queryAllSimple(this, sel)[0] || null;
    },
    querySelectorAll(sel) {
      return queryAllSimple(this, sel);
    },
    getBoundingClientRect() {
      return { left: 0, top: 0, right: 200, bottom: 40, width: 200, height: 40 };
    },
    getContext() {
      return null;
    },
    addEventListener() {},
    removeEventListener() {},
    play() { this.paused = false; return Promise.resolve(); },
    pause() { this.paused = true; },
    load() {}
  };
  el.classList = {
    add(...cls) {
      const set = new Set((el.className || "").split(/\s+/).filter(Boolean));
      cls.forEach(c => set.add(c));
      el.className = Array.from(set).join(" ");
    },
    remove(...cls) {
      const set = new Set((el.className || "").split(/\s+/).filter(Boolean));
      cls.forEach(c => set.delete(c));
      el.className = Array.from(set).join(" ");
    },
    toggle(c, force) {
      const has = this.contains(c);
      const next = force !== undefined ? Boolean(force) : !has;
      if (next) this.add(c); else this.remove(c);
      return next;
    },
    contains(c) {
      return (el.className || "").split(/\s+/).includes(c);
    }
  };
  let rawHTML = "";
  Object.defineProperty(el, "innerHTML", {
    get() { return rawHTML; },
    set(v) {
      rawHTML = String(v || "");
      el.children = parseHTMLFragment(rawHTML, el, ownerDoc);
    }
  });
  return el;
}

function parseHTMLFragment(html, parent, ownerDoc) {
  const roots = [];
  const stack = [ { children: roots } ];
  const tagRe = /<\/?([a-zA-Z0-9-]+)([^>]*?)(\/?)>/g;
  let lastIdx = 0;
  let m;
  while ((m = tagRe.exec(html)) !== null) {
    const textChunk = html.slice(lastIdx, m.index).trim();
    if (textChunk && stack.length > 1) {
      stack[stack.length - 1].textContent += textChunk;
    }
    lastIdx = tagRe.lastIndex;
    const full = m[0];
    const tag = m[1].toLowerCase();
    const attrStr = m[2] || "";
    const selfClose = m[3] === "/" || tag === "img" || tag === "input" || tag === "meta" || tag === "link" || tag === "hr" || tag === "br";
    if (full.startsWith("</")) {
      if (stack.length > 1) stack.pop();
      continue;
    }
    const node = createDOMElement(tag, ownerDoc);
    const attrRe = /([a-zA-Z0-9_:-]+)(?:="([^"]*)")?/g;
    let am;
    while ((am = attrRe.exec(attrStr)) !== null) {
      node.setAttribute(am[1], am[2] !== undefined ? am[2] : "");
    }
    const top = stack[stack.length - 1];
    node.parentNode = top === stack[0] ? parent : top;
    top.children.push(node);
    if (!selfClose) {
      stack.push(node);
    }
  }
  return roots;
}

function matchesSimple(el, sel) {
  if (!el || !el.tagName) return false;
  sel = sel.trim();
  if (sel.includes(",")) {
    return sel.split(",").some(s => matchesSimple(el, s));
  }
  if (sel.includes(" ")) {
    const parts = sel.split(/\s+/);
    if (!matchesSimple(el, parts[parts.length - 1])) return false;
    let cur = el.parentNode;
    for (let i = parts.length - 2; i >= 0; i--) {
      while (cur && !matchesSimple(cur, parts[i])) cur = cur.parentNode;
      if (!cur) return false;
      cur = cur.parentNode;
    }
    return true;
  }
  let rem = sel;
  const attrRe = /\[([a-zA-Z0-9_-]+)(?:="([^"]*)")?\]/g;
  let am;
  while ((am = attrRe.exec(sel)) !== null) {
    const k = am[1];
    const v = am[2];
    if (v !== undefined) {
      if (el.getAttribute(k) !== v) return false;
    } else if (!el.hasAttribute(k)) {
      return false;
    }
  }
  rem = rem.replace(attrRe, "");
  if (rem.includes(":not(.hidden)")) {
    if (el.classList.contains("hidden")) return false;
    rem = rem.replace(":not(.hidden)", "");
  }
  const idMatch = rem.match(/#([a-zA-Z0-9_-]+)/);
  if (idMatch && el.id !== idMatch[1]) return false;
  rem = rem.replace(/#[a-zA-Z0-9_-]+/g, "");
  const clsMatches = rem.match(/\.[a-zA-Z0-9_-]+/g) || [];
  for (const c of clsMatches) {
    if (!el.classList.contains(c.slice(1))) return false;
  }
  rem = rem.replace(/\.[a-zA-Z0-9_-]+/g, "").trim();
  if (rem && el.tagName.toLowerCase() !== rem.toLowerCase()) return false;
  return true;
}

function queryAllSimple(root, sel) {
  const out = [];
  function walk(n) {
    if (!n || !n.children) return;
    for (const c of n.children) {
      if (matchesSimple(c, sel)) out.push(c);
      walk(c);
    }
  }
  walk(root);
  return out;
}

async function runBundleE2E(bundleDir, protocolMode) {
  const indexHTML = fs.readFileSync(path.join(bundleDir, "index.html"), "utf8");
  const appJS = fs.readFileSync(path.join(bundleDir, "app.js"), "utf8");
  const wasmRaw = fs.readFileSync(path.join(bundleDir, "mojra.wasm"));

  const listeners = {};
  const doc = {
    body: null,
    documentElement: null,
    fullscreenElement: null,
    addEventListener(type, fn) {
      (listeners[type] = listeners[type] || []).push(fn);
    },
    getElementById(id) { return queryAllSimple(this.body, "#" + id)[0] || null; },
    querySelector(sel) { return queryAllSimple(this.body, sel)[0] || null; },
    querySelectorAll(sel) { return queryAllSimple(this.body, sel); }
  };
  doc.body = createDOMElement("body", doc);
  doc.documentElement = doc.body;
  const bodyMatch = indexHTML.match(/<body[^>]*>([\s\S]*?)<\/body>/i);
  doc.body.innerHTML = bodyMatch ? bodyMatch[1] : indexHTML;

  const storage = new Map();
  const sandbox = {
    console,
    innerWidth: 1280,
    innerHeight: 800,
    addEventListener() {},
    removeEventListener() {},
    Uint8Array,
    Int8Array,
    Uint16Array,
    Int16Array,
    Uint32Array,
    Int32Array,
    Float32Array,
    Float64Array,
    BigInt64Array,
    BigUint64Array,
    ArrayBuffer,
    DataView,
    WebAssembly,
    TextEncoder,
    TextDecoder,
    Map,
    Set,
    Promise,
    Error,
    TypeError,
    Math,
    Date,
    Number,
    String,
    Boolean,
    Object,
    Array,
    RegExp,
    JSON,
    parseInt,
    parseFloat,
    isNaN,
    isFinite,
    encodeURIComponent,
    decodeURIComponent,
    atob,
    btoa,
    setTimeout,
    clearTimeout,
    requestAnimationFrame: (cb) => setTimeout(cb, 0),
    performance: { now: () => Date.now() },
    crypto: require("crypto").webcrypto,
    URL: {
      createObjectURL: () => "blob:mojra-test-media",
      revokeObjectURL: () => {}
    },
    Blob: function(parts, opts) { this.parts = parts; this.opts = opts; },
    Audio: function() {
      this.muted = false;
      this.paused = true;
      this.play = () => Promise.resolve();
      this.pause = () => {};
    },
    localStorage: {
      getItem: (k) => storage.has(k) ? storage.get(k) : null,
      setItem: (k, v) => storage.set(k, String(v))
    },
    location: {
      protocol: protocolMode,
      href: protocolMode + "//localhost/index.html"
    },
    fetch: async (url) => {
      if (protocolMode === "file:") {
        throw new TypeError("Failed to fetch: file:// URL scheme is not supported");
      }
      if (String(url).endsWith("mojra.wasm")) {
        return {
          ok: true,
          status: 200,
          arrayBuffer: async () => wasmRaw.buffer.slice(wasmRaw.byteOffset, wasmRaw.byteOffset + wasmRaw.byteLength)
        };
      }
      return { ok: false, status: 404 };
    },
    document: doc
  };
  // Deliberately do NOT expose Node's fs on globalThis so wasm_exec.js uses its browser stub!
  sandbox.window = sandbox;
  sandbox.globalThis = sandbox;
  sandbox.self = sandbox;

  vm.createContext(sandbox);
  vm.runInContext(appJS, sandbox);
  (listeners["DOMContentLoaded"] || []).forEach(fn => fn());

  const launchBtn = doc.querySelector("#btn-launch-wasm");
  if (launchBtn) {
    launchBtn.click && launchBtn.click();
  }

  const playBtn = doc.querySelector("#btn-wasm-play");
  if (!playBtn) {
    throw new Error("Missing #btn-wasm-play in " + bundleDir);
  }
  for (const fn of (listeners["click"] || [])) {
    fn({ type: "click", target: playBtn, preventDefault() {}, stopPropagation() {} });
  }
  for (let i = 0; i < 60 && !doc.querySelector("#view-player"); i++) {
    await new Promise(r => setTimeout(r, 20));
  }

  if (typeof sandbox.__wasmBoot !== "function" || typeof sandbox.__wasmAction !== "function") {
    throw new Error("Expected __wasmBoot and __wasmAction to be registered by mojra.wasm in " + bundleDir + " (" + protocolMode + ")");
  }

  const playerView = doc.querySelector("#view-player");
  if (!playerView) {
    throw new Error("Expected #view-player to be mounted after bootWasmPlayer in " + bundleDir + " (" + protocolMode + ")");
  }
  const choiceBtns = doc.querySelectorAll(".choice-btn");
  if (choiceBtns.length < 2) {
    throw new Error("Expected at least 2 branch choice buttons in initial scene, got " + choiceBtns.length);
  }

  // Click first choice via declarative click handler -> window.__wasmAction("ChooseBranch")
  const clickEvt = {
    type: "click",
    target: choiceBtns[0],
    defaultPrevented: false,
    preventDefault() { this.defaultPrevented = true; },
    stopPropagation() {}
  };
  for (const fn of (listeners["click"] || [])) {
    fn(clickEvt);
  }
  await new Promise(r => setTimeout(r, 30));

  const histStack = doc.querySelector("#history-stack");
  if (!histStack) {
    throw new Error("Expected #history-stack to render after ChooseBranch transition in WASM player");
  }
  if (storage.size === 0) {
    throw new Error("Expected WASM player progress to be saved in localStorage");
  }

  // Trigger PromptRewindStep -> ConfirmModalAccept via WASM action bridge
  const rewindBtn = doc.querySelector(".contextual-rewind-btn");
  if (!rewindBtn) {
    throw new Error("Expected .contextual-rewind-btn button in history stack");
  }
  for (const fn of (listeners["click"] || [])) {
    fn({ type: "click", target: rewindBtn, preventDefault() {}, stopPropagation() {} });
  }
  await new Promise(r => setTimeout(r, 20));
  const confirmModal = doc.querySelector("#modal-confirm");
  if (!confirmModal) {
    throw new Error("Expected #modal-confirm to open after PromptRewindStep in WASM player");
  }
  const acceptBtn = doc.querySelector('#modal-confirm .btn-confirm-accept');
  if (!acceptBtn) {
    throw new Error("Expected ConfirmModalAccept button in #modal-confirm");
  }
  for (const fn of (listeners["click"] || [])) {
    fn({ type: "click", target: acceptBtn, preventDefault() {}, stopPropagation() {} });
  }
  await new Promise(r => setTimeout(r, 20));
  if (doc.querySelector("#history-stack")) {
    throw new Error("Expected #history-stack to be cleared after rewinding to step 0");
  }
}

(async () => {
  await runBundleE2E(webDir, "http:");
  if (wwwDir && fs.existsSync(path.join(wwwDir, "mojra.wasm"))) {
    await runBundleE2E(wwwDir, "http:");
  }
  console.log("WASM_E2E=OK");
})().catch(err => {
  console.error("WASM_E2E_ERR=" + (err && err.stack ? err.stack : String(err)));
  process.exit(1);
});
`
	wwwDir := filepath.Join(rootDir, "dist", "www")
	e2eCmd := exec.Command(nodeBin, "-e", wasmE2EScript)
	e2eCmd.Env = append(os.Environ(),
		"ST_WASM_WEB_DIR="+webOutDir,
		"ST_WASM_WWW_DIR="+wwwDir,
	)
	out, err := e2eCmd.CombinedOutput()
	if err != nil || !strings.Contains(string(out), "WASM_E2E=OK") {
		t.Fatalf("WASM E2E runtime test failed: %v\nOutput:\n%s", err, string(out))
	}
}
