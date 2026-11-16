# Plan naprawy dyktafonu (`EditorVoice` / `EditorVoiceWaveform` / `#modal-voice-rec-strip`)

## 1. Co jest obecnie i na czym polegają problemy

1. **Ogromny lag podczas odtwarzania (`laguje okropnie przy playbacku`)**:
   - W `domain/studio/scripts/editor_voice.js` (`togglePreview`, linie 320–338) pętla `requestAnimationFrame(tick)` wywołuje `EditorVoiceWaveform.draw(stripEl, ts, cap, playheadRatio)` 60–120 razy na sekundę.
   - W `domain/studio/scripts/editor_voice_waveform.js` (`draw`, linie 114–179) każda klatka:
     1. Modyfikuje `playheadEl.style.left = ...%` (linia 120), a chwilę później wywołuje `canvas.getBoundingClientRect()` (linia 124), wymuszając synchroniczny reflow DOM (`layout thrashing`) 120 razy na sekundę.
     2. Wywołuje `this.resolvePeaks(stripEl, trimState, capture)` (linia 148), które w linii 85 sprawdza `if (meta.voicePeaks)` **przed** sprawdzeniem `peakCache`, przez co w każdej klatce animacji odpytuje DOM (`resolveStripMeta`) i od nowa parsuje 320-znakowy string (`parseCompactPeaks`: 160× `slice` + `parseInt` + alokacja tablicy).
     3. Czyści i przerysowuje cały canvas 2D (~160 wywołań `ctx.fillRect`) oraz rysuje drugą, zduplikowaną linię playheadu na canvasie (linie 174–177), mimo że sam wykres fali podczas odtwarzania jest w 100% statyczny.

2. **Przerywanie odtwarzania za pierwszym razem (`przerywa przy playbacku, za pierwszym razem zawsze`)**:
   - W `domain/studio/scripts/editor_voice.js` (`togglePreview`, linie 304–341):
     1. Obiekt `new window.Audio(meta.audioUrl)` tworzony jest dopiero w momencie kliknięcia Play, a `startPlay()` odpala się już na zdarzeniu `loadedmetadata` (`readyState >= 1`, zanim bufor audio osiągnie `canplay`).
     2. W `startPlay()` wywoływane jest asynchroniczne `aud.play()`, a **synchronicznie w kolejnej linii** (zanim Promise `aud.play()` się rozstrzygnie i zanim WebKit/CoreAudio wystartuje potok wyjściowy) planowane jest `pb.animFrame = requestAnimationFrame(tick)` (linia 338).
     3. W pierwszej klatce `tick()` (linia 321) warunek `if (!pb.playing || aud.paused)` widzi `aud.paused === true` (ponieważ `aud.play()` jeszcze się inicjalizuje lub wykonuje seek `aud.currentTime = t0`), przez co **natychmiast przerywa stan odtwarzania**: ustawia `pb.playing = false`, przełącza UI w stan `ready` i chowa playhead, podczas gdy Promise `aud.play()` startuje ułamek sekundy później w tle (lub zostaje ucięty przy kolejnym kliknięciu Play).

3. **Po nadpisaniu starego tracka pokazuje nowy waveform, ale odtwarza poprzedni plik i nie przerywa (`przy nadpisaniu starego tracka tego nie robi`, `po nadpisaniu starego tracka potrafi pokazac nowy waveform ale odtworzyc poprzedni plik`)**:
   - W `domain/studio/scripts/editor_voice.js` w `togglePreview` (linia 299) oraz `initStrips` (linia 858) adres audio jest porównywany po odcięciu query stringu:
     ```js
     const baseUrl = meta.audioUrl.split(this.values.querySep)[0];
     if (!pb.audio || pb.sourceUrl !== baseUrl) { ... }
     ```
   - Serwer dokleja do `voiceUrl` znacznik modyfikacji `?v=<modStamp>` (`domain/studio/views/editor_context_view.go:433`, `domain/book/actions/variables_prose.go:844,943`), ale `split("?")[0]` go usuwa.
   - Ponieważ nadpisane nagranie tej samej sceny/sekcji zapisuje się pod tą samą ścieżką (`voice/<scene_id>.flac`), `baseUrl` się nie zmienia:
     - `TextEditorController.data.voicePeaks` zawiera nową obwiednię z Go, więc canvas rysuje **nowy waveform**,
     - natomiast `pb.audio` **nie jest przeładowywane** (`pb.sourceUrl === baseUrl`) i odtwarza zbuforowany w pamięci **stary plik** z poprzedniego nagrania (dlatego też za drugim razem nie buforuje od zera i nie przerywa).
   - Dodatkowo `domain/book/actions/loader.go:1522` ustawia `Cache-Control: private, max-age=3600` również dla plików audio.

