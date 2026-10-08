package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

var releaseDownloadURL = "https://github.com/kazerlelutin/htb/releases/latest/download"

type updateAsset struct {
	archive string
	binary  string
	zip     bool
}

func updateCommand() error {
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate the installed CLI: %w", err)
	}

	fmt.Fprintln(os.Stderr, "Downloading the latest HTB CLI…")
	deferred, err := installLatestUpdate(runtimeGOOS(), runtimeGOARCH(), executable, http.DefaultClient)
	if err != nil {
		return err
	}
	if deferred {
		fmt.Fprintln(os.Stdout, "The update will replace htb.exe as this command exits.")
		return nil
	}
	fmt.Fprintln(os.Stdout, "HTB CLI updated.")
	return nil
}

// These variables keep platform selection independently testable.
var (
	runtimeGOOS   = func() string { return runtime.GOOS }
	runtimeGOARCH = func() string { return runtime.GOARCH }
)

func updateAssetForPlatform(goos, goarch string) (updateAsset, error) {
	binary := "htb"
	if goos == "windows" {
		binary = "htb.exe"
	}

	switch goos {
	case "linux":
		if goarch == "amd64" {
			return updateAsset{archive: "htb_linux_amd64.tar.gz", binary: binary}, nil
		}
	case "darwin":
		if goarch == "amd64" || goarch == "arm64" {
			return updateAsset{archive: "htb_darwin_" + goarch + ".tar.gz", binary: binary}, nil
		}
	case "windows":
		if goarch == "amd64" {
			return updateAsset{archive: "htb_windows_amd64.zip", binary: binary, zip: true}, nil
		}
	}

	return updateAsset{}, fmt.Errorf("automatic updates are unavailable for %s/%s; download a supported release from https://github.com/kazerlelutin/htb/releases", goos, goarch)
}

func installLatestUpdate(goos, goarch, executable string, client *http.Client) (deferred bool, err error) {
	asset, err := updateAssetForPlatform(goos, goarch)
	if err != nil {
		return false, err
	}
	if client == nil {
		return false, errors.New("no HTTP client is available for the update")
	}

	checksums, err := downloadReleaseFile(client, "checksums.txt")
	if err != nil {
		return false, err
	}
	want, err := releaseChecksum(checksums, asset.archive)
	if err != nil {
		return false, err
	}

	workingDir, err := os.MkdirTemp("", "htb-update-")
	if err != nil {
		return false, fmt.Errorf("create update directory: %w", err)
	}
	removeWorkingDir := true
	defer func() {
		if removeWorkingDir {
			_ = os.RemoveAll(workingDir)
		}
	}()

	archivePath := filepath.Join(workingDir, asset.archive)
	if err := downloadReleaseArchive(client, asset.archive, archivePath); err != nil {
		return false, err
	}
	if err := verifyFileChecksum(archivePath, want); err != nil {
		return false, err
	}

	updatedBinary := filepath.Join(workingDir, asset.binary)
	if err := extractUpdateBinary(archivePath, asset, updatedBinary); err != nil {
		return false, err
	}

	if goos == "windows" {
		if err := launchWindowsReplacement(executable, updatedBinary, workingDir); err != nil {
			return false, err
		}
		removeWorkingDir = false
		return true, nil
	}
	if err := replaceExecutable(executable, updatedBinary); err != nil {
		return false, err
	}
	return false, nil
}

func downloadReleaseFile(client *http.Client, name string) ([]byte, error) {
	response, err := client.Get(releaseDownloadURL + "/" + name)
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", name, err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download %s: release returned %s", name, response.Status)
	}
	data, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", name, err)
	}
	return data, nil
}

