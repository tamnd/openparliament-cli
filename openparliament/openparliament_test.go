package openparliament

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// testClient returns a Client pointed at srv with pacing disabled.
func testClient(srv *httptest.Server) *Client {
	cfg := DefaultConfig()
	cfg.BaseURL = srv.URL
	cfg.Rate = 0
	return NewClientWithConfig(cfg)
}

func TestGet_UserAgent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Error("request carried no User-Agent")
		}
		_, _ = w.Write([]byte(`{"objects":[],"pagination":{}}`))
	}))
	defer srv.Close()

	c := testClient(srv)
	_, err := c.get(context.Background(), srv.URL+"/bills/")
	if err != nil {
		t.Fatal(err)
	}
}

func TestGetRetriesOn503(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte(`{"objects":[],"pagination":{}}`))
	}))
	defer srv.Close()

	cfg := DefaultConfig()
	cfg.BaseURL = srv.URL
	cfg.Rate = 0
	cfg.Retries = 5
	c := NewClientWithConfig(cfg)

	start := time.Now()
	_, err := c.get(context.Background(), srv.URL+"/bills/")
	if err != nil {
		t.Fatal(err)
	}
	if hits != 3 {
		t.Errorf("server saw %d hits, want 3", hits)
	}
	if time.Since(start) < 500*time.Millisecond {
		t.Error("retries did not back off")
	}
}

func TestBills(t *testing.T) {
	payload := map[string]any{
		"objects": []map[string]any{
			{
				"url":                    "/bills/44-1/C-1/",
				"number":                 "C-1",
				"name":                   map[string]string{"en": "An Act respecting Oaths", "fr": "..."},
				"introduced":             "2021-11-22",
				"law":                    false,
				"sponsor_politician_url": "/politicians/john-smith/",
				"status":                 map[string]string{"en": "At second reading", "fr": "..."},
			},
			{
				"url":        "/bills/44-1/C-2/",
				"number":     "C-2",
				"name":       map[string]string{"en": "Another Act", "fr": "..."},
				"introduced": "2021-11-23",
				"law":        true,
				"status":     map[string]string{"en": "Royal Assent", "fr": "..."},
			},
		},
		"pagination": map[string]any{"next_url": nil},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/bills/" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(payload)
	}))
	defer srv.Close()

	c := testClient(srv)
	bills, err := c.Bills(context.Background(), BillsOptions{Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(bills) != 2 {
		t.Fatalf("len(bills) = %d, want 2", len(bills))
	}
	if bills[0].Number != "C-1" {
		t.Errorf("bills[0].Number = %q, want C-1", bills[0].Number)
	}
	if bills[0].Name != "An Act respecting Oaths" {
		t.Errorf("bills[0].Name = %q, want English name", bills[0].Name)
	}
	if bills[1].IsLaw != true {
		t.Errorf("bills[1].IsLaw = false, want true")
	}
}

func TestGetBill(t *testing.T) {
	payload := map[string]any{
		"url":                    "/bills/44-1/C-14/",
		"number":                 "C-14",
		"name":                   map[string]string{"en": "Public Safety Bill", "fr": "..."},
		"introduced":             "2022-02-03",
		"law":                    false,
		"sponsor_politician_url": "/politicians/marco-mendicino/",
		"status":                 map[string]string{"en": "In committee", "fr": "..."},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/bills/44-1/C-14/" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(payload)
	}))
	defer srv.Close()

	c := testClient(srv)
	b, err := c.GetBill(context.Background(), "44-1", "C-14")
	if err != nil {
		t.Fatal(err)
	}
	if b.Number != "C-14" {
		t.Errorf("Number = %q, want C-14", b.Number)
	}
	if b.Name != "Public Safety Bill" {
		t.Errorf("Name = %q, want Public Safety Bill", b.Name)
	}
	if b.SponsorURL != "/politicians/marco-mendicino/" {
		t.Errorf("SponsorURL = %q", b.SponsorURL)
	}
}