4. **Metaliczne echo podczas nagrywania, jakby szło kilka strumieni naraz (`czasem przy nagrywaniu jest metaliczne echo jakby iles strumieni szlo naraz?`)**:
   - **Problem A — nakładające się strumienie `ScriptProcessorNode` (wyścig `getUserMedia`)**:
     - `startWebAudioCapture` (`domain/studio/scripts/editor_voice.js:486`) jest funkcją `async`. `stopCaptureTracks()` wywoływane jest w `startCapture` **przed** `await navigator.mediaDevices.getUserMedia(...)`.
     - Gdy nagrywanie zostanie zainicjowane dwukrotnie w krótkim odstępie (np. auto-start w `initStrips` po `ToggleSceneVoiceRecorder` + kliknięcie Record), oba wywołania przechodzą przez `if (!cap.starting)` i tworzą **dwa równoległe grafy `MediaStreamSource -> ScriptProcessorNode`** w tym samym `cap.audioCtx`.
     - Oba callbacki `onaudioprocess` dopisują na zmianę swoje bufory `2048` próbek (~85 ms) do wspólnej tablicy `cap.pcmChunks.push(copy)`, przez co w zapisanym pliku `.flac` każdy 85-milisekundowy blok powtarza się dwukrotnie (dając metaliczny efekt filtru grzebieniowego / dwóch nałożonych głosów).
   - **Problem B — podpięcie `ScriptProcessorNode` bezpośrednio do `audioCtx.destination` oraz WebRTC DSP**:
     - Linia 544 (`proc.connect(cap.audioCtx.destination)`) łączy procesor bezpośrednio z wyjściem audio bez wyzerowania `audioEvent.outputBuffer` i bez wyciszonego `GainNode(0)`.
     - Ponadto `getUserMedia` włącza agresywne filtry konferencyjne (`echoCancellation: true, noiseSuppression: true`), które zniekształcają barwę głosu lektora.
   - **Problem C — brak rozpoznania natywnego hosta macOS `WKWebView`**:
     - W linii 595 `canUseWebMic` sprawdza `!window.mojraNative`, podczas gdy okno macOS (`engine/osbridge/window_darwin.go:229`) rejestruje `window.webkit.messageHandlers.nativeHost`. Przez to natywna aplikacja macOS używała przeglądarkowego `ScriptProcessorNode` zamiast dedykowanego wątku CoreAudio C (`osbridge.StartNativeAudioCapture`).

5. **Kliknięcie na osi waveformu automatycznie włącza odtwarzanie (`klikniecie na osi waveform nie powinno automatycznie odtwarzac, trzeba przycisk play`)**:
   - W `domain/studio/scripts/editor_voice.js` (`startScrubDrag`, linia 833) kliknięcie na `.voice-rec-waveform-wrap` wykonuje:
     ```js
     if (!pb.playing) this.togglePreview(stripEl);
     ```
     co natychmiast uruchamia playback zamiast wyłącznie ustawić kursor (`playhead`) w wybranym miejscu.

