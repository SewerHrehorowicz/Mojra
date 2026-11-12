# Plan usunięcia `createAppIcon` w runtime (`remove_runtime_icon_draw.md`)

## 1. Co tam jest i jakie są problemy

1. **Zduplikowane rysowanie ikony 512×512 w Objective-C podczas każdego startu aplikacji (`internal/osbridge/window_darwin.go:43–137, 343–351`)**:
   - `createAppIcon(void)` (`L43–137`, 95 linii kodu CoreGraphics/NSBezierPath) duplikuje generator ikony z `renderIcon` (`cmd/mojra/main.go:2036`) i rysuje bitmapę `512×512` przy każdym uruchomieniu (zajmując `~52 ms`).
   - W paczce `Mojra.app` ikona `Contents/Resources/AppIcon.icns` jest już wygenerowana podczas buildu (`writeMacOSAppIconICNS`) i zadeklarowana w `Info.plist` (`CFBundleIconFile`), więc macOS ustawia ikonę w Docku automatycznie przed startem procesu.
   - Dla uruchomień z gołej binarki CLI (`go run` / `Mojra-macos-*` poza `.app`), gotowy plik `app/icons/icon.png` jest już wbudowany w `mojra.AppFS` (`embed.go:7` i `embed_prod.go:7`).

---

## 2. Rozwiązania

1. **Całkowite usunięcie `createAppIcon(void)` z `internal/osbridge/window_darwin.go`**:
   - Usunąć 95 linii proceduralnego rysowania `NSBezierPath` w `window_darwin.go:43–137`.
2. **Warunkowe użycie gotowego `app/icons/icon.png` z `mojra.AppFS` wyłącznie poza pakietem `.app`**:
   - W `Mojra.app` (`[[NSBundle mainBundle] bundleIdentifier] != nil`) pominąć ustawianie ikony w runtime (`0 ms`, macOS używa `AppIcon.icns`).
   - Poza `.app` (`[[NSBundle mainBundle] bundleIdentifier] == nil`) przekazać bajty `app/icons/icon.png` z `mojra.AppFS` do `runNativeMacOSWebView` i utworzyć `NSImage` przez `[[NSImage alloc] initWithData:...]` (`<0.2 ms`).

---

## 3. Zmiany plik po pliku, metoda po metodzie, linia po linii

### Plik 1: `internal/osbridge/window_darwin.go`
- **Deklaracja `STAppDelegate` (`L31–38`)**:
  - Dodać właściwość `@property (assign) const void *iconBytes;` oraz `@property (assign) int iconLen;`.
- **Funkcja `createAppIcon` (`L43–137`)**:
  - Całkowicie usunąć funkcję `static NSImage *createAppIcon(void)`.
- **Metoda `applicationDidFinishLaunching:` (`L343–351`)**:
  - Zastąpić blok `dispatch_async` wywołujący `createAppIcon()` sprawdzeniem:
    - Jeśli `[[NSBundle mainBundle] bundleIdentifier] == nil && self.iconBytes != NULL && self.iconLen > 0`, utworzyć `NSData *data = [NSData dataWithBytes:self.iconBytes length:(NSUInteger)self.iconLen]` i `NSImage *icon = [[NSImage alloc] initWithData:data]`, wywołać `[NSApp setApplicationIconImage:icon]` oraz `goLogCocoaStartupStage(2, (char *)"icon.png")`.
- **Funkcja C `runNativeMacOSWebView` (`L402–414`)**:
  - Zmienić sygnaturę z `static void runNativeMacOSWebView(const char *urlStr)` na `static void runNativeMacOSWebView(const char *urlStr, const void *iconBytes, int iconLen)` i przypisać `gDelegate.iconBytes = iconBytes; gDelegate.iconLen = iconLen;`.
- **Importy i stałe Go (`L527–540`)**:
  - Dodać import `"mojra"` oraz stałą `const embeddedAppIconPath = "app/icons/icon.png"`.
- **Funkcja Go `RunNativeOSWindow` (`L570–582`)**:
  - Wczytać `iconData, _ := mojra.AppFS.ReadFile(embeddedAppIconPath)` i przekazać wskaźnik `unsafe.Pointer(&iconData[0])` oraz `C.int(len(iconData))` (lub `nil, 0` gdy puste) do `C.runNativeMacOSWebView`.