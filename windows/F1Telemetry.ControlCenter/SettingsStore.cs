using System.Text.Json;
using F1Telemetry.ControlCenter.Core;

namespace F1Telemetry.ControlCenter;

/// <summary>Persists only local presentation settings; provider secrets stay in the server store.</summary>
public sealed class SettingsStore
{
    private string _path = string.Empty;
    public string PushToTalk { get; set; } = "Ctrl+Alt+Space";
    public bool VoiceEnabled { get; set; } = true;
    public bool FirstRunComplete { get; set; }

    public SettingsStore(string dataDir) => _path = Path.Combine(dataDir, "control-center.json");

    public void Save()
    {
        Directory.CreateDirectory(Path.GetDirectoryName(_path)!);
        File.WriteAllText(_path, JsonSerializer.Serialize(this));
    }

    public static SettingsStore Load(string dataDir)
    {
        var path = Path.Combine(dataDir, "control-center.json");
        if (!File.Exists(path))
        {
            return new SettingsStore(dataDir);
        }
        try
        {
            var loaded = JsonSerializer.Deserialize<SettingsStore>(File.ReadAllText(path));
            if (loaded is null)
            {
                return new SettingsStore(dataDir);
            }
            loaded._path = path;
            return loaded;
        }
        catch (JsonException)
        {
            return new SettingsStore(dataDir);
        }
    }
}
