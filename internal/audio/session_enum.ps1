# Enumerate active WASAPI capture audio sessions. Output: PID<TAB>ProcessName<TAB>DisplayName
$ErrorActionPreference = 'Stop'
Add-Type @'
using System;
using System.Diagnostics;
using System.Runtime.InteropServices;
using System.Text;

[ComImport, Guid("BCDE0395-E52F-467C-8E3D-C4579291692E")]
public class MMDeviceEnumerator { }

[Guid("A95664D2-9614-4FCF-AF66-5586927DFB5E"), InterfaceType(ComInterfaceType.InterfaceIsIUnknown)]
public interface IMMDeviceEnumerator {
    int f(); int g();
    [PreserveSig] int EnumAudioEndpoints(int dataFlow, uint stateMask, out IMMDeviceCollection devices);
}

[Guid("0BD7A1BE-7A1A-44DB-8395-F33449F8F13F"), InterfaceType(ComInterfaceType.InterfaceIsIUnknown)]
public interface IMMDeviceCollection {
    int f(); int g();
    [PreserveSig] int GetCount(out uint count);
    [PreserveSig] int Item(uint index, out IMMDevice device);
}

[Guid("D666063F-1587-4E43-81F1-B948E807363F"), InterfaceType(ComInterfaceType.InterfaceIsIUnknown)]
public interface IMMDevice {
    int f(); int g();
    [PreserveSig] int Activate(ref Guid iid, uint clsCtx, IntPtr activationParams, [MarshalAs(UnmanagedType.IUnknown)] out object iface);
}

[Guid("77AA99A0-1391-4DAA-A4B0-CBF6BDD2CA33"), InterfaceType(ComInterfaceType.InterfaceIsIUnknown)]
public interface IAudioSessionManager2 {
    int f(); int g(); int h(); int i(); int j();
    [PreserveSig] int GetSessionEnumerator(out IAudioSessionEnumerator enumerator);
}

[Guid("E2F5BB11-0570-40CA-ACDD-C9979ADBC35B"), InterfaceType(ComInterfaceType.InterfaceIsIUnknown)]
public interface IAudioSessionEnumerator {
    int f(); int g();
    [PreserveSig] int GetCount(out int count);
    [PreserveSig] int GetSession(int index, out IAudioSessionControl control);
}

[Guid("F4B1A599-7266-4319-A8CA-E70ACB11E8CD"), InterfaceType(ComInterfaceType.InterfaceIsIUnknown)]
public interface IAudioSessionControl {
    int f(); int g();
    [PreserveSig] int GetState(out int state);
    [PreserveSig] int SetState(int state);
    [PreserveSig] int GetDisplayName([MarshalAs(UnmanagedType.LPWStr)] out string name);
}

[Guid("BFB7FF88-7239-4FC9-8FA2-07C950BE9C6D"), InterfaceType(ComInterfaceType.InterfaceIsIUnknown)]
public interface IAudioSessionControl2 {
    int f(); int g();
    [PreserveSig] int GetState(out int state);
    [PreserveSig] int SetState(int state);
    [PreserveSig] int GetDisplayName([MarshalAs(UnmanagedType.LPWStr)] out string name);
    [PreserveSig] int SetDisplayName([MarshalAs(UnmanagedType.LPWStr)] string value, ref Guid eventContext);
    [PreserveSig] int GetIconPath([MarshalAs(UnmanagedType.LPWStr)] out string path);
    [PreserveSig] int SetIconPath([MarshalAs(UnmanagedType.LPWStr)] string path, ref Guid eventContext);
    [PreserveSig] int GetGroupingParam(out Guid param);
    [PreserveSig] int SetGroupingParam(ref Guid param, ref Guid eventContext);
    [PreserveSig] int RegisterAudioSessionNotification(IntPtr notification);
    [PreserveSig] int UnregisterAudioSessionNotification(IntPtr notification);
    [PreserveSig] int GetSessionIdentifier([MarshalAs(UnmanagedType.LPWStr)] out string id);
    [PreserveSig] int GetSessionInstanceIdentifier([MarshalAs(UnmanagedType.LPWStr)] out string id);
    [PreserveSig] int GetProcessId(out uint pid);
    [PreserveSig] int IsSystemSoundsSession();
}

public static class CaptureSessions {
    const int ECapture = 1;
    const uint DEVICE_STATE_ACTIVE = 1;
    const int AudioSessionStateActive = 1;
    const uint CLSCTX_ALL = 23;
    static readonly Guid IID_IAudioSessionManager2 = new Guid("77AA99A0-1391-4DAA-A4B0-CBF6BDD2CA33");

  public static void List() {
    var enumerator = (IMMDeviceEnumerator)new MMDeviceEnumerator();
    IMMDeviceCollection devices;
    if (enumerator.EnumAudioEndpoints(ECapture, DEVICE_STATE_ACTIVE, out devices) != 0) return;
    uint count;
    if (devices.GetCount(out count) != 0) return;
    for (uint i = 0; i < count; i++) {
      IMMDevice device;
      if (devices.Item(i, out device) != 0) continue;
      object mgrObj;
      var iid = IID_IAudioSessionManager2;
      if (device.Activate(ref iid, CLSCTX_ALL, IntPtr.Zero, out mgrObj) != 0) continue;
      var mgr = (IAudioSessionManager2)mgrObj;
      IAudioSessionEnumerator sessions;
      if (mgr.GetSessionEnumerator(out sessions) != 0) continue;
      int n;
      if (sessions.GetCount(out n) != 0) continue;
      for (int j = 0; j < n; j++) {
        IAudioSessionControl control;
        if (sessions.GetSession(j, out control) != 0) continue;
        var c2 = (IAudioSessionControl2)control;
        int state;
        if (c2.GetState(out state) != 0 || state != AudioSessionStateActive) continue;
        if (c2.IsSystemSoundsSession() == 0) continue;
        uint pid;
        if (c2.GetProcessId(out pid) != 0 || pid == 0) continue;
        string display;
        c2.GetDisplayName(out display);
        string name = "";
        try {
          var p = Process.GetProcessById((int)pid);
          name = p.ProcessName;
        } catch { }
        Console.WriteLine(pid + "\t" + name + "\t" + (display ?? ""));
      }
    }
  }
}
'@

[CaptureSessions]::List()
