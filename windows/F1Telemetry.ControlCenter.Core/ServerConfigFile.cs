using System.Text.Json;
using System.Text.Json.Nodes;

namespace F1Telemetry.ControlCenter.Core;

/// <summary>
/// Reads and writes the portable server's config.json and secrets.json without
/// duplicating the server contract in C#. Missing config fields keep the
/// bundled defaults.
/// </summary>
public sealed class ServerConfigFile
{
    public JsonObject Root { get; }
    public Dictionary<string, string> Secrets { get; }

    private ServerConfigFile(JsonObject root, Dictionary<string, string> secrets)
    {
        Root = root;
        Secrets = secrets;
    }

    public static ServerConfigFile Load(string dataDir, string defaultsPath)
    {
        var defaults = ReadObject(defaultsPath) ?? new JsonObject();
        var configPath = Path.Combine(dataDir, "config.json");
        var current = ReadObject(configPath) ?? new JsonObject();
        Merge(defaults, current);

        var secrets = new Dictionary<string, string>(StringComparer.Ordinal);
        var secretsPath = Path.Combine(dataDir, "secrets.json");
        if (File.Exists(secretsPath))
        {
            try
            {
                secrets = JsonSerializer.Deserialize<Dictionary<string, string>>(File.ReadAllText(secretsPath)) ?? secrets;
            }
            catch (JsonException)
            {
                // A malformed local secret file is left untouched until the user saves.
            }
        }
        return new ServerConfigFile(defaults, secrets);
    }

    public string GetString(string path, string fallback = "")
    {
        var node = Find(Root, path);
        try { return node?.GetValue<string>() ?? fallback; }
        catch { return fallback; }
    }

    public int GetInt(string path, int fallback)
    {
        var node = Find(Root, path);
        try { return node?.GetValue<int>() ?? fallback; }
        catch { return fallback; }
    }

    public bool GetBool(string path, bool fallback)
    {
        var node = Find(Root, path);
        try { return node?.GetValue<bool>() ?? fallback; }
        catch { return fallback; }
    }

    public void Set(string path, JsonNode? value)
    {
        var parts = path.Split('.');
        JsonObject cur = Root;
        for (var i = 0; i < parts.Length - 1; i++)
        {
            if (cur[parts[i]] is not JsonObject next)
            {
                next = new JsonObject();
                cur[parts[i]] = next;
            }
            cur = next;
        }
        cur[parts[^1]] = value;
    }

    public void SetString(string path, string value) => Set(path, JsonValue.Create(value));
    public void SetInt(string path, int value) => Set(path, JsonValue.Create(value));
    public void SetBool(string path, bool value) => Set(path, JsonValue.Create(value));

    public void SetSecret(string id, string value)
    {
        if (!string.IsNullOrWhiteSpace(id) && value.Length > 0)
        {
            Secrets[id] = value;
        }
    }

    public void Save(string dataDir)
    {
        Directory.CreateDirectory(dataDir);
        File.WriteAllText(Path.Combine(dataDir, "config.json"), Root.ToJsonString(new JsonSerializerOptions { WriteIndented = true }));
        File.WriteAllText(Path.Combine(dataDir, "secrets.json"), JsonSerializer.Serialize(Secrets, new JsonSerializerOptions { WriteIndented = true }));
    }

    public static bool EnvironmentLocked(string variable) => !string.IsNullOrWhiteSpace(Environment.GetEnvironmentVariable(variable));

    private static JsonObject? ReadObject(string path)
    {
        if (!File.Exists(path)) return null;
        try { return JsonNode.Parse(File.ReadAllText(path)) as JsonObject; }
        catch (JsonException) { return null; }
    }

    private static JsonNode? Find(JsonObject root, string path)
    {
        JsonNode? cur = root;
        foreach (var part in path.Split('.'))
        {
            if (cur is not JsonObject obj) return null;
            cur = obj[part];
        }
        return cur;
    }

    private static void Merge(JsonObject target, JsonObject source)
    {
        foreach (var pair in source)
        {
            if (pair.Value is JsonObject sourceChild && target[pair.Key] is JsonObject targetChild)
            {
                Merge(targetChild, sourceChild);
            }
            else
            {
                target[pair.Key] = pair.Value?.DeepClone();
            }
        }
    }
}
