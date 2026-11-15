"use strict";

const TransportWasm = {
  storagePrefix: "mojra_save_",

  isAvailable() {
    return typeof window !== "undefined" && typeof window.__wasmAction === "function";
  },

  send(payloadBytes) {
    if (!this.isAvailable()) {
      return "";
    }
    const rawStr = typeof payloadBytes === "string"
      ? payloadBytes
      : (FDP.textDecoder ? FDP.textDecoder.decode(payloadBytes) : String(payloadBytes));
    return window.__wasmAction(rawStr);
  },

  storageKey(bookId) {
    return this.storagePrefix + String(bookId || "default");
  },

  loadRaw(bookId) {
    try {
      if (typeof window !== "undefined" && window.localStorage) {
        return window.localStorage.getItem(this.storageKey(bookId)) || "";
      }
    } catch (_) {}
    return "";
  },

  saveRaw(bookId, value) {
    try {
      if (typeof window !== "undefined" && window.localStorage) {
        window.localStorage.setItem(this.storageKey(bookId), String(value || ""));
      }
    } catch (_) {}
  }
};
