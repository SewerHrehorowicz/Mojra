# Plan Hostingu i Publikacji `mojra.app` (`hosting.md`)

## 1. Domena `mojra.app`
- **Rejestrator**: **Cloudflare Registrar** (`dash.cloudflare.com` → *Domain Registration* → *Register Domains*).
- **Koszt**: **$8.20 netto** za pierwszy rok, **$14.20/rok netto** odnowienie (cena hurtowa rejestru Google bez marży).
- **W cenie (0 zł)**:
  - Ukrycie danych w WHOIS (WHOIS Privacy),
  - Automatyczny certyfikat SSL/HTTPS (wymagany dla całej końcówki `.app` wpisanej na listę HSTS preload),
  - Serwery DNS i globalny CDN Cloudflare.
- **Alternatywa**: **Porkbun** ($8.75 1. rok / $14.93 odnowienie), jeśli domena ma korzystać z zewnętrznych serwerów nazw. Unikać polskich rejestratorów (`home.pl`, `nazwa.pl`), gdzie odnowienie `.app` przekracza 150–300 zł/rok.

---

## 2. Darmowy Hosting Statycznej Strony `mojra.app` (Cloudflare Pages — 0 zł)

Cloudflare Pages obsługuje prywatne repozytoria GitHub za darmo (w przeciwieństwie do darmowego GitHub Pages, które wymaga publicznego repozytorium).

### Krok 2.1. Zmiana nazwy repozytorium na GitHubie (opcjonalnie)
1. W ustawieniach repozytorium na GitHubie (*Settings → General → Repository name*) zmień nazwę z `StoryTeller` na `Mojra`.
2. Lokalnie zaktualizuj adres zdalnego repozytorium:
   ```sh
   git remote set-url origin git@github.com:SewerHrehorowicz/Mojra.git
   ```

### Krok 2.2. Osobny, czysty branch `website` na statyczną stronę
Zamiast mieszać kod Go ze zbudowanymi plikami WWW na `main`, publikacja idzie z dedykowanej gałęzi `website`:
1. Zbuduj statyczną stronę WWW (razem z zaszytym `mojra.wasm`):
   ```sh
   ./build.sh --macos
   ```
   Artefakty trafią do katalogu `dist/www/` (`index.html`, `inwestorzy.html`, `style.css`, `app.js`, `wasm_exec.js`, `mojra.wasm`, `icons/`, `fonts/`).
2. Utwórz niezależny branch `website` bez historii kodu źródłowego (lub wypchnij zawartość `dist/www/` na gałąź `website`):
   ```sh
   git worktree add /tmp/mojra-website -B website
   cp -R dist/www/* /tmp/mojra-website/
   git -C /tmp/mojra-website add -A
   git -C /tmp/mojra-website commit -m "Publish mojra.app static site"
   git -C /tmp/mojra-website push -u origin website --force
   git worktree remove /tmp/mojra-website
   ```

### Krok 2.3. Podpięcie Cloudflare Pages do gałęzi `website`
1. W panelu Cloudflare wejdź w **Compute (Workers & Pages)** → **Create** → zakładka **Pages** → **Import an existing Git repository**.
2. Wybierz repozytorium `Mojra` (`StoryTeller`).
3. Ustaw parametry budowania:
   - **Production branch**: `website`
   - **Build command**: *(zostaw puste)*
   - **Build output directory**: `/` *(domyślny katalog główny)*
4. Kliknij **Save and Deploy**.
5. W ustawieniach projektu (*Settings → Builds & deployments*) wyłącz **Preview deployments** dla pozostałych gałęzi (`main`, `screenplay`), aby commity w kodzie Go nie uruchamiały deployów.
6. W zakładce **Custom domains** → **Set up a custom domain** wpisz `mojra.app` i zatwierdź — Cloudflare automatycznie podepnie rekordy DNS i certyfikat SSL.

Od tego momentu każdy `git push origin website` automatycznie aktualizuje `https://mojra.app` w ~10 sekund.

---

## 3. Opcjonalny Serwer Synchronizacji w Go (Grafik Gabinetu / Lekkie API)

Dla prywatnego serwera synchronizacji (kilku użytkowników, wewnętrzny grafik czasu gabinetu, binarka Go lub *Pełnia* działająca w tle):

### Wariant A — Ultratani Polski VPS: **Mikr.us**
- **Mikr.us 1.0 (35 zł/rok brutto)** — 384 MB RAM, 5 GB NVMe (wystarczy z dużym zapasem pod binarkę Go zużywającą ~10–15 MB RAM i lokalny plik bazy/stanu).
- **Mikr.us 2.1 (75 zł/rok brutto)** — 1 GB RAM, 10 GB NVMe, obsługa Dockera.
- Cena odnowienia jest stała (brak podwyżek po pierwszym roku).

### Wariant B — Pełny VPS KVM (gdyby doszły większe pliki)
- **OVHcloud VPS-1 (~20 zł/mies. netto)** lub **Hetzner CX23 (~28 zł/mies. netto)** — 2 vCPU, 4 GB RAM, 40 GB NVMe.

### Wariant C — Bezserwerowo na Cloudflare (0 zł)
- **Cloudflare Workers + D1 (SQLite)**: darmowy limit (100 000 zapytań/dzień, 5 GB SQLite) w 100% pokrywa wewnętrzną synchronizację grafiku dla kilku osób bez utrzymywania Linuksa.

### Ukrycie Serwera Synchronizacji dla Świata (Cloudflare Tunnel + Zero Trust — 0 zł)
1. Na VPS-ie (np. Mikr.us) binarka Go nasłuchuje wyłącznie lokalnie (`127.0.0.1:8080`), bez żadnych otwartych portów publicznych.
2. Demon `cloudflared` (Cloudflare Tunnel) łączy lokalny port z prywatną subdomeną, np. `sync.mojra.app`.
3. W **Cloudflare Zero Trust → Access → Applications** ustawiasz regułę dostępu dla `sync.mojra.app` (np. *Allow → Emails* dla wybranych adresów lub token serwisowy `CF-Access-Client-Id` w aplikacji), dzięki czemu nikt z internetu nie ma dostępu nawet do samego endpointu synchronizacji.