6. **Skaczący przycisk Record, zmieniający się układ UI oraz przestarzałe atrybuty `data-*` (`przycisk record skacze u ui - jesli jest pusty waveform, lest po lewej, a potem po prawej; ui nie moze skakac, osobne przyciski play / record zawsze`)**:
   - W `domain/studio/templates/text_editor_view.tpl` (linie 242–280) oraz `domain/studio/styles/editor_inspector.tss` (linie 1017–1051):
     - Po lewej stronie waveformu istnieje jeden hybrydowy przycisk `.voice-rec-transport-btn`, który w stanie `idle` jest mikrofonem (`Record`), a po nagraniu zamienia się w `Play/Pause`.
     - Jednocześnie po prawej stronie waveformu ukryty przycisk `.voice-rec-rerecord-btn` oraz `.voice-rec-delete-btn` mają `display: none !important` w stanach `idle`/`recording` i `display: inline-flex !important` w stanach `ready`/`playing`.
     - W rezultacie po nagraniu lub usunięciu pliku przycisk Record „przeskakuje” z lewej strony na prawą, a pojawianie się i znikanie przycisków po prawej zmienia szerokość osi waveformu.
     - Ponadto szablon `.voice-rec-strip` i skrypty używają przestarzałych atrybutów `data-voice-*` / `data-scene` do przesyłania stanu przez DOM, co narusza `docs/api/arch.md` §4.3 (wszystkie dane są już w `TextEditorController.data`).

---

## 2. Rozwiązania

1. **Stały układ UI bez skakania oraz zero atrybutów `data-*` na przyciskach i elementach `.voice-rec-strip`**:
   - Układ `.voice-rec-strip` jest zawsze stały we wszystkich stanach (`.is-idle`, `.is-recording`, `.is-ready`, `.is-playing` na `.voice-rec-strip`):
     - **Po lewej stronie waveformu**:
       1. `button.voice-rec-transport-btn.voice-rec-play-btn`: zawiera wyłącznie ikony `voice-play.svg` (`.voice-rec-icon-play`) i `voice-pause.svg` (`.voice-rec-icon-pause`). Gdy brak nagrania (`.is-idle`) lub trwa nagrywanie (`.is-recording`), przycisk pozostaje na swoim miejscu i jest wyłączony (`disabled`, `.is-disabled`).
       2. `button.voice-rec-transport-btn.voice-rec-record-btn`: zawiera wyłącznie ikony `mic.svg` (`.voice-rec-icon-mic`) i `voice-stop.svg` (`.voice-rec-icon-stop`). W stanie `.is-recording` pokazuje ikonę Stop (`voice-stop.svg`) i po kliknięciu kończy oraz zapisuje nagranie; w pozostałych stanach pokazuje ikonę mikrofonu (`mic.svg`) i po kliknięciu rozpoczyna nagrywanie (lub nadpisanie ścieżki).
     - **Środek**: `.voice-rec-waveform-wrap` (stała szerokość `flex: 1`, nigdy nie kurczy się ani nie rozszerza przy zmianie stanu).
     - **Po prawej stronie waveformu**:
       3. `span.voice-rec-timer`.
       4. `button.voice-rec-action-btn.voice-rec-cut-btn` (`scissors.svg`): zawsze widoczny, `disabled` gdy `!isTrimmed || isRecording`.
       5. `button.voice-rec-action-btn.voice-rec-delete-btn` (`trash.svg`): zawsze widoczny, `disabled` gdy `!hasFile || isRecording`.
   - Żaden przycisk w `.voice-rec-strip` nigdy nie otrzymuje `display: none` ani `.hidden`, a cały pasek `.voice-rec-strip` nie używa żadnych atrybutów `data-voice-*` ani `data-scene` (poza `data-hint` do tooltipów). Stan odczytywany jest bezpośrednio z `TextEditorController.data`.

2. **Kliknięcie i przeciąganie po osi waveformu (scrubbing) wyłącznie ustawia kursor (`playhead`), bez auto-playu**:
   - Usuwamy z `startScrubDrag` wywołanie `if (!playbackState.playing) this.togglePreview(stripElement)`.
   - Dodajemy w `playback` pole `scrubRatio` (domyślnie `0`, klamrowane do `[trimStart, trimEnd]`).
   - Kliknięcie/przeciągnięcie na `.voice-rec-waveform-wrap` (również przed pierwszym odtworzeniem, gdy `durationSec === 0`) ustawia `playbackState.scrubRatio = ratio`, aktualizuje `playbackState.audio.currentTime = ratio * playbackState.durationSec` (jeśli audio jest załadowane), ustawia czas na `.voice-rec-timer` oraz pozycjonuje widoczny `.voice-rec-playhead` (`EditorVoiceWaveform.syncPlayhead(stripElement, ratio)`).
   - Dopiero kliknięcie przycisku `.voice-rec-play-btn` (`togglePreview`) rozpoczyna odtwarzanie od pozycji `playbackState.scrubRatio`.
   - Usuwamy również z `initStrips` automatyczny start nagrywania po samym otwarciu paska dyktafonu (`ToggleSceneVoiceRecorder`) — nagrywanie uruchamia wyłącznie kliknięcie przycisku `.voice-rec-record-btn`.

