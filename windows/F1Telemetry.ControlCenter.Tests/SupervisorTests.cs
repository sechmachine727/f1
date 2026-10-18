using F1Telemetry.ControlCenter.Core;
using Xunit;

namespace F1Telemetry.ControlCenter.Tests;

public sealed class FakeProcess : IServerProcess
{
    public int Pid { get; init; } = 4242;
    public string ExecutablePath { get; init; } = @"C:\app\f1tele.exe";
    public bool HasExited { get; set; }
    public int ExitCode { get; set; }
    public bool Killed { get; private set; }
    public void Kill() { HasExited = true; Killed = true; }
}

public sealed class FakeLauncher : IServerProcessLauncher
{
    public FakeProcess Process { get; } = new();
    public int Starts;
    public IServerProcess Start(string executable, IReadOnlyList<string> args, string workingDirectory)
    {
        Starts++;
        return Process;
    }
    public IServerProcess? Find(int pid) => pid == Process.Pid && !Process.HasExited ? Process : null;
}

public sealed class FakeHealth : IHealthProbe
{
    public HealthInfo? Response { get; set; }
    public Task<HealthInfo?> ProbeAsync(CancellationToken ct) => Task.FromResult(Response);
}

public sealed class SupervisorTests
{
    private static ServerSupervisor Build(FakeLauncher l, FakeHealth h, string dir) =>
        new(l, h, dir, Path.Combine(dir, "config.json"), new HttpClient(), TimeSpan.FromSeconds(1));

    [Fact]
    public async Task Starts_owned_child_and_reports_dashboard()
    {
        var dir = Directory.CreateTempSubdirectory().FullName;
        var launcher = new FakeLauncher();
        var health = new FakeHealth { Response = new HealthInfo("inst-1", "1.0.0", "awaiting_telemetry", "recording", null) };
        var sup = Build(launcher, health, dir);
        var st = await sup.StartAsync(@"C:\app\f1tele.exe", Path.Combine(dir, "owner.json"));
        Assert.Equal(SupervisorState.Running, st.State);
        Assert.Equal("inst-1", st.Owned!.InstanceId);
        Assert.Equal(1, launcher.Starts);
    }

    [Fact]
    public async Task Does_not_adopt_unrelated_service_on_port()
    {
        var dir = Directory.CreateTempSubdirectory().FullName;
        var launcher = new FakeLauncher();
        var health = new FakeHealth { Response = new HealthInfo("other-instance", "9.9.9", "live", "recording", null) };
        var ownerPath = Path.Combine(dir, "owner.json");
        await OwnershipStore.SaveAsync(ownerPath, new OwnershipRecord(4242, @"C:\app\f1tele.exe", DateTimeOffset.UtcNow, "inst-1"), default);
        var sup = Build(launcher, health, dir);
        await sup.TryAdoptAsync(@"C:\app\f1tele.exe", ownerPath, default);
        Assert.NotEqual(SupervisorState.Running, sup.Status.State);
        Assert.Equal(0, launcher.Starts);
    }

    [Fact]
    public async Task Adopts_matching_child_identity()
    {
        var dir = Directory.CreateTempSubdirectory().FullName;
        var launcher = new FakeLauncher();
        var health = new FakeHealth { Response = new HealthInfo("inst-1", "1.0.0", "live", "recording", null) };
        var ownerPath = Path.Combine(dir, "owner.json");
        await OwnershipStore.SaveAsync(ownerPath, new OwnershipRecord(4242, @"C:\app\f1tele.exe", DateTimeOffset.UtcNow, "inst-1"), default);
        var sup = Build(launcher, health, dir);
        Assert.True(await sup.TryAdoptAsync(@"C:\app\f1tele.exe", ownerPath, default));
        Assert.Equal(SupervisorState.Running, sup.Status.State);
        Assert.Equal(0, launcher.Starts);
    }

    [Fact]
    public async Task Attribute_rejects_different_executable()
    {
        Assert.False(ServerSupervisor.SameExecutable(@"C:\app\f1tele.exe", @"C:\app\other.exe"));
    }

    [Fact]
    public async Task Stop_flushes_then_reports_stopped()
    {
        var dir = Directory.CreateTempSubdirectory().FullName;
        var launcher = new FakeLauncher();
        var health = new FakeHealth { Response = new HealthInfo("inst-1", "1.0.0", "live", "recording", null) };
        var sup = Build(launcher, health, dir);
        await sup.StartAsync(@"C:\app\f1tele.exe", Path.Combine(dir, "owner.json"));
        var task = sup.StopAsync(TimeSpan.FromMilliseconds(200));
        launcher.Process.HasExited = true;
        var st = await task;
        Assert.Equal(SupervisorState.Stopped, st.State);
    }

    [Fact]
    public async Task Stop_timeout_offers_retry_without_killing()
    {
        var dir = Directory.CreateTempSubdirectory().FullName;
        var launcher = new FakeLauncher();
        var health = new FakeHealth { Response = new HealthInfo("inst-1", "1.0.0", "live", "recording", null) };
        var sup = Build(launcher, health, dir);
        await sup.StartAsync(@"C:\app\f1tele.exe", Path.Combine(dir, "owner.json"));
        var st = await sup.StopAsync(TimeSpan.FromMilliseconds(150));
        Assert.Equal(SupervisorState.Stopping, st.State);
        Assert.False(launcher.Process.Killed);
        Assert.Contains("Retry", st.Detail);
    }

    [Fact]
    public async Task Crash_preserves_exit_code()
    {
        var dir = Directory.CreateTempSubdirectory().FullName;
        var launcher = new FakeLauncher();
        var health = new FakeHealth();
        var sup = Build(launcher, health, dir);
        launcher.Process.HasExited = true;
        launcher.Process.ExitCode = 7;
        var st = await sup.StartAsync(@"C:\app\f1tele.exe", null);
        Assert.Equal(SupervisorState.Crashed, st.State);
        Assert.Contains("7", st.Detail);
    }

    [Fact]
    public void Voice_turn_caps_and_interrupts()
    {
        var turn = new VoiceTurn();
        var start = DateTimeOffset.UtcNow;
        Assert.True(turn.Begin(start, out _));
        Assert.True(turn.Exceeded(start, start.AddSeconds(31)));
        turn.ToSpeaking();
        Assert.True(turn.Begin(start, out _));
        Assert.Equal("Recording", turn.State);
        turn.Muted = true;
        turn.ToSpeaking();
        Assert.Equal("Recording", turn.State);
    }

    [Fact]
    public void Wav_validation_rejects_wrong_format_and_long_audio()
    {
        var good = new byte[44 + 16000 * 2];
        good[0] = (byte)'R'; good[1] = (byte)'I'; good[2] = (byte)'F'; good[3] = (byte)'F';
        BitConverter.GetBytes((short)1).CopyTo(good, 22);
        BitConverter.GetBytes(16000).CopyTo(good, 24);
        BitConverter.GetBytes((short)16).CopyTo(good, 34);
        Assert.True(WavFormat.IsSupported(good, out _));
        var stereo = (byte[])good.Clone();
        BitConverter.GetBytes((short)2).CopyTo(stereo, 22);
        Assert.False(WavFormat.IsSupported(stereo, out _));
        var longAudio = new byte[44 + 16000 * 2 * 31];
        good.AsSpan(0, 44).CopyTo(longAudio);
        Assert.False(WavFormat.IsSupported(longAudio, out var err));
        Assert.Contains("30", err);
    }
}
