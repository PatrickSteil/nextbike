package poller

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/PatrickSteil/nextbike/client"
	"github.com/PatrickSteil/nextbike/db"
)

type Config struct {
	CityUIDs []int
	Domains  []string
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

	t := time.NewTicker(p.cfg.Interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			p.log.Info("poller stopped")
			return
		case <-t.C:
			p.tick(ctx)
		}
	}
}

func (p *Poller) tick(ctx context.Context) {
	fetchCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	resp, err := p.client.Fetch(fetchCtx, client.QueryParams{
		CityUIDs: p.cfg.CityUIDs,
		Domains:  p.cfg.Domains,
	})
	if err != nil {
		p.log.Error("fetch failed", "err", err)
		return
	}

	now := time.Now().UTC()

	tx, err := p.db.Begin(ctx)
	if err != nil {
		p.log.Error("begin tx failed", "err", err)
		return
	}

	var n int
	for _, country := range resp.Countries {
		for _, city := range country.Cities {
			for _, place := range city.Places {
				if err := p.db.Upsert(ctx, tx, db.Station{
					UID:                  place.UID,
					Name:                 place.Name,
					CityUID:              city.UID,
					CityName:             city.Name,
					Lat:                  place.Lat,
					Lng:                  place.Lng,
					BikesAvailableToRent: place.BikesAvailableToRent,
					UpdatedAt:            now,
				}); err != nil {
					_ = tx.Rollback()
					p.log.Error("upsert failed", "uid", place.UID, "err", err)
					return
				}
				n++
			}
		}
	}

	if err := tx.Commit(); err != nil {
		p.log.Error("commit failed", "err", fmt.Errorf("%w", err))
		return
	}

	p.log.Info("poll done", "stations", n, "at", now.Format(time.RFC3339))
}
