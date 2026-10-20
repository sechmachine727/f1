using F1Telemetry.ControlCenter.Core;
using Xunit;

namespace F1Telemetry.ControlCenter.Tests;

public sealed class ConfigurationViewTests
{
    private const string BaseJson = """
    {
      "revision": 2,
      "active_config": {
        "http": { "host": "127.0.0.1", "port": 8081 },
        "udp": { "host": "0.0.0.0", "port": 20777 },
        "data_dir": "C:/data",
        "recording": { "enabled": true, "cap_bytes": 50000000000, "free_reserve_bytes": 5368709120,
                        "chunk_bytes": 1048576, "rotate_bytes": 268435456, "chunk_flush_ms": 1000 },
        "agent": { "enabled": true, "endpoint_id": "main",
                   "endpoints": [{ "id": "main", "base_url": "https://example.invalid/v1", "model": "m",
                                   "protocol": "responses", "api_key_env": "F1TELE_AGENT_API_KEY" }] },
        "search": { "provider": "disabled" },
        "voice": { "enabled": false }
      },
      "secrets_configured": { "F1TELE_AGENT_API_KEY": true },
      "restart_required": [ "udp" ],
      "sources": { "http": "env", "udp": "cli", "data_dir": "env", "recording.cap_bytes": "file" },
      "udp_advertise_host": "192.168.1.20"
    }
    """;

    [Fact]
    public void Shows_effective_values_sources_and_restart_state()
    {
        var rows = ConfigurationView.Rows(BaseJson, @"C:\data\config.json");
        Assert.Contains(rows, r => r.Name == "Telemetry (UDP)" && r.Value == "0.0.0.0:20777" && r.Source == "cli" && r.RestartPending);
        Assert.Contains(rows, r => r.Name == "Dashboard" && r.Source == "env" && !r.RestartPending);
        Assert.Contains(rows, r => r.Name == "Storage cap" && r.Value == "50 GB" && r.Source == "file");
        Assert.Contains(rows, r => r.Name == "Game address" && r.Value == "192.168.1.20:20777");
        Assert.Contains(rows, r => r.Name == "Configuration file" && r.Value == @"C:\data\config.json");
        Assert.Contains(rows, r => r.Section == "Deployment" && r.Name == "Restart required" && r.Value == "udp");
    }

    [Fact]
    public void Reports_configured_key_without_its_value()
    {
        var rows = ConfigurationView.Rows(BaseJson, "config.json");
        var key = Assert.Single(rows, r => r.Name == "API key");
        Assert.Equal("Configured (F1TELE_AGENT_API_KEY)", key.Value);
        Assert.DoesNotContain(rows, r => r.Value.Contains("sk-", StringComparison.Ordinal));
    }

    [Fact]
    public void Reports_missing_key_and_disabled_features()
    {
        var json = BaseJson.Replace("\"F1TELE_AGENT_API_KEY\": true", "\"F1TELE_AGENT_API_KEY\": false")
                           .Replace("\"enabled\": false", "\"enabled\": true");
        var rows = ConfigurationView.Rows(json, "config.json");
        Assert.Contains(rows, r => r.Name == "API key" && r.Value == "Missing: F1TELE_AGENT_API_KEY");
        Assert.Contains(rows, r => r.Section == "Search and voice" && r.Name == "Voice" && r.Value == "Enabled");
    }

    [Fact]
    public void Unconfigured_endpoint_states_so()
    {
        var json = BaseJson.Replace("\"endpoint_id\": \"main\"", "\"endpoint_id\": null");
        var rows = ConfigurationView.Rows(json, "config.json");
        Assert.Contains(rows, r => r.Name == "Endpoint" && r.Value == "not configured");
        Assert.Contains(rows, r => r.Name == "API key" && r.Value == "not configured");
    }

    [Fact]
    public void Environment_locked_values_are_identifiable_for_native_edits()
    {
        Assert.True(ConfigurationView.EnvironmentLocked(BaseJson, "http"));
        Assert.False(ConfigurationView.EnvironmentLocked(BaseJson, "recording.enabled"));
        Assert.False(ConfigurationView.EnvironmentLocked("{}", "http"));
    }
}
