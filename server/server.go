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

func (s *Server) Start(ctx context.Context, addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /cities", s.getCities)
	mux.HandleFunc("GET /cities/{uid}/stations", s.getStationsByCity)
	mux.HandleFunc("GET /stations", s.getAllStations)
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

func (s *Server) getCities(w http.ResponseWriter, r *http.Request) {
	cities, err := s.db.AllCities(r.Context())
	if err != nil {
		s.internalError(w, "allCities", err)
		return
	}
	writeJSON(w, cities)
}

func (s *Server) getAllStations(w http.ResponseWriter, r *http.Request) {
	stations, err := s.db.AllStations(r.Context())
	if err != nil {
		s.internalError(w, "allStations", err)
		return
	}
	writeJSON(w, stations)
}

func (s *Server) getStationsByCity(w http.ResponseWriter, r *http.Request) {
	uid, err := strconv.Atoi(r.PathValue("uid"))
	if err != nil {
		http.Error(w, "uid must be an integer", http.StatusBadRequest)
		return
	}
	stations, err := s.db.StationsByCity(r.Context(), uid)
	if err != nil {
		s.internalError(w, "stationsByCity", err)
		return
	}
	if len(stations) == 0 {
		http.Error(w, "city not found", http.StatusNotFound)
		return
	}
	writeJSON(w, stations)
}

func (s *Server) getStation(w http.ResponseWriter, r *http.Request) {
	uid, err := strconv.Atoi(r.PathValue("uid"))
	if err != nil {
		http.Error(w, "uid must be an integer", http.StatusBadRequest)
		return
	}
	station, err := s.db.ByUID(r.Context(), uid)
	if err != nil {
		s.internalError(w, "byUID", err)
		return
	}
	if station == nil {
		http.Error(w, "station not found", http.StatusNotFound)
		return
	}
	writeJSON(w, station)
}

func (s *Server) internalError(w http.ResponseWriter, op string, err error) {
	s.log.Error("db query failed", "op", op, "err", err)
	http.Error(w, "internal error", http.StatusInternalServerError)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
