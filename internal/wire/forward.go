package wire

import (
	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/forward"
	"github.com/rainchan/webmux/internal/session"
)

func ForwardOptions(o forward.Options) contract.ForwardOptions {
	return contract.ForwardOptions{
		RewriteHost: o.RewriteHost, RewriteOrigin: o.RewriteOrigin,
		ForwardedHeaders: o.ForwardedHeaders, RewriteResponse: o.RewriteResponse,
		Prefix: contract.ForwardPrefix(o.Prefix),
	}
}

// ForwardOptionsInput converts request options; nil stays nil (unchanged or
// default).
func ForwardOptionsInput(o *contract.ForwardOptions) *forward.Options {
	if o == nil {
		return nil
	}
	return &forward.Options{
		RewriteHost: o.RewriteHost, RewriteOrigin: o.RewriteOrigin,
		ForwardedHeaders: o.ForwardedHeaders, RewriteResponse: o.RewriteResponse,
		Prefix: forward.Prefix(o.Prefix),
	}
}

func Forward(f forward.Forward) contract.Forward {
	return contract.Forward{
		Port: f.Port, Path: f.Path(), Label: f.Label, Session: f.Session, Process: f.Process,
		Options: ForwardOptions(f.Options), Status: contract.ForwardStatus(f.Status), CreatedAt: f.CreatedAt,
	}
}

func Forwards(in []forward.Forward) []contract.Forward { return each(in, Forward) }

func ForwardInput(r contract.ForwardRequest) forward.Input {
	return forward.Input{Port: r.Port, Label: r.Label, Session: r.Session, Process: r.Process, Options: ForwardOptionsInput(r.Options)}
}

func ForwardUpdate(r contract.ForwardUpdate) forward.Update {
	return forward.Update{Label: r.Label, Options: ForwardOptionsInput(r.Options)}
}

func ForwardTicket(t forward.Ticket) contract.ForwardTicket {
	return contract.ForwardTicket{Code: t.Code, ExpiresAt: t.ExpiresAt}
}

func ListeningPorts(in []session.ListeningPort) []contract.ListeningPort {
	return each(in, func(p session.ListeningPort) contract.ListeningPort {
		return contract.ListeningPort{Port: p.Port, Process: p.Process}
	})
}
