package main

import (
	"archive/zip"
	"bytes"
	_ "embed"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	envProgramFiles   = "ProgramFiles"
	envAndroidHome    = "ANDROID_HOME"
	envAndroidSDKRoot = "ANDROID_SDK_ROOT"
	envLocalAppData   = "LOCALAPPDATA"
	envAndroidNDKHome = "ANDROID_NDK_HOME"
	envAndroidNDKRoot = "ANDROID_NDK_ROOT"
	envNDKHome        = "NDK_HOME"
	envJavaHome       = "JAVA_HOME"
	envPath           = "PATH"
	envMojraKeystore  = "MOJRA_KEYSTORE"
	envMojraKSPass    = "MOJRA_KEYSTORE_PASS"
	envMojraKeyAlias  = "MOJRA_KEY_ALIAS"
	envMojraKeyPass   = "MOJRA_KEY_PASS"
	envJavaHomePrefix = "JAVA_HOME="
	envPathPrefix     = "PATH="

	unityMacGlobPattern   = "/Applications/Unity/Hub/Editor/*/PlaybackEngines/AndroidPlayer"
	unitySubUnity         = "Unity"
	unitySubHub           = "Hub"
	unitySubEditor        = "Editor"
	unitySubData          = "Data"
	unitySubPlayback      = "PlaybackEngines"
	unitySubAndroidPlayer = "AndroidPlayer"
	unitySubApplications  = "Applications"
	unitySubSDK           = "SDK"
	unitySubNDK           = "NDK"
	unitySubOpenJDK       = "OpenJDK"

	sdkSubAndroid     = "Android"
	sdkSubSdkCap      = "Sdk"
	sdkSubSdkLow      = "sdk"
	sdkSubLibrary     = "Library"
	sdkSubAppData     = "AppData"
	sdkSubLocal       = "Local"
	sdkOptDir         = "/opt/android-sdk"
	sdkUsrLibDir      = "/usr/lib/android-sdk"
	sdkSubNDK         = "ndk"
	sdkSubNDKBundle   = "ndk-bundle"
	sdkSubBuildTools  = "build-tools"
	sdkSubPlatforms   = "platforms"
	sdkPlatGlob       = "android-*"
	sdkAndroidJarFile = "android.jar"
	sdkSubLib         = "lib"

	ndkClangARM64    = "aarch64-linux-android24-clang"
	ndkClangAMD64    = "x86_64-linux-android24-clang"
	ndkSubToolchains = "toolchains"
	ndkSubLLVM       = "llvm"
	ndkSubPrebuilt   = "prebuilt"
	ndkSubBin        = "bin"
	extCmd           = ".cmd"
	extExe           = ".exe"
	extBat           = ".bat"

	toolAapt      = "aapt"
	toolD8        = "d8"
	toolZipalign  = "zipalign"
	toolApksigner = "apksigner"
	toolJavac     = "javac"
	toolKeytool   = "keytool"

	studioSubAndroid = "Android Studio"
	studioSubJBR     = "jbr"
	studioSubJRE     = "jre"
	studioMacJBR     = "/Applications/Android Studio.app/Contents/jbr/Contents/Home"
	studioMacJRE     = "/Applications/Android Studio.app/Contents/jre/Contents/Home"
	studioOptJBR     = "/opt/android-studio/jbr"

	apkTempDirPattern   = "mojra-apk-*"
	apkResDirName       = "res"
	apkMipmapXXXHDPI    = "mipmap-xxxhdpi"
	apkLauncherPNG      = "ic_launcher.png"
	apkLauncherRoundPNG = "ic_launcher_round.png"
	apkSrcDirName       = "src"
	apkPkgPl            = "pl"
	apkPkgMojra         = "mojra"
	apkPkgApp           = "app"
	apkClassesDirName   = "classes"
	apkAssetsDirName    = "assets"
	apkManifestFile     = "AndroidManifest.xml"
	apkMainActivityFile = "MainActivity.java"
	apkRJavaFile        = "R.java"
	apkClassGlob        = "*.class"
	apkUnalignedFile    = "unaligned.apk"
	apkAlignedFile      = "aligned.apk"
	apkClassesDexFile   = "classes.dex"
	apkDebugKeystore    = "debug.keystore"
	apkDefaultPass      = "android"
	apkDefaultAlias     = "androiddebugkey"
	apkDistFile         = "Mojra-android.apk"
	apkPassPrefix       = "pass:"
	apkNativeLibPrefix  = "lib/"
	apkNativeLibSuffix  = "/libmojra.so"

	argPackage       = "package"
	argForce         = "-f"
	argGenR          = "-m"
	argFlagJ         = "-J"
	argFlagM         = "-M"
	argFlagS         = "-S"
	argFlagA         = "-A"
	argFlagI         = "-I"
	argFlagF         = "-F"
	argSource        = "-source"
	argJavaVer       = "1.8"
	argTarget        = "-target"
	argBootClasspath = "-bootclasspath"
	argFlagD         = "-d"
	argD8Lib         = "--lib"
	argD8Output      = "--output"
	argPageAlign     = "-p"
	argAlign4        = "4"
	argGenKeyPair    = "-genkeypair"
	argVerbose       = "-v"
	argKeystore      = "-keystore"
	argStorePass     = "-storepass"
	argAlias         = "-alias"
	argKeyPass       = "-keypass"
	argKeyAlg        = "-keyalg"
	argRSA           = "RSA"
	argKeySize       = "-keysize"
	argKeySize2048   = "2048"
	argValidity      = "-validity"
	argValidityDays  = "10000"
	argDName         = "-dname"
	argDebugDName    = "CN=Android Debug,O=Android,C=US"
	argSign          = "sign"
	argKS            = "--ks"
	argKSPass        = "--ks-pass"
	argKeyPassLong   = "--key-pass"
	argKSKeyAlias    = "--ks-key-alias"
	argOutLong       = "--out"

	errMissingNDKFmt      = "nie znaleziono kompilatora Android NDK (%s) — ustaw ANDROID_NDK_HOME lub zainstaluj NDK w Android SDK"
	errMissingSDK         = "nie znaleziono Android SDK (build-tools + platforms/android-*) — ustaw ANDROID_HOME lub zainstaluj Android SDK"
	errMissingJDK         = "nie znaleziono narzędzi JDK (javac / keytool) — ustaw JAVA_HOME lub zainstaluj OpenJDK"
	errToolFailedFmt      = "%s failed: %v\n%s"
	errMissingKeystorePwd = "ustawiono MOJRA_KEYSTORE, ale brakuje MOJRA_KEYSTORE_PASS"
	msgCreatedAndroidAPK  = "✓ Utworzono podpisany pakiet instalacyjny Android APK: %s\n"
)