3. **Zero-reflow / zero-canvas-redraw podczas odtwarzania (120fps P0)**:
   - Wydzielamy w `EditorVoiceWaveform` metodę `syncPlayhead(stripElement, playheadRatio)`, która aktualizuje wyłącznie pozycję `span.voice-rec-playhead` (`style.left`) oraz jego klasę `.is-visible`, bez wywoływania `getBoundingClientRect()`, `resolvePeaks()` ani `ctx.fillRect()` na canvasie.
   - Z `EditorVoiceWaveform.draw()` usuwamy rysowanie drugiej linii playheadu na canvasie 2D (`ctx.fillRect` dla `playheadRatio`). Canvas rysuje wyłącznie statyczną obwiednię fali (oraz podgląd na żywo podczas nagrywania).
   - W `EditorVoiceWaveform.resolvePeaks()` cache'ujemy sparsowane słupki w `peakCache` po kluczu `meta.audioUrl || meta.voicePeaks`, sprawdzając cache **przed** wywołaniem `parseCompactPeaks`.

4. **Eliminacja przerywania przy pierwszym odtwarzaniu oraz odtwarzania starego pliku po nadpisaniu**:
   - W `playbackState.sourceUrl` przechowujemy **pełny `meta.audioUrl` z parametrem `?v=<stamp>`** (usuwamy `.split("?")[0]`).
   - Dodajemy metodę `resetPlaybackAudio()`, która zatrzymuje i czyści `playbackState.audio` oraz zeruje `sourceUrl = ""`, `playing = false`, `startingPlay = false`, `durationSec = 0`, `scrubRatio = 0` (wywoływaną przy `startCapture`, `stopAndSaveCapture`, `cutTrim` oraz gdy `playbackState.sourceUrl !== meta.audioUrl` w `initStrips`). Wyzerowanie `durationSec = 0` zapobiega nadpisywaniu serwerowego `{SelectedNode.VoiceDurationLabel}` starym czasem trwania w `syncHandles`.
   - W `initStrips` (gdy `meta.audioUrl` istnieje i zmienił się względem `playbackState.sourceUrl`) od razu tworzymy `playbackState.audio = new window.Audio(meta.audioUrl)` z `preload = this.values.preloadAuto`, aby przeglądarka zdekodowała nagranie przed kliknięciem Play.
   - W `togglePreview`:
     - Dodajemy strażnika `playbackState.startingPlay`, aby pętla `requestAnimationFrame(tick)` startowała **dopiero w `audioElement.play().then(...)`** (gdy `audioElement.paused === false`), nigdy przed rozwiązaniem Promise `audioElement.play()`.
     - Gdy `audioElement.readyState < 2` i `audioElement.addEventListener` jest dostępne, czekamy na zdarzenie `this.values.eventCanPlay` (z fallbackiem `this.values.eventLoadedData`), a `audioElement.currentTime` przestawiamy tylko wtedy, gdy `Math.abs(audioElement.currentTime - startSec) > this.audioSpec.seekEpsilonSec`.
   - W `domain/book/actions/loader.go` (`ServeBookMedia`) dla plików audio ustawiamy `Cache-Control: no-cache` zamiast `private, max-age=3600`.

