package openparliament

import (
	"testing"

	"github.com/tamnd/any-cli/kit"
)

// These tests are offline: they exercise the URI driver's pure string functions
// and the host wiring (mint, body, resolve), which need no network. The client's
// HTTP behaviour is covered in openparliament_test.go.

func TestDomainInfo(t *testing.T) {
	info := Domain{}.Info()
	if info.Scheme != "openparliament" {
		t.Errorf("Scheme = %q, want openparliament", info.Scheme)
	}
	if len(info.Hosts) == 0 || info.Hosts[0] != Host {
		t.Errorf("Hosts = %v, want [%s]", info.Hosts, Host)
	}
	if info.Identity.Binary != "openparliament" {
		t.Errorf("Identity.Binary = %q, want openparliament", info.Identity.Binary)
	}
}

func TestClassify(t *testing.T) {
	cases := []struct{ in, typ, id string }{
		{"bills/44-1/C-14", "bill", "bills/44-1/C-14"},
		{"/bills/44-1/C-1/", "bill", "bills/44-1/C-1"},
		{"https://" + Host + "/votes/44-1/1044/", "vote", "votes/44-1/1044"},
		{"politicians/ziad-aboultaif", "politician", "politicians/ziad-aboultaif"},
	}
	for _, tc := range cases {
		typ, id, err := Domain{}.Classify(tc.in)
		if err != nil || typ != tc.typ || id != tc.id {
			t.Errorf("Classify(%q) = (%q, %q, %v), want (%q, %q, nil)",
				tc.in, typ, id, err, tc.typ, tc.id)
		}
	}
}

func TestLocate(t *testing.T) {
	cases := []struct {
		uriType, id, want string
	}{
		{"bill", "bills/44-1/C-14", BaseURL + "/bills/44-1/C-14/"},
		{"vote", "votes/44-1/1044", BaseURL + "/votes/44-1/1044/"},
		{"politician", "politicians/ziad-aboultaif", BaseURL + "/politicians/ziad-aboultaif/"},
	}
	for _, tc := range cases {
		got, err := Domain{}.Locate(tc.uriType, tc.id)
		if err != nil || got != tc.want {
			t.Errorf("Locate(%q, %q) = (%q, %v), want (%q, nil)", tc.uriType, tc.id, got, err, tc.want)
		}
	}
}

// TestHostWiring mounts the driver in a kit Host and checks the round trip:
// a record mints to its URI and a bare id resolves back correctly.
func TestHostWiring(t *testing.T) {
	h, err := kit.Open()
	if err != nil {
		t.Fatal(err)
	}

	b := &Bill{URL: "/bills/44-1/C-14/", Number: "C-14", Name: "Test Bill"}
	u, err := h.Mint(b)
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}
	// Mint uses the kit:"id" field (URL) as the id
	if u.Scheme != "openparliament" {
		t.Errorf("Mint scheme = %q, want openparliament", u.Scheme)
	}

	got, err := h.ResolveOn("openparliament", "bills/44-1/C-14")
	if err != nil {
		t.Fatalf("ResolveOn: %v", err)
	}
	if got.Scheme != "openparliament" {
		t.Errorf("ResolveOn scheme = %q, want openparliament", got.Scheme)
	}
}
