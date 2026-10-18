namespace F1Telemetry.ControlCenter.Core;

/// <summary>Voice turn rules shared by the native capture path and tests.</summary>
public sealed class VoiceTurn
{
    public static readonly TimeSpan MaxUtterance = TimeSpan.FromSeconds(30);
    public string State { get; private set; } = "Idle";
    public bool Muted { get; set; }

    public bool Begin(DateTimeOffset now, out string? error)
    {
        error = null;
        if (State is "Recording" or "Transcribing" or "Thinking" or "Speaking")
        {
            // Push-to-talk interrupts prior playback.
            State = "Recording";
            return true;
        }
        State = "Recording";
        return true;
    }

    public bool Exceeded(DateTimeOffset startedAt, DateTimeOffset now) => now - startedAt >= MaxUtterance;

    public void ToTranscribing() => State = "Transcribing";
    public void ToThinking() => State = "Thinking";
    public void ToSpeaking() { if (!Muted) State = "Speaking"; }
    public void Finish() => State = "Idle";
    public void Fail(string message) { State = "Error"; LastError = message; }
    public string? LastError { get; private set; }
}

public static class WavFormat
{
    public const int SampleRate = 16000;
    public const short Channels = 1;
    public const short BitsPerSample = 16;
    public const int MaxBytes = 1 << 20;

    /// <summary>Validates the bytes the server route accepts: <=30s, mono, 16 kHz, PCM 16-bit, <=1 MiB.</summary>
    public static bool IsSupported(byte[] wav, out string? error)
    {
        error = null;
        if (wav.Length < 44 || wav[0] != 'R' || wav[1] != 'I' || wav[2] != 'F' || wav[3] != 'F')
        {
            error = "Not a RIFF/WAV file.";
            return false;
        }
        if (wav.Length > MaxBytes)
        {
            error = "Audio exceeds 1 MiB.";
            return false;
        }
        var channels = BitConverter.ToInt16(wav, 22);
        var rate = BitConverter.ToInt32(wav, 24);
        var bits = BitConverter.ToInt16(wav, 34);
        if (channels != Channels || rate != SampleRate || bits != BitsPerSample)
        {
            error = "Audio must be mono 16 kHz PCM 16-bit.";
            return false;
        }
        var seconds = (wav.Length - 44) / (double)(SampleRate * Channels * BitsPerSample / 8);
        if (seconds > 30.0)
        {
            error = "Utterance exceeds 30 seconds.";
            return false;
        }
        return true;
    }
}
