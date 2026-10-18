using System.Diagnostics;
using System.Net.Http.Json;
using System.Text.Json.Serialization;

namespace F1Telemetry.ControlCenter.Core;

/// <summary>Health payload fields the supervisor relies on.</summary>
public sealed record HealthInfo(
    [property: JsonPropertyName("instance_id")] string InstanceId,
    [property: JsonPropertyName("server_version")] string ServerVersion,
    [property: JsonPropertyName("state")] string State,
    [property: JsonPropertyName("recording_state")] string RecordingState,
    [property: JsonPropertyName("last_packet_at_ms")] long? LastPacketAtMs);

public enum SupervisorState { Stopped, Starting, Running, Stopping, Failed, PortConflict, Crashed }

public sealed record OwnershipRecord(int Pid, string ExecutablePath, DateTimeOffset StartedAt, string InstanceId);

public sealed record SupervisorStatus(
    SupervisorState State,
    string Detail,
    string? DashboardUrl,
    IReadOnlyList<string> LanAddresses,
    HealthInfo? Health,
    OwnershipRecord? Owned);

/// <summary>
/// Owns exactly one bundled server child. Identification uses PID plus
/// executable path plus start time plus health instance id; a port alone is
/// never treated as ownership.
/// </summary>
public sealed class ServerSupervisor : IAsyncDisposable
{
    private readonly IServerProcessLauncher _launcher;
    private readonly IHealthProbe _health;
    private readonly string _dataDir;
    private readonly string _configPath;
    private readonly TimeSpan _readinessBudget;
    private readonly TimeProvider _time;
    private readonly HttpClient _http;
    private IServerProcess? _child;
    private OwnershipRecord? _owned;

    public ServerSupervisor(
        IServerProcessLauncher launcher,
        IHealthProbe health,
        string dataDir,
        string configPath,
        HttpClient? http = null,
        TimeSpan? readinessBudget = null,
        TimeProvider? time = null)
    {
        _launcher = launcher;
        _health = health;
        _dataDir = dataDir;
        _configPath = configPath;
        _http = http ?? new HttpClient { Timeout = TimeSpan.FromSeconds(5) };
        _readinessBudget = readinessBudget ?? TimeSpan.FromSeconds(10);
        _time = time ?? TimeProvider.System;
    }

    public SupervisorStatus Status { get; private set; } =
        new(SupervisorState.Stopped, "Server not started.", null, Array.Empty<string>(), null, null);

    public OwnershipRecord? Owned => _owned;

    /// <summary>Starts the bundled server, waits for readiness, then opens the dashboard URL.</summary>
    public async Task<SupervisorStatus> StartAsync(string bundledExecutable, string? persistedOwnershipPath, CancellationToken ct = default)
    {
        if (_child is { HasExited: false })
        {
            return Status;
        }
        if (await TryAdoptAsync(bundledExecutable, persistedOwnershipPath, ct))
        {
            return Status;
        }
        Status = Status with { State = SupervisorState.Starting, Detail = "Starting bundled server..." };
        var started = _time.GetUtcNow();
        try
        {
            _child = _launcher.Start(bundledExecutable, new[]
            {
                "serve", "--data-dir", _dataDir, "--config", _configPath,
            }, _dataDir);
        }
        catch (Exception ex)
        {
            Status = Status with { State = SupervisorState.Failed, Detail = "Launch failed: " + ex.Message };
            return Status;
        }

        var deadline = _time.GetUtcNow() + _readinessBudget;
        while (_time.GetUtcNow() < deadline)
        {
            if (_child.HasExited)
            {
                var code = _child.ExitCode;
                Status = Status with
                {
                    State = code == 2 ? SupervisorState.PortConflict : SupervisorState.Crashed,
                    Detail = code == 2
                        ? "The selected port is already in use. Choose another port in Settings."
                        : $"Server exited during startup with code {code}.",
                };
                return Status;
            }
            var health = await _health.ProbeAsync(ct);
            if (health is not null)
            {
                _owned = new OwnershipRecord(_child.Pid, _child.ExecutablePath, started, health.InstanceId);
                if (persistedOwnershipPath is not null)
                {
                    await OwnershipStore.SaveAsync(persistedOwnershipPath, _owned, ct);
                }
                Status = new SupervisorStatus(SupervisorState.Running, "Server running.", DashboardUrl(health),
                    DiscoverLanAddresses(), health, _owned);
                return Status;
            }
            await Task.Delay(200, ct);
        }
        Status = Status with { State = SupervisorState.Failed, Detail = "Server did not become ready within 10 seconds. Check the log path and UDP/HTTP ports." };
        return Status;
    }

