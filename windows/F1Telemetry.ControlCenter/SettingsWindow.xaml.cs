using System.Diagnostics;
using System.Text.Json;
using System.Text.Json.Nodes;
using F1Telemetry.ControlCenter.Core;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Controls;
using Windows.ApplicationModel.DataTransfer;

namespace F1Telemetry.ControlCenter;

public sealed partial class SettingsWindow : Window
{
    private readonly string _dataDir;
    private readonly string _serverExe;
    private readonly bool _firstRun;
    private readonly Func<Task> _onSaved;
    private readonly Func<Task>? _beforeSave;
    private readonly SettingsStore _local;
    private ServerConfigFile _config;
    private bool _saved;

    public SettingsWindow(string dataDir, string serverExe, bool firstRun, Func<Task> onSaved, Func<Task>? beforeSave = null)
    {
        InitializeComponent();
        _dataDir = dataDir;
        _serverExe = serverExe;
        _firstRun = firstRun;
        _onSaved = onSaved;
        _beforeSave = beforeSave;
        _local = SettingsStore.Load(dataDir);
        _config = ServerConfigFile.Load(dataDir, Path.Combine(AppContext.BaseDirectory, "defaults.json"));
        Title = firstRun ? "Set up F1 Telemetry" : "F1 Telemetry Settings";
        TitleText.Text = firstRun ? "Set up F1 Telemetry" : "Settings";
        SubtitleText.Text = firstRun
            ? "Set the dashboard password and the telemetry connection before the server starts."
            : "Changes to the bundled server stop it, write the configuration and restart it.";
        SaveButton.Content = firstRun ? "Save and start" : "Save and restart";
        LoadFields();
        Activated += OnActivated;
        Closed += (_, _) => { if (_firstRun && !_saved) Application.Current.Exit(); };
    }

    private async void OnActivated(Window sender, WindowActivatedEventArgs args)
    {
        Activated -= OnActivated;
        await EnsurePasswordAsync();
    }

    private void LoadFields()
    {
        DataDirBox.Text = _dataDir;
        HttpPortBox.Value = _config.GetInt("http.port", 8081);
        UdpPortBox.Value = _config.GetInt("udp.port", 20777);
        RecordingCapBox.Value = Math.Max(1, _config.GetInt("recording.cap_bytes", 100_000_000_000) / 1_000_000_000.0);
        OriginCheckBox.IsChecked = _config.GetBool("security.origin_check", true);
        EndpointIdBox.Text = _config.GetString("agent.endpoint_id", "windows");
        var endpoints = _config.Root["agent"]?["endpoints"] as JsonArray;
        var endpoint = endpoints is { Count: > 0 } ? endpoints[0] as JsonObject : null;
        BaseUrlBox.Text = endpoint?["base_url"]?.GetValue<string>() ?? "";
        ModelBox.Text = endpoint?["model"]?.GetValue<string>() ?? "";
        var protocol = endpoint?["protocol"]?.GetValue<string>() ?? "chat_completions";
        ProtocolBox.SelectedIndex = protocol == "responses" ? 1 : 0;
        SearchProviderBox.SelectedIndex = Math.Max(0, new[] { "disabled", "tavily", "searxng" }.ToList().IndexOf(_config.GetString("search.provider", "disabled")));
        SearchUrlBox.Text = _config.GetString("search.base_url", "");
        VoiceEnabledBox.IsChecked = _config.GetBool("voice.enabled", false);
        VoiceUrlBox.Text = _config.GetString("voice.transcription_endpoint", "");
        GameAddressText.Text = "Set the game's UDP telemetry address to this machine on port " + Port(UdpPortBox) + ".";
        ApiKeyStatus.Text = _config.Secrets.ContainsKey("F1TELE_AGENT_API_KEY") ? "A key is stored." : "No API key is stored yet.";
        ApplyEnvironmentLocks();
    }