var (
	//go:embed assets/AndroidManifest.xml
	androidManifestXML string

	//go:embed assets/MainActivity.java
	androidMainActivityJava string

	androidABIMap = map[string]string{
		archARM64: "arm64-v8a",
		archAMD64: "x86_64",
	}

	unixExeExts    = []string{""}
	unixScriptExts = []string{"", extBat, extCmd}
	winExeExts     = []string{extExe, extCmd, extBat, ""}
	winScriptExts  = []string{extBat, extCmd, extExe, ""}
)

type androidToolchain struct {
	SDKDir     string
	JDKDir     string
	AndroidJar string
	Aapt       string
	D8         string
	Zipalign   string
	Apksigner  string
	Javac      string
	Keytool    string
}

func candidateUnityAndroidPlayerDirs() []string {
	patterns := []string{
		unityMacGlobPattern,
	}
	if pf := os.Getenv(envProgramFiles); pf != "" {
		patterns = append(patterns, filepath.Join(pf, unitySubUnity, unitySubHub, unitySubEditor, "*", unitySubEditor, unitySubData, unitySubPlayback, unitySubAndroidPlayer))
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		patterns = append(patterns,
			filepath.Join(home, unitySubUnity, unitySubHub, unitySubEditor, "*", unitySubEditor, unitySubData, unitySubPlayback, unitySubAndroidPlayer),
			filepath.Join(home, unitySubApplications, unitySubUnity, unitySubHub, unitySubEditor, "*", unitySubPlayback, unitySubAndroidPlayer),
		)
	}
	var out []string
	seen := make(map[string]bool)
	for _, pat := range patterns {
		if matches, err := filepath.Glob(pat); err == nil {
			for i := len(matches) - 1; i >= 0; i-- {
				clean := filepath.Clean(matches[i])
				if !seen[clean] {
					seen[clean] = true
					out = append(out, clean)
				}
			}
		}
	}
	return out
}

