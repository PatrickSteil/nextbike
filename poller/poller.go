package poller

import (
	"context"
	"log/slog"
	"time"

	"github.com/PatrickSteil/nextbike/client"
	"github.com/PatrickSteil/nextbike/db"
)

const (
	cleanupInterval = 5 * time.Minute
	staleAfter      = 10 * time.Minute
)

type Config struct {
	CityUIDs []int
	Country  []string
	Interval time.Duration
}

type Poller struct {
	cfg    Config
	client *client.Client
	db     *db.DB
	log    *slog.Logger
}

func New(cfg Config, c *client.Client, database *db.DB, log *slog.Logger) *Poller {
	if cfg.Interval == 0 {
		cfg.Interval = 60 * time.Second
	}
	return &Poller{cfg: cfg, client: c, db: database, log: log}
}

func (p *Poller) Run(ctx context.Context) {
	p.log.Info("poller started", "interval", p.cfg.Interval)
	p.tick(ctx)

	pollTicker := time.NewTicker(p.cfg.Interval)
	cleanupTicker := time.NewTicker(cleanupInterval)
	defer pollTicker.Stop()
	defer cleanupTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			p.log.Info("poller stopped")
			return
		case <-pollTicker.C:
			p.tick(ctx)
		case <-cleanupTicker.C:
			p.cleanup(ctx)
		}
	}
}

func (p *Poller) cleanup(ctx context.Context) {
	cleanCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	n, err := p.db.DeleteStale(cleanCtx, staleAfter)
	if err != nil {
		p.log.Error("cleanup failed", "err", err)
		return
	}
	if n > 0 {
		p.log.Info("cleanup removed stale stations", "removed", n)
	}
}

func (p *Poller) tick(ctx context.Context) {
	fetchCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	fetchStart := time.Now()
	resp, err := p.client.Fetch(fetchCtx, client.QueryParams{
		CityUIDs: p.cfg.CityUIDs,
		Country:  p.cfg.Country,
	})
	if err != nil {
		p.log.Error("fetch failed", "err", err)
		return
	}
	fetchDur := time.Since(fetchStart)

	var stations []db.Station
	for _, country := range resp.Countries {
		for _, city := range country.Cities {
			for _, place := range city.Places {
				stations = append(stations, db.Station{
					UID:                  place.UID,
					Name:                 place.Name,
					CityUID:              city.UID,
					CityName:             city.Name,
					Lat:                  place.Lat,
					Lng:                  place.Lng,
					BikesAvailableToRent: place.BikesAvailableToRent,
					UpdatedAt:            fetchStart.UTC(),
				})
			}
		}
	}

	writeStart := time.Now()
	if err := p.db.UpsertLive(ctx, stations); err != nil {
		p.log.Error("upsert live failed", "err", err)
		return
	}
	writeDur := time.Since(writeStart)

	p.log.Info("poll done",
		"stations", len(stations),
		"fetch", fetchDur.Round(time.Millisecond),
		"write", writeDur.Round(time.Millisecond),
	)
}