    /// <summary>Adopts only a child whose PID, executable and health instance id match.</summary>
    public async Task<bool> TryAdoptAsync(string bundledExecutable, string? persistedOwnershipPath, CancellationToken ct)
    {
        if (persistedOwnershipPath is null)
        {
            return false;
        }
        var saved = await OwnershipStore.LoadAsync(persistedOwnershipPath, ct);
        if (saved is null)
        {
            return false;
        }
        var process = _launcher.Find(saved.Pid);
        if (process is null || process.HasExited || !SameExecutable(process.ExecutablePath, bundledExecutable))
        {
            return false;
        }
        var health = await _health.ProbeAsync(ct);
        if (health is null || health.InstanceId != saved.InstanceId)
        {
            return false;
        }
        _child = process;
        _owned = saved;
        Status = new SupervisorStatus(SupervisorState.Running, "Adopted the previously owned server.", DashboardUrl(health), DiscoverLanAddresses(), health, saved);
        return true;
    }

    public static bool SameExecutable(string a, string b) =>
        string.Equals(Path.GetFullPath(a), Path.GetFullPath(b), StringComparison.OrdinalIgnoreCase);

    /// <summary>
    /// Explicit stop: POST control/stop, then wait for the owned child to exit.
    /// The caller chooses retry or force after the 10-second grace period.
    /// </summary>
    public async Task<SupervisorStatus> StopAsync(TimeSpan grace, CancellationToken ct = default)
    {
        if (_child is null || _child.HasExited)
        {
            ClearOwnership();
            Status = new SupervisorStatus(SupervisorState.Stopped, "Server stopped.", null, Array.Empty<string>(), null, null);
            return Status;
        }
        Status = Status with { State = SupervisorState.Stopping, Detail = "Flushing recording and stopping server..." };
        try
        {
            using var resp = await _http.PostAsync("http://127.0.0.1:8081/api/v1/control/stop", content: null, ct);
        }
        catch (Exception ex)
        {
            Status = Status with { Detail = "Stop request failed: " + ex.Message };
        }
        var deadline = _time.GetUtcNow() + grace;
        while (_time.GetUtcNow() < deadline)
        {
            if (_child.HasExited)
            {
                ClearOwnership();
                Status = new SupervisorStatus(SupervisorState.Stopped, "Recording flushed; server stopped.", null, Array.Empty<string>(), null, null);
                return Status;
            }
            await Task.Delay(200, ct);
        }
        Status = Status with
        {
            State = SupervisorState.Stopping,
            Detail = "The server has not stopped within 10 seconds. Retry, or force stop and accept possible loss of unflushed recording data.",
        };
        return Status;
    }

    /// <summary>Only valid after an explicit user confirmation.</summary>
    public void ForceStop()
    {
        if (_child is null || _child.HasExited)
        {
            return;
        }
        _child.Kill();
        ClearOwnership();
        Status = new SupervisorStatus(SupervisorState.Stopped, "Server force-stopped; unflushed recording data may be lost.", null, Array.Empty<string>(), null, null);
    }

    private void ClearOwnership()
    {
        _child = null;
        _owned = null;
    }

    public static string DashboardUrl(HealthInfo? health = null) => "http://127.0.0.1:8081";

    public static IReadOnlyList<string> DiscoverLanAddresses()
    {
        var result = new List<string>();
        foreach (var ni in System.Net.NetworkInformation.NetworkInterface.GetAllNetworkInterfaces())
        {
            if (ni.OperationalStatus != System.Net.NetworkInformation.OperationalStatus.Up)
            {
                continue;
            }
            foreach (var addr in ni.GetIPProperties().UnicastAddresses)
            {
                if (addr.Address.AddressFamily == System.Net.Sockets.AddressFamily.InterNetwork)
                {
                    result.Add($"http://{addr.Address}:8081");
                }
            }
        }
        return result;
    }

    public ValueTask DisposeAsync()
    {
        _http.Dispose();
        return ValueTask.CompletedTask;
    }
}

public static class OwnershipStore
{
    public static async Task SaveAsync(string path, OwnershipRecord record, CancellationToken ct)
    {
        Directory.CreateDirectory(Path.GetDirectoryName(path)!);
        var json = System.Text.Json.JsonSerializer.Serialize(record);
        await File.WriteAllTextAsync(path, json, ct);
    }

    public static async Task<OwnershipRecord?> LoadAsync(string path, CancellationToken ct)
    {
        if (!File.Exists(path))
        {
            return null;
        }
        try
        {
            return System.Text.Json.JsonSerializer.Deserialize<OwnershipRecord>(await File.ReadAllTextAsync(path, ct));
        }
        catch (System.Text.Json.JsonException)
        {
            return null;
        }
    }
}
