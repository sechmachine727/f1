using System.Diagnostics;
using F1Telemetry.ControlCenter.Core;
using Microsoft.UI.Xaml;

namespace F1Telemetry.ControlCenter;

public sealed partial class MainWindow : Window
{
    private readonly ServerSupervisor _supervisor;
    private readonly TrayController _tray;
    private readonly string _ownerPath;
    private bool _closing;

    public MainWindow()
    {
        InitializeComponent();
        var dataDir = Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData), "F1Telemetry");
        Directory.CreateDirectory(dataDir);
        _ownerPath = Path.Combine(dataDir, "supervisor-owner.json");
        var exe = Path.Combine(AppContext.BaseDirectory, "f1tele.exe");
        _supervisor = new ServerSupervisor(
            new SystemLauncher(),
            new HttpHealthProbe(new HttpClient { Timeout = TimeSpan.FromSeconds(5) }),
            dataDir,
            Path.Combine(dataDir, "config.json"));
        _tray = new TrayController(this, _supervisor, exe, dataDir);
        Closed += OnClosed;
        _ = StartAsync(exe);
    }

    private async Task StartAsync(string exe)
    {
        var status = await _supervisor.StartAsync(exe, _ownerPath);
        Apply(status);
        if (status.State == SupervisorState.Running)
        {
            _tray.Install();
            Process.Start(new ProcessStartInfo(status.DashboardUrl!) { UseShellExecute = true });
        }
        else if (status.State == SupervisorState.PortConflict)
        {
            // Never terminate the unrelated process holding the port.
            DetailText.Text += " Open Settings to choose another port; the other process was left running.";
        }
    }

    public void Apply(SupervisorStatus status)
    {
        StatusText.Text = status.State switch
        {
            SupervisorState.Running => "Server running",
            SupervisorState.Starting => "Starting server…",
            SupervisorState.Stopping => "Stopping server…",
            SupervisorState.PortConflict => "Port conflict",
            SupervisorState.Crashed => "Server crashed",
            SupervisorState.Failed => "Server error",
            _ => "Server stopped",
        };
        DetailText.Text = status.Detail;
        var addrs = status.LanAddresses.Count == 0 ? "LAN address unavailable" : string.Join(", ", status.LanAddresses);
        AddressText.Text = status.State == SupervisorState.Running
            ? $"Dashboard: {status.DashboardUrl}\nLAN: {addrs}\nRecording: {status.Health?.RecordingState ?? "unknown"}"
            : "Waiting for telemetry is not an error; start the game with UDP telemetry enabled.";
    }

    public void SetVoiceState(string state) => VoiceText.Text = state;

    private void Open_Click(object sender, RoutedEventArgs e)
    {
        if (_supervisor.Status.State == SupervisorState.Running)
        {
            Process.Start(new ProcessStartInfo(_supervisor.Status.DashboardUrl ?? "http://127.0.0.1:8081") { UseShellExecute = true });
        }
    }

    private async void Stop_Click(object sender, RoutedEventArgs e)
    {
        var status = await _supervisor.StopAsync(TimeSpan.FromSeconds(10));
        Apply(status);
        if (status.State == SupervisorState.Stopping)
        {
            var dialog = new ContentDialog
            {
                XamlRoot = Content.XamlRoot,
                Title = "Server did not stop",
                Content = "Recording may not be flushed. Retry, or force stop and accept possible data loss in the current recording file.",
                PrimaryButtonText = "Retry",
                SecondaryButtonText = "Force stop",
                CloseButtonText = "Cancel",
            };
            var result = await dialog.ShowAsync();
            if (result == ContentDialogResult.Primary)
            {
                Stop_Click(sender, e);
            }
            else if (result == ContentDialogResult.Secondary)
            {
                _supervisor.ForceStop();
                Apply(_supervisor.Status);
            }
        }
    }

    private void Exit_Click(object sender, RoutedEventArgs e)
    {
        _closing = true;
        Close();
    }

    private void OnClosed(object sender, WindowEventArgs args)
    {
        if (!_closing)
        {
            // Closing the window hides to the tray; the server keeps recording.
            args.Handled = true;
            HideToTray();
            return;
        }
        _tray.Remove();
        Environment.Exit(0);
    }

    public void HideToTray() => WindowVisibility.Hide(this);

    public void ShowFromTray()
    {
        WindowVisibility.Show(this);
        Activate();
    }
}
