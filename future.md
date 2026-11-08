# Mojra & Playbook Studio — Katalog Przyszłych Funkcji (`future.md`)

Dokument roboczy gromadzący koncepcje funkcji audio i produkcyjnych do analizy, dopracowania i ewaluacji w kolejnych iteracjach po domknięciu MVP.

---

## Feature 1: Wielowarstwowy Silnik Audio Sceny (Voiceover + Mastering, Ambient, SFX Timeline)

### Koncepcja: 3 Warstwy Audio
Udźwiękowienie w Mojrze działa jako jeden spójny system złożony z trzech warstw zsynchronizowanych wokół nagrania lektorskiego:

1. **Warstwa 1 — Voiceover + Auto-Mastering DSP (Główna oś czasu sceny)**:
   - Ścieżka głosu (`voice: voice/<scene_id>.flac`) stanowi tak jak dotychczas bazową warstwę i wyznacza długość oraz oś czasu (`waveform`) sceny.
   - **Auto-Mastering w Go (`internal/book/audio_flac.go`)** wykonywany bezpośrednio na próbkach PCM przed zapisem `EncodeFLACMono16`:
     - **High-Pass (`80 Hz`)**: Wycięcie dudnienia biurka i podmuchów w mikrofon.
     - **Odszumianie (Noise Gate / Spectral Subtraction)**: Tłumienie szumu tła pokoju w pauzach między zdaniami.
     - **Kompresor (`3:1`) + True-Peak Limiter (`-1.0 dBTP`) + Normalizacja (`-16 LUFS`)**: Wyrównanie głośności między szeptem a krzykiem oraz spójny poziom wszystkich scen w książce.

2. **Warstwa 2 — Ambient (Pętla rozdziału / sceny z katalogu)**:
   - Katalog pętli tła (wbudowane + dodawane przez autora własne pliki w projekcie).
   - Deklaracja `ambient: <nazwa>` działa jako wyzwalacz (trigger) w `.playtxt`:
     - Ustawiony na poziomie rozdziału (`chapter:`) gra w pętli przez cały rozdział bez restartowania przy zmianie sceny.
     - Ustawiony na poziomie sceny (`scene:`) może albo podmienić bieżący ambient (trigger zmieniający tło), albo nałożyć dodatkową warstwę środowiskową tylko na czas trwania tej sceny.
   - Automatyczny **ducking** (delikatne ściszenie pętli ambientu, gdy lektor mówi, i powrót w pauzach / na ekranie wyborów) oraz opcjonalny prosty crossfade przy zmianie tła.

3. **Warstwa 3 — SFX na Osi Czasu (Markery Eventów w stylu Unity Animator)**:
   - Bezpośrednio pod falą głosową (`waveform`) voiceoveru znajduje się pasek znaczników zdarzeń (`event timeline`).
   - Zamiast wielościeżkowego edytora DAW autor wstawia na osi czasu **piny/eventy** (jak *Animation Events* w Unity Animatorze): wskazuje punkt w czasie pod voiceoverem (np. `04.25s`) i wybiera z biblioteki co ma zagrać (`gunshot`, `laugh`, `dog_growl`, `drumroll`, własny plik).
   - Podczas odtwarzania sceny odtwarzacz śledzi czas głosu lektora i w oznaczonych milisekundach wyzwala przypisane sample SFX.
   - Zapis w `.playtxt` jako czytelne wpisy czasowe przypięte do sceny (np. `sfx: 4.25s gunshot`).

### Otwarte Pytania do Decyzji
- **Ambient (Scena vs Rozdział)**: Czy `ambient:` na scenie ma domyślnie podmieniać ambient rozdziału (z powrotem do ambientu rozdziału w kolejnej scenie), czy nakładać się jako druga jednoczesna pętla (np. rozdział = `nocny_las`, scena = `ognisko`)?
- **Fade w Ambientach**: Stały, automatyczny crossfade (np. `1.5s` bez konfiguracji w UI) czy opcjonalny parametr?
- **SFX w scenach bez Voiceoveru**: Czy w scenach czysto tekstowych (lub przed nagraniem głosu) dopuszczamy pojedynczy trigger `sfx:` odpalany na starcie sceny (`0.0s`)?

