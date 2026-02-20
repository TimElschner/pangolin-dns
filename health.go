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
