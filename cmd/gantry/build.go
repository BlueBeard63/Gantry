package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"flag"
	"fmt"
	"image"
	_ "image/png"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/tc-hib/winres"
)

// buildTarget is one os/arch build. OS uses gantry's names: windows,
// linux, mac (mapped to GOOS darwin).
type buildTarget struct {
	OS   string
	Arch string
}

func (t buildTarget) goos() string {
	if t.OS == "mac" {
		return "darwin"
	}
	return t.OS
}

// cmdBuild builds the frontend once, then every configured target into
// dist/<os>/<arch>/, optionally packaging installers.
func cmdBuild(args []string) error {
	fs := flag.NewFlagSet("build", flag.ExitOnError)
	console := fs.Bool("console", false, "keep the console window on Windows builds (debug)")
	targetsFlag := fs.String("targets", "", "comma list of os/arch targets (overrides gantry.json), e.g. windows/amd64,linux/amd64")
	installer := fs.Bool("installer", false, "also produce install artifacts (overrides gantry.json)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	appDir, cfg, err := findApp()
	if err != nil {
		return err
	}
	synthDir, err := writeSynth(appDir, cfg)
	if err != nil {
		return err
	}
	if err := writeRegistry(appDir); err != nil {
		return err
	}
	if err := writeIcons(appDir, cfg); err != nil {
		return err
	}

	fmt.Println("gantry: building frontend (vite)")
	vite := exec.Command(npx(), "vite", "build")
	vite.Dir = synthDir
	vite.Stdout = os.Stdout
	vite.Stderr = os.Stderr
	if err := vite.Run(); err != nil {
		return fmt.Errorf("vite build failed: %w", err)
	}

	targets, err := resolveTargets(*targetsFlag, cfg)
	if err != nil {
		return err
	}
	makeInstaller := *installer || cfg.Build.Installer
	useConsole := *console || cfg.Build.Console

	built := 0
	for _, t := range targets {
		if t.goos() == "linux" && runtime.GOOS != "linux" {
			fmt.Fprintf(os.Stderr, "gantry: skipping %s/%s - Linux builds need a Linux machine (WSL works: run gantry build there)\n", t.OS, t.Arch)
			continue
		}
		if err := buildOne(appDir, cfg, t, useConsole, makeInstaller); err != nil {
			return err
		}
		built++
	}
	if built == 0 {
		return fmt.Errorf("no targets were built")
	}
	fmt.Printf("gantry: done - %s\n", filepath.Join(appDir, "dist"))
	return nil
}

// resolveTargets parses the flag or gantry.json list; empty = the
// current machine.
func resolveTargets(flagVal string, cfg appConfig) ([]buildTarget, error) {
	raw := cfg.Build.Targets
	if flagVal != "" {
		raw = strings.Split(flagVal, ",")
	}
	if len(raw) == 0 {
		hostOS := runtime.GOOS
		if hostOS == "darwin" {
			hostOS = "mac"
		}
		return []buildTarget{{OS: hostOS, Arch: runtime.GOARCH}}, nil
	}
	var out []buildTarget
	for _, r := range raw {
		parts := strings.Split(strings.TrimSpace(strings.ToLower(r)), "/")
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return nil, fmt.Errorf("bad target %q (want os/arch, e.g. windows/amd64)", r)
		}
		os := parts[0]
		if os == "darwin" {
			os = "mac"
		}
		if os != "windows" && os != "linux" && os != "mac" {
			return nil, fmt.Errorf("unknown target os %q (windows, linux or mac)", parts[0])
		}
		out = append(out, buildTarget{OS: os, Arch: parts[1]})
	}
	return out, nil
}

func buildOne(appDir string, cfg appConfig, t buildTarget, console, installer bool) error {
	outDir := filepath.Join(appDir, "dist", t.OS, t.Arch)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	exeName := cfg.Name
	if t.OS == "windows" {
		exeName += ".exe"
		if err := writeSyso(appDir, cfg, t.Arch); err != nil {
			fmt.Fprintf(os.Stderr, "gantry: exe icon skipped: %v\n", err)
		}
	}
	outPath := filepath.Join(outDir, exeName)

	fmt.Printf("gantry: building %s/%s (go)\n", t.OS, t.Arch)
	goArgs := []string{"build", "-o", outPath}
	if t.OS == "windows" && !console {
		goArgs = append(goArgs, "-ldflags", "-H windowsgui")
	}
	goArgs = append(goArgs, ".")
	cmd := exec.Command("go", goArgs...)
	cmd.Dir = appDir
	cmd.Env = append(os.Environ(), "GOOS="+t.goos(), "GOARCH="+t.Arch)
	if t.OS == "mac" {
		// Mac has no native shell yet (browser fallback), so the build
		// is pure Go and cross-compiles from anywhere.
		cmd.Env = append(cmd.Env, "CGO_ENABLED=0")
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go build %s/%s failed: %w", t.OS, t.Arch, err)
	}

	if !installer {
		return nil
	}
	switch t.OS {
	case "windows":
		return windowsInstaller(appDir, cfg, t, outDir, exeName)
	case "linux":
		archive := filepath.Join(outDir, fmt.Sprintf("%s-linux-%s.tar.gz", cfg.Name, t.Arch))
		fmt.Printf("gantry: packaging %s\n", filepath.Base(archive))
		return tarGz(archive, outPath, cfg.Name)
	case "mac":
		archive := filepath.Join(outDir, fmt.Sprintf("%s-mac-%s.zip", cfg.Name, t.Arch))
		fmt.Printf("gantry: packaging %s\n", filepath.Base(archive))
		return zipOne(archive, outPath, cfg.Name)
	}
	return nil
}

