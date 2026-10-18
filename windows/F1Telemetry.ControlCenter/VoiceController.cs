using System.Net.Http.Headers;
using System.Net.Http.Json;
using System.Runtime.InteropServices;
using System.Text.Json;
using F1Telemetry.ControlCenter.Core;
using Windows.Media.Capture;
using Windows.Media.MediaProperties;
using Windows.Media.Playback;
using Windows.Storage.Streams;
using WinRT.Interop;

namespace F1Telemetry.ControlCenter;

/// <summary>
/// Native push-to-talk: MediaCapture writes WAV PCM16 mono 16 kHz to a bounded
/// buffer, the transcript becomes a live_question request, and MediaPlayer plays
/// the returned speech. Keys and model endpoints stay on the local server.
/// </summary>
public sealed class VoiceController : IDisposable
{
    private readonly MainWindow _window;
    private readonly HttpClient _http;
    private readonly VoiceTurn _turn = new();
    private readonly MediaPlayer _player = new();
    private MediaCapture? _capture;
    private InMemoryRandomAccessStream? _stream;
    private DateTimeOffset _recordingStarted;
    private IntPtr _hwnd;

    public VoiceController(MainWindow window, HttpClient http)
    {
        _window = window;
        _http = http;
    }

    public bool RegisterHotkey()
    {
        _hwnd = WindowNative.GetWindowHandle(_window);
        var ok = NativeMethods.RegisterHotKey(_hwnd, NativeMethods.PTT_HOTKEY_ID, NativeMethods.MOD_CONTROL | NativeMethods.MOD_ALT, 0x20 /* VK_SPACE */);
        if (!ok)
        {
            _window.SetVoiceState("Push-to-talk key unavailable (another app owns Ctrl+Alt+Space). Rebind in Settings; text requests still work.");
        }
        return ok;
    }

    public async Task HandleHotkeyAsync()
    {
        if (NativeMethods.GetAsyncKeyState(0x20) == 0)
        {
            return;
        }
        if (_turn.State == "Recording")
        {
            await StopAndTranscribeAsync();
            return;
        }
        if (_turn.State == "Speaking")
        {
            // PTT interrupts prior speech.
            _player.Pause();
        }
        await StartCaptureAsync();
    }

    private async Task StartCaptureAsync()
    {
        try
        {
            _capture ??= new MediaCapture();
            if (_stream is null)
            {
                await _capture.InitializeAsync(new MediaCaptureInitializationSettings
                {
                    StreamingCaptureMode = StreamingCaptureMode.Audio,
                });
            }
            _stream = new InMemoryRandomAccessStream();
            var profile = MediaEncodingProfile.CreateWav(AudioEncodingQuality.Low);
            profile.Audio = AudioEncodingProperties.CreatePcm(16000, 1, 16);
            await _capture.StartRecordToStreamAsync(profile, _stream);
            _recordingStarted = DateTimeOffset.UtcNow;
            _turn.Begin(_recordingStarted, out _);
            _window.SetVoiceState("Recording — release to send");
            _ = MonitorReleaseAsync();
        }
        catch (Exception ex)
        {
            _turn.Fail(ex.Message);
            _window.SetVoiceState("Microphone unavailable: " + ex.Message + ". Text requests remain available.");
        }
    }

    private async Task MonitorReleaseAsync()
    {
        // 16 ms key-state polling only while an utterance is active.
        while (_turn.State == "Recording")
        {
            await Task.Delay(16);
            var released = NativeMethods.GetAsyncKeyState(0x20) == 0;
            var capped = _turn.Exceeded(_recordingStarted, DateTimeOffset.UtcNow);
            if (!released && !capped)
            {
                continue;
            }
            if (capped)
            {
                _window.SetVoiceState("Utterance stopped at the 30-second limit.");
            }
            await StopAndTranscribeAsync();
            return;
        }
    }

