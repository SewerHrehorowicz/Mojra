@echo off
setlocal EnableExtensions
chcp 65001 >nul
cd /d "%~dp0"

set "AUTO_YES=0"
for %%A in (%*) do (
    if /I "%%~A"=="-y" set "AUTO_YES=1"
    if /I "%%~A"=="--yes" set "AUTO_YES=1"
)

:: 1. Sprawdz czy Go jest juz w PATH lub w standardowych lokalizacjach instalacji
where go >nul 2>nul
if %ERRORLEVEL% EQU 0 goto :go_ready

if exist "%ProgramFiles%\Go\bin\go.exe" (
    set "PATH=%ProgramFiles%\Go\bin;%PATH%"
    goto :go_ready
)
if exist "%LOCALAPPDATA%\Programs\Go\bin\go.exe" (
    set "PATH=%LOCALAPPDATA%\Programs\Go\bin;%PATH%"
    goto :go_ready
)
if exist "%LOCALAPPDATA%\Programs\go\bin\go.exe" (
    set "PATH=%LOCALAPPDATA%\Programs\go\bin;%PATH%"
    goto :go_ready
)
if exist "%USERPROFILE%\scoop\apps\go\current\bin\go.exe" (
    set "PATH=%USERPROFILE%\scoop\apps\go\current\bin;%PATH%"
    goto :go_ready
)
if exist "C:\Go\bin\go.exe" (
    set "PATH=C:\Go\bin;%PATH%"
    goto :go_ready
)

echo ==================================================================
echo   [BRAK ZALEZNOSCI] Nie znaleziono kompilatora Go ^(Golang^)
echo ==================================================================
echo Do zbudowania aplikacji StoryTeller wymagany jest kompilator Go
echo oraz zaleznosci projektu.
echo.

if "%AUTO_YES%"=="1" goto :install_go

set "USER_CONFIRM="
set /p "USER_CONFIRM=Czy wyrazasz zgode na automatyczna instalacje Go i zaleznosci? [T/N] (Y/N): "
if /I "%USER_CONFIRM%"=="T" goto :install_go
if /I "%USER_CONFIRM%"=="TAK" goto :install_go
if /I "%USER_CONFIRM%"=="Y" goto :install_go
if /I "%USER_CONFIRM%"=="YES" goto :install_go

echo.
echo [ANULOWANO] Instalacja zaleznosci zostala odrzucona przez uzytkownika.
echo Mozesz zainstalowac Go recznie ze strony: https://go.dev/dl/
exit /b 1

:install_go
echo.
echo [INSTALACJA] Rozpoczynanie instalacji kompilatora Go...

:: Proba 1: winget (jesli dostepny)
where winget >nul 2>nul
if %ERRORLEVEL% EQU 0 (
    echo [INSTALACJA] Instalowanie Go przy uzyciu winget...
    winget install --id GoLang.Go -e --source winget --accept-package-agreements --accept-source-agreements
)

call :refresh_go_path
where go >nul 2>nul
if %ERRORLEVEL% EQU 0 goto :go_ready

:: Proba 2: Chocolatey (jesli dostepny)
where choco >nul 2>nul
if %ERRORLEVEL% EQU 0 (
    echo [INSTALACJA] Instalowanie Go przy uzyciu Chocolatey...
    choco install golang -y
)

call :refresh_go_path
where go >nul 2>nul
if %ERRORLEVEL% EQU 0 goto :go_ready

:: Proba 3: Scoop (jesli dostepny)
where scoop >nul 2>nul
if %ERRORLEVEL% EQU 0 (
    echo [INSTALACJA] Instalowanie Go przy uzyciu Scoop...
    scoop install go
)

call :refresh_go_path
where go >nul 2>nul
if %ERRORLEVEL% EQU 0 goto :go_ready

