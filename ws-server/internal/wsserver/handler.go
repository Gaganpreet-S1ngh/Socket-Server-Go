package wsserver

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Gaganpreet-S1ngh/Socket-Server-Go/internal/auth"
	"github.com/Gaganpreet-S1ngh/Socket-Server-Go/internal/client"
	"github.com/Gaganpreet-S1ngh/Socket-Server-Go/internal/hub"
	"github.com/Gaganpreet-S1ngh/Socket-Server-Go/internal/ratelimit"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

type WSHandler interface {
	ServeWS(w http.ResponseWriter, r *http.Request)
}

type wsHandler struct {
	cookieSecret   string
	logger         *zap.Logger
	hub            hub.Hub
	auth           auth.Auth
	upgrader       websocket.Upgrader
	allowedOrigins []string
}

func NewWSHandler(hub hub.Hub, auth auth.Auth, cookieSecret string, allowedOrigins []string, logger *zap.Logger) WSHandler {

	// build a map for quicker search
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, o := range allowedOrigins {
		allowed[o] = struct{}{}
	}

	return &wsHandler{
		cookieSecret: cookieSecret,
		hub:          hub,
		auth:         auth,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  4096,
			WriteBufferSize: 4096,

			// CheckOrigin is the browser-facing CSRF-style defense for
			// websockets: browsers don't enforce CORS on WS upgrades,
			// so without this check any website could open a connection
			// to your server using a logged-in user's cookies/token.

			CheckOrigin: func(r *http.Request) bool {
				origin := r.Header.Get("Origin")
				if origin == "" {
					// No Origin header usually means a non-browser client
					// (mobile app, server-to-server, curl/wscat). Allow it -
					// auth (JWT) is still required either way.
					return true
				}

				_, ok := allowed[origin]
				if !ok {
					logger.Warn("Rejected websocket upgrade : Origin not allowed", zap.String("Origin : ", origin))
				}
				return ok
			},
		},
	}
}

// ServeWS implements [WSHandler].
func (ws *wsHandler) ServeWS(w http.ResponseWriter, r *http.Request) {
	// handles Get /ws

	// get the token
	token, err := r.Cookie("session")

	if err != nil {
		if err == http.ErrNoCookie {
			http.Error(w, "no cookie", http.StatusUnauthorized)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// verify if the cookie is signed
	rawToken, err := ws.verifySignedCookie(token.Value)

	fmt.Println(rawToken)

	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	claims, err := ws.auth.VerifyAccessToken(rawToken)
	if err != nil {
		ws.logger.Error("Websocket Auth Failed : ", zap.String("Remote_Addr : ", r.RemoteAddr), zap.Error(err))
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	conn, err := ws.upgrader.Upgrade(w, r, nil)
	if err != nil {
		ws.logger.Error("Websocket upgrade failed", zap.Error(err))
		return
	}

	clientID := newClientID()
	limiter := ratelimit.New(5, 10)

	newClient := client.NewClient(clientID, claims.UserID, conn, ws.hub, client.Config{
		PingInterval:    30 * time.Second,
		PongWait:        60 * time.Second,
		WriteWait:       10 * time.Second,
		MaxMessageBytes: 32768,
		SendBufferSize:  256,
	}, limiter, ws.logger)

	ws.logger.Info("Client Connected", zap.String("Client_ID", clientID), zap.String("User_ID", claims.UserID))

	go newClient.Run()

}

//==========================================//
//             PRIVATE FUNCTIONS            //
//==========================================//

func (ws *wsHandler) verifySignedCookie(cookieValue string) (string, error) {
	decoded, err := url.QueryUnescape(cookieValue)

	if err != nil {
		return "", fmt.Errorf("failed to decode cookie: %w", err)
	}

	if !strings.HasPrefix(decoded, "s:") {
		return "", fmt.Errorf("not a signed cookie")
	}

	signedValue := strings.TrimPrefix(decoded, "s:")

	idx := strings.LastIndex(signedValue, ".")
	if idx == -1 {
		return "", fmt.Errorf("invalid signed cookie format")
	}

	value := signedValue[:idx]
	providedSignature := signedValue[idx+1:]

	mac := hmac.New(sha256.New, []byte(ws.cookieSecret))
	mac.Write([]byte(value))

	expectedSignature := base64.RawStdEncoding.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(providedSignature), []byte(expectedSignature)) {
		return "", fmt.Errorf("invalid signature")
	}

	return value, nil
}

func newClientID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
