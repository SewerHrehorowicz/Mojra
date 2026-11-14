"use strict";

const Fdp = {
  settings: {
    actionKey: "action",
    bodyKey: "_body",
    htmlKey: "html",
    textKey: "text",
    sepLF: "\n\n",
    sepCRLF: "\r\n\r\n",
    nl: "\n",
    cr: "\r",
    eq: "=",
    bs: "\\",
    escNL: "\\n",
    escCR: "\\r",
    escBS: "\\\\",
    escQuote: '\\"'
  },

  escapeVal(val) {
    const s = String(val === undefined || val === null ? "" : val);
    const st = this.settings;
    return s.replace(/\\/g, st.escBS).replace(/\n/g, st.escNL).replace(/\r/g, st.escCR);
  },

  unescapeVal(s) {
    const st = this.settings;
    if (!s || s.indexOf(st.bs) < 0) return s || "";
    return s
      .replace(/\\n/g, st.nl)
      .replace(/\\r/g, st.cr)
      .replace(/\\"/g, '"')
      .replace(/\\\\/g, st.bs);
  },

  encode(paramsObj) {
    if (!paramsObj) return "";
    const st = this.settings;
    const keys = Object.keys(paramsObj).sort();
    let bodyKey = "";
    let bodyVal = "";
    for (let i = 0; i < keys.length; i++) {
      const k = keys[i];
      const v = String(paramsObj[k] === undefined || paramsObj[k] === null ? "" : paramsObj[k]);
      if (v.indexOf(st.nl) >= 0 || v.indexOf(st.cr) >= 0) {
        if (!bodyKey || k === st.textKey || k === st.htmlKey) {
          bodyKey = k;
          bodyVal = v;
        }
      }
    }
    const lines = [];
    if (paramsObj[st.actionKey] !== undefined) {
      lines.push(st.actionKey + st.eq + this.escapeVal(paramsObj[st.actionKey]));
    }
    for (let i = 0; i < keys.length; i++) {
      const k = keys[i];
      if (k === st.actionKey) continue;
      if (k === bodyKey) {
        lines.push(st.bodyKey + st.eq + k);
        continue;
      }
      lines.push(k + st.eq + this.escapeVal(paramsObj[k]));
    }
    if (bodyKey) {
      return lines.join(st.nl) + st.sepLF + bodyVal;
    }
    return lines.join(st.nl);
  },

  decode(rawStr) {
    const out = {};
    const s = String(rawStr || "");
    if (!s) return out;
    const st = this.settings;
    let head = s;
    let body = "";
    let hasBody = false;
    const lfIdx = s.indexOf(st.sepLF);
    const crlfIdx = s.indexOf(st.sepCRLF);
    if (lfIdx >= 0 && (crlfIdx < 0 || lfIdx < crlfIdx)) {
      head = s.slice(0, lfIdx);
      body = s.slice(lfIdx + st.sepLF.length);
      hasBody = true;
    } else if (crlfIdx >= 0) {
      head = s.slice(0, crlfIdx);
      body = s.slice(crlfIdx + st.sepCRLF.length);
      hasBody = true;
    }
    let bodyKey = "";
    const lines = head.split(/\r?\n/);
    for (let i = 0; i < lines.length; i++) {
      const ln = lines[i];
      if (!ln) continue;
      const eq = ln.indexOf(st.eq);
      if (eq <= 0) continue;
      const k = ln.slice(0, eq).trim();
      const v = ln.slice(eq + 1);
      if (k === st.bodyKey) {
        bodyKey = v.trim();
        continue;
      }
      out[k] = this.unescapeVal(v);
    }
    if (bodyKey && hasBody) {
      out[bodyKey] = body;
    }
    return out;
  }
};
