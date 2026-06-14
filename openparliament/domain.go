package openparliament

import (
	"context"
	"strings"

	"github.com/tamnd/any-cli/kit"
	"github.com/tamnd/any-cli/kit/errs"
)

// domain.go exposes openparliament as a kit Domain: a driver that a multi-domain
// host (ant) enables with a single blank import,
//
//	import _ "github.com/tamnd/openparliament-cli/openparliament"
//
// exactly as a database/sql program enables a driver with `import _
// "github.com/lib/pq"`. The init below registers it; the host then dereferences
// openparliament:// URIs by routing to the operations Register installs. The same
// Domain also builds the standalone openparliament binary, so binary and host
// share one source of truth.
func init() { kit.Register(Domain{}) }

// Domain is the openparliament driver. It carries no state; the per-run client
// is built by the factory Register hands kit.
type Domain struct{}

// Info describes the scheme, the hostnames a pasted link is matched against,
// and the identity reused for the binary's help and version.
func (Domain) Info() kit.DomainInfo {
	return kit.DomainInfo{
		Scheme: "openparliament",
		Hosts:  []string{Host},
		Identity: kit.Identity{
			Binary: "openparliament",
			Short:  "Read public Canadian Parliament data from OpenParliament.",
			Long: `Read public Canadian Parliament data from OpenParliament.

openparliament reads bills, votes, and politician data from
api.openparliament.ca over plain HTTPS and shapes it into clean records
that pipe into the rest of your tools. No API key required.`,
			Site: "https://api.openparliament.ca",
			Repo: "https://github.com/tamnd/openparliament-cli",
		},
	}
}

// Register installs the client factory and every operation onto app.
func (Domain) Register(app *kit.App) {
	app.SetClient(newClient)

	// bills — list parliamentary bills
	kit.Handle(app, kit.OpMeta{Name: "bills", Group: "read", List: true,
		Summary: "List parliamentary bills"}, listBills)

	// bill — fetch a single bill by session and number
	kit.Handle(app, kit.OpMeta{Name: "bill", Group: "read", Single: true,
		Summary: "Get a single bill by session and number",
		Args: []kit.Arg{
			{Name: "session", Help: "parliament session (e.g. 44-1)"},
			{Name: "number", Help: "bill number (e.g. C-14)"},
		}}, getBill)

	// votes — list parliamentary votes
	kit.Handle(app, kit.OpMeta{Name: "votes", Group: "read", List: true,
		Summary: "List parliamentary votes"}, listVotes)

	// politicians — list members of Parliament
	kit.Handle(app, kit.OpMeta{Name: "politicians", Group: "read", List: true,
		Summary: "List members of Parliament"}, listPoliticians)
}

// newClient builds the client from the host-resolved config.
func newClient(_ context.Context, cfg kit.Config) (any, error) {
	c := DefaultConfig()
	if cfg.UserAgent != "" {
		c.UserAgent = cfg.UserAgent
	}
	if cfg.Rate > 0 {
		c.Rate = cfg.Rate
	}
	if cfg.Retries > 0 {
		c.Retries = cfg.Retries
	}
	if cfg.Timeout > 0 {
		c.Timeout = cfg.Timeout
	}
	return NewClientWithConfig(c), nil
}

// ---- input structs ----

type billsInput struct {
	Session string  `kit:"flag" help:"parliament session (e.g. 44-1)"`
	Limit   int     `kit:"flag,inherit" help:"max results"`
	Client  *Client `kit:"inject"`
}

type billInput struct {
	Session string  `kit:"arg" help:"parliament session (e.g. 44-1)"`
	Number  string  `kit:"arg" help:"bill number (e.g. C-14)"`
	Client  *Client `kit:"inject"`
}

type votesInput struct {
	Session string  `kit:"flag" help:"parliament session (e.g. 44-1)"`
	Bill    string  `kit:"flag" help:"bill URL fragment (e.g. /bills/44-1/C-14/)"`
	Limit   int     `kit:"flag,inherit" help:"max results"`
	Client  *Client `kit:"inject"`
}

type politiciansInput struct {
	Party  string  `kit:"flag" help:"filter by party short name (e.g. Conservative)"`
	Limit  int     `kit:"flag,inherit" help:"max results"`
	Client *Client `kit:"inject"`
}

// ---- handlers ----

func listBills(ctx context.Context, in billsInput, emit func(*Bill) error) error {
	bills, err := in.Client.Bills(ctx, BillsOptions{
		Session: in.Session,
		Limit:   in.Limit,
	})
	if err != nil {
		return mapErr(err)
	}
	for _, b := range bills {
		if err := emit(b); err != nil {
			return err
		}
	}
	return nil
}

func getBill(ctx context.Context, in billInput, emit func(*Bill) error) error {
	b, err := in.Client.GetBill(ctx, in.Session, in.Number)
	if err != nil {
		return mapErr(err)
	}
	return emit(b)
}

func listVotes(ctx context.Context, in votesInput, emit func(*Vote) error) error {
	votes, err := in.Client.Votes(ctx, VotesOptions{
		Session: in.Session,
		Bill:    in.Bill,
		Limit:   in.Limit,
	})
	if err != nil {
		return mapErr(err)
	}
	for _, v := range votes {
		if err := emit(v); err != nil {
			return err
		}
	}
	return nil
}

func listPoliticians(ctx context.Context, in politiciansInput, emit func(*Politician) error) error {
	politicians, err := in.Client.Politicians(ctx, PoliticiansOptions{
		Party: in.Party,
		Limit: in.Limit,
	})
	if err != nil {
		return mapErr(err)
	}
	for _, p := range politicians {
		if err := emit(p); err != nil {
			return err
		}
	}
	return nil
}

// ---- Resolver: pure string functions, no network ----

// Classify turns any accepted input into the canonical (type, id).
func (Domain) Classify(input string) (uriType, id string, err error) {
	input = strings.TrimSpace(input)
	// strip URL scheme+host
	if strings.HasPrefix(input, "https://") || strings.HasPrefix(input, "http://") {
		for _, prefix := range []string{"https://", "http://"} {
			if strings.HasPrefix(input, prefix) {
				input = input[len(prefix):]
			}
		}
		if slash := strings.IndexByte(input, '/'); slash >= 0 {
			input = input[slash:]
		} else {
			input = ""
		}
	}
	input = strings.Trim(input, "/")
	if input == "" {
		return "", "", errs.Usage("unrecognized openparliament reference: empty")
	}
	switch {
	case strings.HasPrefix(input, "bills/"):
		return "bill", input, nil
	case strings.HasPrefix(input, "votes/"):
		return "vote", input, nil
	case strings.HasPrefix(input, "politicians/"):
		return "politician", input, nil
	default:
		return "page", input, nil
	}
}

// Locate returns the live https URL for a (type, id).
func (Domain) Locate(uriType, id string) (string, error) {
	switch uriType {
	case "bill", "vote", "politician", "page":
		return BaseURL + "/" + strings.Trim(id, "/") + "/", nil
	default:
		return "", errs.Usage("openparliament has no resource type %q", uriType)
	}
}

// mapErr converts library errors into kit error kinds with the right exit code.
func mapErr(err error) error {
	return err
}
