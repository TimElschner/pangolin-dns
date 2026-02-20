package main

import (
	"context"
	"log"
	"net"
	"strings"
	"time"

	"github.com/miekg/dns"
)

type DNSServer struct {
	cfg       *Config
	store     *RecordStore
	udpServer *dns.Server
	tcpServer *dns.Server
}

func NewDNSServer(cfg *Config, store *RecordStore) *DNSServer {
	return &DNSServer{cfg: cfg, store: store}
}

// ServeDNS handles incoming DNS queries.
func (s *DNSServer) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
	msg := new(dns.Msg)
	msg.SetReply(r)
	msg.Authoritative = true

	for _, q := range r.Question {
		switch q.Qtype {
		case dns.TypeA:
			fqdn := strings.ToLower(q.Name)
			if ip, ok := s.store.Lookup(fqdn); ok {
				msg.Answer = append(msg.Answer, &dns.A{
					Hdr: dns.RR_Header{
						Name:   q.Name,
						Rrtype: dns.TypeA,
						Class:  dns.ClassINET,
						Ttl:    60,
					},
					A: net.ParseIP(ip),
				})
				log.Printf("dns: %s -> %s (local)", q.Name, ip)
			} else {
				s.forward(w, r)
				return
			}
		default:
			s.forward(w, r)
			return
		}
	}

	w.WriteMsg(msg)
}

// forward sends the query to the upstream DNS server and relays the response.
func (s *DNSServer) forward(w dns.ResponseWriter, r *dns.Msg) {
	client := new(dns.Client)

	// Use TCP if the original query came over TCP
	if _, ok := w.RemoteAddr().(*net.TCPAddr); ok {
		client.Net = "tcp"
	}

	resp, _, err := client.Exchange(r, s.cfg.UpstreamDNS)
	if err != nil {
		log.Printf("dns: upstream error for %s: %v", r.Question[0].Name, err)
		msg := new(dns.Msg)
		msg.SetRcode(r, dns.RcodeServerFailure)
		w.WriteMsg(msg)
		return
	}

	w.WriteMsg(resp)
}

// ListenAndServe starts both UDP and TCP DNS listeners and blocks until ctx is
// cancelled or one of the servers returns an error.
func (s *DNSServer) ListenAndServe(ctx context.Context) error {
	addr := ":" + s.cfg.DNSPort

	s.udpServer = &dns.Server{Addr: addr, Net: "udp", Handler: s}
	s.tcpServer = &dns.Server{Addr: addr, Net: "tcp", Handler: s}

	errCh := make(chan error, 2)

	go func() {
		log.Printf("dns: listening on %s/udp", addr)
		errCh <- s.udpServer.ListenAndServe()
	}()

	go func() {
		log.Printf("dns: listening on %s/tcp", addr)
		errCh <- s.tcpServer.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		log.Println("dns: shutting down")
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.udpServer.ShutdownContext(shutCtx)
		s.tcpServer.ShutdownContext(shutCtx)
		return nil
	}
}
