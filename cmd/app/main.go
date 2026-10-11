package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/kejne/gascity-irc-pack/internal/bridge"
)

func main() {
	cfg, err := bridge.LoadConfig(os.Getenv)
	if err != nil {
		log.Fatal(err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	client := &bridge.Bridge{Config: cfg}
	if cfg.RegisterAdapter {
		if err := client.RegisterAdapter(ctx); err != nil {
			log.Fatalf("register extmsg adapter: %v", err)
		}
	}
	if err := client.Run(ctx); err != nil && ctx.Err() == nil {
		log.Fatal(err)
	}
}
