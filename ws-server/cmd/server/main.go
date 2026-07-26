package main

import (
	"context"

	"github.com/Gaganpreet-S1ngh/Socket-Server-Go/internal/auth"
	"github.com/Gaganpreet-S1ngh/Socket-Server-Go/internal/config"
	"github.com/Gaganpreet-S1ngh/Socket-Server-Go/internal/hub"
)

func main() {
	/* INITIALIZE ENV AND CONTEXTS */
	cfg := config.LoadConfig()
	rootCtx, rootCancel := context.WithCancel(context.Background())

	h := hub.NewHub(cfg.Logger)
	auth := auth.NewAuth(cfg.JWTSecret, cfg.JWTIssuer)

}