:: Proba 4: Bezposrednie pobranie oficjalnej paczki z https://go.dev/dl/ przez PowerShell
echo [INSTALACJA] Pobieranie oficjalnego pakietu Go z https://go.dev/dl/ ...
powershell -NoProfile -ExecutionPolicy Bypass -Command ^
    "$ErrorActionPreference = 'Stop'; " ^
    "[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12; " ^
    "$arch = if ($env:PROCESSOR_ARCHITECTURE -eq 'ARM64' -or $env:PROCESSOR_ARCHITEW6432 -eq 'ARM64') { 'arm64' } else { 'amd64' }; " ^
    "$verRaw = Invoke-RestMethod -Uri 'https://go.dev/VERSION?m=text' -UseBasicParsing; " ^
    "$verText = ($verRaw -split \"`r?`n\")[0].Trim(); " ^
    "if (-not $verText) { throw 'Nie udalo sie pobrac wersji Go z go.dev' }; " ^
    "$url = \"https://go.dev/dl/$verText.windows-$arch.zip\"; " ^
    "$zipPath = Join-Path $env:TEMP \"$verText.windows-$arch.zip\"; " ^
    "$installRoot = Join-Path $env:LOCALAPPDATA 'Programs'; " ^
    "$goDir = Join-Path $installRoot 'go'; " ^
    "Write-Host \"Pobieranie $url ...\"; " ^
    "Invoke-WebRequest -Uri $url -OutFile $zipPath -UseBasicParsing; " ^
    "if (Test-Path $goDir) { Remove-Item -Path $goDir -Recurse -Force }; " ^
    "New-Item -ItemType Directory -Force -Path $installRoot | Out-Null; " ^
    "Write-Host \"Rozpakowywanie do $goDir ...\"; " ^
    "Expand-Archive -Path $zipPath -DestinationPath $installRoot -Force; " ^
    "Remove-Item -Path $zipPath -Force -ErrorAction SilentlyContinue; " ^
    "$binPath = Join-Path $goDir 'bin'; " ^
    "$userPath = [Environment]::GetEnvironmentVariable('Path', 'User'); " ^
    "if (-not $userPath) { $userPath = '' }; " ^
    "if ($userPath.Split(';') -notcontains $binPath) { " ^
    "    $newPath = if ($userPath) { \"$userPath;$binPath\" } else { $binPath }; " ^
    "    [Environment]::SetEnvironmentVariable('Path', $newPath, 'User'); " ^
    "}"

call :refresh_go_path
where go >nul 2>nul
if %ERRORLEVEL% NEQ 0 (
    echo [BLAD] Nie udalo sie automatycznie zainstalowac kompilatora Go.
    echo Pobierz i zainstaluj Go recznie ze strony: https://go.dev/dl/
    exit /b 1
)

:go_ready
set "GOTOOLCHAIN=auto"
go mod download >nul 2>nul
if %ERRORLEVEL% NEQ 0 (
    echo [BLAD] Nie udalo sie pobrac zaleznosci projektu Go.
    go mod download
    exit /b %ERRORLEVEL%
)

go run ./cmd/storyteller build %*
exit /b %ERRORLEVEL%

:refresh_go_path
if exist "%ProgramFiles%\Go\bin\go.exe" set "PATH=%ProgramFiles%\Go\bin;%PATH%"
if exist "%LOCALAPPDATA%\Programs\Go\bin\go.exe" set "PATH=%LOCALAPPDATA%\Programs\Go\bin;%PATH%"
if exist "%LOCALAPPDATA%\Programs\go\bin\go.exe" set "PATH=%LOCALAPPDATA%\Programs\go\bin;%PATH%"
if exist "%USERPROFILE%\scoop\apps\go\current\bin\go.exe" set "PATH=%USERPROFILE%\scoop\apps\go\current\bin;%PATH%"
if exist "C:\Go\bin\go.exe" set "PATH=C:\Go\bin;%PATH%"
if exist "C:\tools\go\bin\go.exe" set "PATH=C:\tools\go\bin;%PATH%"
exit /b 0
