package main

import (
	"archive/zip"
	"bytes"
	_ "embed"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

const (
	tmpExt               = ".tmp"
	codesignBinName      = "codesign"
	codesignFlagForce    = "--force"
	codesignFlagSign     = "--sign"
	codesignAdhocIdent   = "-"
	codesignFlagEntitle  = "--entitlements"
	entitlementsTempGlob = "mojra-entitlements-*.plist"

	macOSAppBundleName = "Mojra.app"
	macOSContentsDir   = "Contents"
	macOSExecDir       = "MacOS"
	macOSExecName      = "Mojra"
	macOSInfoPlistFile = "Info.plist"
	macOSIconFile      = "AppIcon.icns"
	macOSZipFile       = "Mojra-macos.zip"

	winBundleDirName = "Mojra-windows"
	winIconFile      = "Mojra.ico"
	winLauncherFile  = "Uruchom Mojra.bat"
	winZipFile       = "Mojra-windows.zip"

	linuxBundleDirName = "Mojra-linux"
	linuxIconFile      = "Mojra.png"
	linuxLauncherFile  = "Uruchom Mojra.sh"
	linuxDesktopFile   = "Mojra.desktop"
	linuxZipFile       = "Mojra-linux.zip"

	icnsMagicHeader = "icns"
	icnsChunkPNG256 = "ic08"
	coffSecNameRsrc = ".rsrc"

	errCopyWinIconFmt  = "błąd kopiowania ikony Windows (.ico): %w"
	errZipWinBundleFmt = "błąd tworzenia archiwum ZIP dla Windows: %w"
	errCopyLinIconFmt  = "błąd kopiowania ikony Linux (.png): %w"
	errZipLinBundleFmt = "błąd tworzenia archiwum ZIP dla Linux: %w"
	errInvalidICOFmt   = "nieprawidłowy plik ICO (%d bajtów)"
	errUnsupportedSYSO = "nieobsługiwana architektura PE .syso: %s"

	msgCreatedMacOSApp  = "✓ Utworzono natywną aplikację macOS: %s\n"
	msgCreatedMacOSZip  = "✓ Utworzono archiwum dystrybucyjne macOS: %s\n"
	msgCreatedWinExe    = "✓ Utworzono aplikację Windows z ikoną: %s\n"
	msgCreatedWinIcon   = "✓ Utworzono ikonę aplikacji Windows: %s\n"
	msgCreatedWinZip    = "✓ Utworzono archiwum dystrybucyjne Windows: %s\n"
	msgCreatedLinuxBin  = "✓ Utworzono aplikację Linux: %s\n"
	msgCreatedLinuxIcon = "✓ Utworzono ikonę aplikacji Linux: %s\n"
	msgCreatedLinuxZip  = "✓ Utworzono archiwum dystrybucyjne Linux: %s\n"
)

var (
	//go:embed assets/entitlements.plist
	macOSEntitlementsPlist string

	//go:embed assets/Info.plist
	macOSInfoPlist string

	//go:embed assets/launcher.bat
	windowsLauncherBat string

	//go:embed assets/launcher.sh
	linuxLauncherSh string

	//go:embed assets/Mojra.desktop
	linuxDesktopEntry string

	//go:embed assets/app.manifest
	windowsAppManifestXML string
)

type coffArchInfo struct {
	machine   uint16
	relocType uint16
}

var coffArchTable = map[string]coffArchInfo{
	"":        {machine: 0x8664, relocType: 0x0003},
	archAMD64: {machine: 0x8664, relocType: 0x0003},
	archARM64: {machine: 0xAA64, relocType: 0x0002},
	arch386:   {machine: 0x014c, relocType: 0x0007},
}

type winIconFrame struct {
	width      byte
	height     byte
	colorCount byte
	reserved   byte
	planes     uint16
	bitCount   uint16
	data       []byte
}

func copyFileWithMode(src, dst string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	_ = os.Remove(dst)
	tmpDst := dst + tmpExt
	_ = os.Remove(tmpDst)
	out, err := os.OpenFile(tmpDst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		_ = os.Remove(tmpDst)
		return err
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(tmpDst)
		return err
	}
	return os.Rename(tmpDst, dst)
}

func adhocSignMacOSPath(targetPath string) {
	if runtime.GOOS != osDarwin {
		return
	}
	if codesignPath, err := exec.LookPath(codesignBinName); err == nil && codesignPath != "" {
		tmpEnt, tmpErr := os.CreateTemp("", entitlementsTempGlob)
		if tmpErr == nil {
			entPath := tmpEnt.Name()
			_, _ = tmpEnt.WriteString(macOSEntitlementsPlist)
			_ = tmpEnt.Close()
			defer os.Remove(entPath)
			_ = exec.Command(codesignPath, codesignFlagForce, codesignFlagSign, codesignAdhocIdent, codesignFlagEntitle, entPath, targetPath).Run()
			return
		}
		_ = exec.Command(codesignPath, codesignFlagForce, codesignFlagSign, codesignAdhocIdent, targetPath).Run()
	}
}

func writeMacOSAppIconICNS(srcPNGPath, outPath string) error {
	pngData, err := os.ReadFile(srcPNGPath)
	if err != nil {
		return err
	}
	var icnsBuf bytes.Buffer
	icnsBuf.WriteString(icnsMagicHeader)
	_ = binary.Write(&icnsBuf, binary.BigEndian, uint32(16+len(pngData)))
	icnsBuf.WriteString(icnsChunkPNG256)
	_ = binary.Write(&icnsBuf, binary.BigEndian, uint32(8+len(pngData)))
	icnsBuf.Write(pngData)
	return os.WriteFile(outPath, icnsBuf.Bytes(), 0644)
}

func parseICOFrames(icoBytes []byte) ([]winIconFrame, error) {
	if len(icoBytes) < 6 {
		return nil, fmt.Errorf(errInvalidICOFmt, len(icoBytes))
	}
	reserved := binary.LittleEndian.Uint16(icoBytes[0:2])
	icoType := binary.LittleEndian.Uint16(icoBytes[2:4])
	count := int(binary.LittleEndian.Uint16(icoBytes[4:6]))
	if reserved != 0 || icoType != 1 || count <= 0 || len(icoBytes) < 6+16*count {
		return nil, fmt.Errorf(errInvalidICOFmt, len(icoBytes))
	}
	frames := make([]winIconFrame, 0, count)
	for i := 0; i < count; i++ {
		ent := icoBytes[6+16*i : 6+16*(i+1)]
		size := binary.LittleEndian.Uint32(ent[8:12])
		offset := binary.LittleEndian.Uint32(ent[12:16])
		if size == 0 || uint64(offset)+uint64(size) > uint64(len(icoBytes)) {
			return nil, fmt.Errorf(errInvalidICOFmt, len(icoBytes))
		}
		frames = append(frames, winIconFrame{
			width:      ent[0],
			height:     ent[1],
			colorCount: ent[2],
			reserved:   ent[3],
			planes:     binary.LittleEndian.Uint16(ent[4:6]),
			bitCount:   binary.LittleEndian.Uint16(ent[6:8]),
			data:       icoBytes[offset : offset+size],
		})
	}
	return frames, nil
}

func writeWindowsResourceSYSOForArch(icoPath, outPath, goarch string) error {
	archInfo, ok := coffArchTable[goarch]
	if !ok {
		return fmt.Errorf(errUnsupportedSYSO, goarch)
	}
	icoBytes, err := os.ReadFile(icoPath)
	if err != nil {
		return err
	}
	frames, err := parseICOFrames(icoBytes)
	if err != nil {
		return err
	}
	nIcons := len(frames)

	var grpBuf bytes.Buffer
	_ = binary.Write(&grpBuf, binary.LittleEndian, uint16(0))
	_ = binary.Write(&grpBuf, binary.LittleEndian, uint16(1))
	_ = binary.Write(&grpBuf, binary.LittleEndian, uint16(nIcons))
	for i, fr := range frames {
		grpBuf.WriteByte(fr.width)
		grpBuf.WriteByte(fr.height)
		grpBuf.WriteByte(fr.colorCount)
		grpBuf.WriteByte(fr.reserved)
		_ = binary.Write(&grpBuf, binary.LittleEndian, fr.planes)
		_ = binary.Write(&grpBuf, binary.LittleEndian, fr.bitCount)
		_ = binary.Write(&grpBuf, binary.LittleEndian, uint32(len(fr.data)))
		_ = binary.Write(&grpBuf, binary.LittleEndian, uint16(i+1))
	}
	grpData := grpBuf.Bytes()
	manifestData := []byte(windowsAppManifestXML)

	iconL2Off := 40
	grpL2Off := iconL2Off + 16 + 8*nIcons
	manL2Off := grpL2Off + 24
	iconL3Base := manL2Off + 24
	grpL3Off := iconL3Base + 24*nIcons
	manL3Off := grpL3Off + 24
	dataEntriesOff := manL3Off + 24
	totalDataEntries := nIcons + 2
	payloadStart := dataEntriesOff + 16*totalDataEntries

	curOff := payloadStart
	frameOffsets := make([]int, nIcons)
	for i, fr := range frames {
		frameOffsets[i] = curOff
		curOff += (len(fr.data) + 7) &^ 7
	}
	grpDataOffset := curOff
	curOff += (len(grpData) + 7) &^ 7
	manDataOffset := curOff
	rsrcSize := manDataOffset + ((len(manifestData) + 7) &^ 7)

	rsrc := make([]byte, rsrcSize)
	putDir := func(off int, idEntries uint16) {
		binary.LittleEndian.PutUint16(rsrc[off+14:], idEntries)
	}
	putEntry := func(off int, id uint32, target uint32, isSubdir bool) {
		binary.LittleEndian.PutUint32(rsrc[off:], id)
		if isSubdir {
			target |= 0x80000000
		}
		binary.LittleEndian.PutUint32(rsrc[off+4:], target)
	}

	putDir(0, 3)
	putEntry(16, 3, uint32(iconL2Off), true)
	putEntry(24, 14, uint32(grpL2Off), true)
	putEntry(32, 24, uint32(manL2Off), true)

	putDir(iconL2Off, uint16(nIcons))
	for i := 0; i < nIcons; i++ {
		putEntry(iconL2Off+16+8*i, uint32(i+1), uint32(iconL3Base+24*i), true)
	}
	putDir(grpL2Off, 1)
	putEntry(grpL2Off+16, 1, uint32(grpL3Off), true)
	putDir(manL2Off, 1)
	putEntry(manL2Off+16, 1, uint32(manL3Off), true)

	for i, fr := range frames {
		l3 := iconL3Base + 24*i
		de := dataEntriesOff + 16*i
		putDir(l3, 1)
		putEntry(l3+16, 0x0409, uint32(de), false)
		binary.LittleEndian.PutUint32(rsrc[de:], uint32(frameOffsets[i]))
		binary.LittleEndian.PutUint32(rsrc[de+4:], uint32(len(fr.data)))
		copy(rsrc[frameOffsets[i]:], fr.data)
	}

	grpDE := dataEntriesOff + 16*nIcons
	putDir(grpL3Off, 1)
	putEntry(grpL3Off+16, 0x0409, uint32(grpDE), false)
	binary.LittleEndian.PutUint32(rsrc[grpDE:], uint32(grpDataOffset))
	binary.LittleEndian.PutUint32(rsrc[grpDE+4:], uint32(len(grpData)))
	copy(rsrc[grpDataOffset:], grpData)

	manDE := dataEntriesOff + 16*(nIcons+1)
	putDir(manL3Off, 1)
	putEntry(manL3Off+16, 0x0409, uint32(manDE), false)
	binary.LittleEndian.PutUint32(rsrc[manDE:], uint32(manDataOffset))
	binary.LittleEndian.PutUint32(rsrc[manDE+4:], uint32(len(manifestData)))
	copy(rsrc[manDataOffset:], manifestData)

	rawOffset := uint32(60)
	relocOffset := rawOffset + uint32(rsrcSize)
	symOffset := relocOffset + uint32(10*totalDataEntries)

	var coff bytes.Buffer
	_ = binary.Write(&coff, binary.LittleEndian, archInfo.machine)
	_ = binary.Write(&coff, binary.LittleEndian, uint16(1))
	_ = binary.Write(&coff, binary.LittleEndian, uint32(0))
	_ = binary.Write(&coff, binary.LittleEndian, symOffset)
	_ = binary.Write(&coff, binary.LittleEndian, uint32(1))
	_ = binary.Write(&coff, binary.LittleEndian, uint16(0))
	_ = binary.Write(&coff, binary.LittleEndian, uint16(0))

	var secName [8]byte
	copy(secName[:], coffSecNameRsrc)
	coff.Write(secName[:])
	_ = binary.Write(&coff, binary.LittleEndian, uint32(rsrcSize))
	_ = binary.Write(&coff, binary.LittleEndian, uint32(0))
	_ = binary.Write(&coff, binary.LittleEndian, uint32(rsrcSize))
	_ = binary.Write(&coff, binary.LittleEndian, rawOffset)
	_ = binary.Write(&coff, binary.LittleEndian, relocOffset)
	_ = binary.Write(&coff, binary.LittleEndian, uint32(0))
	_ = binary.Write(&coff, binary.LittleEndian, uint16(totalDataEntries))
	_ = binary.Write(&coff, binary.LittleEndian, uint16(0))
	_ = binary.Write(&coff, binary.LittleEndian, uint32(0x40000040))

	coff.Write(rsrc)

	for i := 0; i < totalDataEntries; i++ {
		de := uint32(dataEntriesOff + 16*i)
		_ = binary.Write(&coff, binary.LittleEndian, de)
		_ = binary.Write(&coff, binary.LittleEndian, uint32(0))
		_ = binary.Write(&coff, binary.LittleEndian, archInfo.relocType)
	}

	coff.Write(secName[:])
	_ = binary.Write(&coff, binary.LittleEndian, uint32(0))
	_ = binary.Write(&coff, binary.LittleEndian, int16(1))
	_ = binary.Write(&coff, binary.LittleEndian, uint16(0))
	coff.WriteByte(3)
	coff.WriteByte(0)
	_ = binary.Write(&coff, binary.LittleEndian, uint32(4))

	return os.WriteFile(outPath, coff.Bytes(), 0644)
}

func packageMacOSBundle(rootDir string, archBins map[string]string) error {
	bundleRoot := filepath.Join(rootDir, distDirName, macOSAppBundleName)
	contentsDir := filepath.Join(bundleRoot, macOSContentsDir)
	macOSDir := filepath.Join(contentsDir, macOSExecDir)
	resDir := filepath.Join(contentsDir, resourcesDir)

	_ = os.RemoveAll(bundleRoot)
	_ = os.MkdirAll(macOSDir, 0755)
	_ = os.MkdirAll(resDir, 0755)

	binPath := filepath.Join(macOSDir, macOSExecName)
	arm64Bin := archBins[archARM64]
	amd64Bin := archBins[archAMD64]

	chosen := archBins[runtime.GOARCH]
	if chosen == "" {
		if arm64Bin != "" {
			chosen = arm64Bin
		} else {
			chosen = amd64Bin
		}
	}
	if chosen == "" {
		if err := buildPlatformBinary(rootDir, platformTarget{OS: osDarwin, Arch: archARM64}, binPath, buildBinaryMode{}); err != nil {
			return err
		}
	} else {
		if err := copyFileWithMode(chosen, binPath, 0755); err != nil {
			return err
		}
	}

	if runtime.GOOS == osDarwin {
		hostBin := archBins[runtime.GOARCH]
		if hostBin == "" {
			hostBin = binPath
		}
		rootBin := filepath.Join(rootDir, binMojra)
		_ = copyFileWithMode(hostBin, rootBin, 0755)
		adhocSignMacOSPath(rootBin)
	}

	if err := os.WriteFile(filepath.Join(contentsDir, macOSInfoPlistFile), []byte(macOSInfoPlist), 0644); err != nil {
		return err
	}
	srcIconPNG := filepath.Join(rootDir, appDirName, iconsDirName, iconPNGFile)
	if err := writeMacOSAppIconICNS(srcIconPNG, filepath.Join(resDir, macOSIconFile)); err != nil {
		return err
	}
	if err := packageBooksForDist(filepath.Join(rootDir, booksDirName), filepath.Join(resDir, booksDirName)); err != nil {
		return err
	}
	adhocSignMacOSPath(binPath)
	adhocSignMacOSPath(bundleRoot)

	zipPath := filepath.Join(rootDir, distDirName, macOSZipFile)
	if err := zipDirRecursive(bundleRoot, zipPath, macOSAppBundleName); err != nil {
		return err
	}

	fmt.Printf(msgCreatedMacOSApp, bundleRoot)
	fmt.Printf(msgCreatedMacOSZip, zipPath)
	return nil
}

func packageWindowsBundle(rootDir string, archBins map[string]string) error {
	booksDir := filepath.Join(rootDir, booksDirName)
	outDir := filepath.Join(rootDir, distDirName, winBundleDirName)
	_ = os.RemoveAll(outDir)
	_ = os.MkdirAll(outDir, 0755)

	srcICOPath := filepath.Join(rootDir, appDirName, iconsDirName, faviconICOFile)
	icoPath := filepath.Join(outDir, winIconFile)
	if err := copyFileWithMode(srcICOPath, icoPath, 0644); err != nil {
		return fmt.Errorf(errCopyWinIconFmt, err)
	}

	exePath := filepath.Join(outDir, binWinCap)
	primaryBin := archBins[archAMD64]
	if primaryBin == "" {
		primaryBin = archBins[archARM64]
	}
	if primaryBin != "" {
		if err := copyFileWithMode(primaryBin, exePath, 0755); err != nil {
			return err
		}
	} else {
		if err := buildPlatformBinary(rootDir, platformTarget{OS: osWindows, Arch: archAMD64}, exePath, buildBinaryMode{}); err != nil {
			return err
		}
	}

	if runtime.GOOS == osWindows {
		_ = copyFileWithMode(exePath, filepath.Join(rootDir, binWinCap), 0755)
		_ = copyFileWithMode(icoPath, filepath.Join(rootDir, winIconFile), 0644)
	}

	if err := packageBooksForDist(booksDir, filepath.Join(outDir, booksDirName)); err != nil {
		return err
	}

	if err := os.WriteFile(filepath.Join(outDir, winLauncherFile), []byte(windowsLauncherBat), 0644); err != nil {
		return err
	}

	zipPath := filepath.Join(rootDir, distDirName, winZipFile)
	if err := zipDirRecursive(outDir, zipPath, winBundleDirName); err != nil {
		return fmt.Errorf(errZipWinBundleFmt, err)
	}

	fmt.Printf(msgCreatedWinExe, exePath)
	fmt.Printf(msgCreatedWinIcon, icoPath)
	fmt.Printf(msgCreatedWinZip, zipPath)
	return nil
}

func packageLinuxBundle(rootDir string, archBins map[string]string) error {
	booksDir := filepath.Join(rootDir, booksDirName)
	outDir := filepath.Join(rootDir, distDirName, linuxBundleDirName)
	_ = os.RemoveAll(outDir)
	_ = os.MkdirAll(outDir, 0755)

	srcIconPath := filepath.Join(rootDir, appDirName, iconsDirName, iconPNGFile)
	iconPath := filepath.Join(outDir, linuxIconFile)
	if err := copyFileWithMode(srcIconPath, iconPath, 0644); err != nil {
		return fmt.Errorf(errCopyLinIconFmt, err)
	}

	binPath := filepath.Join(outDir, binMojra)
	primaryBin := archBins[archAMD64]
	if primaryBin == "" {
		primaryBin = archBins[archARM64]
	}
	if primaryBin != "" {
		if err := copyFileWithMode(primaryBin, binPath, 0755); err != nil {
			return err
		}
	} else {
		if err := buildPlatformBinary(rootDir, platformTarget{OS: osLinux, Arch: archAMD64}, binPath, buildBinaryMode{}); err != nil {
			return err
		}
	}

	if runtime.GOOS == osLinux {
		hostBin := archBins[runtime.GOARCH]
		if hostBin == "" {
			hostBin = binPath
		}
		_ = copyFileWithMode(hostBin, filepath.Join(rootDir, binMojra), 0755)
	}

	if err := packageBooksForDist(booksDir, filepath.Join(outDir, booksDirName)); err != nil {
		return err
	}

	if err := os.WriteFile(filepath.Join(outDir, linuxLauncherFile), []byte(linuxLauncherSh), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outDir, linuxDesktopFile), []byte(linuxDesktopEntry), 0755); err != nil {
		return err
	}

	zipPath := filepath.Join(rootDir, distDirName, linuxZipFile)
	if err := zipDirRecursive(outDir, zipPath, linuxBundleDirName); err != nil {
		return fmt.Errorf(errZipLinBundleFmt, err)
	}

	fmt.Printf(msgCreatedLinuxBin, binPath)
	fmt.Printf(msgCreatedLinuxIcon, iconPath)
	fmt.Printf(msgCreatedLinuxZip, zipPath)
	return nil
}

func zipDirRecursive(srcDir, outZipPath, rootPrefix string) error {
	_ = os.Remove(outZipPath)
	zf, err := os.Create(outZipPath)
	if err != nil {
		return err
	}
	defer zf.Close()

	zw := zip.NewWriter(zf)
	err = filepath.WalkDir(srcDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		zipName := filepath.ToSlash(filepath.Join(rootPrefix, rel))
		info, err := d.Info()
		if err != nil {
			return err
		}
		hdr, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		hdr.Name = zipName
		hdr.Method = zip.Deflate
		w, err := zw.CreateHeader(hdr)
		if err != nil {
			return err
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		_, err = io.Copy(w, in)
		return err
	})
	if err != nil {
		_ = zw.Close()
		return err
	}
	return zw.Close()
}
