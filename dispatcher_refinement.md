# Plan udoskonalenia `Dispatcher` i cyklu życia kontrolerów zgodnie z `@ARCH.md` (`dispatcher_refinement.md`)

## 1. Co tam jest i jakie są problemy (What is there & Problems)

1. **Martwy kod routingu atrybutowego w `app/js/dispatcher.js` (naruszenie `ARCH.md §4.1` i `§5 pkt 2`)**:
   - Szablony `app/templates/*.tpl` są już w 100% prezentacyjne i nie używają `action:`, `kind:`, `target:`, `event-*` ani `param-*`, lecz `app/js/dispatcher.js` nadal utrzymuje ~95 linii martwych ścieżek z ery `WFP`:
     - `Dispatcher.modules.backend` (`Proxy`, `L77–81`) oraz wywołanie fallbacku `this.modules[hit.kind][hit.action]` (`L387–389`).
     - Gałęzie `pPrefix + a.action` (`L199–210`), `evEl = t.closest("[" + evAttr + "]")` (`L212–223`) oraz fallback `t.closest(s.selectors.action)` (`L247–260`) w `Dispatcher.resolve(e)`.
     - Pętle zbierające `param-*` i `event-*-param-*` w `Dispatcher.collectParams(hit, e)` (`L281–282`, `L288–296`, `L309–310`).
     - `Dispatcher.action(actionName, mapFn)` (`L110–116`) nadal przyjmuje wycofany callback `mapFn`, zakazany przez `ARCH.md §5 pkt 2 (JS_BINDINGS_LOGIC)`.
2. **Utrata focusu i pozycji kursora (`selectionStart` / `selectionEnd`) podczas `swapTarget` (`app/js/dispatcher.js:323–333`)**:
   - Gdy akcja `change` / `submit` / skrót klawiszowy podmienia `targetEl.innerHTML = html` (np. `#editor-context-view` lub `#modals-root`), aktywne pole `<input>` / `<textarea>` wewnątrz wymienianego kontenera traci focus oraz zaznaczenie tekstu.
3. **Brak symetrycznego hooka `destroy` przed podmianą DOM w `swapTarget` (`app/js/dispatcher.js:153–174`, `323–333`)**:
   - `Dispatcher.mountAll(root, skipInit)` wywołuje `init` po zamontowaniu nowego DOM, ale przed `targetEl.innerHTML = html` żaden kontroler nie dostaje sygnału `destroy` dla usuwanych węzłów (timery, pętle animacji i uchwyty audio/WebGL muszą być czyszczone ręcznie w ad-hocowych miejscach).
4. **Brak ochrony przed wyścigiem odpowiedzi (`out-of-order POST responses`) w `Dispatcher.post` / `Dispatcher.applyResponse` (`app/js/dispatcher.js:335–366`)**:
   - Przy szybkich akcjach użytkownika (np. przełączanie scen, szybkie klikanie opcji, auto-zapis i natychmiastowa zmiana widoku) wolniejsza odpowiedź z wcześniejszego `POST /api/action` może dotrzeć po nowszej i nadpisać `ctrl.data` oraz `innerHTML` nieaktualnym stanem.

---

## 2. Rozwiązania (Solutions)

1. **Czysty `Dispatcher` oparty wyłącznie na `bindings()` (`ARCH.md §4.1` i `§5`)**:
   - Usunięcie z `app/js/dispatcher.js` całej obsługi atrybutów `action`, `kind`, `target`, `event-*`, `param-*` oraz obiektu `modules.backend`.
   - Uproszczenie `Dispatcher.action(actionName)` do pojedynczego parametru bez `mapFn`.
   - Przekazywanie `skipInit` (`frame.keepScroll === "true"`) z `applyResponse` do `swapTarget(t, f.html || "", skipInit)`.
