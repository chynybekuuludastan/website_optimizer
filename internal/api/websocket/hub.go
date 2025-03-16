package websocket

import (
	"encoding/json"
	"log"
	"sync"

	"github.com/gofiber/websocket/v2"
	"github.com/google/uuid"
)

// Client represents a connected WebSocket client
type Client struct {
	conn       *websocket.Conn
	analysisID uuid.UUID
	send       chan []byte
}

// Message represents a WebSocket message
type Message struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

// Hub maintains the set of active clients and broadcasts messages
type Hub struct {
	// Registered clients by analysis ID
	clients map[uuid.UUID]map[*Client]bool

	// Register requests from clients
	register chan *Client

	// Unregister requests from clients
	unregister chan *Client

	// Guard clients map
	mu sync.RWMutex
}

// NewHub creates a new websocket hub
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[uuid.UUID]map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

// Run starts the hub's message handling loop
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			// Create map for analysis if it doesn't exist
			if _, ok := h.clients[client.analysisID]; !ok {
				h.clients[client.analysisID] = make(map[*Client]bool)
			}
			h.clients[client.analysisID][client] = true
			h.mu.Unlock()

		case client := <-h.unregister:
			h.mu.Lock()
			// Remove client
			if _, ok := h.clients[client.analysisID]; ok {
				delete(h.clients[client.analysisID], client)
				close(client.send)

				// If no more clients for this analysis, remove the map
				if len(h.clients[client.analysisID]) == 0 {
					delete(h.clients, client.analysisID)
				}
			}
			h.mu.Unlock()
		}
	}
}

// Register registers a new client connection
func (h *Hub) Register(client *Client) {
	h.register <- client
}

// Unregister removes a client connection
func (h *Hub) Unregister(client *Client) {
	h.unregister <- client
}

// BroadcastToAnalysis sends a message to all clients subscribed to an analysis
func (h *Hub) BroadcastToAnalysis(analysisID uuid.UUID, message Message) {
	// Convert message to JSON
	messageJSON, err := json.Marshal(message)
	if err != nil {
		log.Printf("Error marshalling WebSocket message: %v", err)
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	// Get clients for this analysis
	clients, ok := h.clients[analysisID]
	if !ok {
		return // No clients for this analysis
	}

	// Send to all clients
	for client := range clients {
		select {
		case client.send <- messageJSON:
			// Message sent successfully
		default:
			// Client's send buffer is full, unregister
			go h.Unregister(client)
		}
	}
}

// HandleConnection handles an incoming WebSocket connection
func (h *Hub) HandleConnection(conn *websocket.Conn, analysisID uuid.UUID) {
	client := &Client{
		conn:       conn,
		analysisID: analysisID,
		send:       make(chan []byte, 256),
	}

	// Register client
	h.Register(client)

	// Send initial status
	initialMsg := Message{
		Type: "connected",
		Data: map[string]interface{}{
			"analysis_id": analysisID.String(),
			"status":      "connected",
		},
	}
	msgJSON, _ := json.Marshal(initialMsg)
	client.send <- msgJSON

	// Start goroutines for reading and writing
	go client.writePump()
	go client.readPump(h)
}

// writePump pumps messages from the hub to the websocket connection
func (c *Client) writePump() {
	defer func() {
		c.conn.Close()
	}()

	for {
		message, ok := <-c.send
		if !ok {
			// Hub closed the channel
			c.conn.WriteMessage(websocket.CloseMessage, []byte{})
			return
		}

		err := c.conn.WriteMessage(websocket.TextMessage, message)
		if err != nil {
			return
		}
	}
}

// readPump pumps messages from the websocket connection to the hub
func (c *Client) readPump(h *Hub) {
	defer func() {
		h.Unregister(c)
		c.conn.Close()
	}()

	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}
		// We don't need to process incoming messages for this implementation
	}
}
