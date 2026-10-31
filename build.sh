#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$ROOT_DIR"

AUTO_YES=0
for arg in "$@"; do
    case "$arg" in
        -y|--yes)
            AUTO_YES=1
            ;;
    esac
done

refresh_go_path() {
    local candidate
    if [ "${ST_DISABLE_SYSTEM_GO_FALLBACK:-0}" != "1" ]; then
        for candidate in "/usr/local/go/bin" "/opt/homebrew/bin"; do
            if [ -x "$candidate/go" ] && [[ ":$PATH:" != *":$candidate:"* ]]; then
                export PATH="$candidate:$PATH"
            fi
        done
    fi
    for candidate in \
        "$HOME/.local/go/bin" \
        "$HOME/sdk/go/bin" \
        "$HOME/go/bin"
    do
        if [ -x "$candidate/go" ] && [[ ":$PATH:" != *":$candidate:"* ]]; then
            export PATH="$candidate:$PATH"
        fi
    done
}

install_go_from_official_tarball() {
    local os_name arch_name go_os go_arch go_version url tmp_tar install_dir
    os_name="$(uname -s)"
    arch_name="$(uname -m)"

    case "$os_name" in
        Darwin) go_os="darwin" ;;
        Linux)  go_os="linux" ;;
        *)
            echo "[BŁĄD] Nieobsługiwany system operacyjny dla instalacji automatycznej: $os_name" >&2
            return 1
            ;;
    esac

    case "$arch_name" in
        x86_64|amd64)  go_arch="amd64" ;;
        arm64|aarch64) go_arch="arm64" ;;
        *)
            echo "[BŁĄD] Nieobsługiwana architektura procesora: $arch_name" >&2
            return 1
            ;;
    esac

    if ! command -v curl >/dev/null 2>&1; then
        echo "[BŁĄD] Brak narzędzia 'curl' wymaganego do pobrania pakietu Go." >&2
        return 1
    fi

    go_version="$(curl -fsSL "https://go.dev/VERSION?m=text" | head -n 1 | tr -d '\r\n')"
    if [ -z "$go_version" ]; then
        echo "[BŁĄD] Nie udało się pobrać numeru najnowszej wersji Go z https://go.dev/VERSION?m=text." >&2
        return 1
    fi

    url="https://go.dev/dl/${go_version}.${go_os}-${go_arch}.tar.gz"
    tmp_tar="$(mktemp "${TMPDIR:-/tmp}/go-install.XXXXXX.tar.gz")"
    install_dir="$HOME/.local"

    echo "[INSTALACJA] Pobieranie $url ..."
    curl -fL "$url" -o "$tmp_tar"

    mkdir -p "$install_dir"
    rm -rf "$install_dir/go"
    echo "[INSTALACJA] Rozpakowywanie Go do $install_dir/go ..."
    tar -C "$install_dir" -xzf "$tmp_tar"
    rm -f "$tmp_tar"

    export PATH="$install_dir/go/bin:$PATH"
}

install_missing_go() {
    local os_name
    os_name="$(uname -s)"

    echo ""
    echo "[INSTALACJA] Rozpoczynanie instalacji kompilatora Go..."

    if [ "$os_name" = "Darwin" ]; then
        if command -v brew >/dev/null 2>&1; then
            echo "[INSTALACJA] Instalowanie Go przy użyciu Homebrew..."
            brew install go || true
        fi
    elif [ "$os_name" = "Linux" ]; then
        local sudo_cmd=""
        if [ "$(id -u)" -ne 0 ] && command -v sudo >/dev/null 2>&1; then
            sudo_cmd="sudo"
        fi
        if command -v apt-get >/dev/null 2>&1; then
            echo "[INSTALACJA] Instalowanie zależności przy użyciu apt-get..."
            $sudo_cmd apt-get update && $sudo_cmd apt-get install -y golang-go curl tar || true
        elif command -v dnf >/dev/null 2>&1; then
            echo "[INSTALACJA] Instalowanie zależności przy użyciu dnf..."
            $sudo_cmd dnf install -y golang curl tar || true
        elif command -v pacman >/dev/null 2>&1; then
            echo "[INSTALACJA] Instalowanie zależności przy użyciu pacman..."
            $sudo_cmd pacman -Sy --noconfirm go curl tar || true
        elif command -v zypper >/dev/null 2>&1; then
            echo "[INSTALACJA] Instalowanie zależności przy użyciu zypper..."
            $sudo_cmd zypper --non-interactive install go curl tar || true
        elif command -v apk >/dev/null 2>&1; then
            echo "[INSTALACJA] Instalowanie zależności przy użyciu apk..."
            $sudo_cmd apk add --no-cache go curl tar || true
        fi
    fi

    refresh_go_path
    if ! command -v go >/dev/null 2>&1; then
        install_go_from_official_tarball
        refresh_go_path
    fi
}

refresh_go_path

if ! command -v go >/dev/null 2>&1; then
    echo "==================================================================" >&2
    echo "  [BRAK ZALEŻNOŚCI] Nie znaleziono kompilatora Go (Golang)" >&2
    echo "==================================================================" >&2
    echo "Do zbudowania aplikacji StoryTeller wymagany jest kompilator Go" >&2
    echo "oraz zależności projektu." >&2
    echo "" >&2

    if [ "$AUTO_YES" -ne 1 ]; then
        printf "Czy wyrażasz zgodę na automatyczną instalację Go i zależności? [t/N] (y/N): " >&2
        if ! read -r USER_CONFIRM; then
            USER_CONFIRM=""
        fi
        case "$USER_CONFIRM" in
            t|T|tak|TAK|y|Y|yes|YES)
                ;;
            *)
                echo "" >&2
                echo "[ANULOWANO] Instalacja zależności została odrzucona przez użytkownika." >&2
                echo "Możesz zainstalować Go ręcznie ze strony: https://go.dev/dl/" >&2
                exit 1
                ;;
        esac
    fi

    install_missing_go

    if ! command -v go >/dev/null 2>&1; then
        echo "[BŁĄD] Nie udało się automatycznie zainstalować kompilatora Go (https://go.dev/dl/)." >&2
        exit 1
    fi
fi

export GOTOOLCHAIN=auto
if ! go mod download >/dev/null 2>&1; then
    echo "[BŁĄD] Nie udało się pobrać zależności modułów Go." >&2
    go mod download
    exit 1
fi

exec go run ./cmd/storyteller build "$@"