5. **Eliminacja metalicznego echa i duplikacji strumieni podczas nagrywania**:
   - W `EditorVoice` wykrywamy natywny host macOS (`window.webkit && window.webkit.messageHandlers && window.webkit.messageHandlers.nativeHost`) obok `window.mojraNative`, dzięki czemu aplikacja macOS nagrywa przez natywny CoreAudio `AudioQueue` (`startNativeCapture` -> `osbridge.StartNativeAudioCapture`), a nie przez WebAudio `ScriptProcessorNode`.
   - W `capture` dodajemy inkrementowany identyfikator sesji `sessionSeq`:
     - `startCapture` i `stopCaptureTracks` zwiększają `captureState.sessionSeq`.
     - W `startNativeCapture` ustawiamy `captureState.nativeMode = true` przed `await Dispatcher.post(...)`; jeśli po `await` sesja została anulowana (`!captureState.starting || sessionId !== captureState.sessionSeq`), wysyłamy `StopSceneVoiceCapture`, aby nie pozostawić otwartej kolejki CoreAudio w tle.
     - Po każdym `await` w `startWebAudioCapture` sprawdzamy `if (!captureState.starting || sessionId !== captureState.sessionSeq)` i w razie niezgodności natychmiast zamykamy strumień, a tuż przed podpięciem nowego `sourceNode` / `processorNode` wywołujemy `this.stopCaptureTracks()`.
   - W `startWebAudioCapture`:
     - Ustawiamy `echoCancellation: false, noiseSuppression: false, autoGainControl: false`.
     - W `processorNode.onaudioprocess` zerujemy bufor wyjściowy (`if (audioEvent.outputBuffer && audioEvent.outputBuffer.getChannelData) audioEvent.outputBuffer.getChannelData(0).fill(0)`) i podpinamy `processorNode` przez wyciszony `GainNode` (`gain.value = 0`, jeśli `createGain` jest dostępne) do `captureState.audioCtx.destination`.

---

## 3. Zmiany plik po pliku, metoda po metodzie i linia po linii

### 1. `domain/studio/templates/text_editor_view.tpl` (linie 242–280)
- W `.voice-rec-strip.modal-voice-rec-strip#modal-voice-rec-strip`:
  - Całkowite usunięcie atrybutów `data-voice-*` i `data-scene` z `.voice-rec-strip`, przycisków, `.voice-rec-waveform-wrap`, `canvas`, `.voice-rec-playhead`, `.voice-rec-trim-handle` i `.voice-rec-timer`.
  - Dodanie warunkowych klas stanu `.is-ready?: {SelectedNode.HasVoiceFile}` oraz `.is-idle?: {!SelectedNode.HasVoiceFile}` na `.voice-rec-strip`.
  - Rozdzielenie dotychczasowego hybrydowego `button.voice-rec-transport-btn` oraz skaczącego po prawej `.voice-rec-rerecord-btn` na dwa stałe przyciski po lewej stronie `.voice-rec-waveform-wrap`:
    1. `button.voice-rec-transport-btn.voice-rec-play-btn` (`disabled?: {!SelectedNode.HasVoiceFile}`, `.is-disabled?: {!SelectedNode.HasVoiceFile}`):
       - `svg.voice-rec-icon.voice-rec-icon-play src: /icons/voice-play.svg`
       - `svg.voice-rec-icon.voice-rec-icon-pause src: /icons/voice-pause.svg`
    2. `button.voice-rec-transport-btn.voice-rec-record-btn`:
       - `svg.voice-rec-icon.voice-rec-icon-mic src: /icons/mic.svg`
       - `svg.voice-rec-icon.voice-rec-icon-stop src: /icons/voice-stop.svg`
  - Po prawej stronie `.voice-rec-waveform-wrap` pozostają w stałych miejscach (bez `.hidden` i bez `data-*` poza `data-hint`):
    3. `span.voice-rec-timer` (`{SelectedNode.VoiceDurationLabel}`)
    4. `button.voice-rec-action-btn.voice-rec-cut-btn` (`data-hint: "{t_voice_trim_destructive_hint}"`, `disabled?: {!SelectedNode.HasVoiceTrim}`, `.is-disabled?: {!SelectedNode.HasVoiceTrim}`)
    5. `button.voice-rec-action-btn.voice-rec-delete-btn` (`disabled?: {!SelectedNode.HasVoiceFile}`, `.is-disabled?: {!SelectedNode.HasVoiceFile}`)

