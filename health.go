package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"
)

type HealthServer struct {
	cfg    *Config
	poller *Poller
	store  *RecordStore
}

func NewHealthServer(cfg *Config, poller *Poller, store *RecordStore) *HealthServer {
	return &HealthServer{cfg: cfg, poller: poller, store: store}
}

type healthResponse struct {
	Status     string `json:"status"`
	Records    int    `json:"records"`
	LastPoll   string `json:"last_poll,omitempty"`
	PollErrors int64  `json:"poll_errors"`
}

func (h *HealthServer) Run(ctx context.Context) {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", h.handleHealth)
	mux.HandleFunc("/poll", h.handlePoll)
	mux.HandleFunc("/domains", h.handleDomains)

	srv := &http.Server{
		Addr:    ":" + h.cfg.HealthPort,
		Handler: mux,
	}

	go func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		srv.Shutdown(shutCtx)
	}()

	log.Printf("health: listening on :%s/http", h.cfg.HealthPort)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Printf("health: server error: %v", err)
	}
}

func (h *HealthServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	resp := healthResponse{
		Status:     "ok",
		Records:    h.store.Count(),
		PollErrors: h.poller.pollErrors.Load(),
	}
	if t := h.poller.lastPoll.Load(); t != nil {
		resp.LastPoll = t.(time.Time).UTC().Format(time.RFC3339)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// handlePoll triggers an immediate re-poll of the Pangolin API and returns
// the updated health status. Only accepts POST requests.
func (h *HealthServer) handlePoll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	log.Println("health: manual poll triggered")
	h.poller.Poll()

	h.handleHealth(w, r)
}

// handleDomains returns the list of DNS records currently held in the store.
func (h *HealthServer) handleDomains(w http.ResponseWriter, r *http.Request) {
	type domainsResponse struct {
		Domains []string `json:"domains"`
	}

	domains := h.store.Domains()
	if domains == nil {
		domains = []string{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(domainsResponse{Domains: domains})
}