func candidateAndroidSDKDirs() []string {
	var dirs []string
	addDir := func(p string) {
		p = strings.TrimSpace(p)
		if p != "" {
			dirs = append(dirs, filepath.Clean(p))
		}
	}
	addDir(os.Getenv(envAndroidHome))
	addDir(os.Getenv(envAndroidSDKRoot))
	if localAppData := os.Getenv(envLocalAppData); localAppData != "" {
		addDir(filepath.Join(localAppData, sdkSubAndroid, sdkSubSdkCap))
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		addDir(filepath.Join(home, sdkSubLibrary, sdkSubAndroid, sdkSubSdkLow))
		addDir(filepath.Join(home, sdkSubAppData, sdkSubLocal, sdkSubAndroid, sdkSubSdkCap))
		addDir(filepath.Join(home, sdkSubAndroid, sdkSubSdkCap))
		addDir(filepath.Join(home, sdkSubAndroid, sdkSubSdkLow))
	}
	addDir(sdkOptDir)
	addDir(sdkUsrLibDir)
	for _, ap := range candidateUnityAndroidPlayerDirs() {
		addDir(filepath.Join(ap, unitySubSDK))
	}
	return dirs
}

func findAndroidNDKClang(arch string) (string, error) {
	prefix := ndkClangARM64
	if arch == archAMD64 {
		prefix = ndkClangAMD64
	}
	binNames := []string{prefix}
	if runtime.GOOS == osWindows {
		binNames = []string{prefix + extCmd, prefix + extExe, prefix}
	}

	var ndkRoots []string
	addNDK := func(p string) {
		p = strings.TrimSpace(p)
		if p != "" {
			ndkRoots = append(ndkRoots, filepath.Clean(p))
		}
	}
	addNDK(os.Getenv(envAndroidNDKHome))
	addNDK(os.Getenv(envAndroidNDKRoot))
	addNDK(os.Getenv(envNDKHome))

	for _, sdkDir := range candidateAndroidSDKDirs() {
		if m, err := filepath.Glob(filepath.Join(sdkDir, sdkSubNDK, "*")); err == nil {
			for i := len(m) - 1; i >= 0; i-- {
				addNDK(m[i])
			}
		}
		addNDK(filepath.Join(sdkDir, sdkSubNDKBundle))
	}
	for _, ap := range candidateUnityAndroidPlayerDirs() {
		addNDK(filepath.Join(ap, unitySubNDK))
	}

	for _, root := range ndkRoots {
		for _, binName := range binNames {
			if m, err := filepath.Glob(filepath.Join(root, ndkSubToolchains, ndkSubLLVM, ndkSubPrebuilt, "*", ndkSubBin, binName)); err == nil {
				for i := len(m) - 1; i >= 0; i-- {
					if info, err := os.Stat(m[i]); err == nil && !info.IsDir() {
						return m[i], nil
					}
				}
			}
		}
	}
	return "", fmt.Errorf(errMissingNDKFmt, prefix)
}

func resolveAndroidToolBinary(dir, name string, exts ...string) string {
	for _, ext := range exts {
		p := filepath.Join(dir, name+ext)
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			return p
		}
	}
	return ""
}