### 2. `domain/studio/styles/editor_inspector.tss` (linie 893–1067)
- Zastąpienie wszystkich selektorów atrybutowych `[data-voice-strip][data-voice-state="..."]`, `[data-voice-record-start="true"]`, `[data-voice-delete="true"]` czystymi selektorami klas `.voice-rec-strip.is-recording`, `.voice-rec-strip.is-playing`, `.voice-rec-strip.is-ready`, `.voice-rec-strip.is-idle` (z zachowaniem limitu maks. 4 selektorów po przecinku zgodnie z `TPL_ARCH`).
- Dodanie stylów `.voice-rec-transport-btn:disabled, .voice-rec-transport-btn.is-disabled` (`opacity: 0.28`, `color: --text-muted`, `cursor: default`, `pointer-events: none`).
- Kolor aktywnego przycisku nagrywania (Stop) w stanie nagrywania: `.voice-rec-strip.is-recording .voice-rec-record-btn` otrzymuje `color: --accent-danger`.
- Przełączanie ikon wewnątrz osobnych przycisków:
  - W `.voice-rec-play-btn`: domyślnie `.voice-rec-icon-play` ma `display: block`, `.voice-rec-icon-pause` ma `display: none`; dla `.voice-rec-strip.is-playing` odwrotnie.
  - W `.voice-rec-record-btn`: domyślnie `.voice-rec-icon-mic` ma `display: block`, `.voice-rec-icon-stop` ma `display: none`; dla `.voice-rec-strip.is-recording` odwrotnie.
- Usunięcie reguł `display: none !important` / `display: inline-flex !important` dla przycisków po prawej — wszystkie przyciski są zawsze `display: inline-flex`.
- Widoczność `.voice-rec-playhead`: `display: block` dla `.voice-rec-strip.is-playing .voice-rec-playhead` oraz `.voice-rec-playhead.is-visible`.

### 3. `domain/studio/scripts/editor_voice_waveform.js`
- Pełna zgodność z `@AGENTS.md` §0: zastąpienie wszystkich skrótowych nazw zmiennych i parametrów (`m`, `th`, `stripEl`, `timeEl`, `playheadEl`, `cssW`, `cssH`, `targetW`, `targetH`, `ctx`, `fgActive`, `fgMuted`, `fgRec`, `barH`, `maxBarH`, `i0`, `i1`, `sIdx`, `chData`, `resp`, `buf`) pełnymi opisowymi nazwami `camelCase` oraz przeniesienie `"undefined"` do `values.undefType`.
- Zastąpienie selektorów `[data-voice-*]` selektorami klas (`.voice-rec-waveform-canvas`, `.voice-rec-playhead`, `.voice-rec-trim-handle.trim-start`, `.voice-rec-trim-handle.trim-end`, `.voice-rec-timer`) oraz dodanie `classes: { visible: "is-visible" }`.
- **`resolvePeaks(stripElement, trimState, captureState)` (linie 80–96)**:
  - Sprawdzenie `peakCache` po kluczu `cacheKey = meta.audioUrl || meta.voicePeaks` **przed** wywołaniem `this.parseCompactPeaks(meta.voicePeaks)`, aby uniknąć parsowania stringa przy każdym rysowaniu.
- **`syncPlayhead(stripElement, playheadRatio)` (nowa metoda)**:
  - Jeśli `playheadRatio >= 0 && playheadRatio <= 1`, ustawia `playheadElement.style.left = (playheadRatio * 100).toFixed(2) + "%"` oraz dodaje klasę `playheadElement.classList.add(this.classes.visible)`; gdy `playheadRatio < 0`, usuwa klasę `this.classes.visible`.
- **`draw(stripElement, trimState, captureState, playheadRatio)` (linie 114–179)**:
  - Wywołuje `this.syncPlayhead(stripElement, playheadRatio)` na końcu (po odczycie `canvas.getBoundingClientRect()`, eliminując wymuszony synchroniczny reflow).
  - Usunięcie z `draw` rysowania czerwonego prostokąta playheadu na canvasie (linie 174–177).

### 4. `domain/studio/scripts/editor_voice.js`
- Pełna zgodność z `@AGENTS.md` §0 i `docs/api/arch.md`:
  - Brak skrótowych nazw zmiennych i parametrów (`el`, `pb`, `ts`, `cap`, `aud`, `dur`, `t0`, `t1` zastąpione pełnymi nazwami `stripElement`, `buttonElement`, `playbackState`, `trimState`, `captureState`, `audioElement`, `durationSec`, `startSec`, `endSec`).
  - Brak magicznych stringów w ciałach metod (wszystkie stałe w `values`, `classes`, `dom.selectors`, `actions`) i brak funkcji przelotowych (`pass-through`).
