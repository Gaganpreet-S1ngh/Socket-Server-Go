package wsserver

import (
	"encoding/json"
	"net/http"

	"github.com/Gaganpreet-S1ngh/Socket-Server-Go/internal/hub"

	"go.uber.org/zap"
)

// PublishRequest is the payload your Node.js backend sends to push
// data into the websocket layer. Two delivery modes:  - Topic: fan out to everyone subscribed to a topic (e.g. price feed)  - UserID: deliver to one specific user's connection(s) (notification)
type PublishRequest struct {
	Topic   string `json:"topic,omitempty"`
	UserID  string `json:"user_id,omitempty"`
	Type    string `json:"type"`
	Payload any    `json:"payload"`
}

type PublishHandler interface {
	ServeHTTP(w http.ResponseWriter, r *http.Request)
}

type publishHandler struct {
	hub         hub.Hub
	logger      *zap.Logger
	internalKey string // Shared between internal server and this service (eg nodejs and ws)
}

func NewPublishHandler(hub hub.Hub, internalKey string, logger *zap.Logger) PublishHandler {
	return &publishHandler{
		hub:         hub,
		logger:      logger,
		internalKey: internalKey,
	}
}

// ServeHTTP implements [PublishHandler].
func (p *publishHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Handles post /internal/publish
	// never exposed publicly only from private

	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if r.Header.Get("X-Internal-Key") != p.internalKey || p.internalKey == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req PublishRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json body", http.StatusBadRequest)
		return
	}

	if req.Topic == "" && req.UserID == "" {
		http.Error(w, "either topic or user_id is required", http.StatusBadRequest)
		return
	}

	body, err := json.Marshal(map[string]any{
		"type":    req.Type,
		"topic":   req.Topic,
		"payload": req.Payload,
	})

	if err != nil {
		http.Error(w, "failed to encode message", http.StatusInternalServerError)
		return
	}

	var delivered int
	if req.UserID != "" {
		delivered = p.hub.PublishToUser(req.UserID, body)
	} else {
		delivered = p.hub.Publish(req.Topic, body)
	}

	p.logger.Info("Published Message", zap.String("Topic : ", req.Topic), zap.String("User_ID : ", req.UserID))

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"delivered": delivered})
}