func resolveAndroidToolchain() (androidToolchain, error) {
	var tc androidToolchain
	exeExts := unixExeExts
	scriptExts := unixScriptExts
	if runtime.GOOS == osWindows {
		exeExts = winExeExts
		scriptExts = winScriptExts
	}

	for _, sdkDir := range candidateAndroidSDKDirs() {
		btMatches, _ := filepath.Glob(filepath.Join(sdkDir, sdkSubBuildTools, "*"))
		platMatches, _ := filepath.Glob(filepath.Join(sdkDir, sdkSubPlatforms, sdkPlatGlob))
		if len(btMatches) == 0 || len(platMatches) == 0 {
			continue
		}
		btDir := btMatches[len(btMatches)-1]
		androidJar := filepath.Join(platMatches[len(platMatches)-1], sdkAndroidJarFile)
		if info, err := os.Stat(androidJar); err != nil || info.IsDir() {
			continue
		}
		aapt := resolveAndroidToolBinary(btDir, toolAapt, exeExts...)
		d8 := resolveAndroidToolBinary(btDir, toolD8, scriptExts...)
		if d8 == "" {
			d8 = resolveAndroidToolBinary(filepath.Join(btDir, sdkSubLib), toolD8, scriptExts...)
		}
		zipalign := resolveAndroidToolBinary(btDir, toolZipalign, exeExts...)
		apksigner := resolveAndroidToolBinary(btDir, toolApksigner, scriptExts...)
		if aapt != "" && d8 != "" && zipalign != "" && apksigner != "" {
			tc.SDKDir = sdkDir
			tc.AndroidJar = androidJar
			tc.Aapt = aapt
			tc.D8 = d8
			tc.Zipalign = zipalign
			tc.Apksigner = apksigner
			break
		}
	}
	if tc.SDKDir == "" {
		return tc, fmt.Errorf(errMissingSDK)
	}

	var jdkCandidates []string
	if javaHome := strings.TrimSpace(os.Getenv(envJavaHome)); javaHome != "" {
		jdkCandidates = append(jdkCandidates, filepath.Clean(javaHome))
	}
	for _, ap := range candidateUnityAndroidPlayerDirs() {
		jdkCandidates = append(jdkCandidates, filepath.Join(ap, unitySubOpenJDK))
	}
	if pf := os.Getenv(envProgramFiles); pf != "" {
		jdkCandidates = append(jdkCandidates,
			filepath.Join(pf, sdkSubAndroid, studioSubAndroid, studioSubJBR),
			filepath.Join(pf, sdkSubAndroid, studioSubAndroid, studioSubJRE),
		)
	}
	jdkCandidates = append(jdkCandidates, studioMacJBR, studioMacJRE, studioOptJBR)
	for _, jdkDir := range jdkCandidates {
		binDir := filepath.Join(jdkDir, ndkSubBin)
		javac := resolveAndroidToolBinary(binDir, toolJavac, exeExts...)
		keytool := resolveAndroidToolBinary(binDir, toolKeytool, exeExts...)
		if javac != "" && keytool != "" {
			tc.JDKDir = jdkDir
			tc.Javac = javac
			tc.Keytool = keytool
			break
		}
	}
	if tc.Javac == "" {
		if javacPath, err := exec.LookPath(toolJavac); err == nil {
			if keytoolPath, kErr := exec.LookPath(toolKeytool); kErr == nil {
				tc.Javac = javacPath
				tc.Keytool = keytoolPath
				tc.JDKDir = filepath.Dir(filepath.Dir(javacPath))
			}
		}
	}
	if tc.Javac == "" || tc.Keytool == "" {
		return tc, fmt.Errorf(errMissingJDK)
	}
	return tc, nil
}

