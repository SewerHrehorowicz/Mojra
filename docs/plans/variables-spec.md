# Specyfikacja Zmiennych i Logiki Warunkowej (`{{ }}`) — Mojra (Playbook Studio)

## 1. Założenia Architektoniczne
- **Zero JSON / Zero YAML / Zero SQL**: cała konfiguracja zmiennych i logiki narracyjnej jest zapisywana w czytelnych plikach tekstowych (`data.txt`, `story.txt` / `<rozdzial>.txt`) oraz binarnie w pliku zapisu gracza (`saves/<book-id>.sav`).
- **Jednolita składnia `{{ ... }}`**: wszystkie wyrażenia logiczne, modyfikacje zmiennych, wstawki w tekście i bloki warunkowe używają podwójnych nawiasów klamrowych `{{ ... }}`. Zwykłe nawiasy okrągłe `(...)` w prozie i didaskaliach pozostają nienaruszone, a wewnątrz `{{ ... }}` służą do grupowania warunków (`&&`, `||`).
- **Bez `$` i bez `""`**: nazwy zmiennych oraz dozwolone wartości wyliczeniowe (`enum`) są zadeklarowane w `data.txt`, dzięki czemu parser jednoznacznie odróżnia zmienne od stanów i natychmiast wykrywa literówki.

---

## 2. Typy Zmiennych i Deklaracja w `data.txt`

Obsługiwane są 3 typy zmiennych:
1. **`bool` (Flaga: `true` / `false`)** — przedmioty, odkryte tropy, stany binarne.
2. **`int` (Liczba całkowita)** — punkty relacji, zasoby, amunicja, liczniki czasu.
3. **`enum` (Słownik stanów)** — zdefiniowany przez `enum: <nazwa_typu> = stan1, stan2, ...`, przypisywany zmiennej przez podanie jednego ze stanów jako wartości początkowej.

### Przykład w `data.txt`
```txt
enum: frakcje = neutralny, rzeczni, przemytnicy, zakon
enum: pogoda = mgla, ulewa, susza

var: latarka = false
var: klucz_sluzy = false
var: zaufanie = 0
var: naboje = 6
var: frakcja = neutralny
var: aura = mgla
```

---

## 3. Operatory Warunków i Modyfikacji

### Warunki (wewnątrz `{{ ... }}`)
- **Flagi (`bool`)**:
  - `{{latarka}}` — prawda (`true`)
  - `{{!latarka}}` — fałsz (`false`)
- **Porównania (`int` oraz `enum`)**:
  - `==`, `!=`, `>=`, `<=`, `>`, `<`
  - Przykład: `{{zaufanie >= 2}}`, `{{naboje > 0}}`, `{{frakcja == rzeczni}}`, `{{frakcja != zakon}}`
- **Łączenie warunków i grupowanie**:
  - `&&` (oraz), `||` (lub), `(...)` (priorytet)
  - Przykład: `{{latarka && (klucz_sluzy || zaufanie >= 3)}}`

### Modyfikacje stanu (efekty)
Kilka operacji w jednym bloku `{{ ... }}` oddzielamy przecinkiem `,`:
- **Skróty dla `bool` i `int`**:
  - `+latarka` → ustawia `latarka = true` (dla `bool`) lub zwiększa o `1` (dla `int`: `+zaufanie`)
  - `-latarka` → ustawia `latarka = false` (dla `bool`) lub zmniejsza o `1` (dla `int`: `-naboje`)
- **Jawne przypisania i arytmetyka**:
  - `=` (przypisanie dla `bool`, `int`, `enum`): `frakcja = rzeczni`, `naboje = 6`, `latarka = true`
  - `+=`, `-=` (zmiana o wartość dla `int`): `zaufanie += 2`, `naboje -= 1`

---

## 4. Składnia w `story.txt` / `<rozdzial>.txt`

### 4.1. Efekty wejścia do sceny
Umieszczone bezpośrednio pod nagłówkiem sceny (przed prozą) wykonują się automatycznie w momencie wejścia gracza do sceny:
```txt
scene: Zatopiona śluza
id: zatopiona_sluza
{{+odwiedzono_sluze, zaufanie += 1, frakcja = rzeczni}}
```

### 4.2. Interpolacja i warunki wewnątrz zdania (Inline Prose)
- **Wstawienie wartości zmiennej**: `{{nazwa_zmiennej}}`
- **Tekst warunkowy w zdaniu (`warunek: tekst_gdy_prawda | tekst_gdy_fałsz`)**:
  Część `| tekst_gdy_fałsz` jest opcjonalna.