func TestVotes(t *testing.T) {
	billURL := "/bills/44-1/C-14/"
	payload := map[string]any{
		"objects": []map[string]any{
			{
				"url":         "/votes/44-1/1044/",
				"date":        "2022-06-01",
				"description": map[string]string{"en": "Third reading of Bill C-14", "fr": "..."},
				"yea_total":   180,
				"nay_total":   120,
				"bill_url":    billURL,
				"session":     "44-1",
			},
			{
				"url":         "/votes/44-1/1045/",
				"date":        "2022-06-02",
				"description": map[string]string{"en": "Time allocation", "fr": "..."},
				"yea_total":   170,
				"nay_total":   130,
				"bill_url":    "",
				"session":     "44-1",
			},
		},
		"pagination": map[string]any{"next_url": nil},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/votes/" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(payload)
	}))
	defer srv.Close()

	c := testClient(srv)
	votes, err := c.Votes(context.Background(), VotesOptions{Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(votes) != 2 {
		t.Fatalf("len(votes) = %d, want 2", len(votes))
	}
	if votes[0].Description != "Third reading of Bill C-14" {
		t.Errorf("votes[0].Description = %q", votes[0].Description)
	}
	if votes[0].YeaTotal != 180 {
		t.Errorf("votes[0].YeaTotal = %d, want 180", votes[0].YeaTotal)
	}
	if votes[0].BillURL != billURL {
		t.Errorf("votes[0].BillURL = %q, want %q", votes[0].BillURL, billURL)
	}
	if votes[1].BillURL != "" {
		t.Errorf("votes[1].BillURL = %q, want empty (null bill)", votes[1].BillURL)
	}
}

func TestPoliticians(t *testing.T) {
	payload := map[string]any{
		"objects": []map[string]any{
			{
				"url":  "/politicians/ziad-aboultaif/",
				"name": "Ziad Aboultaif",
				"current_party": map[string]any{
					"short_name": map[string]string{"en": "Conservative", "fr": "Conservateur"},
				},
				"riding": map[string]any{
					"name": map[string]string{"en": "Edmonton Manning", "fr": "..."},
				},
				"email":  "ziad.aboultaif@parl.gc.ca",
				"gender": "M",
				"image":  "/media/polpics/aboultaif.jpg",
			},
			{
				"url":  "/politicians/jody-wilson-raybould/",
				"name": "Jody Wilson-Raybould",
				"current_party": map[string]any{
					"short_name": map[string]string{"en": "Independent", "fr": "Indépendant"},
				},
				"riding": map[string]any{
					"name": map[string]string{"en": "Vancouver Granville", "fr": "..."},
				},
				"email":  "jody.wilson-raybould@parl.gc.ca",
				"gender": "F",
			},
		},
		"pagination":  map[string]any{"next_url": nil},
		"total_count": 2,
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/politicians/" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(payload)
	}))
	defer srv.Close()

	c := testClient(srv)
	politicians, err := c.Politicians(context.Background(), PoliticiansOptions{Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(politicians) != 2 {
		t.Fatalf("len(politicians) = %d, want 2", len(politicians))
	}
	if politicians[0].Name != "Ziad Aboultaif" {
		t.Errorf("politicians[0].Name = %q", politicians[0].Name)
	}
	if politicians[0].Party != "Conservative" {
		t.Errorf("politicians[0].Party = %q, want Conservative", politicians[0].Party)
	}
	if politicians[0].Riding != "Edmonton Manning" {
		t.Errorf("politicians[0].Riding = %q, want Edmonton Manning", politicians[0].Riding)
	}
}

func TestPoliticians_PartyFilter(t *testing.T) {
	payload := map[string]any{
		"objects": []map[string]any{
			{
				"url":  "/politicians/a/",
				"name": "Alice",
				"current_party": map[string]any{
					"short_name": map[string]string{"en": "Liberal", "fr": "..."},
				},
				"riding": map[string]any{"name": map[string]string{"en": "Riding A", "fr": "..."}},
			},
			{
				"url":  "/politicians/b/",
				"name": "Bob",
				"current_party": map[string]any{
					"short_name": map[string]string{"en": "Conservative", "fr": "..."},
				},
				"riding": map[string]any{"name": map[string]string{"en": "Riding B", "fr": "..."}},
			},
		},
		"pagination": map[string]any{"next_url": nil},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(payload)
	}))
	defer srv.Close()

	c := testClient(srv)
	politicians, err := c.Politicians(context.Background(), PoliticiansOptions{Party: "Liberal"})
	if err != nil {
		t.Fatal(err)
	}
	if len(politicians) != 1 {
		t.Fatalf("len(politicians) = %d, want 1 (Liberal filter)", len(politicians))
	}
	if politicians[0].Name != "Alice" {
		t.Errorf("politicians[0].Name = %q, want Alice", politicians[0].Name)
	}
}
