package voice

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"f1tele/server/internal/config"
	"f1tele/server/internal/contract"
)

func storeWithVoice(t *testing.T, patch func(*contract.VoiceConfig)) *config.Store {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("F1TELE_DATA_DIR", dir)
	store, err := config.Load(config.Overrides{DataDir: dir, ConfigPath: filepath.Join(dir, "config.json")})
	if err != nil {
		t.Fatal(err)
	}
	cfg, _ := config.Defaults()
	cfg.DataDir = &dir
	cfg.Voice.Enabled = true
	patch(&cfg.Voice)
	rev := store.Revision()
	if _, err := store.ApplyPatch(contract.ConfigPatch{Config: cfg}, &rev); err != nil {
		t.Fatal(err)
	}
	return store
}

func wav() []byte {
	b := make([]byte, 44+3200)
	copy(b, "RIFF")
	b[8] = 'W'
	b[9] = 'A'
	b[10] = 'V'
	b[11] = 'E'
	return b
}

func TestTranscriptionUsesServerKeyAndMultipart(t *testing.T) {
	var sawAuth, sawFile, sawModel bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawAuth = r.Header.Get("Authorization") == "Bearer sk-test"
		if err := r.ParseMultipartForm(2 << 20); err != nil {
			t.Errorf("multipart: %v", err)
		}
		_, fh, err := r.FormFile("file")
		sawFile = err == nil && fh.Filename == "utterance.wav"
		sawModel = r.FormValue("model") == "whisper-test"
		json.NewEncoder(w).Encode(map[string]string{"text": "box this lap"})
	}))
	defer srv.Close()
	env := "F1TELE_TEST_TRANSCRIBE"
	store := storeWithVoice(t, func(v *contract.VoiceConfig) {
		e := srv.URL + "/v1/audio/transcriptions"
		m := "whisper-test"
		v.TranscriptionEndpoint, v.TranscriptionModel, v.TranscriptionAPIKeyEnv = &e, &m, &env
	})
	rev := store.Revision()
	cfg := store.Config()
	if _, err := store.ApplyPatch(contract.ConfigPatch{Config: cfg, Secrets: []contract.SecretInput{{ID: env, Value: "sk-test"}}}, &rev); err != nil {
		t.Fatal(err)
	}
	svc := New(store)
	text, err := svc.Transcribe(context.Background(), wav())
	if err != nil {
		t.Fatal(err)
	}
	if text != "box this lap" || !sawAuth || !sawFile || !sawModel {
		t.Fatalf("text=%q auth=%v file=%v model=%v", text, sawAuth, sawFile, sawModel)
	}
}

func TestSpeechSpeaksAcceptedInvocationOnly(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&got)
		w.Header().Set("Content-Type", "audio/wav")
		w.Write(wav())
	}))
	defer srv.Close()
	env := "F1TELE_TEST_SPEECH"
	store := storeWithVoice(t, func(v *contract.VoiceConfig) {
		e := srv.URL + "/v1/audio/speech"
		m := "tts-test"
		voice := "alloy"
		v.SpeechEndpoint, v.SpeechModel, v.SpeechVoice, v.SpeechAPIKeyEnv = &e, &m, &voice, &env
	})
	rev := store.Revision()
	cfg := store.Config()
	if _, err := store.ApplyPatch(contract.ConfigPatch{Config: cfg, Secrets: []contract.SecretInput{{ID: env, Value: "sk-test"}}}, &rev); err != nil {
		t.Fatal(err)
	}
	svc := New(store)
	svc.AcceptedText = func(_ context.Context, id string) (string, error) { return "Box this lap", nil }
	audio, err := svc.Speech(context.Background(), "inv-1")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(audio, []byte("RIFF")) {
		t.Fatal("not wav")
	}
	if got["input"] != "Box this lap" || got["voice"] != "alloy" || got["response_format"] != "wav" {
		t.Fatalf("payload %#v", got)
	}
	_ = mime.TypeByExtension
	_ = io.Discard
}

func TestVoiceDisabledFails(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("F1TELE_DATA_DIR", dir)
	store, err := config.Load(config.Overrides{DataDir: dir, ConfigPath: filepath.Join(dir, "config.json")})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(store).Transcribe(context.Background(), wav()); err == nil {
		t.Fatal("disabled voice accepted a request")
	}
}
