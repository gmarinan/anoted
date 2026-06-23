//go:build windows

package audio

import (
	"bufio"
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"unsafe"

	"github.com/go-ole/go-ole"
	"github.com/moutend/go-wca/pkg/wca"
	"golang.org/x/sys/windows"
)

//go:embed session_enum.ps1
var sessionEnumScript string

var (
	iidIMMDeviceEnumeratorMS  = ole.NewGUID("{A95664D2-9614-4FCF-AF66-5586927DFB5E}")
	iidIAudioSessionManager2MS = ole.NewGUID("{77AA99A0-1391-4DAA-A4B0-CBF6BDD2CA33}")
)

// ListCaptureSessions enumerates active WASAPI capture audio sessions.
func ListCaptureSessions() ([]CaptureSession, error) {
	sessions, err := listCaptureSessionsWCA()
	if err == nil {
		return sessions, nil
	}
	psSessions, psErr := listCaptureSessionsPowerShell()
	if psErr == nil {
		return psSessions, nil
	}
	return nil, fmt.Errorf("WASAPI: %w; PowerShell: %v", err, psErr)
}

func listCaptureSessionsWCA() ([]CaptureSession, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if err := initCOM(); err != nil {
		return nil, err
	}
	defer ole.CoUninitialize()

	mmde, err := createDeviceEnumeratorWCA()
	if err != nil {
		return nil, err
	}
	defer mmde.Release()

	var devices *wca.IMMDeviceCollection
	if err := mmde.EnumAudioEndpoints(wca.ECapture, wca.DEVICE_STATE_ACTIVE, &devices); err != nil {
		return nil, fmt.Errorf("EnumAudioEndpoints: %w", err)
	}
	defer devices.Release()

	var count uint32
	if err := devices.GetCount(&count); err != nil {
		return nil, fmt.Errorf("GetCount: %w", err)
	}

	var sessions []CaptureSession
	seen := make(map[string]struct{})
	for i := uint32(0); i < count; i++ {
		var device *wca.IMMDevice
		if err := devices.Item(i, &device); err != nil {
			continue
		}
		deviceSessions, err := captureSessionsOnDeviceWCA(device)
		device.Release()
		if err != nil {
			continue
		}
		for _, s := range deviceSessions {
			key := fmt.Sprintf("%d:%s:%s", s.ProcessID, s.ProcessName, s.DisplayName)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			sessions = append(sessions, s)
		}
	}
	return sessions, nil
}

func createDeviceEnumeratorWCA() (*wca.IMMDeviceEnumerator, error) {
	var mmde *wca.IMMDeviceEnumerator
	for _, iid := range []*ole.GUID{iidIMMDeviceEnumeratorMS, wca.IID_IMMDeviceEnumerator} {
		if err := wca.CoCreateInstance(wca.CLSID_MMDeviceEnumerator, 0, wca.CLSCTX_ALL, iid, &mmde); err == nil && mmde != nil {
			return mmde, nil
		}
	}
	return nil, fmt.Errorf("CoCreateInstance MMDeviceEnumerator: 0x80004002")
}

func captureSessionsOnDeviceWCA(device *wca.IMMDevice) ([]CaptureSession, error) {
	var asm2 *wca.IAudioSessionManager2
	if err := device.Activate(iidIAudioSessionManager2MS, wca.CLSCTX_ALL, nil, &asm2); err != nil {
		if err2 := device.Activate(wca.IID_IAudioSessionManager2, wca.CLSCTX_ALL, nil, &asm2); err2 != nil {
			return nil, err
		}
	}
	defer asm2.Release()

	var sessionEnum *wca.IAudioSessionEnumerator
	if err := asm2.GetSessionEnumerator(&sessionEnum); err != nil {
		return nil, err
	}
	defer sessionEnum.Release()

	var n int
	if err := sessionEnum.GetCount(&n); err != nil {
		return nil, err
	}

	var out []CaptureSession
	for i := 0; i < n; i++ {
		var asc *wca.IAudioSessionControl
		if err := sessionEnum.GetSession(i, &asc); err != nil {
			continue
		}
		s, ok := sessionFromControlWCA(asc)
		asc.Release()
		if ok {
			out = append(out, s)
		}
	}
	return out, nil
}

func sessionFromControlWCA(asc *wca.IAudioSessionControl) (CaptureSession, bool) {
	asc2 := (*wca.IAudioSessionControl2)(unsafe.Pointer(asc))

	var state uint32
	if err := asc.GetState(&state); err != nil || state != wca.AudioSessionStateActive {
		return CaptureSession{}, false
	}
	if err := asc2.IsSystemSoundsSession(); err == nil {
		return CaptureSession{}, false
	}

	var pid uint32
	if err := asc2.GetProcessId(&pid); err != nil || pid == 0 {
		return CaptureSession{}, false
	}

	var display string
	_ = asc.GetDisplayName(&display)

	return CaptureSession{
		ProcessID:   pid,
		ProcessName: processBasename(pid),
		DisplayName: display,
	}, true
}

func listCaptureSessionsPowerShell() ([]CaptureSession, error) {
	scriptPath := filepath.Join(os.TempDir(), "anoted-session-enum.ps1")
	if err := os.WriteFile(scriptPath, []byte(sessionEnumScript), 0o600); err != nil {
		return nil, err
	}
	defer os.Remove(scriptPath)

	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-STA", "-ExecutionPolicy", "Bypass", "-File", scriptPath)
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("%w: %s", err, strings.TrimSpace(string(ee.Stderr)))
		}
		return nil, err
	}

	var sessions []CaptureSession
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) < 2 {
			continue
		}
		var pid uint32
		if _, err := fmt.Sscanf(parts[0], "%d", &pid); err != nil || pid == 0 {
			continue
		}
		name := parts[1]
		display := ""
		if len(parts) > 2 {
			display = parts[2]
		}
		sessions = append(sessions, CaptureSession{
			ProcessID:   pid,
			ProcessName: name,
			DisplayName: display,
		})
	}
	return sessions, nil
}

func initCOM() error {
	err := ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED)
	if err == nil {
		return nil
	}
	oleErr, ok := err.(*ole.OleError)
	if ok && oleErr.Code() == 0x80010106 { // RPC_E_CHANGED_MODE
		return nil
	}
	return fmt.Errorf("CoInitializeEx: %w", err)
}

func processBasename(pid uint32) string {
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return ""
	}
	defer windows.CloseHandle(handle)

	var size uint32 = windows.MAX_PATH
	buf := make([]uint16, size)
	if err := windows.QueryFullProcessImageName(handle, 0, &buf[0], &size); err != nil {
		return ""
	}
	return strings.TrimSuffix(filepath.Base(windows.UTF16ToString(buf[:size])), ".exe")
}
