using System.Text.Json.Nodes;
using F1Telemetry.ControlCenter.Core;
using Xunit;

namespace F1Telemetry.ControlCenter.Tests;

public sealed class ServerConfigFileTests
{
    [Fact]
    public void Loads_defaults_merges_local_values_and_saves_secrets()
    {
        var dir = Path.Combine(Path.GetTempPath(), "f1tele-settings-" + Guid.NewGuid().ToString("N"));
        Directory.CreateDirectory(dir);
        var defaults = Path.Combine(dir, "defaults.json");
        File.WriteAllText(defaults, """
        { "http": { "host": "0.0.0.0", "port": 8081 }, "agent": { "enabled": false, "endpoints": [] } }
        """);
        File.WriteAllText(Path.Combine(dir, "config.json"), """{ "http": { "port": 9090 } }""");
        var config = ServerConfigFile.Load(dir, defaults);
        Assert.Equal("0.0.0.0", config.GetString("http.host"));
        Assert.Equal(9090, config.GetInt("http.port", 0));
        config.SetBool("security.origin_check", true);
        config.SetString("agent.endpoint_id", "windows");
        config.Set("agent.endpoints", new JsonArray(new JsonObject { ["id"] = "windows", ["model"] = "m" }));
        config.SetSecret("F1TELE_AGENT_API_KEY", "secret");
        config.Save(dir);
        var reloaded = ServerConfigFile.Load(dir, defaults);
        Assert.True(reloaded.GetBool("security.origin_check", false));
        Assert.Equal("windows", reloaded.GetString("agent.endpoint_id"));
        Assert.Equal("secret", reloaded.Secrets["F1TELE_AGENT_API_KEY"]);
        Assert.Equal(9090, reloaded.GetInt("http.port", 0));
    }
}
