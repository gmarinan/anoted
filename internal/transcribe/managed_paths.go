package transcribe

import (
	"os"
	"path/filepath"
	"runtime"
)

func venvBinDir(venv string) string {
	if runtime.GOOS == "windows" {
		return filepath.Join(venv, "Scripts")
	}
	return filepath.Join(venv, "bin")
}

func venvPythonPath(venv string) string {
	name := "python"
	if runtime.GOOS == "windows" {
		name = "python.exe"
	}
	return filepath.Join(venvBinDir(venv), name)
}

func venvPipPath(venv string) string {
	name := "pip"
	if runtime.GOOS == "windows" {
		name = "pip.exe"
	}
	return filepath.Join(venvBinDir(venv), name)
}

func venvWhisperPath(venv string) string {
	name := "whisper"
	if runtime.GOOS == "windows" {
		name = "whisper.exe"
	}
	return filepath.Join(venvBinDir(venv), name)
}

func managedVenvDirForApp(appName string) string {
	if runtime.GOOS == "windows" {
		if local := os.Getenv("LOCALAPPDATA"); local != "" {
			return filepath.Join(local, appName, managedVenvName)
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return filepath.Join("AppData", "Local", appName, managedVenvName)
		}
		return filepath.Join(home, "AppData", "Local", appName, managedVenvName)
	}
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return filepath.Join(xdg, appName, managedVenvName)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".local", "share", appName, managedVenvName)
	}
	return filepath.Join(home, ".local", "share", appName, managedVenvName)
}