    private async Task StopAndTranscribeAsync()
    {
        if (_capture is null || _stream is null)
        {
            _turn.Finish();
            return;
        }
        try
        {
            await _capture.StopRecordAsync();
            _stream.Seek(0);
            var bytes = new byte[_stream.Size];
            using var reader = new DataReader(_stream.GetInputStreamAt(0));
            await reader.LoadAsync((uint)_stream.Size);
            reader.ReadBytes(bytes);
            if (!WavFormat.IsSupported(bytes, out var err))
            {
                _turn.Fail(err!);
                _window.SetVoiceState("Voice error: " + err);
                return;
            }
            _turn.ToTranscribing();
            _window.SetVoiceState("Transcribing…");
            using var content = new ByteArrayContent(bytes);
            content.Headers.ContentType = new MediaTypeHeaderValue("audio/wav");
            using var resp = await _http.PostAsync("http://127.0.0.1:8081/api/v1/voice/transcriptions", content);
            if (!resp.IsSuccessStatusCode)
            {
                _turn.Fail("transcription failed");
                _window.SetVoiceState("Transcription failed: " + resp.ReasonPhrase);
                return;
            }
            var transcript = await resp.Content.ReadFromJsonAsync<JsonElement>();
            var text = transcript.GetProperty("text").GetString() ?? string.Empty;
            if (string.IsNullOrWhiteSpace(text))
            {
                _turn.Fail("empty transcript");
                _window.SetVoiceState("No speech recognized. Try again or type the question.");
                return;
            }
            _turn.ToThinking();
            _window.SetVoiceState("Thinking: " + text);
            var session = await _http.GetFromJsonAsync<JsonElement>("http://127.0.0.1:8081/api/v1/health");
            var sessionId = session.TryGetProperty("active_session_id", out var sid) ? sid.GetString() : null;
            if (sessionId is null)
            {
                _turn.Fail("no session");
                _window.SetVoiceState("No active session yet; the question was not sent.");
                return;
            }
            var req = new { profile = "live_question", session_id = sessionId, branch_id = session.GetProperty("instance_id").GetString(), observation_seq = (string?)null, request_text = text };
            using var invocation = await _http.PostAsJsonAsync("http://127.0.0.1:8081/api/v1/engineer/requests", req);
            if (!invocation.IsSuccessStatusCode)
            {
                _turn.Fail("request failed");
                _window.SetVoiceState("Engineer request failed: " + invocation.ReasonPhrase);
                return;
            }
            var inv = await invocation.Content.ReadFromJsonAsync<JsonElement>();
            var invId = inv.GetProperty("id").GetString();
            await SpeakAsync(invId);
        }
        catch (Exception ex)
        {
            _turn.Fail(ex.Message);
            _window.SetVoiceState("Voice error: " + ex.Message);
        }
        finally
        {
            _stream = null;
        }
    }

    private async Task SpeakAsync(string? invocationId)
    {
        if (invocationId is null)
        {
            _turn.Finish();
            return;
        }
        using var req = new HttpRequestMessage(HttpMethod.Post, "http://127.0.0.1:8081/api/v1/voice/speech")
        {
            Content = JsonContent.Create(new { invocation_id = invocationId }),
        };
        using var resp = await _http.SendAsync(req);
        if (!resp.IsSuccessStatusCode)
        {
            _turn.Fail("speech failed");
            _window.SetVoiceState("Speech unavailable; the text answer remains in the engineer panel.");
            return;
        }
        var wav = await resp.Content.ReadAsByteArrayAsync();
        var file = Path.Combine(Path.GetTempPath(), "f1tele-speech.wav");
        await File.WriteAllBytesAsync(file, wav);
        _turn.ToSpeaking();
        _window.SetVoiceState("Speaking…");
        _player.Source = Windows.Media.Core.MediaSource.CreateFromUri(new Uri(file));
        _player.Play();
        _turn.Finish();
    }

    public void Dispose()
    {
        _capture?.Dispose();
        _player.Dispose();
    }
}
