using System.Runtime.InteropServices;
using F1Telemetry.ControlCenter.Core;
using Microsoft.UI.Xaml;
using WinRT.Interop;

namespace F1Telemetry.ControlCenter;

/// <summary>
/// Tray continuation built on Shell_NotifyIcon. Closing the window keeps the
/// server and recording alive; only Stop or Exit flush.
/// </summary>
public sealed class TrayController
{
    private const uint CmdOpenDashboard = 1;
    private const uint CmdShowWindow = 2;
    private const uint CmdStart = 3;
    private const uint CmdStop = 4;
    private const uint CmdExit = 5;
    private const uint WM_TRAY = NativeMethods.WM_APP + 1;

    private readonly MainWindow _window;
    private readonly ServerSupervisor _supervisor;
    private readonly string _exe;
    private readonly string _dataDir;
    private IntPtr _hwnd;
    private bool _installed;
    private bool _announced;

    public TrayController(MainWindow window, ServerSupervisor supervisor, string exe, string dataDir)
    {
        _window = window;
        _supervisor = supervisor;
        _exe = exe;
        _dataDir = dataDir;
    }

    public void Install()
    {
        _hwnd = WindowNative.GetWindowHandle(_window);
        var data = new NativeMethods.NOTIFYICONDATA
        {
            cbSize = Marshal.SizeOf<NativeMethods.NOTIFYICONDATA>(),
            hWnd = _hwnd,
            uID = 1,
            uFlags = NativeMethods.NIF_MESSAGE | NativeMethods.NIF_TIP,
            uCallbackMessage = WM_TRAY,
            szTip = "F1 Telemetry — recording continues",
        };
        _installed = NativeMethods.Shell_NotifyIcon(NativeMethods.NIM_ADD, ref data);
        if (_installed && !_announced)
        {
            _announced = true;
            _window.SetVoiceState("Window closed to tray; recording continues.");
        }
    }

    public void Remove()
    {
        if (!_installed)
        {
            return;
        }
        var data = new NativeMethods.NOTIFYICONDATA
        {
            cbSize = Marshal.SizeOf<NativeMethods.NOTIFYICONDATA>(),
            hWnd = _hwnd,
            uID = 1,
        };
        NativeMethods.Shell_NotifyIcon(NativeMethods.NIM_DELETE, ref data);
        _installed = false;
    }

    /// <summary>Invoked from the window message loop when the tray is clicked.</summary>
    public void HandleMessage(int message, IntPtr wParam, IntPtr lParam)
    {
        if (message != WM_TRAY || (int)lParam != 0x0205 /* WM_RBUTTONUP */)
        {
            return;
        }
        NativeMethods.GetCursorPos(out var pt);
        var menu = NativeMethods.CreatePopupMenu();
        NativeMethods.AppendMenu(menu, NativeMethods.MF_STRING, CmdOpenDashboard, "Open dashboard");
        NativeMethods.AppendMenu(menu, NativeMethods.MF_STRING, CmdShowWindow, "Show control center");
        NativeMethods.AppendMenu(menu, NativeMethods.MF_STRING, CmdStart, "Start server");
        NativeMethods.AppendMenu(menu, NativeMethods.MF_STRING, CmdStop, "Stop server");
        NativeMethods.AppendMenu(menu, NativeMethods.MF_STRING, CmdExit, "Exit");
        var cmd = NativeMethods.TrackPopupMenu(menu, NativeMethods.TPM_RIGHTBUTTON, pt.X, pt.Y, 0, _hwnd, IntPtr.Zero);
        NativeMethods.DestroyMenu(menu);
        Dispatch((uint)cmd);
    }

    public async void Dispatch(uint cmd)
    {
        switch (cmd)
        {
            case CmdOpenDashboard:
                System.Diagnostics.Process.Start(new System.Diagnostics.ProcessStartInfo("http://127.0.0.1:8081") { UseShellExecute = true });
                break;
            case CmdShowWindow:
                _window.ShowFromTray();
                break;
            case CmdStart:
                _window.Apply(await _supervisor.StartAsync(_exe, Path.Combine(_dataDir, "supervisor-owner.json")));
                break;
            case CmdStop:
                _window.Apply(await _supervisor.StopAsync(TimeSpan.FromSeconds(10)));
                break;
            case CmdExit:
                Remove();
                Environment.Exit(0);
                break;
        }
    }
}