---

## Feature 2: Integracja z ElevenLabs AI TTS (Generacja Lektora + Kalkulator Kosztu + Bezstratny FLAC)

### Problem i Cel
Przy dużych projektach (np. 300+ scen) ręczne nagranie roboczego lub finalnego voiceoveru zajmuje wiele dni. Integracja z API ElevenLabs pozwoli wygenerować lektora dla pojedynczej sceny lub całej książki z pełną kontrolą zużycia znaków/kredytów, odpornością na przerwanie połączenia oraz bezpośrednim zapisem do bezstratnego formatu `.flac`.

### Architektura Techniczna
- **Zapis bezpośrednio do `.flac` bez zewnętrznych bibliotek**:
  - API ElevenLabs nie zwraca natywnego kontenera `.flac`, ale udostępnia surowy, bezstratny strumień `pcm_24000` (dostępny na wszystkich planach) oraz `pcm_44100` / `pcm_48000` (`16-bit signed Little-Endian mono PCM`).
  - Mojra pobiera strumień `pcm_24000` / `pcm_44100`, opcjonalnie przepuszcza go przez potok `Auto-Mastering DSP` (Feature 1) i w locie koduje istniejącym koderem `book.EncodeFLACMono16` (`internal/book/audio_flac.go`) wprost do `voice/<scene_id>.flac`.
- **Kalkulator Tokenów/Kredytów i Kosztu przed wysyłką**:
  - ElevenLabs rozlicza wywołania w skali `1 znak UTF-8 = 1 kredyt` (lub `0.5 kredytu` dla modeli Flash/Turbo).
  - Przed uruchomieniem generacji Mojra zlicza `utf8.RuneCountInString` czystego tekstu scen (po odfiltrowaniu znaczników wyborów i komentarzy), odpytuje stan konta (`GET /v1/user/subscription`: `character_count` / `character_limit`) i wyświetla dokładne podsumowanie:
    - Liczba scen do wygenerowania (np. `42 z 300` — tylko nowe lub zmienione),
    - Koszt w znakach/kredytach oraz pozostały limit konta po operacji.
- **Kolejka Masowa (300+ scen) Odporna na Zamknięcie Aplikacji i Utratę Sieci**:
  - **Idempotentność oparta o hash tekstu**: Każda wygenerowana scena zapisuje plik `voice/<scene_id>.flac` atomowo (`.tmp` + `os.Rename`) oraz sumę kontrolną tekstu sceny (`SHA-256` / `FNV-1a`).
  - **Wznawianie (Resume)**: Zamknięcie aplikacji lub utrata internetu w połowie (np. na `140/300` scenie) przerywa wyłącznie bieżący request HTTP. Po ponownym uruchomieniu kolejka pomija wszystkie sceny, których plik `.flac` już istnieje i zgadza się z hashem tekstu, wznawiając pracę od `141` sceny bez podwójnego naliczania kredytów.
  - **Rate-limiting i Backoff**: Pula `2–3` równoległych workerów w Go z automatycznym `Exponential Backoff` przy odpowiedziach `HTTP 429`.
- **Spójność Intonacji Między Scenami**:
  - Przekazywanie końcówki poprzedniej sceny i początku kolejnej w polach kontekstowych (`previous_text`, `next_text`) wzdłuż grafu fabuły, aby głos zachował naturalną ciągłość narracji.

### Otwarte Pytania do Decyzji
- Czy przypisywać jeden głos lektora na całą książkę, czy umożliwić mapowanie osobnych `voice_id` dla poszczególnych postaci z raportu obsady (`screenplay.go`)?
- Czy klucz API (`xi-api-key`) przechowywać wyłącznie lokalnie w `saves/settings.ini` (`[audio]`)?

---

## Feature 3: Asystent AI — Dramaturg, Linter Ciągłości i Konsultant Merytoryczny (Read-Only „Script Doctor”)