func writeAndroidLauncherResources(srcIconPNG, resRootDir string) error {
	xxxhdpiDir := filepath.Join(resRootDir, apkMipmapXXXHDPI)
	if err := os.MkdirAll(xxxhdpiDir, 0755); err != nil {
		return err
	}
	for _, dstName := range []string{apkLauncherPNG, apkLauncherRoundPNG} {
		if err := copyFileWithMode(srcIconPNG, filepath.Join(xxxhdpiDir, dstName), 0644); err != nil {
			return err
		}
	}
	return nil
}

func packageAndroidBundle(rootDir string, archBins map[string]string) error {
	tc, err := resolveAndroidToolchain()
	if err != nil {
		return err
	}

	workDir, err := os.MkdirTemp("", apkTempDirPattern)
	if err != nil {
		return err
	}
	defer os.RemoveAll(workDir)

	srcDir := filepath.Join(workDir, apkSrcDirName, apkPkgPl, apkPkgMojra, apkPkgApp)
	classesDir := filepath.Join(workDir, apkClassesDirName)
	assetsBooksDir := filepath.Join(workDir, apkAssetsDirName, booksDirName)
	resDir := filepath.Join(workDir, apkResDirName)
	_ = os.MkdirAll(srcDir, 0755)
	_ = os.MkdirAll(classesDir, 0755)
	_ = os.MkdirAll(assetsBooksDir, 0755)

	srcIconPNG := filepath.Join(rootDir, appDirName, iconsDirName, iconPNGFile)
	if err := writeAndroidLauncherResources(srcIconPNG, resDir); err != nil {
		return err
	}
	if err := packageBooksForDist(filepath.Join(rootDir, booksDirName), assetsBooksDir); err != nil {
		return err
	}

	manifestPath := filepath.Join(workDir, apkManifestFile)
	if err := os.WriteFile(manifestPath, []byte(androidManifestXML), 0644); err != nil {
		return err
	}

	javaPath := filepath.Join(srcDir, apkMainActivityFile)
	if err := os.WriteFile(javaPath, []byte(androidMainActivityJava), 0644); err != nil {
		return err
	}

	pathSep := string(os.PathListSeparator)
	envWithJava := append(os.Environ(),
		envJavaHomePrefix+tc.JDKDir,
		envPathPrefix+filepath.Join(tc.JDKDir, ndkSubBin)+pathSep+os.Getenv(envPath),
	)
	runTool := func(bin string, toolArgs ...string) error {
		c := exec.Command(bin, toolArgs...)
		c.Dir = workDir
		c.Env = envWithJava
		out, err := c.CombinedOutput()
		if err != nil {
			return fmt.Errorf(errToolFailedFmt, filepath.Base(bin), err, string(out))
		}
		return nil
	}

	if err := runTool(tc.Aapt, argPackage, argForce, argGenR, argFlagJ, filepath.Join(workDir, apkSrcDirName), argFlagM, manifestPath, argFlagS, resDir, argFlagI, tc.AndroidJar); err != nil {
		return err
	}
	rJavaPath := filepath.Join(srcDir, apkRJavaFile)
	javacArgs := []string{argSource, argJavaVer, argTarget, argJavaVer, argBootClasspath, tc.AndroidJar, argFlagD, classesDir, javaPath}
	if _, err := os.Stat(rJavaPath); err == nil {
		javacArgs = append(javacArgs, rJavaPath)
	}
	if err := runTool(tc.Javac, javacArgs...); err != nil {
		return err
	}

	classFiles, _ := filepath.Glob(filepath.Join(classesDir, apkPkgPl, apkPkgMojra, apkPkgApp, apkClassGlob))
	d8Args := append([]string{argD8Lib, tc.AndroidJar, argD8Output, workDir}, classFiles...)
	if err := runTool(tc.D8, d8Args...); err != nil {
		return err
	}

	unalignedAPK := filepath.Join(workDir, apkUnalignedFile)
	if err := runTool(tc.Aapt, argPackage, argForce, argFlagM, manifestPath, argFlagS, resDir, argFlagA, filepath.Join(workDir, apkAssetsDirName), argFlagI, tc.AndroidJar, argFlagF, unalignedAPK); err != nil {
		return err
	}

	if err := appendAndroidNativeAndDexToAPK(unalignedAPK, filepath.Join(workDir, apkClassesDexFile), archBins); err != nil {
		return err
	}

	alignedAPK := filepath.Join(workDir, apkAlignedFile)
	if err := runTool(tc.Zipalign, argForce, argPageAlign, argAlign4, unalignedAPK, alignedAPK); err != nil {
		return err
	}

	ksPath := strings.TrimSpace(os.Getenv(envMojraKeystore))
	ksPass := strings.TrimSpace(os.Getenv(envMojraKSPass))
	keyAlias := strings.TrimSpace(os.Getenv(envMojraKeyAlias))
	keyPass := strings.TrimSpace(os.Getenv(envMojraKeyPass))
	if ksPath == "" {
		ksPath = filepath.Join(workDir, apkDebugKeystore)
		ksPass = apkDefaultPass
		keyAlias = apkDefaultAlias
		keyPass = apkDefaultPass
		if err := runTool(tc.Keytool, argGenKeyPair, argVerbose, argKeystore, ksPath, argStorePass, ksPass, argAlias, keyAlias, argKeyPass, keyPass, argKeyAlg, argRSA, argKeySize, argKeySize2048, argValidity, argValidityDays, argDName, argDebugDName); err != nil {
			return err
		}
	} else {
		if ksPass == "" {
			return fmt.Errorf(errMissingKeystorePwd)
		}
		if keyPass == "" {
			keyPass = ksPass
		}
	}

	outAPK := filepath.Join(rootDir, distDirName, apkDistFile)
	signArgs := []string{argSign, argKS, ksPath, argKSPass, apkPassPrefix + ksPass, argKeyPassLong, apkPassPrefix + keyPass}
	if keyAlias != "" {
		signArgs = append(signArgs, argKSKeyAlias, keyAlias)
	}
	signArgs = append(signArgs, argOutLong, outAPK, alignedAPK)
	if err := runTool(tc.Apksigner, signArgs...); err != nil {
		return err
	}

	fmt.Printf(msgCreatedAndroidAPK, outAPK)
	return nil
}

