using System.Text.Json;

namespace F1Telemetry.ControlCenter.Core;

/// <summary>One effective configuration value shown in native Settings.</summary>
public sealed record ConfigRow(string Section, string Name, string Value, string? Source, bool RestartOnly, bool RestartPending);

/// <summary>
/// Turns the server's ConfigView into display rows. Secrets are never copied:
/// the API only reports whether a key is configured.
/// </summary>
public static class ConfigurationView
{
    public static IReadOnlyList<ConfigRow> Rows(string json, string configurationPath)
    {
        using var doc = JsonDocument.Parse(json);
        var root = doc.RootElement;
        var active = root.GetProperty("active_config");
        var rows = new List<ConfigRow>();
        string? Source(string key) =>
            root.TryGetProperty("sources", out var sources) && sources.TryGetProperty(key, out var v) ? v.GetString() : null;
        bool Pending(string key) =>
            root.TryGetProperty("restart_required", out var pending) &&
            pending.EnumerateArray().Any(x => x.GetString() == key);

        var http = active.GetProperty("http");
        var udp = active.GetProperty("udp");
        rows.Add(new("Server", "Dashboard", $"{http.GetProperty("host").GetString()}:{http.GetProperty("port").GetInt32()}", Source("http"), true, Pending("http")));
        rows.Add(new("Server", "Telemetry (UDP)", $"{udp.GetProperty("host").GetString()}:{udp.GetProperty("port").GetInt32()}", Source("udp"), true, Pending("udp")));
        var advertise = root.TryGetProperty("udp_advertise_host", out var host) ? host.GetString() : null;
        rows.Add(new("Server", "Game address", string.IsNullOrEmpty(advertise) ? "unknown" : $"{advertise}:{udp.GetProperty("port").GetInt32()}", Source("udp_advertise_host"), false, false));
        var dataDir = active.TryGetProperty("data_dir", out var dd) && dd.ValueKind == JsonValueKind.String ? dd.GetString() : null;
        rows.Add(new("Server", "Data directory", dataDir ?? "platform default", Source("data_dir"), true, Pending("data_dir")));

        var rec = active.GetProperty("recording");
        rows.Add(new("Recording", "Recording", rec.GetProperty("enabled").GetBoolean() ? "Enabled" : "Disabled", Source("recording.enabled"), false, false));
        rows.Add(new("Recording", "Storage cap", Bytes(rec.GetProperty("cap_bytes").GetInt64()), Source("recording.cap_bytes"), false, false));
        rows.Add(new("Recording", "Free reserve", Bytes(rec.GetProperty("free_reserve_bytes").GetInt64()), Source("recording.free_reserve_bytes"), false, false));
        rows.Add(new("Recording", "Chunk size", Bytes(rec.GetProperty("chunk_bytes").GetInt64()), Source("recording.chunk_bytes"), false, false));
        rows.Add(new("Recording", "Rotation size", Bytes(rec.GetProperty("rotate_bytes").GetInt64()), Source("recording.rotate_bytes"), false, false));
        rows.Add(new("Recording", "Checkpoint cadence", $"{rec.GetProperty("chunk_flush_ms").GetInt32()} ms", Source("recording.chunk_flush_ms"), false, false));

        var agent = active.GetProperty("agent");
        var agentEnabled = agent.GetProperty("enabled").GetBoolean();
        rows.Add(new("Race engineer", "Enabled", agentEnabled ? "Yes" : "No", Source("agent.enabled"), false, false));
        var endpointId = agent.TryGetProperty("endpoint_id", out var eid) && eid.ValueKind == JsonValueKind.String ? eid.GetString() : null;
        rows.Add(new("Race engineer", "Endpoint", endpointId ?? "not configured", Source("agent.endpoint_id"), false, false));
        JsonElement? endpoint = null;
        if (endpointId is not null && agent.TryGetProperty("endpoints", out var endpoints))
        {
            foreach (var candidate in endpoints.EnumerateArray())
            {
                if (candidate.TryGetProperty("id", out var id) && id.GetString() == endpointId)
                {
                    endpoint = candidate;
                }
            }
        }
        var baseUrl = endpoint?.TryGetProperty("base_url", out var bu) == true ? bu.GetString() : null;
        var model = endpoint?.TryGetProperty("model", out var mo) == true ? mo.GetString() : null;
        var protocol = endpoint?.TryGetProperty("protocol", out var pr) == true ? pr.GetString() : null;
        var keyName = endpoint?.TryGetProperty("api_key_env", out var ke) == true ? ke.GetString() : null;
        rows.Add(new("Race engineer", "URL", string.IsNullOrEmpty(baseUrl) ? "not configured" : baseUrl, endpoint is null ? null : Source("agent.endpoint"), false, false));
        rows.Add(new("Race engineer", "Model", string.IsNullOrEmpty(model) ? "not configured" : model, null, false, false));
        rows.Add(new("Race engineer", "Protocol", string.IsNullOrEmpty(protocol) ? "not configured" : protocol, null, false, false));
        var configured = keyName is not null && root.TryGetProperty("secrets_configured", out var sc) && sc.TryGetProperty(keyName, out var kv) && kv.GetBoolean();
        rows.Add(new("Race engineer", "API key", keyName is null ? "not configured" : configured ? $"Configured ({keyName})" : $"Missing: {keyName}", null, false, false));

        var search = active.GetProperty("search");
        rows.Add(new("Search and voice", "Search provider", search.GetProperty("provider").GetString() ?? "disabled", Source("search.provider"), false, false));
        var voice = active.GetProperty("voice");
        rows.Add(new("Search and voice", "Voice", voice.GetProperty("enabled").GetBoolean() ? "Enabled" : "Disabled", Source("voice.enabled"), false, false));

        rows.Add(new("Deployment", "Configuration file", configurationPath, null, false, false));
        var pending = root.TryGetProperty("restart_required", out var rl) && rl.GetArrayLength() > 0
            ? string.Join(", ", rl.EnumerateArray().Select(x => x.GetString()))
            : "None pending";
        rows.Add(new("Deployment", "Restart required", pending, null, false, false));
        return rows;
    }

    private static string Bytes(long n) => n >= 1_000_000_000 ? $"{n / 1_000_000_000.0:0.#} GB" : $"{n / 1_048_576.0:0.#} MiB";

    /// <summary>
    /// True when an effective value comes from the process environment, so the
    /// native page must not offer an edit that would be overridden. The Windows
    /// shipment is otherwise configured here, not in the web UI.
    /// </summary>
    public static bool EnvironmentLocked(string json, string key)
    {
        using var doc = JsonDocument.Parse(json);
        return doc.RootElement.TryGetProperty("sources", out var sources) &&
               sources.TryGetProperty(key, out var source) &&
               source.GetString() == "env";
    }
}
