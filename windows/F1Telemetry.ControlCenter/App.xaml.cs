using Microsoft.UI.Xaml;

namespace F1Telemetry.ControlCenter;

public partial class App : Application
{
    public static MainWindow? Window { get; private set; }

    public App()
    {
        InitializeComponent();
        var key = Microsoft.Windows.AppLifecycle.AppInstance.GetCurrent().Key;
        var primary = Microsoft.Windows.AppLifecycle.AppInstance.FindOrRegisterForKey("F1Telemetry.ControlCenter");
        if (!primary.IsCurrent)
        {
            // Redirect activation to the existing instance; never start a second server.
            primary.RedirectActivationToAsync(
                Microsoft.Windows.AppLifecycle.AppInstance.GetCurrent().GetActivatedEventArgs()).AsTask().Wait();
            Environment.Exit(0);
        }
    }

    protected override void OnLaunched(LaunchActivatedEventArgs args)
    {
        var dataDir = Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData), "F1Telemetry");
        var settings = SettingsStore.Load(dataDir);
        if (!settings.FirstRunComplete)
        {
            var first = new SettingsWindow(
                dataDir,
                Path.Combine(AppContext.BaseDirectory, "f1tele.exe"),
                firstRun: true,
                onSaved: () =>
                {
                    Window = new MainWindow();
                    Window.Activate();
                    return Task.CompletedTask;
                });
            Window = first;
            first.Activate();
            return;
        }
        Window = new MainWindow();
        Window.Activate();
    }
}
