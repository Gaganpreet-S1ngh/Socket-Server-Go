package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Gaganpreet-S1ngh/Socket-Server-Go/internal/auth"
	"github.com/Gaganpreet-S1ngh/Socket-Server-Go/internal/config"
	"github.com/Gaganpreet-S1ngh/Socket-Server-Go/internal/hub"
	"github.com/Gaganpreet-S1ngh/Socket-Server-Go/internal/middleware"
	"github.com/Gaganpreet-S1ngh/Socket-Server-Go/internal/wsserver"
	"go.uber.org/zap"
)

func main() {
	/* INITIALIZE ENV AND CONTEXTS */
	cfg := config.LoadConfig()

	h := hub.NewHub(cfg.Logger)
	auth := auth.NewAuth(cfg.JWTSecret, cfg.JWTIssuer)
	wsHandler := wsserver.NewWSHandler(h, auth, cfg.CookieSecret, cfg.AllowedOrigins, cfg.Logger)
	publishHandler := wsserver.NewPublishHandler(h, cfg.InternalKey, cfg.Logger)

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", wsHandler.ServeWS)
	mux.Handle("/internal/publish", publishHandler)
	mux.HandleFunc("/healthz", healthzHandler(h))

	handler := middleware.Recover(mux)

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      handler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		cfg.Logger.Info("Starting Websocket Server", zap.String("Port : ", cfg.Port))

		err := srv.ListenAndServe()

		if err != nil {
			log.Fatalf("Server Failed : %s", err.Error())
		}

	}()

	// Graceful shutdown: on SIGINT/SIGTERM (what Docker/Kubernetes send),
	// stop accepting new connections and give existing ones time to close
	// cleanly instead of just killing the process mid-write.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	cfg.Logger.Info("shutdown signal received, draining connections")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		cfg.Logger.Info("Forcing Server Shutdown")
		_ = srv.Close()
	}

	cfg.Logger.Info("Server Closed")

}

func healthzHandler(h hub.Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		clients, topics := h.Stats()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":  "ok",
			"clients": clients,
			"topics":  topics,
		})
	}
}