2. **Automatyczne zachowanie focusu i kursora w `swapTarget`**:
   - Wydzielenie `captureFocusState(targetEl)` i `restoreFocusState(targetEl, savedFocus)` w `Dispatcher`: przed podmianą `targetEl.innerHTML` sprawdzamy, czy `document.activeElement` leży wewnątrz `targetEl`; zapisujemy jego identyfikator (`id` lub `name` + `tagName`) i `selectionStart`/`selectionEnd`, a po podmianie DOM przywracamy focus i kursor bez ingerencji w kontrolery.
3. **Symetryczny cykl życia `init` / `destroy` w `bindings()` (`Dispatcher.unmountAll(root)`)**:
   - Dodanie metody `Dispatcher.unmountAll(root)` wywoływanej w `swapTarget` tuż przed `targetEl.innerHTML = html`.
   - Jeśli kontroler zadeklaruje `destroy: this.cleanupMethod` na selektorze w `bindings()`, `unmountAll(targetEl)` wywoła `cleanupMethod.call(ctrl, { idx: el._idx }, el)` dla każdego dopasowanego elementu wewnątrz usuwanego drzewa.
4. **Monotoniczny licznik sekwencji żądań (`reqSeq` / `appliedSeq`) w `Dispatcher.post` i `Dispatcher.applyResponse`**:
   - Każde wywołanie `Dispatcher.post` inkrementuje `this.reqSeq` i przekazuje `seq` do `applyResponse(resp, action, params, target, seq)`.
   - Jeśli po `await fetch(...)` / `await resp.text()` okaże się, że `seq < this.appliedSeq`, odpowiedź z przestarzałego żądania jest odrzucana zanim zmutuje `ctrl.data` lub DOM.

---

## 3. Zmiany plik po pliku, metoda po metodzie, linia po linii

### 1. `app/js/dispatcher.js`

- **`L9–22` (pola stanu `Dispatcher`)**:
  - Dodać liczniki sekwencji żądań:
    ```js
    reqSeq: 0,
    appliedSeq: 0,
    ```
- **`L23–71` (`settings`)**:
  - Usunąć martwe klucze: `paramPrefixLen`, `attrs.action`, `attrs.kind`, `attrs.target`, `attrs.eventPrefix`, `attrs.paramPrefix`, `selectors.action`, `vals.backend`.
  - Dodać w `settings.vals`: `keepScrollKey: "keepScroll"`.
- **`L72–84` (`controllers`, `modules`, `register`)**:
  - Usunąć pole `modules: { backend: new Proxy(...) }` oraz przypisanie `this.modules[name] = ctrl` w `register(name, ctrl)`.
- **`L110–116` (`action(actionName)`)**:
  - Zmienić z `action(actionName, mapFn)` na:
    ```js
    action(actionName) {
      return function (params, _el, e) {
        if (e && e.preventDefault) e.preventDefault();
        Dispatcher.post(actionName, params);
      };
    },
    ```
- **`L153–174` (`mountAll` + nowa metoda `unmountAll`)**:
  - Dodać przed `mountAll(root, skipInit)` symetryczną metodę `unmountAll(root)`:
    ```js
    unmountAll(root) {
      if (!root) return;
      const entries = this.bindingsByEvent.destroy;
      if (!entries) return;
      for (let i = 0; i < entries.length; i++) {
        const entry = entries[i];
        if (root.matches && root.matches(entry.selector)) {
          entry.handler.call(entry.ctrl, { idx: root._idx || 0 }, root);
        }
        const matched = root.querySelectorAll ? root.querySelectorAll(entry.selector) : [];
        for (let j = 0; j < matched.length; j++) {
          entry.handler.call(entry.ctrl, { idx: matched[j]._idx || j }, matched[j]);
        }
      }
    },
    ```
- **`L176–261` (`resolve(e)`)**:
  - Usunąć gałęzie `pPrefix + a.action` (`L199–209`), `evEl = t.closest("[" + evAttr + "]")` (`L212–223`) oraz fallback `[action]` (`L247–260`).
  - Zostawić wyłącznie obsługę aktywnego przeciągania `this.activePointerEl` (`L181–198`) oraz dopasowanie najgłębszego selektora z `this.bindingsByEvent[e.type]` (`L225–246`).