    private void ApplyEnvironmentLocks()
    {
        DisableIfEnv(HttpPortBox, "F1TELE_HTTP_ADDR");
        DisableIfEnv(UdpPortBox, "F1TELE_UDP_ADDR");
        DisableIfEnv(OriginCheckBox, "F1TELE_SECURITY_ORIGIN_CHECK");
        DisableIfEnv(RecordingCapBox, "F1TELE_RECORDING_CAP_BYTES");
        DisableIfEnv(EndpointIdBox, "F1TELE_AGENT_ENDPOINT_ID");
        DisableIfEnv(BaseUrlBox, "F1TELE_AGENT_BASE_URL");
        DisableIfEnv(ModelBox, "F1TELE_AGENT_MODEL");
        DisableIfEnv(ProtocolBox, "F1TELE_AGENT_PROTOCOL");
        DisableIfEnv(ApiKeyBox, "F1TELE_AGENT_API_KEY");
    }

    private static void DisableIfEnv(Control control, string variable)
    {
        if (!ServerConfigFile.EnvironmentLocked(variable)) return;
        control.IsEnabled = false;
        ToolTipService.SetToolTip(control, variable + " owns this value.");
    }

    private async Task EnsurePasswordAsync()
    {
        var result = await RunAuth("ensure", new[] { "--json" }, null);
        if (result.ExitCode != 0)
        {
            PasswordStatus.Text = "Password check failed: " + result.Stderr.Trim();
            return;
        }
        try
        {
            using var doc = JsonDocument.Parse(result.Stdout);
            var root = doc.RootElement;
            if (root.TryGetProperty("created", out var created) && created.GetBoolean())
            {
                var password = root.GetProperty("password").GetString() ?? "";
                await ShowGeneratedPasswordAsync(password);
            }
            else if (root.TryGetProperty("source", out var source))
            {
                PasswordStatus.Text = "Dashboard password source: " + source.GetString() + ".";
            }
        }
        catch (JsonException)
        {
            PasswordStatus.Text = "The password command returned an unreadable response.";
        }
    }

    private async Task ShowGeneratedPasswordAsync(string password)
    {
        var box = new TextBox { Text = password, IsReadOnly = true, FontFamily = new Microsoft.UI.Xaml.Media.FontFamily("Consolas"), TextWrapping = TextWrapping.Wrap };
        var copy = new Button { Content = "Copy password" };
        copy.Click += (_, _) =>
        {
            var data = new DataPackage();
            data.SetText(password);
            Clipboard.SetContent(data);
            copy.Content = "Copied";
        };
        var panel = new StackPanel { Spacing = 8 };
        panel.Children.Add(new TextBlock { Text = "This password is shown once. Store it now; reset it later if it is lost.", TextWrapping = TextWrapping.Wrap });
        panel.Children.Add(box);
        panel.Children.Add(copy);
        var dialog = new ContentDialog
        {
            Title = "Dashboard password",
            Content = panel,
            CloseButtonText = "I saved it",
            XamlRoot = Content.XamlRoot,
        };
        await dialog.ShowAsync();
        PasswordStatus.Text = "Dashboard password generated.";
    }

