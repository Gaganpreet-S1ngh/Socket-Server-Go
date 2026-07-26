package hub

import (
	"sync"

	"go.uber.org/zap"
)

// Hub needs a minimum interface to maintain clients instead of importing client we make interface here

type Client interface {
	ID() string
	UserID() string
	Send([]byte) bool // returns full if the client's outbound buffer is full or closed
}

// Central registry to maintain clients of socket connection and manage their channels
type Hub interface {
	Register(c Client)
	Unregister(c Client)
	Subscribe(c Client, topic string)
	Unubscribe(c Client, topic string)
	Publish(topic string, message []byte) int
	PublishToUser(userID string, message []byte) int
	Stats() (int, int)
}

type hub struct {
	mu        sync.RWMutex
	logger     *zap.Logger   
	topics    map[string]map[string]Client   // Topic -> set of Client IDS subscribed to it (string - map)
	clients   map[string]Client              // ClientID -> Client for direct lookups (string - interface)
	userConns map[string]map[string]struct{} // userID -> set of client IDS (string - array) , for eg a user with 2 tabs open 
}

func newHub(logger *zap.Logger) Hub {
	return &hub{
		topics:    make(map[string]map[string]Client),
		clients:   make(map[string]Client),
		userConns: make(map[string]map[string]struct{}),
	}
}

// Publish implements [Hub].
func (h *hub) Publish(topic string, message []byte) int {
	// Sends message to everyone who is subscribed to the topic
	h.mu.RLock()

	// get all the subscribers
	subs := h.topics[topic]

	// Copy all the subscribers under read lock then release it for safe concurrency

	targets := make([]Client , 0 , len(subs))

	for _ , c := range subs {
		targets = append(targets, c)
	}

	h.mu.RUnlock()
	sent := 0

	// Send message to everyone 
	for _ , c := range targets {
		if c.Send(message){
			sent++
		}
	}

	return sent
}

// PublishToUser implements [Hub].
func (h *hub) PublishToUser(userID string, message []byte) int {
	// Sends to all active connection for a particular user

	h.mu.RLock()
	connIDS := h.userConns[userID]
	targets := make([]Client , 0 , len(connIDS))

	for id := range connIDS {
		if c , ok := h.clients[id]; ok {
			targets = append(targets, c)
		}
	}

	h.mu.RUnlock()

	sent := 0

	// Send message to everyone 
	for _ ,c := range targets {
		if c.Send(message){
			sent++
		}
	}

	return sent

}

// Stats implements [Hub].
func (h *hub) Stats() (int, int) {
		h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients), len(h.topics)
}

// Subscribe implements [Hub].
func (h *hub) Subscribe(c Client, topic string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.topics[topic] == nil {
		h.topics[topic] = make(map[string]Client)
	}

	h.topics[topic][c.ID()] = c
}

// Unregister implements [Hub].
func (h *hub) Unregister(c Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Remove the client from clients
	delete(h.clients , c.ID())

	userID := c.UserID()
	// remove that client from users map and if the users map becomes empty for that key then delete the user from the map
	if conns , ok := h.userConns[userID]; ok {
		delete(conns , c.ID())
		if len(conns) == 0 {
			delete(h.userConns , userID)
		}
	}

	// remove the client from the topic and if no one for the topic remove the topic
	for topic , subs := range h.topics {
		if _ , ok := subs[c.ID()] ; ok {
			delete(subs , c.ID())
			if len(subs) == 0 {
				delete(h.topics , topic)
			}
		}
	}

	h.logger.Info("Client Unregistered" , zap.String("Client_ID" , c.ID()) , zap.String("User_ID" , userID))
}

// Unubscribe implements [Hub].
func (h *hub) Unubscribe(c Client, topic string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if subs, ok := h.topics[topic]; ok {
		delete(subs, c.ID())
		if len(subs) == 0 {
			delete(h.topics, topic)
		}
	}
}


// Register implements [Hub].
func (h *hub) Register(c Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	// first add the client
	h.clients[c.ID()] = c
	
	// get the userID from the client
	userID := c.UserID()

	// put the user to map if not exists make one first
	if h.userConns[userID] == nil {
		h.userConns[userID] = make(map[string]struct{})
	}

	h.userConns[userID][c.ID()] = struct{}{} // simply every user id is assigned an array of client ids for multiple tabs

	h.logger.Info("Client Registered" , zap.String("Client_ID" , c.ID()) , zap.String("User_ID" , userID))

}