### Problem i Ochrona Zaufania Twórców
Autorzy słusznie obawiają się dwóch rzeczy: wykorzystania ich niepublikowanych tekstów do trenowania modeli AI oraz psucia stylu przez generatywne „dopisywanie” tekstu. Rozwiązaniem jest ścisłe pozycjonowanie modułu jako **zewnętrznego konsultanta dramaturgicznego (Read-Only Script Doctor)**:
1. **Zero Ghostwritingu (Ścisłe Read-Only)**: Moduł nie posiada funkcji generowania ani przepisywania prozy/dialogów i nigdy nie modyfikuje plików `.playtxt`. Zwraca wyłącznie raport analityczny, wykresy napięcia i pytania redaktorskie.
2. **Całkowity Opt-In / Wyższy Tier**: Domyślnie wyłączony w standardowym `Playbook Studio` (brak jakichkolwiek połączeń sieciowych bez jawnego włączenia modułu).
3. **Architektura Dwupoziomowa Chmurowa z Zero-Data-Retention (Brak słabych modeli lokalnych)**:
   - Lokalne modele (`Ollama` 8B–32B) nie radzą sobie z polszczyzną literacką, subtelną psychologią postaci ani oknem kontekstowym rzędu `100k–200k` tokenów dla wielowątkowego grafu.
   - Wszystkie wywołania idą wyłącznie przez komercyjne endpointy API z wymuszonym **Zero Data Retention** (gwarancja kontraktowa, że przesłany tekst nie jest zapisywany ani używany do trenowania modeli — w przeciwieństwie do darmowego konsumenckiego czatu WWW):
     - **Warstwa Szybka (System 1 — `TypeSafe Jev` / `jev-1.14.0`)**: Ultraszybki (`~200 ms`), bardzo tani (`$0.042 / 1M` tokenów wejściowych, darmowy output) model decyzyjny, który z definicji **nie potrafi generować tekstu**, a zwraca wyłącznie typowane odpowiedzi: prawdopodobieństwo `tak/nie` (`noul`), ocenę na skali (`score`) lub wybór etykiety (`choice`). Idealny do masowego skanowania 300 scen pod kątem ciągłości faktów, wykresu napięcia i dobierania ambientu.
     - **Warstwa Głęboka (System 2 — `Claude Opus/Sonnet`, `Gemini Pro`, `GPT`)**: Duży model z oknem `200k+` tokenów uruchamiany na żądanie do pełnej analizy dramaturgicznej, psychologicznej i merytorycznej całych ścieżek fabularnych.

