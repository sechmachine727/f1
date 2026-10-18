// Package setup searches configured providers and retains inspected source provenance.
package setup

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"f1tele/server/internal/config"
	"f1tele/server/internal/contract"
	"f1tele/server/internal/history"
	"f1tele/server/internal/httpapi"
)

type Service struct {
	Store   *config.Store
	Catalog *history.Catalog
	Client  *http.Client
	Now     func() time.Time
}

func New(store *config.Store, cat *history.Catalog) *Service {
	return &Service{Store: store, Catalog: cat, Client: &http.Client{Timeout: 15 * time.Second}, Now: time.Now}
}

func (s *Service) Search(ctx context.Context, q httpapi.SetupQuery) (contract.SearchResult, error) {
	cfg := s.Store.Config()
	if cfg.Search.Provider == "disabled" {
		return contract.SearchResult{}, errors.New("search provider is disabled; supply a direct source URL")
	}
	if q.MaxResults <= 0 || q.MaxResults > 5 {
		q.MaxResults = cfg.Search.MaxResults
	}
	var body []byte
	var err error
	switch cfg.Search.Provider {
	case "tavily":
		body, err = s.tavily(ctx, cfg, q)
	case "searxng":
		body, err = s.searx(ctx, cfg, q)
	default:
		err = fmt.Errorf("unsupported search provider %q", cfg.Search.Provider)
	}
	if err != nil {
		return contract.SearchResult{}, err
	}
	return parseSearch(body, cfg.Search.Provider, q, s.Now().UnixMilli())
}

func (s *Service) tavily(ctx context.Context, cfg contract.Config, q httpapi.SetupQuery) ([]byte, error) {
	base := "https://api.tavily.com"
	if cfg.Search.BaseURL != nil && *cfg.Search.BaseURL != "" {
		base = strings.TrimRight(*cfg.Search.BaseURL, "/")
	}
	payload, _ := json.Marshal(map[string]any{"query": q.Query, "max_results": q.MaxResults, "search_depth": "basic"})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/search", strings.NewReader(string(payload)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if cfg.Search.APIKeyEnv != nil {
		req.Header.Set("Authorization", "Bearer "+s.Store.Secret(*cfg.Search.APIKeyEnv))
	}
	return s.do(req, 2<<20)
}

func (s *Service) searx(ctx context.Context, cfg contract.Config, q httpapi.SetupQuery) ([]byte, error) {
	if cfg.Search.BaseURL == nil || *cfg.Search.BaseURL == "" {
		return nil, errors.New("searxng base_url is required")
	}
	u, err := url.Parse(strings.TrimRight(*cfg.Search.BaseURL, "/") + "/search")
	if err != nil {
		return nil, err
	}
	qs := u.Query()
	qs.Set("q", q.Query)
	qs.Set("format", "json")
	u.RawQuery = qs.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	return s.do(req, 2<<20)
}

func (s *Service) do(req *http.Request, limit int64) ([]byte, error) {
	resp, err := s.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, limit))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == 429 || resp.StatusCode == 403 {
		return nil, fmt.Errorf("search provider returned %d; check credentials and format=json support", resp.StatusCode)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("search provider returned %d", resp.StatusCode)
	}
	if strings.HasPrefix(strings.TrimSpace(string(body)), "<") {
		return nil, errors.New("search provider returned HTML; JSON format is not enabled")
	}
	return body, nil
}

func parseSearch(body []byte, provider string, q httpapi.SetupQuery, now int64) (contract.SearchResult, error) {
	var doc map[string]any
	if err := json.Unmarshal(body, &doc); err != nil {
		return contract.SearchResult{}, fmt.Errorf("search provider returned malformed JSON: %w", err)
	}
	items, _ := doc["results"].([]any)
	if items == nil {
		items, _ = doc["Items"].([]any)
	}
	out := contract.SearchResult{Sources: []contract.Source{}, Leads: []contract.SearchLead{}}
	for _, iv := range items {
		m, _ := iv.(map[string]any)
		title, _ := m["title"].(string)
		link, _ := m["url"].(string)
		if link == "" {
			link, _ = m["link"].(string)
		}
		snippet, _ := m["content"].(string)
		if snippet == "" {
			snippet, _ = m["snippet"].(string)
		}
		if link == "" {
			continue
		}
		id := fmt.Sprintf("src-%d", len(out.Sources)+1)
		out.Sources = append(out.Sources, contract.Source{ID: id, Title: title, URL: link, RetrievedAtMs: now,
			Conditions: q.Conditions, Assists: q.Assists, UseCase: q.UseCase, Quality: "unknown", Limitations: []string{"search result; full source not inspected"}})
		out.Leads = append(out.Leads, contract.SearchLead{SourceID: id, Snippet: snippet})
		if len(out.Sources) >= q.MaxResults {
			out.Truncated = true
			break
		}
	}
	if len(out.Sources) == 0 {
		return out, errors.New("search returned no usable sources")
	}
	return out, nil
}

// Fetch inspects one HTTP(S) source with bounded redirects, body and text.
func (s *Service) Fetch(ctx context.Context, rawURL, sourceID string) (contract.SourceInspection, error) {
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return contract.SourceInspection{}, errors.New("only HTTP(S) source URLs are supported")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return contract.SourceInspection{}, err
	}
	client := &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(r *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return errors.New("too many redirects")
		}
		if r.URL.Scheme != "http" && r.URL.Scheme != "https" {
			return errors.New("redirect left HTTP(S)")
		}
		return nil
	}}
	resp, err := client.Do(req)
	if err != nil {
		return contract.SourceInspection{}, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, (2<<20)+1))
	if err != nil {
		return contract.SourceInspection{}, err
	}
	truncated := len(body) > 2<<20
	if truncated {
		body = body[:2<<20]
	}
	text := stripHTML(string(body))
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return contract.SourceInspection{Source: contract.Source{ID: sourceID, URL: rawURL, RetrievedAtMs: s.Now().UnixMilli() - 0}, Inspected: false, Text: "", Truncated: truncated}, fmt.Errorf("source returned %d", resp.StatusCode)
	}
	sum := sha256.Sum256([]byte(text))
	hash := hex.EncodeToString(sum[:])
	return contract.SourceInspection{
		Source:    contract.Source{ID: sourceID, Title: u.Host, URL: rawURL, RetrievedAtMs: s.Now().UnixMilli(), Conditions: "", Assists: "", UseCase: "unknown", Quality: "unknown", Limitations: []string{"community or official page; verify applicability"}},
		Inspected: true, Text: text, Truncated: truncated, ContentSHA256: &hash,
	}, nil
}

func stripHTML(s string) string {
	var b strings.Builder
	depth := 0
	for _, r := range s {
		switch r {
		case '<':
			depth++
		case '>':
			if depth > 0 {
				depth--
				b.WriteRune(' ')
			}
		default:
			if depth == 0 {
				b.WriteRune(r)
			}
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}
