using Microsoft.UI.Xaml;
using WinRT.Interop;

namespace F1Telemetry.ControlCenter;

internal static class WindowVisibility
{
    public static void Hide(Window window)
    {
        var hwnd = WindowNative.GetWindowHandle(window);
        NativeMethods.ShowWindow(hwnd, NativeMethods.SW_HIDE);
    }

    public static void Show(Window window)
    {
        var hwnd = WindowNative.GetWindowHandle(window);
        NativeMethods.ShowWindow(hwnd, NativeMethods.SW_SHOW);
        NativeMethods.SetForegroundWindow(hwnd);
    }
}
