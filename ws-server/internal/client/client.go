package client

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/Gaganpreet-S1ngh/Socket-Server-Go/internal/hub"
	"github.com/Gaganpreet-S1ngh/Socket-Server-Go/internal/ratelimit"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// Message sent by the client to the server
type InboundMessage struct {
	Action string `json:"action"` // "subscribe" | "unsubscribe" | "ping"
	Topic  string `json:"topic,omitempty"`
}

// OutboundMessage is what the server pushes to clients.
type OutboundMessage struct {
	Type    string `json:"type"`
	Topic   string `json:"topic,omitempty"`
	Payload any    `json:"payload,omitempty"`
}

type Config struct {
	PingInterval    time.Duration
	PongWait        time.Duration
	WriteWait       time.Duration
	MaxMessageBytes int64
	SendBufferSize  int
}

// Client represents one live websocket connection.

type Client interface {
	Send(msg []byte) bool
	ID() string
	UserID() string
	Close()
	Run()
}

type client struct {
	id     string
	userID string

	conn   *websocket.Conn
	hub    hub.Hub
	cfg    Config
	logger *zap.Logger

	limiter *ratelimit.Limiter

	// Queue for sending message to the client
	send chan []byte

	// subscribed topics, tracked locally so we can clean up on disconnect
	// and enforce a max-subscriptions-per-client limit
	mu     sync.Mutex
	topics map[string]struct{}

	// use to call a function once only among various goroutines
	closeOnce sync.Once
	done      chan struct{}
}

func NewClient(clientID string, userID string, conn *websocket.Conn, hub hub.Hub, cfg Config, limiter *ratelimit.Limiter, logger *zap.Logger) Client {
	return &client{
		id:      clientID,
		userID:  userID,
		conn:    conn,
		hub:     hub,
		cfg:     cfg,
		logger:  logger,
		limiter: limiter,
		send:    make(chan []byte, cfg.SendBufferSize),
		topics:  make(map[string]struct{}),
		done:    make(chan struct{}),
	}
}

// Close implements [Client].
func (c *client) Close() {
	c.closeOnce.Do(func() {
		close(c.done) // calls <-done so that goroutines know that the connection is closed
		c.hub.Unregister(c)
		_ = c.conn.Close()
	})
}

// ID implements [Client].
func (c *client) ID() string {
	return c.id
}

// Run implements [Client].
func (c *client) Run() {
	// Starts the readLoop and writeLoop and blocks until the connection closes
	c.hub.Register(c)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		c.readLoop()
	}()
	go func() {
		defer wg.Done()
		c.writeLoop()
	}()
	wg.Wait()
}

// Send implements [Client].
func (c *client) Send(msg []byte) bool {
	// Queue a message to the client non blocking if the client buffer full (slow / stuck consumer) we drop the connection rather than let one bad client back pressure whole server memory
	select {
	case c.send <- msg:
		return true
	default:
		c.logger.Warn("Send buffer full , dropping connection", zap.String("Client_ID", c.id))
		c.Close()
		return false
	}
}

// UserID implements [Client].
func (c *client) UserID() string {
	return c.userID
}

//==========================================//
//             PRIVATE FUNCTIONS            //
//==========================================//

func (c *client) readLoop() {
	defer c.Close()

	// Set the read limit as conn is unlimited hence doesnt know when to stop
	c.conn.SetReadLimit(c.cfg.MaxMessageBytes)
	_ = c.conn.SetReadDeadline(time.Now().Add(c.cfg.PongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(c.cfg.PongWait))
	})

	for {
		_, raw, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err,
				websocket.CloseGoingAway,
				websocket.CloseNormalClosure,
				websocket.CloseNoStatusReceived) {
				c.logger.Error("Unexpected Close", zap.Error(err))
			}
			return
		}

		if !c.limiter.Allow() {
			c.sendError("rate_limited", "too many messages, slow down")
			continue
		}

		c.handleMessage(raw)

	}

}

func (c *client) handleMessage(raw []byte) {
	var msg InboundMessage
	if err := json.Unmarshal(raw, &msg); err != nil {
		c.sendError("bad_request", "invalid message format")
		return
	}

	switch msg.Action {
	case "subscribe":
		if msg.Topic == "" {
			c.sendError("bad_request", "topic is required")
			return
		}
		c.mu.Lock()
		if len(c.topics) >= 40 {
			c.mu.Unlock()
			c.sendError("limit_exceeded", "max subscriptions reached")
			return
		}

		c.topics[msg.Topic] = struct{}{}
		c.mu.Unlock()

		c.hub.Subscribe(c, msg.Topic)
		c.sendAck("subscribed", msg.Topic)

	case "unsubscribe":
		c.mu.Lock()
		delete(c.topics, msg.Topic)
		c.mu.Unlock()

		c.hub.Unsubscribe(c, msg.Topic)
		c.sendAck("unsubscribed", msg.Topic)

	case "ping":
		c.sendAck("pong", "")

	default:
		c.sendError("bad_request", "unknown action")
	}
}

func (c *client) writeLoop() {
	ticker := time.NewTicker(c.cfg.PingInterval)
	defer func() {
		ticker.Stop()
		c.Close()
	}()

	for {
		select {
		case msg, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(c.cfg.WriteWait))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}

		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(c.cfg.WriteWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}

		case <-c.done:
			return
		}
	}
}

func (c *client) sendAck(typ, topic string) {
	b, _ := json.Marshal(OutboundMessage{Type: typ, Topic: topic})
	c.Send(b)

}

func (c *client) sendError(code, message string) {
	b, _ := json.Marshal(OutboundMessage{Type: "error", Payload: map[string]string{"code": code, "message": message}})
	c.Send(b)
}