func appendAndroidNativeAndDexToAPK(apkPath, dexPath string, archBins map[string]string) error {
	rawAPK, err := os.ReadFile(apkPath)
	if err != nil {
		return err
	}
	zr, err := zip.NewReader(bytes.NewReader(rawAPK), int64(len(rawAPK)))
	if err != nil {
		return err
	}
	var outBuf bytes.Buffer
	zw := zip.NewWriter(&outBuf)

	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			return err
		}
		hdr := f.FileHeader
		w, err := zw.CreateHeader(&hdr)
		if err != nil {
			_ = rc.Close()
			return err
		}
		_, _ = io.Copy(w, rc)
		_ = rc.Close()
	}

	if dexBytes, err := os.ReadFile(dexPath); err == nil {
		w, err := zw.Create(apkClassesDexFile)
		if err != nil {
			return err
		}
		_, _ = w.Write(dexBytes)
	}

	selectedAndroidArch := archARM64
	if archBins[archARM64] == "" && archBins[archAMD64] != "" {
		selectedAndroidArch = archAMD64
	}
	if binPath := archBins[selectedAndroidArch]; binPath != "" {
		abi := androidABIMap[selectedAndroidArch]
		if binBytes, err := os.ReadFile(binPath); err == nil {
			hdr := &zip.FileHeader{
				Name:   apkNativeLibPrefix + abi + apkNativeLibSuffix,
				Method: zip.Store,
			}
			hdr.SetMode(0755)
			w, err := zw.CreateHeader(hdr)
			if err != nil {
				return err
			}
			_, _ = w.Write(binBytes)
		}
	}

	if err := zw.Close(); err != nil {
		return err
	}
	return os.WriteFile(apkPath, outBuf.Bytes(), 0644)
}
