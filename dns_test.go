package main

import (
	"net"
	"testing"

	"github.com/miekg/dns"
)

func newTestDNSServer(records map[string]string) *DNSServer {
	cfg := &Config{
		UpstreamDNS: "1.1.1.1:53",
		DNSPort:     "0",
	}
	store := NewRecordStore()
	if records != nil {
		store.Update(records)
	}
	return NewDNSServer(cfg, store)
}

// dnsRecorder captures the written DNS response.
type dnsRecorder struct {
	msg *dns.Msg
}

func (r *dnsRecorder) LocalAddr() net.Addr        { return &net.UDPAddr{} }
func (r *dnsRecorder) RemoteAddr() net.Addr        { return &net.UDPAddr{} }
func (r *dnsRecorder) WriteMsg(m *dns.Msg) error  { r.msg = m; return nil }
func (r *dnsRecorder) Write(b []byte) (int, error) { return len(b), nil }
func (r *dnsRecorder) Close() error                { return nil }
func (r *dnsRecorder) TsigStatus() error           { return nil }
func (r *dnsRecorder) TsigTimersOnly(bool)         {}
func (r *dnsRecorder) Hijack()                     {}

func makeQuery(name string, qtype uint16) *dns.Msg {
	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn(name), qtype)
	return m
}

func TestDNSServer_KnownDomain_ReturnsLocalIP(t *testing.T) {
	srv := newTestDNSServer(map[string]string{
		"app.example.com.": "10.0.0.5",
	})

	w := &dnsRecorder{}
	srv.ServeDNS(w, makeQuery("app.example.com", dns.TypeA))

	if w.msg == nil {
		t.Fatal("no response written")
	}
	if len(w.msg.Answer) != 1 {
		t.Fatalf("expected 1 answer, got %d", len(w.msg.Answer))
	}
	a, ok := w.msg.Answer[0].(*dns.A)
	if !ok {
		t.Fatal("answer is not an A record")
	}
	if a.A.String() != "10.0.0.5" {
		t.Errorf("expected 10.0.0.5, got %s", a.A.String())
	}
}

func TestDNSServer_CaseInsensitive(t *testing.T) {
	srv := newTestDNSServer(map[string]string{
		"app.example.com.": "10.0.0.5",
	})

	w := &dnsRecorder{}
	srv.ServeDNS(w, makeQuery("APP.EXAMPLE.COM", dns.TypeA))

	if w.msg == nil || len(w.msg.Answer) != 1 {
		t.Error("uppercase query should match lowercase store entry")
	}
}

func TestDNSServer_NonTypeA_DoesNotPanic(t *testing.T) {
	srv := newTestDNSServer(nil)
	w := &dnsRecorder{}
	// AAAA query — must not panic (will attempt upstream forward which may fail,
	// but the server itself should not crash)
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("ServeDNS panicked: %v", r)
		}
	}()
	srv.ServeDNS(w, makeQuery("example.com", dns.TypeAAAA))
}

func TestDNSServer_UnknownDomain_AttemptsForward(t *testing.T) {
	// Unknown domain should NOT produce a local answer — it should attempt
	// to forward (which will fail with SERVFAIL since we don't have a real
	// upstream in unit tests, but the recorder will still capture a response).
	srv := newTestDNSServer(map[string]string{
		"known.example.com.": "10.0.0.1",
	})
	// Point upstream to an invalid address so forward() returns SERVFAIL
	srv.cfg.UpstreamDNS = "127.0.0.1:1"

	w := &dnsRecorder{}
	srv.ServeDNS(w, makeQuery("unknown.example.com", dns.TypeA))

	if w.msg == nil {
		t.Fatal("expected a response (SERVFAIL) for unknown domain")
	}
	if w.msg.Rcode != dns.RcodeServerFailure {
		t.Errorf("expected SERVFAIL for forward error, got rcode %d", w.msg.Rcode)
	}
}