    private async void Save_Click(object sender, RoutedEventArgs e)
    {
        try
        {
            var endpointId = string.IsNullOrWhiteSpace(EndpointIdBox.Text) ? "windows" : EndpointIdBox.Text.Trim();
            _config.SetInt("http.port", Port(HttpPortBox));
            _config.SetInt("udp.port", Port(UdpPortBox));
            _config.SetBool("security.origin_check", OriginCheckBox.IsChecked == true);
            _config.Set("recording.cap_bytes", JsonValue.Create((long)(Math.Max(1, RecordingCapBox.Value) * 1_000_000_000)));
            var endpointEnabled = !string.IsNullOrWhiteSpace(BaseUrlBox.Text) && !string.IsNullOrWhiteSpace(ModelBox.Text);
            _config.SetBool("agent.enabled", endpointEnabled);
            if (endpointEnabled)
            {
                _config.SetString("agent.endpoint_id", endpointId);
                var endpoint = new JsonObject
                {
                    ["id"] = endpointId,
                    ["protocol"] = ProtocolBox.SelectedIndex == 1 ? "responses" : "chat_completions",
                    ["base_url"] = BaseUrlBox.Text.Trim(),
                    ["model"] = ModelBox.Text.Trim(),
                    ["api_key_env"] = "F1TELE_AGENT_API_KEY",
                    ["context_limit_tokens"] = 128000,
                    ["input_usd_per_million"] = null,
                    ["output_usd_per_million"] = null,
                };
                _config.Set("agent.endpoints", new JsonArray(endpoint));
            }
            else
            {
                _config.Set("agent.endpoint_id", null);
                _config.Set("agent.endpoints", new JsonArray());
            }
            var provider = SearchProviderBox.SelectedItem is ComboBoxItem item ? item.Content?.ToString() ?? "disabled" : "disabled";
            _config.SetString("search.provider", provider);
            _config.Set("search.base_url", string.IsNullOrWhiteSpace(SearchUrlBox.Text) ? null : JsonValue.Create(SearchUrlBox.Text.Trim()));
            _config.SetBool("voice.enabled", VoiceEnabledBox.IsChecked == true);
            _config.Set("voice.transcription_endpoint", string.IsNullOrWhiteSpace(VoiceUrlBox.Text) ? null : JsonValue.Create(VoiceUrlBox.Text.Trim()));
            _config.Set("voice.speech_endpoint", string.IsNullOrWhiteSpace(VoiceUrlBox.Text) ? null : JsonValue.Create(VoiceUrlBox.Text.Trim()));
            if (!string.IsNullOrWhiteSpace(ApiKeyBox.Password)) _config.SetSecret("F1TELE_AGENT_API_KEY", ApiKeyBox.Password);
            if (_beforeSave != null) await _beforeSave();
            _config.Save(_dataDir);
            _local.FirstRunComplete = true;
            _local.Save();
            _saved = true;
            StatusText.Text = "Configuration saved.";
            await _onSaved();
            Close();
        }
        catch (Exception ex)
        {
            StatusText.Text = "Save failed: " + ex.Message;
        }
    }

    private async void ResetPassword_Click(object sender, RoutedEventArgs e)
    {
        if (NewPasswordBox.Password.Length < 8)
        {
            PasswordStatus.Text = "Password must be at least 8 characters.";
            return;
        }
        if (NewPasswordBox.Password != ConfirmPasswordBox.Password)
        {
            PasswordStatus.Text = "The passwords do not match.";
            return;
        }
        var result = await RunAuth("reset", new[] { "--stdin" }, NewPasswordBox.Password);
        PasswordStatus.Text = result.ExitCode == 0
            ? "Password reset. Existing browser sessions were invalidated."
            : "Password reset failed: " + result.Stderr.Trim();
        if (result.ExitCode == 0)
        {
            NewPasswordBox.Password = "";
            ConfirmPasswordBox.Password = "";
        }
    }

    private void Cancel_Click(object sender, RoutedEventArgs e)
    {
        if (_firstRun) Application.Current.Exit();
        else Close();
    }

    private static int Port(NumberBox box) => (int)Math.Clamp(double.IsNaN(box.Value) ? 0 : box.Value, 1, 65535);

    private async Task<AuthRun> RunAuth(string command, string[] extra, string? stdin)
    {
        var psi = new ProcessStartInfo(_serverExe)
        {
            UseShellExecute = false, CreateNoWindow = true, RedirectStandardOutput = true, RedirectStandardError = true, RedirectStandardInput = stdin is not null,
        };
        psi.ArgumentList.Add("auth");
        psi.ArgumentList.Add(command);
        foreach (var arg in extra) psi.ArgumentList.Add(arg);
        psi.ArgumentList.Add("--data-dir");
        psi.ArgumentList.Add(_dataDir);
        using var process = Process.Start(psi) ?? throw new InvalidOperationException("Could not start the server authentication command.");
        if (stdin is not null)
        {
            await process.StandardInput.WriteLineAsync(stdin);
            process.StandardInput.Close();
        }
        var stdout = await process.StandardOutput.ReadToEndAsync();
        var stderr = await process.StandardError.ReadToEndAsync();
        await process.WaitForExitAsync();
        return new AuthRun(process.ExitCode, stdout, stderr);
    }

    private sealed record AuthRun(int ExitCode, string Stdout, string Stderr);
}
