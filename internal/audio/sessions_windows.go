//go:build windows

package audio

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"unsafe"

	"github.com/go-ole/go-ole"
	"github.com/moutend/go-wca/pkg/wca"
	"golang.org/x/sys/windows"
)

var (
	iidIMMDeviceEnumeratorMS   = ole.NewGUID("{A95664D2-9614-4FCF-AF66-5586927DFB5E}")
	iidIAudioSessionManager2MS = ole.NewGUID("{77AA99A0-1391-4DAA-A4B0-CBF6BDD2CA33}")
)

// ListCaptureSessions enumerates active WASAPI capture audio sessions.
func ListCaptureSessions() ([]CaptureSession, error) {
	sessions, err := listCaptureSessionsWCA()
	if err == nil {
		return sessions, nil
	}
	return nil, fmt.Errorf("enumerate capture sessions: %w", err)
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