- **`L263–321` (`collectParams(hit, e)`)**:
  - Usunąć gałęzie `paramPrefix` (`L281–282`, `L309–310`) oraz pętlę `hit.eventPrefix` (`L288–296`).
  - Zostawić odczyt `formEl.elements`, `el.name`, atrybutów `data-*` (z wyjątkiem `data-hint`), współrzędnych `client_x`/`client_y` oraz `drop_target` ze stref `[drop-zone]`.
- **`L323–333` (`swapTarget` + zachowanie focusu/kursora i `unmountAll`)**:
  - Dodać pomocnicze metody `captureFocus(targetEl)` i `restoreFocus(targetEl, focusState)` oraz zaktualizować `swapTarget(targetSel, html, skipInit)`:
    ```js
    captureFocus(targetEl) {
      const active = document.activeElement;
      if (!active || !targetEl || !targetEl.contains || !targetEl.contains(active)) return null;
      if (!active.id && !active.name) return null;
      return {
        id: active.id || "",
        name: active.name || "",
        start: typeof active.selectionStart === this.settings.vals.numType ? active.selectionStart : null,
        end: typeof active.selectionEnd === this.settings.vals.numType ? active.selectionEnd : null
      };
    },

    restoreFocus(targetEl, state) {
      if (!state || !targetEl) return;
      let next = state.id ? document.getElementById(state.id) : null;
      if (!next && state.name && targetEl.querySelector) {
        const list = targetEl.querySelectorAll("[name]");
        for (let i = 0; i < list.length; i++) {
          if (list[i].name === state.name) {
            next = list[i];
            break;
          }
        }
      }
      if (!next || typeof next.focus !== this.settings.vals.fnType) return;
      next.focus();
      if (state.start !== null && typeof next.setSelectionRange === this.settings.vals.fnType) {
        next.setSelectionRange(state.start, state.end !== null ? state.end : state.start);
      }
    },

    swapTarget(targetSel, html, skipInit) {
      let targetEl = document.querySelector(targetSel);
      if (!targetEl) return;
      const focusState = this.captureFocus(targetEl);
      this.unmountAll(targetEl);
      targetEl.innerHTML = html;
      if (targetEl.id && targetEl.firstElementChild && targetEl.firstElementChild.id === targetEl.id && targetEl.parentElement) {
        const unwrapped = targetEl.firstElementChild;
        targetEl.parentElement.replaceChild(unwrapped, targetEl);
        targetEl = unwrapped;
      }
      this.mountAll(targetEl, skipInit);
      this.restoreFocus(targetEl, focusState);
    },
    ```
- **`L335–366` (`applyResponse` i `post` — sekwencjonowanie i `skipInit`)**:
  - W `post(action, params, target)` przydzielić `const seq = ++this.reqSeq;` i przekazać `seq` do `this.applyResponse(resp, action, params, target, seq)`.
  - W `applyResponse(resp, action, params, targetOverride, seq)` odrzucić odpowiedź jeśli `seq !== undefined && seq < this.appliedSeq`, w przeciwnym razie zaktualizować `this.appliedSeq = seq` i przekazać `const skipInit = f[s.vals.keepScrollKey] === s.vals.trueStr;` do `this.swapTarget(t, f.html || "", skipInit)`.
- **`L368–390` (`handle(e)`)**:
  - Uprościć `handle(e)` do wywołania `hit.handler.call(hit.ctrl, params, hit.el, e, hit)` (usunięcie martwego fallbacku `this.modules[hit.kind]`).

---

### 2. `ARCH.md`

- **`§5 pkt 5` (`L258–260`, Universal `init` / `destroy` Lifecycle)**:
  - Udokumentować opcjonalny hook `destroy: this.myCleanupMethod` w `bindings()`, wywoływany przez `Dispatcher.unmountAll(targetEl)` przed podmianą `targetEl.innerHTML`, oraz automatyczne przywracanie focusu/kursora i ochronę sekwencji `reqSeq` w `Dispatcher.swapTarget` / `Dispatcher.post`.
