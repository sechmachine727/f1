using System.Diagnostics;
using System.Net.Http.Json;

namespace F1Telemetry.ControlCenter.Core;

public sealed class SystemProcess : IServerProcess
{
    private readonly Process _process;
    public SystemProcess(Process process, string executablePath)
    {
        _process = process;
        ExecutablePath = executablePath;
    }
    public int Pid => _process.Id;
    public string ExecutablePath { get; }
    public bool HasExited => _process.HasExited;
    public int ExitCode => _process.ExitCode;
    public void Kill() => _process.Kill(entireProcessTree: true);
}

public sealed class SystemLauncher : IServerProcessLauncher
{
    private readonly Dictionary<int, Process> _started = new();

    public IServerProcess Start(string executable, IReadOnlyList<string> args, string workingDirectory)
    {
        var psi = new ProcessStartInfo(executable)
        {
            WorkingDirectory = workingDirectory,
            UseShellExecute = false,
            CreateNoWindow = true,
        };
        foreach (var a in args)
        {
            psi.ArgumentList.Add(a);
        }
        var p = Process.Start(psi) ?? throw new InvalidOperationException("Process.Start returned null");
        _started[p.Id] = p;
        return new SystemProcess(p, executable);
    }

    public IServerProcess? Find(int pid)
    {
        if (_started.TryGetValue(pid, out var known) && !known.HasExited)
        {
            return new SystemProcess(known, known.MainModule?.FileName ?? string.Empty);
        }
        try
        {
            var p = Process.GetProcessById(pid);
            return new SystemProcess(p, p.MainModule?.FileName ?? string.Empty);
        }
        catch (ArgumentException)
        {
            return null;
        }
    }
}

public sealed class HttpHealthProbe : IHealthProbe
{
    private readonly HttpClient _http;
    public HttpHealthProbe(HttpClient http) => _http = http;

    public async Task<HealthInfo?> ProbeAsync(CancellationToken ct)
    {
        try
        {
            return await _http.GetFromJsonAsync<HealthInfo>("http://127.0.0.1:8081/api/v1/health", ct);
        }
        catch (Exception)
        {
            return null;
        }
    }
}
