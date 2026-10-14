package main

import (
	"context"
	"encoding/json"
	"flag"
	"github.com/mbfczzz/FlowLight-Accelerator/internal/agent"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	path := flag.String("config", "agent.json", "node configuration file")
	flag.Parse()
	b, e := os.ReadFile(*path)
	if e != nil {
		log.Fatal(e)
	}
	var cfg agent.Config
	if e = json.Unmarshal(b, &cfg); e != nil {
		log.Fatal(e)
	}
	client, e := agent.Open(cfg)
	if e != nil {
		log.Fatal(e)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	log.Print("FlowLight node agent started")
	_ = client.Run(ctx)
}