func downloadReleaseArchive(client *http.Client, name, destination string) error {
	response, err := client.Get(releaseDownloadURL + "/" + name)
	if err != nil {
		return fmt.Errorf("download %s: %w", name, err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: release returned %s", name, response.Status)
	}
	file, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return fmt.Errorf("create update archive: %w", err)
	}
	_, copyErr := io.Copy(file, response.Body)
	closeErr := file.Close()
	if copyErr != nil {
		return fmt.Errorf("save %s: %w", name, copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("save %s: %w", name, closeErr)
	}
	return nil
}

func releaseChecksum(checksums []byte, name string) (string, error) {
	for _, line := range strings.Split(string(checksums), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 || strings.TrimPrefix(fields[1], "*") != name {
			continue
		}
		if len(fields[0]) != sha256.Size*2 {
			break
		}
		return strings.ToLower(fields[0]), nil
	}
	return "", fmt.Errorf("no SHA-256 checksum was published for %s", name)
}

func verifyFileChecksum(path, want string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open update archive: %w", err)
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return fmt.Errorf("hash update archive: %w", err)
	}
	if got := fmt.Sprintf("%x", hash.Sum(nil)); !strings.EqualFold(got, want) {
		return errors.New("checksum verification failed; HTB was not updated")
	}
	return nil
}

func extractUpdateBinary(archivePath string, asset updateAsset, destination string) error {
	if asset.zip {
		return extractZipBinary(archivePath, asset.binary, destination)
	}
	return extractTarBinary(archivePath, asset.binary, destination)
}

func extractTarBinary(archivePath, binary, destination string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("open update archive: %w", err)
	}
	defer file.Close()
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("read update archive: %w", err)
	}
	defer gzipReader.Close()

	reader := tar.NewReader(gzipReader)
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("read update archive: %w", err)
		}
		if header.Name != binary && header.Name != "./"+binary {
			continue
		}
		if header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeRegA {
			return errors.New("release archive contains an invalid CLI binary")
		}
		return writeUpdateBinary(destination, reader, os.FileMode(header.Mode))
	}
	return errors.New("release archive does not contain the HTB CLI")
}

func extractZipBinary(archivePath, binary, destination string) error {
	archive, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("read update archive: %w", err)
	}
	defer archive.Close()
	for _, file := range archive.File {
		if file.Name != binary && file.Name != "./"+binary {
			continue
		}
		if file.FileInfo().IsDir() {
			return errors.New("release archive contains an invalid CLI binary")
		}
		reader, err := file.Open()
		if err != nil {
			return fmt.Errorf("read update binary: %w", err)
		}
		err = writeUpdateBinary(destination, reader, file.Mode())
		closeErr := reader.Close()
		if err != nil {
			return err
		}
		return closeErr
	}
	return errors.New("release archive does not contain the HTB CLI")
}

func writeUpdateBinary(destination string, source io.Reader, mode os.FileMode) error {
	file, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0700)
	if err != nil {
		return fmt.Errorf("create updated CLI: %w", err)
	}
	_, copyErr := io.Copy(file, source)
	if copyErr == nil && mode.Perm() != 0 {
		copyErr = file.Chmod(mode.Perm())
	}
	closeErr := file.Close()
	if copyErr != nil {
		return fmt.Errorf("extract updated CLI: %w", copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("extract updated CLI: %w", closeErr)
	}
	return nil
}

func replaceExecutable(destination, source string) error {
	info, err := os.Stat(destination)
	if err != nil {
		return fmt.Errorf("inspect installed CLI: %w", err)
	}

	temporary, err := os.CreateTemp(filepath.Dir(destination), ".htb-update-")
	if err != nil {
		return fmt.Errorf("prepare updated CLI: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	reader, err := os.Open(source)
	if err != nil {
		_ = temporary.Close()
		return fmt.Errorf("open updated CLI: %w", err)
	}
	_, copyErr := io.Copy(temporary, reader)
	reader.Close()
	if copyErr == nil {
		copyErr = temporary.Chmod(info.Mode().Perm())
	}
	closeErr := temporary.Close()
	if copyErr != nil {
		return fmt.Errorf("prepare updated CLI: %w", copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("prepare updated CLI: %w", closeErr)
	}
	if err := os.Rename(temporaryPath, destination); err != nil {
		return fmt.Errorf("replace installed CLI: %w", err)
	}
	return nil
}

func launchWindowsReplacement(destination, source, workingDir string) error {
	command := fmt.Sprintf(`timeout /t 2 /nobreak > NUL & move /Y "%s" "%s" > NUL && rmdir /S /Q "%s"`, cmdPath(source), cmdPath(destination), cmdPath(workingDir))
	if err := exec.Command("cmd", "/c", command).Start(); err != nil {
		return fmt.Errorf("schedule Windows update: %w", err)
	}
	return nil
}

func cmdPath(path string) string {
	return strings.ReplaceAll(path, "%", "%%")
}
