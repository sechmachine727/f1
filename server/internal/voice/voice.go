// Package voice proxies speech and transcription to the configured provider.
// Keys stay on the server; callers send audio bytes or an invocation ID only.
package voice

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"f1tele/server/internal/config"
	"f1tele/server/internal/contract"
)

type Service struct {
	Store  *config.Store
	Client *http.Client
	// AcceptedText returns the driver message of an accepted invocation.
	AcceptedText func(ctx context.Context, id string) (string, error)
}

func New(store *config.Store) *Service {
	return &Service{Store: store, Client: &http.Client{Timeout: 30 * time.Second}}
}

func (s *Service) cfg() (contract.VoiceConfig, error) {
	c := s.Store.Config().Voice
	if !c.Enabled {
		return c, errors.New("voice is disabled in configuration")
	}
	if c.TranscriptionEndpoint == nil || c.TranscriptionModel == nil {
		return c, errors.New("transcription endpoint and model are required")
	}
	return c, nil
}

func (s *Service) Transcribe(ctx context.Context, wav []byte) (string, error) {
	c, err := s.cfg()
	if err != nil {
		return "", err
	}
	var body bytes.Buffer
	ww := multipart.NewWriter(&body)
	if err := ww.WriteField("model", *c.TranscriptionModel); err != nil {
		return "", err
	}
	if err := ww.WriteField("response_format", "json"); err != nil {
		return "", err
	}
	part, err := ww.CreateFormFile("file", "utterance.wav")
	if err != nil {
		return "", err
	}
	if _, err := part.Write(wav); err != nil {
		return "", err
	}
	if err := ww.Close(); err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, *c.TranscriptionEndpoint, &body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", ww.FormDataContentType())
	s.authorize(req, c.TranscriptionAPIKeyEnv)
	resp, err := s.Client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return "", fmt.Errorf("transcription provider returned %d", resp.StatusCode)
	}
	var parsed struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", fmt.Errorf("transcription provider returned malformed JSON: %w", err)
	}
	if strings.TrimSpace(parsed.Text) == "" {
		return "", errors.New("transcription was empty")
	}
	return parsed.Text, nil
}

func (s *Service) Speech(ctx context.Context, invocationID string) ([]byte, error) {
	c := s.Store.Config().Voice
	if !c.Enabled || c.SpeechEndpoint == nil || c.SpeechModel == nil {
		return nil, errors.New("speech endpoint and model are required")
	}
	if s.AcceptedText == nil {
		return nil, errors.New("speech source is unavailable")
	}
	text, err := s.AcceptedText(ctx, invocationID)
	if err != nil {
		return nil, err
	}
	payload := map[string]any{"model": *c.SpeechModel, "input": text, "response_format": "wav"}
	if c.SpeechVoice != nil {
		payload["voice"] = *c.SpeechVoice
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, *c.SpeechEndpoint, bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	s.authorize(req, c.SpeechAPIKeyEnv)
	resp, err := s.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	audio, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("speech provider returned %d", resp.StatusCode)
	}
	if len(audio) < 4 || string(audio[:4]) != "RIFF" {
		return nil, errors.New("speech provider did not return WAV audio")
	}
	return audio, nil
}

func (s *Service) authorize(req *http.Request, env *string) {
	if env == nil || *env == "" {
		return
	}
	if key := s.Store.Secret(*env); key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
}
