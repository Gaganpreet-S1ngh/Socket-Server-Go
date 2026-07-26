package main

import (
	"context"

	"github.com/Gaganpreet-S1ngh/Socket-Server-Go/internal/config"
)

func main() {
	/* INITIALIZE ENV AND CONTEXTS */
	cfg := config.LoadConfig()
	rootCtx, rootCancel := context.WithCancel(context.Background())

}