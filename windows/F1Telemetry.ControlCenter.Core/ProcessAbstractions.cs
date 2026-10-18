namespace F1Telemetry.ControlCenter.Core;

public interface IServerProcess
{
    int Pid { get; }
    string ExecutablePath { get; }
    bool HasExited { get; }
    int ExitCode { get; }
    void Kill();
}

public interface IServerProcessLauncher
{
    IServerProcess Start(string executable, IReadOnlyList<string> args, string workingDirectory);
    IServerProcess? Find(int pid);
}

public interface IHealthProbe
{
    Task<HealthInfo?> ProbeAsync(CancellationToken ct);
}
