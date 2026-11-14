"use strict";

const Dispatcher = {
  _ready: typeof document !== "undefined" ? document.addEventListener("DOMContentLoaded", () => Dispatcher.boot()) : null,
  endpoint: "/api/action",
  activePointerEl: null,
  lastAction: "",
  lastParams: null,
  events: [
    "click", "input", "change", "submit", "focus", "blur",
    "keydown", "keyup", "scroll", "wheel",
    "pointerdown", "pointermove", "pointerup",
    "mouseover", "mouseout", "mousemove", "mousedown",
    "timeupdate", "ended"
  ],

  settings: {
    noContentStatus: 204,
    paramPrefixLen: 6,
    dataPrefixLen: 5,
    attrs: {
      action: "action",
      kind: "kind",
      target: "target",
      eventPrefix: "event-",
      paramPrefix: "param-",
      dataPrefix: "data-",
      dataHint: "data-hint"
    },
    selectors: {
      action: "[action]",
      dropZone: "[drop-zone]"
    },
    headers: {
      contentType: "Content-Type"
    },
    tags: {
      form: "form",
      input: "input",
      textarea: "textarea",
      select: "select",
      radio: "radio"
    },
    evNames: {
      click: "click",
      change: "change",
      submit: "submit",
      pointerdown: "pointerdown",
      pointermove: "pointermove",
      pointerup: "pointerup"
    },
    vals: {
      backend: "backend",
      none: "none",
      trueStr: "true",
      falseStr: "false",
      htmlKey: "html",
      bodyKey: "_body",
      strType: "string",
      numType: "number",
      fnType: "function",
      objType: "object"
    },
    http: {
      post: "POST",
      fdpMime: "text/x-fdp;charset=UTF-8"
    }
  },

  controllers: {},
  bindingsByEvent: {},
  boundSelectors: [],

  modules: {
    backend: new Proxy({}, {
      get: (_, action) => (params, _el, _e, hit) => Dispatcher.post(action, params, hit && hit.target)
    })
  },

  register(name, ctrl) {
    this.modules[name] = ctrl;
    this.controllers[name] = ctrl;
    if (ctrl && ctrl.bindings) {
      const map = ctrl.bindings();
      const selectors = Object.keys(map || {});
      for (let i = 0; i < selectors.length; i++) {
        const selector = selectors[i];
        if (this.boundSelectors.indexOf(selector) < 0) {
          this.boundSelectors.push(selector);
        }
        const evMap = map[selector];
        const evNames = Object.keys(evMap || {});
        for (let j = 0; j < evNames.length; j++) {
          const evName = evNames[j];
          if (!this.bindingsByEvent[evName]) this.bindingsByEvent[evName] = [];
          this.bindingsByEvent[evName].push({
            ctrl: ctrl,
            selector: selector,
            handler: evMap[evName]
          });
        }
      }
    }
    return ctrl;
  },

  action(actionName, mapFn) {
    return function (params, el, e) {
      if (e && e.preventDefault) e.preventDefault();
      const extra = typeof mapFn === Dispatcher.settings.vals.fnType ? mapFn.call(this, params, el, e) : null;
      Dispatcher.post(actionName, Object.assign({}, params, extra));
    };
  },

  assignDataValue(dataObj, field, valStr) {
    const v = this.settings.vals;
    if (valStr === v.trueStr) {
      dataObj[field] = true;
    } else if (valStr === v.falseStr) {
      dataObj[field] = false;
    } else if (typeof dataObj[field] === v.numType) {
      dataObj[field] = Number(valStr) || 0;
    } else {
      dataObj[field] = valStr;
    }
  },

  hydrate(decoded) {
    if (!decoded) return;
    const s = this.settings;
    const ctrlNames = Object.keys(this.controllers);
    const keys = Object.keys(decoded);
    for (let i = 0; i < keys.length; i++) {
      const key = keys[i];
      if (key === s.vals.htmlKey || key === s.vals.bodyKey) continue;
      const valStr = decoded[key];
      for (let c = 0; c < ctrlNames.length; c++) {
        const ctrl = this.controllers[ctrlNames[c]];
        if (!ctrl || !ctrl.data || typeof ctrl.data !== s.vals.objType || !(key in ctrl.data)) continue;
        this.assignDataValue(ctrl.data, key, valStr);
      }
    }
  },

  notifyHydrated(decoded, action, params) {
    if (!decoded) return;
    const s = this.settings;
    const ctrlNames = Object.keys(this.controllers);
    for (let c = 0; c < ctrlNames.length; c++) {
      const ctrl = this.controllers[ctrlNames[c]];
      if (ctrl && typeof ctrl.onHydrate === s.vals.fnType) {
        ctrl.onHydrate(decoded, action, params);
      }
    }
  },

  mountAll(root, skipInit) {
    const scope = root || document;
    for (let s = 0; s < this.boundSelectors.length; s++) {
      const list = document.querySelectorAll(this.boundSelectors[s]);
      for (let k = 0; k < list.length; k++) {
        list[k]._idx = k;
      }
    }
    if (skipInit) return;
    const entries = this.bindingsByEvent.init;
    if (!entries) return;
    for (let i = 0; i < entries.length; i++) {
      const entry = entries[i];
      if (scope.matches && scope.matches(entry.selector)) {
        entry.handler.call(entry.ctrl, { idx: scope._idx || 0 }, scope);
      }
      const matched = scope.querySelectorAll ? scope.querySelectorAll(entry.selector) : [];
      for (let j = 0; j < matched.length; j++) {
        entry.handler.call(entry.ctrl, { idx: j }, matched[j]);
      }
    }
  },

  resolve(e) {
    const s = this.settings;
    const a = s.attrs;
    const ev = s.evNames;
    const t = e.target && e.target.closest ? e.target : null;
    const isDropEvt = e.type === ev.pointermove || e.type === ev.pointerup;
    if (isDropEvt && this.activePointerEl) {
      const pEl = this.activePointerEl;
      const pBound = this.bindingsByEvent[e.type];
      if (pBound) {
        for (let i = 0; i < pBound.length; i++) {
          const entry = pBound[i];
          if (pEl.matches && pEl.matches(entry.selector)) {
            return {
              el: pEl,
              ctrl: entry.ctrl,
              handler: entry.handler,
              idx: pEl._idx !== undefined ? pEl._idx : 0,
              eventPrefix: ""
            };
          }
        }
      }
      const pPrefix = a.eventPrefix + e.type + "-";
      const pAttr = pPrefix + a.action;
      if (pEl.getAttribute && pEl.getAttribute(pAttr)) {
        return {
          el: pEl,
          action: pEl.getAttribute(pAttr),
          kind: pEl.getAttribute(pPrefix + a.kind) || pEl.getAttribute(a.kind) || s.vals.backend,
          target: pEl.getAttribute(pPrefix + a.target) || pEl.getAttribute(a.target) || "",
          eventPrefix: pPrefix
        };
      }
    }
    if (!t) return null;
    const evPrefix = a.eventPrefix + e.type + "-";
    const evAttr = evPrefix + a.action;
    const evEl = t.closest("[" + evAttr + "]");
    if (evEl) {
      return {
        el: evEl,
        action: evEl.getAttribute(evAttr),
        kind: evEl.getAttribute(evPrefix + a.kind) || evEl.getAttribute(a.kind) || s.vals.backend,
        target: evEl.getAttribute(evPrefix + a.target) || evEl.getAttribute(a.target) || "",
        eventPrefix: evPrefix
      };
    }
    const isDefaultEvt = e.type === ev.click || e.type === ev.change || e.type === ev.submit;
    const boundList = this.bindingsByEvent[e.type];
    if (boundList && !(isDefaultEvt && t.closest(s.selectors.action))) {
      let bestEntry = null;
      let bestEl = null;
      for (let i = 0; i < boundList.length; i++) {
        const entry = boundList[i];
        const matchedEl = t.closest(entry.selector);
        if (matchedEl && (!bestEl || (bestEl !== matchedEl && bestEl.contains && bestEl.contains(matchedEl)))) {
          bestEntry = entry;
          bestEl = matchedEl;
        }
      }
      if (bestEntry && bestEl) {
        return {
          el: bestEl,
          ctrl: bestEntry.ctrl,
          handler: bestEntry.handler,
          idx: bestEl._idx !== undefined ? bestEl._idx : 0,
          eventPrefix: ""
        };
      }
    }
    if (!isDefaultEvt) return null;
    const el = t.closest(s.selectors.action);
    if (!el) return null;
    const tag = (el.localName || el.tagName).toLowerCase();
    const tg = s.tags;
    const expected = tag === tg.form ? ev.submit : (tag === tg.input || tag === tg.textarea || tag === tg.select ? ev.change : ev.click);
    if (e.type !== expected) return null;
    return {
      el: el,
      action: el.getAttribute(a.action),
      kind: el.getAttribute(a.kind) || s.vals.backend,
      target: el.getAttribute(a.target) || "",
      eventPrefix: ""
    };
  },

  collectParams(hit, e) {
    const s = this.settings;
    const params = {};
    const el = hit.el;
    const tag = (el.localName || el.tagName || "").toLowerCase();
    const formEl = tag === s.tags.form ? el : (el.form || (el.closest ? el.closest(s.tags.form) : null));
    if (formEl && formEl.elements) {
      for (let i = 0; i < formEl.elements.length; i++) {
        const f = formEl.elements[i];
        if (f.name && (f.type !== s.tags.radio || f.checked)) {
          params[f.name] = f.value || "";
        }
      }
    }
    if (el.name) params[el.name] = el.value || "";
    if (el.attributes) {
      for (let i = 0; i < el.attributes.length; i++) {
        const a = el.attributes[i];
        if (a.name.startsWith(s.attrs.paramPrefix)) {
          params[a.name.slice(s.paramPrefixLen).replace(/-/g, "_")] = a.value;
        } else if (a.name.startsWith(s.attrs.dataPrefix) && a.name !== s.attrs.dataHint) {
          const dKey = a.name.slice(s.dataPrefixLen).replace(/-/g, "_");
          if (params[dKey] === undefined) params[dKey] = a.value;
        }
      }
      if (hit.eventPrefix) {
        const evParam = hit.eventPrefix + s.attrs.paramPrefix;
        for (let i = 0; i < el.attributes.length; i++) {
          const a = el.attributes[i];
          if (a.name.startsWith(evParam)) {
            params[a.name.slice(evParam.length).replace(/-/g, "_")] = a.value;
          }
        }
      }
    }
    if (e && e.clientX !== undefined) {
      params.client_x = e.clientX;
      params.client_y = e.clientY;
    }
    if (e && (e.type === s.evNames.pointermove || e.type === s.evNames.pointerup) && document.elementFromPoint) {
      const under = document.elementFromPoint(e.clientX, e.clientY);
      const dropEl = under && under.closest ? under.closest(s.selectors.dropZone) : null;
      if (dropEl) {
        const drop = {};
        for (let i = 0; i < dropEl.attributes.length; i++) {
          const a = dropEl.attributes[i];
          if (a.name.startsWith(s.attrs.paramPrefix)) {
            drop[a.name.slice(s.paramPrefixLen).replace(/-/g, "_")] = a.value;
          } else if (a.name.startsWith(s.attrs.dataPrefix) && a.name !== s.attrs.dataHint) {
            const dKey = a.name.slice(s.dataPrefixLen).replace(/-/g, "_");
            if (drop[dKey] === undefined) drop[dKey] = a.value;
          }
        }
        params.drop_target = drop;
        params.drop_el = dropEl;
      }
    }
    return params;
  },

  swapTarget(targetSel, html) {
    let targetEl = document.querySelector(targetSel);
    if (!targetEl) return;
    targetEl.innerHTML = html;
    if (targetEl.id && targetEl.firstElementChild && targetEl.firstElementChild.id === targetEl.id && targetEl.parentElement) {
      const unwrapped = targetEl.firstElementChild;
      targetEl.parentElement.replaceChild(unwrapped, targetEl);
      targetEl = unwrapped;
    }
    this.mountAll(targetEl);
  },

  async applyResponse(resp, action, params, targetOverride) {
    if (!resp) return null;
    this.lastAction = action || "";
    this.lastParams = params || null;
    const s = this.settings;
    const raw = typeof resp === s.vals.strType ? resp : (resp.html !== undefined ? resp.html : await resp.text());
    const decoded = Fdp.decode(raw);
    this.hydrate(decoded);
    const t = targetOverride || decoded.target;
    if (t && t !== s.vals.none) {
      this.swapTarget(t, decoded.html || "");
    }
    this.notifyHydrated(decoded, action, params);
    return decoded;
  },

  async post(action, params, target) {
    const s = this.settings;
    const resp = await fetch(this.endpoint, {
      method: s.http.post,
      headers: { [s.headers.contentType]: s.http.fdpMime },
      body: Fdp.encode(Object.assign({ action: action }, params))
    });
    return await this.applyResponse(resp, action, params, target);
  },

  handle(e) {
    const ev = this.settings.evNames;
    const hit = this.resolve(e);
    if (!hit) {
      if (e.type === ev.pointermove || e.type === ev.pointerup) this.activePointerEl = null;
      return;
    }
    if (hit.el.disabled) return;
    if (e.type === ev.pointerdown) this.activePointerEl = hit.el;
    else if (e.type === ev.pointerup) this.activePointerEl = null;
    const isPreventEvt = e.type === ev.click || e.type === ev.submit;
    if (isPreventEvt && !hit.handler) e.preventDefault();
    const params = this.collectParams(hit, e);
    if (hit.handler) {
      if (e.type === ev.submit) e.preventDefault();
      if (params.idx === undefined) params.idx = hit.idx;
      hit.handler.call(hit.ctrl, params, hit.el, e, hit);
      return;
    }
    const mod = this.modules[hit.kind];
    const fn = mod && mod[hit.action];
    if (fn) fn.call(mod, params, hit.el, e, hit);
  },

  bind() {
    const handler = (e) => this.handle(e);
    this.events.forEach((evt) => document.addEventListener(evt, handler, true));
  },

  boot() {
    this.bind();
    this.mountAll(document);
  }
};
