# voice_rerecord_fix.md

## 1. Co jest i jakie są problemy
1. **Deadlock CoreAudio/OpenSLES przy zatrzymaniu (`audio_capture_darwin.go`, `audio_capture_android.go`)**: `stAudioInputCallback` (linia 84) i `stAndroidMicCallback` (linia 92) wywołują `Enqueue` poza mutexem. Gdy `stStopNativeAudioCapture` (linia 194) woła synchroniczne `AudioQueueStop(queue, true)` na wątku HTTP pod `s.Mu.Lock()`, dochodzi do inwersji blokad CoreAudio i zawieszenia całej aplikacji.
2. **Blokowanie globalnego `s.Mu` na operacjach sprzętowych audio (`actions_dispatch.go`, `actions_editor_scene.go`)**: `StartSceneVoiceCapture` i `StopSceneVoiceCapture` wykonują blokujące wywołania CGO (`AVCaptureDevice`, `AudioQueueStop`, `AudioQueueDispose`) wewnątrz `s.Mu.Lock()` (linia 548).
3. **Wyścig Stop → Re-record w UI (`app/js/editor_voice.js`)**: `stopVoiceRecordingAndSave` (linia 869) nie blokuje kolejnych kliknięć na czas trwania `StopSceneVoiceCapture` / `SaveSceneVoiceRecording`, a `stopVoicePlayback` (linia 508) nie zwalnia `playbackAudio.src`, blokując strumień pliku przy nadpisywaniu tej samej ścieżki.

## 2. Rozwiązania
1. Przenieść `AudioQueueEnqueueBuffer` / `(*bq)->Enqueue` do wnętrza sekcji krytycznej przed `pthread_mutex_unlock` oraz przekazać `NULL` jako `inCallbackRunLoopMode` w `AudioQueueNewInput`.
2. Dodać flagę `isSaving` w `voiceAudioState` blokującą `startVoiceRecording` i przyciski transportu do zakończenia `StopSceneVoiceCapture` / `SaveSceneVoiceRecording` oraz czyścić `playbackAudio.removeAttribute("src"); playbackAudio.load()` w `stopVoicePlayback`.
3. Wykonywać `osbridge.StartNativeAudioCapture()` i `osbridge.StopNativeAudioCapture()` poza `s.Mu.Lock()` w `HandleAction`.

## 3. Zmiany plik po pliku i metoda po metodzie

### `internal/osbridge/audio_capture_darwin.go`
- `stAudioInputCallback` (linie 53–86):
  - Przenieść `if (inAQ != NULL && inBuffer != NULL) { AudioQueueEnqueueBuffer(inAQ, inBuffer, 0, NULL); }` przed `pthread_mutex_unlock(&gVoiceCaptureMu)` (linia 81), aby po `gVoiceIsCapturing = 0` żaden bufor nie trafił do kolejki.
- `stStartNativeAudioCapture` (linie 109–174):
  - Jeśli `gVoiceQueue != NULL` (poprzednia sesja nie została zamknięta), najpierw zatrzymać i zwolnić starą kolejkę poza mutexem zamiast zwracać `0` (linie 114–119).
  - W `AudioQueueNewInput` (linia 144) zmienić `kCFRunLoopCommonModes` na `NULL` (gdy `inCallbackRunLoop == NULL`).

### `internal/osbridge/audio_capture_android.go`
- `stAndroidMicCallback` (linie 65–94):
  - Przenieść `if (bq != NULL) { (*bq)->Enqueue(...); }` przed `pthread_mutex_unlock(&gAndroidMicMu)` (linia 90).

### `internal/actions/actions_dispatch.go` i `internal/actions/actions_editor_scene.go`
- `HandleAction` (`actions_dispatch.go`, przed `s.Mu.Lock()` w linii 548):
  - Dla `StartSceneVoiceCapture` i `StopSceneVoiceCapture` wywołać zatrzymanie/start `osbridge` poza `s.Mu.Lock()`, przekazując odebrany bufor PCM do handlera stanu.
- `startSceneVoiceCaptureAction` / `stopSceneVoiceCaptureAction` (`actions_editor_scene.go`, linie 1580–1644):
  - Usunąć bezpośrednie wywołania CGO wewnątrz locka stanu; operować wyłącznie na przekazanym wyniku i zapisie `book.SaveSceneVoiceSectionPCM`.

### `app/js/editor_voice.js`
- `voiceAudioState` (linie 3–38):
  - Dodać pole `isSaving: false`.
- `stopVoicePlayback` (linie 495–514):
  - Po `voiceAudioState.playbackAudio.pause()` (linia 510) dodać `voiceAudioState.playbackAudio.removeAttribute("src"); voiceAudioState.playbackAudio.removeAttribute("data-src"); voiceAudioState.playbackAudio.load();`.
- `startVoiceRecording` (linia 668):
  - Zmienić strażnika w linii 669 na `if (voiceAudioState.isRecording || voiceAudioState.isSaving) return;`.
- `stopVoiceRecordingAndSave` (linie 869–971):
  - Zmienić na `async function stopVoiceRecordingAndSave()`, ustawić `voiceAudioState.isSaving = true` przed `await sendServerAction(...)` (`StopSceneVoiceCapture` w linii 906 i `SaveSceneVoiceRecording` w linii 894/962) i zwalniać `voiceAudioState.isSaving = false` w `finally`.