### Przewaga nad zwykłym ChatGPT (Dlaczego osobny Tier ma sens)
Wklejenie tekstu do darmowego czatu wystarcza tylko przy krótkim, liniowym opowiadaniu. W `Playbook Studio` wbudowany moduł robi rzeczy, których zwykły czat nie potrafi:
1. **Świadomość Grafu, Ścieżek i Zmiennych (`var:` / `if:`) — Wykrywanie Dziur Ciągłości (Plot Holes)**:
   - Zwykły czat nie rozumie 300 rozgałęzionych węzłów i warunków zmiennych. Mojra automatycznie składa dla modelu konkretne możliwe ścieżki przejścia gracza i wyłapuje błędy przyczynowo-skutkowe (np. *„W ścieżce A→D→K bohater wspomina w scenie #42 postać lub przedmiot ze sceny #18, której w tej gałęzi nigdy nie odwiedził”*).
2. **Twarde Metryki Produkcyjne (`screenplay.go`) zamiast zgadywania**:
   - Moduł łączy analizę tekstu z wyliczonym co do sekundy czasem trwania scen, ułamkami stron `1/8`, balansem aktów i czasem obecności każdej postaci w konkretnej gałęzi.
3. **Porównywanie Wariantów na Osi Czasu (`history.go`)**:
   - Jednoklikowa odpowiedź na pytanie: *„Czy nowy wariant fabuły poprawił tempo II aktu i domknął wątek poboczny względem poprzedniej migawki?”*.
4. **Brak Limitów Kopiuj-Wklej i Gwarancja Nietrenowania**:
   - Cała książka/scenariusz mieści się z pełną mapą połączeń bez ręcznego dzielenia na części, a w darmowym ChatGPT wklejony tekst domyślnie zasila trening modeli.

### Architektura Techniczna: Kontekst z AST i Grafu Go (`*book.Book`)
AI nie dostaje surowego zrzutu plików tekstowych — wcześniej deterministyczny silnik Go (`internal/book/`) kompiluje z AST gotowy pakiet analityczny:
1. **Mapa Topologii i Stanów Zmiennych (BFS/DFS)**:
   - Wyliczone w Go drzewo wszystkich osiągalnych ścieżek (`start -> ... -> end`), lista krawędzi wejścia/wyjścia dla każdego węzła (`in_edges`, `out_edges`) oraz symulacja stanów zmiennych (`var:`, warunki `if:`, mutacje `set:`) w każdym punkcie grafu.
2. **Macierz Obecności Postaci i Metryk (`screenplay.go`)**:
   - Gotowa tabela: w których węzłach i gałęziach pojawia się dana postać/lokacja, ile dokładnie trwa każda scena (sekundy / strony `1/8`) i gdzie wypadają granice aktów.
3. **Szybki Audytor Scen `Jev` (System 1)**:
   - **Weryfikator Kanonu (`noul`)**: Dla każdej sceny sprawdza listę faktów z danej ścieżki (np. *„Czy ta scena zakłada znajomość faktu X lub obecność przedmiotu Y?”*) i flaguje sprzeczności z prawdopodobieństwem `0–100%`.
   - **Krzywa Napięcia i Emocji (`score 1–10`)**: Ocenia każdą scenę na skalach `napięcie`, `tempo`, `konflikt`, rysując na grafie lub w panelu wykres dramaturgiczny całej opowieści.
   - **Klasyfikator Tła (`choice`)**: Automatycznie sugeruje pasujący tag `ambient:` z katalogu dla sceny/rozdziału.
4. **Głęboki Konsultant Redaktorski (System 2 — Odpowiedź Powiązana z Węzłami AST)**:
   - Duży model otrzymuje ustrukturyzowane ścieżki z identyfikatorami `#scene_id` oraz flagi z `Jev` i zwraca pogłębione uwagi przypięte bezpośrednio do konkretnych `#scene_id` lub ścieżek (`#scena_12 -> #scena_19`).

### Zakres Analizy (4 Soczewki Redaktorskie)
1. **Konstrukcja Fabularna i Rytm (Arystoteles / Podróż Bohatera / 3 Akty)**:
   - Analiza każdej ścieżki w grafie od `start:` do `end: true` pod kątem zawiązania akcji, punktów zwrotnych (*midpoint*, *all is lost*, *climax*) i rozwiązania.
   - Wykrywanie problemów strukturalnych: zbyt długa ekspozycja bez konfliktu, spadek tempa w II akcie, *deus ex machina* lub pozorne wybory bez konsekwencji dramaturgicznych.
2. **Łuki Postaci (Character Arcs) i Wiarygodność Psychologiczna**:
   - Śledzenie motywacji, przemiany wewnętrznej i spójności głosu każdej postaci wzdłuż rozgałęzień fabuły.
   - Wyłapywanie sprzeczności psychologicznych (np. nagła zmiana postawy bohatera w scenie `#24` bez wcześniejszego bodźca w danej gałęzi wyboru).
3. **Toposy Literackie i Klisze Gatunkowe**:
   - Identyfikacja archetypów, motywów przewodnich i ostrzeganie przed niezamierzonymi kliszami gatunkowymi.
4. **Research Miejsc i Konsultacja Wiedzy Fachowej (Fact-Checking)**:
   - Weryfikacja realiów geograficznych, historycznych, topografii miast oraz terminologii branżowej (procedury śledcze/policyjne, medycyna, żargon rzeczny/morski, prawo, broń) z listą konkretnych nieścisłości.

### Prezentacja w UI (`320px` Panel Redaktorski)
- Wyniki pojawiają się jako lista zwięzłych kart uwag w prawym panelu bocznym (`320px`) z podziałem na kategorie (`Struktura`, `Psychologia`, `Toposy`, `Wiedza fachowa`).
- Każda uwaga zawiera klikalny odnośnik do węzła sceny (`SelectStudioScene`), który centruje kamerę grafu WebGL na problematycznym miejscu bez ingerencji w sam edytor tekstu.
