package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/PatrickSteil/nextbike/client"
	"github.com/PatrickSteil/nextbike/db"
	"github.com/PatrickSteil/nextbike/poller"
	"github.com/PatrickSteil/nextbike/server"
)

var version = "dev"

func main() {
	fmt.Println("nextbike version:", version)

	log := slog.New(slog.NewTextHandler(os.Stdout, nil))

	database, err := db.Open("nextbike.db")
	if err != nil {
		log.Error("failed to open db", "err", err)
		os.Exit(1)
	}
	defer database.Close()

	p := poller.New(poller.Config{
		Country:  []string{"DE"},
		Interval: 60 * time.Second,
	}, client.New(), database, log)

	srv := server.New(database, log)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go p.Run(ctx)
	go srv.Start(ctx, ":8080")

	<-ctx.Done()
	log.Info("shutting down")
}