- **`bindings()` (linie 6–24)**:
  - `".voice-rec-strip"`: `{ init: this.initStrips }`
  - `".voice-rec-strip .voice-rec-play-btn"`: `{ click: this.togglePreview }`
  - `".voice-rec-strip .voice-rec-record-btn"`: `{ click: this.toggleRecord }`
  - `".voice-rec-strip .voice-rec-waveform-wrap"`: `{ pointerdown: this.startScrubDrag, pointermove: this.moveTrimDrag, pointerup: this.endTrimDrag }`
  - `".voice-rec-strip .voice-rec-trim-handle"`: `{ pointerdown: this.startTrimDrag, pointermove: this.moveTrimDrag, pointerup: this.endTrimDrag }`
  - `".voice-rec-strip .voice-rec-cut-btn"`: `{ click: this.cutTrim }`
  - `".voice-rec-strip .voice-rec-delete-btn"`: `{ click: Dispatcher.action("DeleteSceneVoice") }`
- **`resolveStripMeta()`**:
  - Odczytuje `sceneId`, `voiceSection`, `voiceUrl`, `voicePeaks`, `voiceTrimStart`, `voiceTrimEnd` bezpośrednio z `TextEditorController.data` (zero wywołań `getAttribute` na DOM).
- **`syncStripState(stripElement)`**:
  - Przełącza klasy stanu `.is-idle`, `.is-recording`, `.is-ready`, `.is-playing` na `stripElement`.
  - Aktualizuje `disabled` oraz klasę `.is-disabled` na `.voice-rec-play-btn` (`isRecording || !hasFile`), `.voice-rec-cut-btn` (`isRecording || !isTrimmed`) i `.voice-rec-delete-btn` (`isRecording || !hasFile`).
  - Aktualizuje `UIController.setHint` osobno na `.voice-rec-play-btn` i `.voice-rec-record-btn`.
- **`resetPlaybackAudio()` (nowa metoda)**:
  - Zatrzymuje i czyści `playbackState.audio`, zeruje `sourceUrl = ""`, `playing = false`, `startingPlay = false`, `durationSec = 0`, `scrubRatio = 0`.
- **`stopPreview(stripElement)` & `togglePreview(_params, targetElement)`**:
  - Porównuje pełny `playbackState.sourceUrl !== meta.audioUrl` (z `?v=<stamp>`).
  - Przy kliknięciu Pause (`if (playbackState.playing || playbackState.startingPlay)`) zapisuje bieżącą pozycję do `playbackState.scrubRatio`, zatrzymuje odtwarzanie i pozostawia playhead widoczny w miejscu pauzy; po dojściu do końca (`audioElement.currentTime >= endSec`) zeruje `playbackState.scrubRatio = trimState.trimStart` i chowa playhead (`spec.playheadOff`).
  - Ustala punkt startu `startSec` z `playbackState.scrubRatio` (jeśli mieści się w `[trimStart, trimEnd)`), w przeciwnym razie `trimStart * durationSec`.
  - Ustawia `playbackState.startingPlay = true`, wywołuje `audioElement.play().then(...)` i **dopiero po rozwiązaniu Promise** uruchamia `playbackState.animFrame = requestAnimationFrame(tick)`.
  - W `tick()` wywołuje wyłącznie `EditorVoiceWaveform.syncPlayhead(stripElement, ratio)` oraz aktualizuje timer bez przerysowywania canvasu (`EditorVoiceWaveform.draw`).
- **`stopCaptureTracks()`, `startWebAudioCapture(stripElement)` & `startNativeCapture(stripElement)`**:
  - Inkrementacja `captureState.sessionSeq` przy starcie/zatrzymaniu; weryfikacja `sessionId === captureState.sessionSeq` po każdym `await`.
  - Wykrywanie natywnego hosta macOS (`window.webkit && window.webkit.messageHandlers && window.webkit.messageHandlers.nativeHost`) w `isNativeHostCaptureAvailable()`, aby macOS korzystał z `startNativeCapture` (CoreAudio).
  - W `startWebAudioCapture`: `echoCancellation: false, noiseSuppression: false, autoGainControl: false`, zerowanie `outputBuffer` w `onaudioprocess` oraz podłączenie przez wyciszony `GainNode` (`gain.value = 0`).
