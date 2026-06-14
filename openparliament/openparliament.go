// Package openparliament is the library behind the openparliament command line:
// the HTTP client, request shaping, and typed data models for the OpenParliament Canada API.
//
// The Client sets a real User-Agent, paces requests to stay polite, and retries
// transient failures (429 and 5xx). All endpoint calls append format=json automatically.
package openparliament

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Host is the canonical API hostname.
const Host = "api.openparliament.ca"

// BaseURL is the root every request is built from.
const BaseURL = "https://" + Host

// Config holds the client configuration.
type Config struct {
	BaseURL   string
	UserAgent string
	Rate      time.Duration
	Timeout   time.Duration
	Retries   int
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		BaseURL:   BaseURL,
		UserAgent: "openparliament-cli/0.1 (tamnd87@gmail.com)",
		Rate:      300 * time.Millisecond,
		Timeout:   15 * time.Second,
		Retries:   3,
	}
}

// Client talks to the OpenParliament API over HTTPS.
type Client struct {
	http *http.Client
	cfg  Config

	mu   sync.Mutex
	last time.Time
}

// NewClient returns a Client using DefaultConfig.
func NewClient() *Client {
	cfg := DefaultConfig()
	return &Client{
		http: &http.Client{Timeout: cfg.Timeout},
		cfg:  cfg,
	}
}

// NewClientWithConfig returns a Client using the given Config.
func NewClientWithConfig(cfg Config) *Client {
	return &Client{
		http: &http.Client{Timeout: cfg.Timeout},
		cfg:  cfg,
	}
}

// get fetches rawURL, automatically appending format=json.
func (c *Client) get(ctx context.Context, rawURL string) ([]byte, error) {
	sep := "?"
	if strings.Contains(rawURL, "?") {
		sep = "&"
	}
	rawURL += sep + "format=json"

	var lastErr error
	for attempt := 0; attempt <= c.cfg.Retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff(attempt)):
			}
		}
		body, retry, err := c.do(ctx, rawURL)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if !retry {
			return nil, err
		}
	}
	return nil, fmt.Errorf("get %s: %w", rawURL, lastErr)
}

func (c *Client) do(ctx context.Context, url string) (body []byte, retry bool, err error) {
	c.pace()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("User-Agent", c.cfg.UserAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, true, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return nil, true, fmt.Errorf("http %d", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("http %d", resp.StatusCode)
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, true, err
	}
	return b, false, nil
}

// pace blocks until at least Rate has elapsed since the previous request.
func (c *Client) pace() {
	if c.cfg.Rate <= 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if wait := c.cfg.Rate - time.Since(c.last); wait > 0 {
		time.Sleep(wait)
	}
	c.last = time.Now()
}

func backoff(attempt int) time.Duration {
	d := time.Duration(attempt) * 500 * time.Millisecond
	if d > 5*time.Second {
		d = 5 * time.Second
	}
	return d
}

// ---- Wire types ----

type wireLang struct {
	En string `json:"en"`
	Fr string `json:"fr"`
}

type wireListResp[T any] struct {
	Objects    []T        `json:"objects"`
	Pagination wirePaging `json:"pagination"`
}

type wirePaging struct {
	NextURL *string `json:"next_url"`
}

// ---- Output types ----

// Bill is a Canadian parliamentary bill.
type Bill struct {
	URL        string `kit:"id" json:"url"`
	Number     string `json:"number"`
	Name       string `json:"name"`
	Introduced string `json:"introduced"`
	IsLaw      bool   `json:"is_law"`
	Status     string `json:"status"`
	SponsorURL string `json:"sponsor_url"`
}

// Vote is a recorded vote in Parliament.
type Vote struct {
	URL         string `kit:"id" json:"url"`
	Date        string `json:"date"`
	Description string `json:"description"`
	YeaTotal    int    `json:"yea_total"`
	NayTotal    int    `json:"nay_total"`
	BillURL     string `json:"bill_url"`
}

// Politician is a member of Parliament.
type Politician struct {
	URL    string `kit:"id" json:"url"`
	Name   string `json:"name"`
	Party  string `json:"party"`
	Riding string `json:"riding"`
	Email  string `json:"email"`
}

// ---- API methods ----

// BillsOptions controls the /bills/ list call.
type BillsOptions struct {
	Session string // e.g. "44-1"
	Limit   int
}

// Bills lists parliamentary bills.
func (c *Client) Bills(ctx context.Context, opts BillsOptions) ([]*Bill, error) {
	u := c.cfg.BaseURL + "/bills/?"
	params := []string{}
	if opts.Session != "" {
		params = append(params, "session="+opts.Session)
	}
	if opts.Limit > 0 {
		params = append(params, fmt.Sprintf("limit=%d", opts.Limit))
	}
	if len(params) > 0 {
		u += strings.Join(params, "&")
	}
	u = strings.TrimRight(u, "&?")

	body, err := c.get(ctx, u)
	if err != nil {
		return nil, err
	}

	type wireBill struct {
		URL                  string   `json:"url"`
		Number               string   `json:"number"`
		Name                 wireLang `json:"name"`
		Introduced           string   `json:"introduced"`
		Session              string   `json:"session"`
		Law                  bool     `json:"law"`
		SponsorPoliticianURL string   `json:"sponsor_politician_url"`
		Status               wireLang `json:"status"`
	}

	var resp wireListResp[wireBill]
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode bills: %w", err)
	}

	out := make([]*Bill, len(resp.Objects))
	for i, w := range resp.Objects {
		out[i] = &Bill{
			URL:        w.URL,
			Number:     w.Number,
			Name:       w.Name.En,
			Introduced: w.Introduced,
			IsLaw:      w.Law,
			Status:     w.Status.En,
			SponsorURL: w.SponsorPoliticianURL,
		}
	}
	return out, nil
}

