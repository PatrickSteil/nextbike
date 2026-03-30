package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/PatrickSteil/nextbike/db"
)

type Server struct {
	db  *db.DB
	log *slog.Logger
}

func New(database *db.DB, log *slog.Logger) *Server {
	return &Server{db: database, log: log}
}

// Start listens on addr (e.g. ":8080") until ctx is cancelled.
func (s *Server) Start(ctx context.Context, addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /stations/{uid}", s.getStation)

	srv := &http.Server{Addr: addr, Handler: mux}

	go func() {
		<-ctx.Done()
		srv.Shutdown(context.Background())
	}()

	s.log.Info("http server listening", "addr", addr)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		return fmt.Errorf("http server: %w", err)
	}
	return nil
}

func (s *Server) getStation(w http.ResponseWriter, r *http.Request) {
	uid, err := strconv.Atoi(r.PathValue("uid"))
	if err != nil {
		http.Error(w, "uid must be an integer", http.StatusBadRequest)
		return
	}

	station, err := s.db.ByUID(r.Context(), uid)
	if err != nil {
		s.log.Error("db query failed", "uid", uid, "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if station == nil {
		http.Error(w, "station not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(station)
}