// writeSyso embeds icons/icon.ico (or .png) into the exe as its icon
// resource by generating rsrc_windows_<arch>.syso, which go build
// picks up automatically. The file is gitignored and regenerated.
func writeSyso(appDir string, cfg appConfig, arch string) error {
	dir := cfg.Icons
	if dir == "" {
		dir = "icons"
	}
	icoPath := filepath.Join(appDir, dir, "icon.ico")
	pngPath := filepath.Join(appDir, dir, "icon.png")

	var icon *winres.Icon
	switch {
	case fileExists(icoPath):
		f, err := os.Open(icoPath)
		if err != nil {
			return err
		}
		defer f.Close()
		icon, err = winres.LoadICO(f)
		if err != nil {
			return fmt.Errorf("reading %s: %w", icoPath, err)
		}
	case fileExists(pngPath):
		f, err := os.Open(pngPath)
		if err != nil {
			return err
		}
		defer f.Close()
		img, _, err := image.Decode(f)
		if err != nil {
			return fmt.Errorf("reading %s: %w", pngPath, err)
		}
		icon, err = winres.NewIconFromResizedImage(img, nil)
		if err != nil {
			return err
		}
	default:
		return nil // no icons: plain exe
	}

	var wrArch winres.Arch
	switch arch {
	case "amd64":
		wrArch = winres.ArchAMD64
	case "arm64":
		wrArch = winres.ArchARM64
	case "386":
		wrArch = winres.ArchI386
	default:
		return fmt.Errorf("no syso support for arch %s", arch)
	}

	rs := winres.ResourceSet{}
	if err := rs.SetIcon(winres.ID(1), icon); err != nil {
		return err
	}
	out, err := os.Create(filepath.Join(appDir, "rsrc_windows_"+arch+".syso"))
	if err != nil {
		return err
	}
	defer out.Close()
	return rs.WriteObject(out, wrArch)
}

// windowsInstaller generates an Inno Setup script and compiles it to
// <name>-setup.exe when the Inno Setup compiler (iscc) is available;
// otherwise the .iss is left ready to compile.
func windowsInstaller(appDir string, cfg appConfig, t buildTarget, outDir, exeName string) error {
	iss := fmt.Sprintf(`; Generated by gantry build - compile with Inno Setup 6 (iscc)
[Setup]
AppName=%s
AppVersion=%s
AppPublisher=%s
DefaultDirName={autopf}\%s
DefaultGroupName=%s
DisableProgramGroupPage=yes
UninstallDisplayIcon={app}\%s
OutputDir=.
OutputBaseFilename=%s-setup
ArchitecturesInstallIn64BitMode=x64compatible

[Files]
Source: "%s"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{group}\%s"; Filename: "{app}\%s"
Name: "{autodesktop}\%s"; Filename: "{app}\%s"; Tasks: desktopicon

[Tasks]
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; Flags: unchecked

[Run]
Filename: "{app}\%s"; Description: "{cm:LaunchProgram,%s}"; Flags: nowait postinstall skipifsilent
`,
		cfg.Title, cfg.Version, cfg.Title, cfg.Title, cfg.Title, exeName,
		cfg.Name, exeName, cfg.Title, exeName, cfg.Title, exeName, exeName, cfg.Title)

	issPath := filepath.Join(outDir, cfg.Name+".iss")
	if err := os.WriteFile(issPath, []byte(iss), 0o644); err != nil {
		return err
	}

	iscc := findISCC()
	if iscc == "" {
		fmt.Fprintf(os.Stderr, "gantry: Inno Setup not found - %s is ready; install Inno Setup 6 (https://jrsoftware.org/isinfo.php) to produce %s-setup.exe\n", issPath, cfg.Name)
		return nil
	}
	fmt.Printf("gantry: packaging %s-setup.exe (Inno Setup)\n", cfg.Name)
	cmd := exec.Command(iscc, "/Q", filepath.Base(issPath))
	cmd.Dir = outDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("iscc failed: %w", err)
	}
	return nil
}

func findISCC() string {
	if p, err := exec.LookPath("iscc"); err == nil {
		return p
	}
	if p, err := exec.LookPath("ISCC.exe"); err == nil {
		return p
	}
	for _, guess := range []string{
		`C:\Program Files (x86)\Inno Setup 6\ISCC.exe`,
		`C:\Program Files\Inno Setup 6\ISCC.exe`,
	} {
		if fileExists(guess) {
			return guess
		}
	}
	return ""
}

// tarGz packs one file (as name, executable) into a .tar.gz.
func tarGz(archivePath, filePath, name string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}
	out, err := os.Create(archivePath)
	if err != nil {
		return err
	}
	defer out.Close()
	gz := gzip.NewWriter(out)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(data))}); err != nil {
		return err
	}
	if _, err := tw.Write(data); err != nil {
		return err
	}
	if err := tw.Close(); err != nil {
		return err
	}
	return gz.Close()
}

// zipOne packs one file into a .zip.
func zipOne(archivePath, filePath, name string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}
	out, err := os.Create(archivePath)
	if err != nil {
		return err
	}
	defer out.Close()
	zw := zip.NewWriter(out)
	hdr := &zip.FileHeader{Name: name, Method: zip.Deflate}
	hdr.SetMode(0o755)
	w, err := zw.CreateHeader(hdr)
	if err != nil {
		return err
	}
	if _, err := w.Write(data); err != nil {
		return err
	}
	return zw.Close()
}