// GetBill fetches a single bill by session and number.
func (c *Client) GetBill(ctx context.Context, session, number string) (*Bill, error) {
	u := fmt.Sprintf("%s/bills/%s/%s/", c.cfg.BaseURL, session, number)
	body, err := c.get(ctx, u)
	if err != nil {
		return nil, err
	}

	var w struct {
		URL                  string   `json:"url"`
		Number               string   `json:"number"`
		Name                 wireLang `json:"name"`
		Introduced           string   `json:"introduced"`
		Law                  bool     `json:"law"`
		SponsorPoliticianURL string   `json:"sponsor_politician_url"`
		Status               wireLang `json:"status"`
	}
	if err := json.Unmarshal(body, &w); err != nil {
		return nil, fmt.Errorf("decode bill: %w", err)
	}
	return &Bill{
		URL:        w.URL,
		Number:     w.Number,
		Name:       w.Name.En,
		Introduced: w.Introduced,
		IsLaw:      w.Law,
		Status:     w.Status.En,
		SponsorURL: w.SponsorPoliticianURL,
	}, nil
}

// VotesOptions controls the /votes/ list call.
type VotesOptions struct {
	Session string // e.g. "44-1"
	Bill    string // bill URL fragment like "/bills/44-1/C-14/"
	Limit   int
}

// Votes lists parliamentary votes.
func (c *Client) Votes(ctx context.Context, opts VotesOptions) ([]*Vote, error) {
	u := c.cfg.BaseURL + "/votes/?"
	params := []string{}
	if opts.Bill != "" {
		params = append(params, "bill="+opts.Bill)
	}
	if opts.Limit > 0 {
		params = append(params, fmt.Sprintf("limit=%d", opts.Limit))
	}
	if len(params) > 0 {
		u += strings.Join(params, "&")
	}
	u = strings.TrimRight(u, "&?")

	body, err := c.get(ctx, u)
	if err != nil {
		return nil, err
	}

	type wireVote struct {
		URL         string   `json:"url"`
		Date        string   `json:"date"`
		Description wireLang `json:"description"`
		NayTotal    int      `json:"nay_total"`
		YeaTotal    int      `json:"yea_total"`
		// The API returns bill_url in list responses
		BillURL string `json:"bill_url"`
		// Some API variants return bill as a nullable string
		Bill    *string `json:"bill"`
		Session string  `json:"session"`
	}

	var resp wireListResp[wireVote]
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode votes: %w", err)
	}

	out := make([]*Vote, len(resp.Objects))
	for i, w := range resp.Objects {
		billURL := w.BillURL
		if billURL == "" && w.Bill != nil {
			billURL = *w.Bill
		}
		out[i] = &Vote{
			URL:         w.URL,
			Date:        w.Date,
			Description: w.Description.En,
			YeaTotal:    w.YeaTotal,
			NayTotal:    w.NayTotal,
			BillURL:     billURL,
		}
	}
	return out, nil
}

// PoliticiansOptions controls the /politicians/ list call.
type PoliticiansOptions struct {
	Party string
	Limit int
}

// Politicians lists members of Parliament.
func (c *Client) Politicians(ctx context.Context, opts PoliticiansOptions) ([]*Politician, error) {
	u := c.cfg.BaseURL + "/politicians/?"
	params := []string{}
	if opts.Limit > 0 {
		params = append(params, fmt.Sprintf("limit=%d", opts.Limit))
	}
	if len(params) > 0 {
		u += strings.Join(params, "&")
	}
	u = strings.TrimRight(u, "&?")

	body, err := c.get(ctx, u)
	if err != nil {
		return nil, err
	}

	type wirePartyName struct {
		En string `json:"en"`
	}
	type wireParty struct {
		ShortName wirePartyName `json:"short_name"`
	}
	type wireRidingName struct {
		En string `json:"en"`
	}
	type wireRiding struct {
		Name     wireRidingName `json:"name"`
		Province string         `json:"province"`
	}
	type wirePolitician struct {
		URL           string      `json:"url"`
		Name          string      `json:"name"`
		CurrentParty  *wireParty  `json:"current_party"`
		CurrentRiding *wireRiding `json:"current_riding"`
		// some endpoints also use "riding"
		Riding *wireRiding `json:"riding"`
		Email  string      `json:"email"`
		Gender string      `json:"gender"`
		Image  string      `json:"image"`
	}

	var resp wireListResp[wirePolitician]
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode politicians: %w", err)
	}

	var out []*Politician
	for _, w := range resp.Objects {
		party := ""
		if w.CurrentParty != nil {
			party = w.CurrentParty.ShortName.En
		}
		riding := ""
		switch {
		case w.CurrentRiding != nil:
			riding = w.CurrentRiding.Name.En
		case w.Riding != nil:
			riding = w.Riding.Name.En
		}
		p := &Politician{
			URL:    w.URL,
			Name:   w.Name,
			Party:  party,
			Riding: riding,
			Email:  w.Email,
		}
		// client-side party filter since the API doesn't support it natively
		if opts.Party != "" && !strings.EqualFold(p.Party, opts.Party) {
			continue
		}
		out = append(out, p)
	}
	return out, nil
}