```txt
Strażnik patrzy na ciebie {{zaufanie >= 2: z szacunkiem | podejrzliwie}}.
W magazynku zostało ci {{naboje}} naboi{{naboje == 0: — broń jest pusta}}.
```

### 4.3. Wieloakapitowe bloki warunkowe
Otwierane przez `{{warunek}}` w osobnej linii, opcjonalnie przedzielone `{{:}}` (`else`), zamykane przez `{{/}}`:
```txt
{{latarka && (klucz_sluzy || zaufanie >= 2)}}
Schodzisz po śliskich stopniach pod samą kratę.

Snop światła przebija mętną wodę i odsłania zatopioną skrzynię.
{{:}}
W całkowitej ciemności słyszysz tylko plusk wody pod stopami.
{{/}}
```

### 4.4. Wybory (warunki przed etykietą, efekty po etykiecie)
- `{{warunek}}` przed tekstem wyboru: jeśli warunek nie jest spełniony, wybór jest **ukryty**.
- `{{? warunek}}` przed tekstem wyboru: jeśli warunek nie jest spełniony, wybór jest **widoczny jako zablokowany (wyszarzony)** w odtwarzaczu.
- `{{efekty}}` po tekście wyboru (przed `->`): wykonują się w chwili kliknięcia wyboru przez gracza.

```txt
{{latarka && naboje > 0}} Przestrzel zardzewiałą kłódkę {{-naboje, +klucz_sluzy}} -> korytarz
{{? klucz_sluzy}} Otwórz właz kluczem {{zaufanie += 2}} -> sluzownia
Wycofaj się w trzciny {{-zaufanie}} -> ucieczka_przez_szuwary
```

### 4.5. Automatyczne rozwidlenia `Dalej`
Gdy scena ma rozwidlić dalszy bieg historii bez pytania gracza o decyzję:
```txt
{{zaufanie >= 3}} Dalej -> osada_sojusznik
{{zaufanie < 3}} Dalej -> zasadzka_w_szuwarach
```

---

## 5. Minimalistyczny Interfejs dla Pisarza (`designs/variables-panel.png`)

1. **Panel Zmiennych (`#editor-variables-panel`)**:
   - Trzy proste listy bez technicznych selektorów typów: **Flagi (Tak / Nie)** (Toggle `Tak`/`Nie`), **Liczniki** (liczba na linii `1px`) oraz **Stany (Warianty)** (lista wariantów po przecinku).
   - Przyciski `+ Flaga`, `+ Licznik`, `+ Stan`.
2. **Inspektor Sceny (`#editor-context-panel`) — scalony potrójny input wyboru + pomoc `[?]`**:
   - W nagłówku sceny pole `wejście:` z szarą składnią `{{ ... }}` (np. `{{ +odwiedzono_sluze, zaufanie += 1 }}`).
   - Przy wyborze kliknięcie `{}` (aktywny wskaźnik 4 narożników `[ ]`) zamienia pojedynczy input wyboru w **scalony w pionie 3-wierszowy blok** spięty z lewej strony klamrą (bez etykiet `gdy:` / `zmień:`):
     - **Górny wiersz (warunek)**: szare nawiasy `{{ ... }}` (np. `{{ latarka && zaufanie >= 2 }}`).
     - **Środkowy wiersz (etykieta wyboru)**: główny biały tekst wyboru.
     - **Dolny wiersz (wynik / zmiana)**: szare nawiasy `{{ ... }}` (np. `{{ -naboje, +klucz_uzyty }}`).
   - Gdy `{}` jest włączone, obok pojawia się ikona `[?]` otwierająca zwięzły popup ze ściągawką składni `{{ ... }}`.
3. **Płótno Node Studio (diamentowy port + linia ciągła)**:
   - Wybór z warunkiem ma **diamentowy port wyjściowy `◆`** i zwykłą **ciągłą linię drutu** (linia przerywana nie jest używana, bo służy już do innych połączeń).
4. **Edytor Treści Sceny (`#modal-scene-text`) — 3 stany i 3 ikonki podglądu**:
   - **Normalnie (`#btn-scene-preview-normal`)**: edycja treści sceny.
   - **Podświetlony stan (`#btn-scene-preview-highlight`)**: pasek **Stan testowy** do przełączania zmiennych na żywo; składnia `{{ ... }}` jest wyświetlana na szaro, aktywne gałęzie warunkowe są jasne z lewym wskaźnikiem, a nieaktywne gałęzie i wybory są przygaszone.
   - **Tylko wynikowy tekst (`#btn-scene-preview-output`)**: pasek **Stan testowy** oraz czysty, wyrenderowany tekst i odfiltrowane wybory dokładnie tak, jak widzi je gracz (bez znaczników `{{ ... }}`).