- **`toggleRecord(params, buttonElement)`**:
  - Jeśli `captureState.recording || captureState.starting`, wywołuje `stopAndSaveCapture(stripElement)`; w przeciwnym razie `startCapture(stripElement, params)`.
- **`startTrimDrag(params, handleElement)`, `startScrubDrag(params, wrapElement)` & `moveTrimDrag(params)`**:
  - `startTrimDrag` rozpoznaje uchwyt po klasie `handleElement.classList.contains(this.classes.trimEnd)` (`end` vs `start`).
  - Kliknięcie/przeciągnięcie po `.voice-rec-waveform-wrap` ustawia `playbackState.scrubRatio = clamped`, aktualizuje `playbackState.audio.currentTime` (gdy załadowane) oraz wywołuje `EditorVoiceWaveform.syncPlayhead(stripElement, clamped)` — **bez** wywoływania `this.togglePreview`.
- **`initStrips(_params, mountedElement)`**:
  - Gdy `!captureState.recording && !captureState.starting`, zawsze synchronizuje `trimState` z `meta` i porównuje pełny `playbackState.sourceUrl !== meta.audioUrl` (z `?v=`, w tym po `DeleteSceneVoice`, `SaveSceneVoiceRecording` i `TrimSceneVoice`); przy każdej zmianie wywołuje `resetPlaybackAudio()` i (gdy `meta.audioUrl` jest niepuste) preloaduje `new window.Audio(meta.audioUrl)` (`preload = this.values.preloadAuto`, z nasłuchiwaniem `loadedmetadata` ustawiającym `playbackState.durationSec`).
  - Usunięcie auto-startu nagrywania przy `ToggleSceneVoiceRecorder`.

### 5. `domain/book/actions/loader.go` (linia 1522)
- W `ServeBookMedia`: wydzielenie stałych nagłówków HTTP (`headerCacheControl`, `cacheControlNoCache`, `cacheControlMediaPrivate`) i pełnych nazw parametrów/zmiennych (`@AGENTS.md` §0); dla plików audio (`isAudioFilename(cleanRel)`) ustawienie `Cache-Control: no-cache` zamiast `private, max-age=3600`, aby WebKit zawsze respektował nową wersję pliku po nadpisaniu.

### 6. `domain/server_js_runtime_test.go` (linie 1084–1109, 1538–1626)
- W mocku `win.Audio`: dodanie `this.src = src || ""`, `this.readyState = 4` oraz `addEventListener(ev, fn) { if (typeof fn === "function") fn(); }`.
- W mocku `win.AudioContext`: dodanie `createGain() { return { gain: { value: 1 }, connect() {}, disconnect() {} }; }` oraz `outputBuffer: { getChannelData: () => new Float32Array(24000) }`.
- Aktualizacja testu dyktafonu do selektorów klasowych (`.voice-rec-play-btn`, `.voice-rec-record-btn`, `.voice-rec-cut-btn`, `.voice-rec-delete-btn`, `.voice-rec-waveform-wrap`, `.voice-rec-trim-handle.trim-end`) i klas stanu (`.is-idle`, `.is-recording`, `.is-ready`, `.is-playing`):
  - Weryfikacja, że po otwarciu `#modal-voice-rec-strip` nagrywanie nie startuje samo (`.is-idle`), a przyciski Play, Record, Scissors i Delete są jednocześnie obecne w DOM i żaden nie ma klasy `.hidden`.
  - Kliknięcie `.voice-rec-record-btn` rozpoczyna nagrywanie (`.is-recording`), a ponowne kliknięcie `.voice-rec-record-btn` zatrzymuje je i zapisuje `.flac` (`.is-ready`).
  - Weryfikacja, że kliknięcie na `.voice-rec-waveform-wrap` ustawia pozycję playheadu (`EditorVoice.playback.scrubRatio > 0`, `.voice-rec-playhead.is-visible`), ale **nie** uruchamia odtwarzania (`EditorVoice.playback.playing === false`), dopóki użytkownik nie kliknie `.voice-rec-play-btn`.
  - Weryfikacja, że po nadpisaniu nagrania `EditorVoice.playback.sourceUrl` aktualizuje się do nowego URL-a z nowym `?v=`.
