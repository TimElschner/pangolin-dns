package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lmsgprefix)
	log.SetPrefix("[pangolin-dns] ")

	cfg, err := LoadConfig()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	log.Printf("Pangolin API: %s", cfg.PangolinAPIURL)
	log.Printf("Local IP: %s", cfg.PangolinLocalIP)
	log.Printf("Upstream DNS: %s", cfg.UpstreamDNS)
	log.Printf("Poll interval: %s", cfg.PollInterval)
	log.Printf("Local prefix: %v", cfg.EnableLocalPrefix)

	store := NewRecordStore()
	poller := NewPoller(cfg, store)
	dnsServer := NewDNSServer(cfg, store)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start poller in background
	go poller.Run(ctx)

	// Handle shutdown signals
	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		<-sig
		log.Println("shutting down...")
		cancel()
	}()

	// Start DNS server (blocks)
	if err := dnsServer.ListenAndServe(); err != nil {
		log.Fatalf("dns server: %v", err)
	}
}
