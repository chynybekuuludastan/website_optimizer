package websocket

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gofiber/websocket/v2"
	"github.com/google/uuid"
)

// MessageType defines standard message types for the WebSocket system
type MessageType string

const (
	// Analysis status message types
	TypeAnalysisStarted             MessageType = "analysis_started"
	TypeAnalysisProgress            MessageType = "analysis_progress"
	TypeAnalysisError               MessageType = "analysis_error"
	TypeAnalysisCompleted           MessageType = "analysis_completed"
	TypePartialResults              MessageType = "partial_results"
	TypeContentImprovementStarted   MessageType = "content_improvement_started"
	TypeContentImprovementCompleted MessageType = "content_improvement_completed"
	TypeContentImprovementFailed    MessageType = "content_improvement_failed"

	// System message types
	TypePing                  MessageType = "ping"
	TypePong                  MessageType = "pong"
	TypeAcknowledgment        MessageType = "ack"
	TypeConnectionEstablished MessageType = "connection_established"
	TypeSystemNotification    MessageType = "system_notification"
	TypeWarning               MessageType = "warning"

	// Control message types
	TypeControlPause        MessageType = "control_pause"
	TypeControlResume       MessageType = "control_resume"
	TypeControlCancel       MessageType = "control_cancel"
	TypeControlUpdateParams MessageType = "control_update_params"
	TypeControlResponse     MessageType = "control_response"

	// Room message types
	TypeRoomJoined     MessageType = "room_joined"
	TypeRoomLeft       MessageType = "room_left"
	TypeRoomMessage    MessageType = "room_message"
	TypeRoomUserJoined MessageType = "room_user_joined"
	TypeRoomUserLeft   MessageType = "room_user_left"
)

// AnalysisCategory defines the categories for analysis components
type AnalysisCategory string

const (
	CategorySEO           AnalysisCategory = "seo"
	CategoryPerformance   AnalysisCategory = "performance"
	CategoryStructure     AnalysisCategory = "structure"
	CategoryAccessibility AnalysisCategory = "accessibility"
	CategorySecurity      AnalysisCategory = "security"
	CategoryMobile        AnalysisCategory = "mobile"
	CategoryContent       AnalysisCategory = "content"
	CategoryLighthouse    AnalysisCategory = "lighthouse"
	CategoryAll           AnalysisCategory = "all"
)

// Message represents a standardized WebSocket message
type Message struct {
	Type       MessageType            `json:"type"`                  // Message type
	Status     string                 `json:"status,omitempty"`      // Status information
	Progress   float64                `json:"progress,omitempty"`    // Progress percentage (0-100)
	Category   AnalysisCategory       `json:"category,omitempty"`    // Analysis category
	Data       interface{}            `json:"data,omitempty"`        // Payload data
	Timestamp  time.Time              `json:"timestamp"`             // Message timestamp
	SequenceID int64                  `json:"sequence_id,omitempty"` // Message sequence number
	RequestID  string                 `json:"request_id,omitempty"`  // ID for messages requiring acknowledgment
	IsAck      bool                   `json:"is_ack,omitempty"`      // Indicates an acknowledgment message
	AckID      string                 `json:"ack_id,omitempty"`      // ID of the acknowledged message
	Meta       map[string]interface{} `json:"meta,omitempty"`        // Additional metadata
}

// Client represents a connected WebSocket client
type Client struct {
	conn         *websocket.Conn    // WebSocket connection
	analysisID   uuid.UUID          // Analysis the client is subscribed to
	rooms        map[string]bool    // Rooms the client has joined
	send         chan []byte        // Channel for messages to be sent
	lastActivity time.Time          // Timestamp of the last activity
	pendingAcks  map[string]Message // Messages waiting for acknowledgment
	receivedAcks map[string]bool    // Tracks received acknowledgments
	ackTimeout   time.Duration      // Timeout for acknowledgment
	userID       uuid.UUID          // User ID for authorization purposes
	username     string             // Username for identification
	permissions  map[string]bool    // User permissions
	hub          *Hub               // Reference to the hub
	closeSignal  chan struct{}      // Signal for closing the client
	writeMutex   sync.Mutex         // Mutex for write operations
	isClosed     bool               // Indicates if the client is closed
}

// RoomRequest represents a request to join or leave a room
type RoomRequest struct {
	Client *Client
	RoomID string
}

// ControlRequest represents a request to control an analysis
type ControlRequest struct {
	Client     *Client
	AnalysisID uuid.UUID
	Action     MessageType
	Params     map[string]interface{}
}

// Hub maintains the set of active clients and broadcasts messages
type Hub struct {
	// Maps for client management
	clients    map[uuid.UUID]map[*Client]bool // Clients by analysis ID
	rooms      map[string]map[*Client]bool    // Clients by room ID
	allClients map[*Client]bool               // All connected clients

	// Control channels
	register   chan *Client         // Register requests
	unregister chan *Client         // Unregister requests
	joinRoom   chan *RoomRequest    // Room join requests
	leaveRoom  chan *RoomRequest    // Room leave requests
	control    chan *ControlRequest // Analysis control requests

	// Messaging channels
	broadcastToAnalysis chan *BroadcastMessage // Messages to analysis clients
	broadcastToRoom     chan *BroadcastMessage // Messages to room clients
	broadcastToAll      chan *BroadcastMessage // Messages to all clients

	// Sequence counter for message ordering
	sequence int64

	// Synchronization primitives
	mu        sync.RWMutex // Mutex for client/room maps
	controlMu sync.RWMutex // Mutex for control operations

	// Configuration
	bufferSize   int           // Message buffer size per client
	pingInterval time.Duration // Interval for sending ping messages

	// Message buffers for temporary disconnections
	messageBuffers map[uuid.UUID][]Message // Buffer for analysis messages
	roomBuffers    map[string][]Message    // Buffer for room messages

	// Control handlers
	controlHandlers map[MessageType]func(*ControlRequest) interface{}
}

// BroadcastMessage represents a message to be broadcast
type BroadcastMessage struct {
	TargetID   interface{} // UUID for analysis, string for room
	Message    Message     // Message to be sent
	RequireAck bool        // Whether acknowledgment is required
}

// NewHub creates a new hub
func NewHub() *Hub {
	return &Hub{
		clients:             make(map[uuid.UUID]map[*Client]bool),
		rooms:               make(map[string]map[*Client]bool),
		allClients:          make(map[*Client]bool),
		register:            make(chan *Client),
		unregister:          make(chan *Client),
		joinRoom:            make(chan *RoomRequest),
		leaveRoom:           make(chan *RoomRequest),
		control:             make(chan *ControlRequest),
		broadcastToAnalysis: make(chan *BroadcastMessage),
		broadcastToRoom:     make(chan *BroadcastMessage),
		broadcastToAll:      make(chan *BroadcastMessage),
		messageBuffers:      make(map[uuid.UUID][]Message),
		roomBuffers:         make(map[string][]Message),
		controlHandlers:     make(map[MessageType]func(*ControlRequest) interface{}),
		bufferSize:          100,
		pingInterval:        30 * time.Second,
	}
}

// RegisterControlHandler registers a handler for a control action
func (h *Hub) RegisterControlHandler(actionType MessageType, handler func(*ControlRequest) interface{}) {
	h.controlMu.Lock()
	defer h.controlMu.Unlock()
	h.controlHandlers[actionType] = handler
}

// Run starts the hub
func (h *Hub) Run() {
	// Start the ping service
	pingTicker := time.NewTicker(h.pingInterval)
	defer pingTicker.Stop()

	for {
		select {
		case client := <-h.register:
			h.registerClient(client)

		case client := <-h.unregister:
			h.unregisterClient(client)

		case roomReq := <-h.joinRoom:
			h.joinClientToRoom(roomReq.Client, roomReq.RoomID)

		case roomReq := <-h.leaveRoom:
			h.removeClientFromRoom(roomReq.Client, roomReq.RoomID)

		case controlReq := <-h.control:
			h.handleControlRequest(controlReq)

		case broadcast := <-h.broadcastToAnalysis:
			h.sendToAnalysisClients(broadcast)

		case broadcast := <-h.broadcastToRoom:
			h.sendToRoomClients(broadcast)

		case broadcast := <-h.broadcastToAll:
			h.sendToAllClients(broadcast)

		case <-pingTicker.C:
			h.sendPingToAllClients()
		}
	}
}

// registerClient registers a new client
func (h *Hub) registerClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Initialize client in all clients map
	h.allClients[client] = true

	// Initialize client for this analysis if necessary
	if _, ok := h.clients[client.analysisID]; !ok {
		h.clients[client.analysisID] = make(map[*Client]bool)
	}
	h.clients[client.analysisID][client] = true

	// Send connection established message
	connMsg := Message{
		Type:       TypeConnectionEstablished,
		Timestamp:  time.Now(),
		SequenceID: h.nextSequence(),
		Data: map[string]interface{}{
			"analysis_id": client.analysisID.String(),
			"message":     "WebSocket connection established",
		},
	}

	msgJSON, _ := json.Marshal(connMsg)
	client.send <- msgJSON

	// Send any buffered messages for this analysis
	if messages, ok := h.messageBuffers[client.analysisID]; ok {
		for _, msg := range messages {
			msgJSON, _ := json.Marshal(msg)
			client.send <- msgJSON
		}
		// Clear the buffer after sending
		delete(h.messageBuffers, client.analysisID)
	}
}

// unregisterClient removes a client
func (h *Hub) unregisterClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Check if the client exists in the hub
	if _, ok := h.allClients[client]; !ok {
		return
	}

	// Remove from all clients
	delete(h.allClients, client)

	// Remove from analysis clients
	if clients, ok := h.clients[client.analysisID]; ok {
		delete(clients, client)
		// If no more clients for this analysis, remove the map
		if len(clients) == 0 {
			delete(h.clients, client.analysisID)
		}
	}

	// Remove from all rooms
	for roomID := range client.rooms {
		if roomClients, ok := h.rooms[roomID]; ok {
			delete(roomClients, client)
			// If no more clients in the room, remove the room
			if len(roomClients) == 0 {
				delete(h.rooms, roomID)
				delete(h.roomBuffers, roomID)
			} else {
				// Notify other clients in the room
				leaveMsg := Message{
					Type:       TypeRoomUserLeft,
					Timestamp:  time.Now(),
					SequenceID: h.nextSequence(),
					Data: map[string]interface{}{
						"room_id":  roomID,
						"username": client.username,
						"user_id":  client.userID.String(),
					},
				}
				h.broadcastToRoom <- &BroadcastMessage{
					TargetID: roomID,
					Message:  leaveMsg,
				}
			}
		}
	}

	// Close the send channel
	close(client.send)
}

// joinClientToRoom adds a client to a room
func (h *Hub) joinClientToRoom(client *Client, roomID string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Initialize room if necessary
	if _, ok := h.rooms[roomID]; !ok {
		h.rooms[roomID] = make(map[*Client]bool)
	}

	// Add client to room
	h.rooms[roomID][client] = true

	// Update client's room membership
	client.rooms[roomID] = true

	// Send join confirmation to client
	joinMsg := Message{
		Type:       TypeRoomJoined,
		Timestamp:  time.Now(),
		SequenceID: h.nextSequence(),
		Data: map[string]interface{}{
			"room_id": roomID,
			"message": fmt.Sprintf("Joined room: %s", roomID),
		},
	}

	msgJSON, _ := json.Marshal(joinMsg)
	client.send <- msgJSON

	// Notify other clients in the room
	userJoinedMsg := Message{
		Type:       TypeRoomUserJoined,
		Timestamp:  time.Now(),
		SequenceID: h.nextSequence(),
		Data: map[string]interface{}{
			"room_id":  roomID,
			"username": client.username,
			"user_id":  client.userID.String(),
		},
	}

	// Send to all clients in the room except the joining client
	for roomClient := range h.rooms[roomID] {
		if roomClient != client {
			msgJSON, _ := json.Marshal(userJoinedMsg)
			roomClient.send <- msgJSON
		}
	}

	// Send buffered messages for this room
	if messages, ok := h.roomBuffers[roomID]; ok {
		for _, msg := range messages {
			msgJSON, _ := json.Marshal(msg)
			client.send <- msgJSON
		}
	}
}

// removeClientFromRoom removes a client from a room
func (h *Hub) removeClientFromRoom(client *Client, roomID string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Check if room exists
	roomClients, ok := h.rooms[roomID]
	if !ok {
		return
	}

	// Remove client from room
	delete(roomClients, client)
	delete(client.rooms, roomID)

	// If room is empty, remove it
	if len(roomClients) == 0 {
		delete(h.rooms, roomID)
		delete(h.roomBuffers, roomID)
	} else {
		// Notify other clients
		leaveMsg := Message{
			Type:       TypeRoomUserLeft,
			Timestamp:  time.Now(),
			SequenceID: h.nextSequence(),
			Data: map[string]interface{}{
				"room_id":  roomID,
				"username": client.username,
				"user_id":  client.userID.String(),
			},
		}

		msgJSON, _ := json.Marshal(leaveMsg)
		for roomClient := range roomClients {
			roomClient.send <- msgJSON
		}
	}

	// Send confirmation to client
	leaveConfirmMsg := Message{
		Type:       TypeRoomLeft,
		Timestamp:  time.Now(),
		SequenceID: h.nextSequence(),
		Data: map[string]interface{}{
			"room_id": roomID,
			"message": fmt.Sprintf("Left room: %s", roomID),
		},
	}

	msgJSON, _ := json.Marshal(leaveConfirmMsg)
	client.send <- msgJSON
}

// handleControlRequest processes control requests
func (h *Hub) handleControlRequest(req *ControlRequest) {
	h.controlMu.RLock()
	handler, exists := h.controlHandlers[req.Action]
	h.controlMu.RUnlock()

	// If no handler exists, send error response
	if !exists {
		errMsg := Message{
			Type:       TypeControlResponse,
			Status:     "error",
			Timestamp:  time.Now(),
			SequenceID: h.nextSequence(),
			Data: map[string]interface{}{
				"message":     "Unsupported control action",
				"action":      req.Action,
				"analysis_id": req.AnalysisID.String(),
			},
		}

		msgJSON, _ := json.Marshal(errMsg)
		req.Client.send <- msgJSON
		return
	}

	// Execute the handler
	result := handler(req)

	// Send response
	respMsg := Message{
		Type:       TypeControlResponse,
		Status:     "success",
		Timestamp:  time.Now(),
		SequenceID: h.nextSequence(),
		Data: map[string]interface{}{
			"action":      req.Action,
			"analysis_id": req.AnalysisID.String(),
			"result":      result,
		},
	}

	msgJSON, _ := json.Marshal(respMsg)
	req.Client.send <- msgJSON
}

// sendToAnalysisClients sends a message to all clients of an analysis
func (h *Hub) sendToAnalysisClients(broadcast *BroadcastMessage) {
	analysisID, ok := broadcast.TargetID.(uuid.UUID)
	if !ok {
		log.Printf("Invalid analysis ID type: %T", broadcast.TargetID)
		return
	}

	// Set timestamp and sequence ID if not already set
	if broadcast.Message.Timestamp.IsZero() {
		broadcast.Message.Timestamp = time.Now()
	}
	if broadcast.Message.SequenceID == 0 {
		broadcast.Message.SequenceID = h.nextSequence()
	}

	h.mu.RLock()
	clients, exists := h.clients[analysisID]
	h.mu.RUnlock()

	// If no clients are connected, buffer the message
	if !exists || len(clients) == 0 {
		h.bufferAnalysisMessage(analysisID, broadcast.Message)
		return
	}

	// Convert message to JSON
	msgJSON, err := json.Marshal(broadcast.Message)
	if err != nil {
		log.Printf("Error marshalling message: %v", err)
		return
	}

	// Send to all clients for this analysis
	for client := range clients {
		// If acknowledgment is required, store in pending acks
		if broadcast.RequireAck {
			client.pendingAcks[broadcast.Message.RequestID] = broadcast.Message

			// Schedule cleanup in case acknowledgment never arrives
			go func(c *Client, reqID string) {
				select {
				case <-time.After(c.ackTimeout):
					// Remove from pending if still there
					if _, exists := c.pendingAcks[reqID]; exists {
						delete(c.pendingAcks, reqID)

						// Send timeout notification
						timeoutMsg := Message{
							Type:       TypeWarning,
							Status:     "ack_timeout",
							Timestamp:  time.Now(),
							SequenceID: h.nextSequence(),
							Data: map[string]interface{}{
								"request_id": reqID,
								"message":    "Acknowledgment timeout",
							},
						}

						msgJSON, _ := json.Marshal(timeoutMsg)
						select {
						case c.send <- msgJSON:
						default:
							// Channel is full or closed
						}
					}
				case <-c.closeSignal:
					// Client is closing, no need to clean up
					return
				}
			}(client, broadcast.Message.RequestID)
		}

		// Try to send the message
		select {
		case client.send <- msgJSON:
			// Message sent successfully
		default:
			// Channel is full or closed, unregister the client
			h.unregister <- client
		}
	}
}

// sendToRoomClients sends a message to all clients in a room
func (h *Hub) sendToRoomClients(broadcast *BroadcastMessage) {
	roomID, ok := broadcast.TargetID.(string)
	if !ok {
		log.Printf("Invalid room ID type: %T", broadcast.TargetID)
		return
	}

	// Set timestamp and sequence ID if not already set
	if broadcast.Message.Timestamp.IsZero() {
		broadcast.Message.Timestamp = time.Now()
	}
	if broadcast.Message.SequenceID == 0 {
		broadcast.Message.SequenceID = h.nextSequence()
	}

	h.mu.RLock()
	clients, exists := h.rooms[roomID]
	h.mu.RUnlock()

	// If no clients are in the room, buffer the message
	if !exists || len(clients) == 0 {
		h.bufferRoomMessage(roomID, broadcast.Message)
		return
	}

	// Convert message to JSON
	msgJSON, err := json.Marshal(broadcast.Message)
	if err != nil {
		log.Printf("Error marshalling message: %v", err)
		return
	}

	// Send to all clients in the room
	for client := range clients {
		select {
		case client.send <- msgJSON:
			// Message sent successfully
		default:
			// Channel is full or closed, unregister the client
			h.unregister <- client
		}
	}
}

// sendToAllClients sends a message to all connected clients
func (h *Hub) sendToAllClients(broadcast *BroadcastMessage) {
	// Set timestamp and sequence ID if not already set
	if broadcast.Message.Timestamp.IsZero() {
		broadcast.Message.Timestamp = time.Now()
	}
	if broadcast.Message.SequenceID == 0 {
		broadcast.Message.SequenceID = h.nextSequence()
	}

	// Convert message to JSON
	msgJSON, err := json.Marshal(broadcast.Message)
	if err != nil {
		log.Printf("Error marshalling message: %v", err)
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	// Send to all clients
	for client := range h.allClients {
		select {
		case client.send <- msgJSON:
			// Message sent successfully
		default:
			// Channel is full or closed, unregister the client
			go func(c *Client) {
				h.unregister <- c
			}(client)
		}
	}
}

// bufferAnalysisMessage buffers a message for an analysis with no connected clients
func (h *Hub) bufferAnalysisMessage(analysisID uuid.UUID, msg Message) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Initialize buffer if needed
	if _, ok := h.messageBuffers[analysisID]; !ok {
		h.messageBuffers[analysisID] = make([]Message, 0, h.bufferSize)
	}

	// Add message to buffer, respecting size limit
	buffer := h.messageBuffers[analysisID]
	if len(buffer) >= h.bufferSize {
		// Remove oldest message
		buffer = buffer[1:]
	}
	buffer = append(buffer, msg)
	h.messageBuffers[analysisID] = buffer
}

// bufferRoomMessage buffers a message for a room with no connected clients
func (h *Hub) bufferRoomMessage(roomID string, msg Message) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Initialize buffer if needed
	if _, ok := h.roomBuffers[roomID]; !ok {
		h.roomBuffers[roomID] = make([]Message, 0, h.bufferSize)
	}

	// Add message to buffer, respecting size limit
	buffer := h.roomBuffers[roomID]
	if len(buffer) >= h.bufferSize {
		// Remove oldest message
		buffer = buffer[1:]
	}
	buffer = append(buffer, msg)
	h.roomBuffers[roomID] = buffer
}

// sendPingToAllClients sends a ping message to all clients
func (h *Hub) sendPingToAllClients() {
	pingMsg := Message{
		Type:       TypePing,
		Timestamp:  time.Now(),
		SequenceID: h.nextSequence(),
	}

	msgJSON, _ := json.Marshal(pingMsg)

	h.mu.RLock()
	defer h.mu.RUnlock()

	for client := range h.allClients {
		// Check if client is inactive for too long
		if time.Since(client.lastActivity) > 2*h.pingInterval {
			// Client is inactive, unregister
			go func(c *Client) {
				h.unregister <- c
			}(client)
			continue
		}

		// Send ping
		select {
		case client.send <- msgJSON:
			// Ping sent successfully
		default:
			// Channel is full or closed, unregister the client
			go func(c *Client) {
				h.unregister <- c
			}(client)
		}
	}
}

// nextSequence generates the next sequence number
func (h *Hub) nextSequence() int64 {
	return atomic.AddInt64(&h.sequence, 1)
}

// BroadcastToAnalysis sends a message to all clients subscribed to an analysis
func (h *Hub) BroadcastToAnalysis(analysisID uuid.UUID, message Message) {
	h.broadcastToAnalysis <- &BroadcastMessage{
		TargetID: analysisID,
		Message:  message,
	}
}

// BroadcastToRoom sends a message to all clients in a room
func (h *Hub) BroadcastToRoom(roomID string, message Message) {
	h.broadcastToRoom <- &BroadcastMessage{
		TargetID: roomID,
		Message:  message,
	}
}

// BroadcastToAll sends a message to all connected clients
func (h *Hub) BroadcastToAll(message Message) {
	h.broadcastToAll <- &BroadcastMessage{
		Message: message,
	}
}

// HandleConnection manages a WebSocket connection
func (h *Hub) HandleConnection(conn *websocket.Conn, analysisID uuid.UUID) {
	client := &Client{
		conn:         conn,
		analysisID:   analysisID,
		rooms:        make(map[string]bool),
		send:         make(chan []byte, 256),
		lastActivity: time.Now(),
		pendingAcks:  make(map[string]Message),
		receivedAcks: make(map[string]bool),
		ackTimeout:   30 * time.Second,
		closeSignal:  make(chan struct{}),
		hub:          h,
	}

	// Register the client
	h.register <- client

	// Start the client's read and write pumps
	go client.writePump()
	go client.readPump()
}

func (c *Client) GetPermissions() map[string]bool {
	return c.permissions
}

func (c *Client) GetUsername() string {
	return c.username
}

func (c *Client) GetUserID() uuid.UUID {
	return c.userID
}

// writePump pumps messages from the hub to the WebSocket connection
func (c *Client) writePump() {
	ticker := time.NewTicker(54 * time.Second) // WebSocket ping period
	defer func() {
		ticker.Stop()
		c.conn.Close()
		close(c.closeSignal)
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.writeMutex.Lock()
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				// The hub closed the channel
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				c.writeMutex.Unlock()
				return
			}

			// Write the message
			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				c.writeMutex.Unlock()
				return
			}
			c.writeMutex.Unlock()

		case <-ticker.C:
			c.writeMutex.Lock()
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				c.writeMutex.Unlock()
				return
			}
			c.writeMutex.Unlock()
		}
	}
}

// readPump pumps messages from the WebSocket connection to the hub
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(4096) // Maximum message size
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		c.lastActivity = time.Now()
		return nil
	})

	for {
		_, msgBytes, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err,
				websocket.CloseGoingAway,
				websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket read error: %v", err)
			}
			break
		}

		// Update last activity
		c.lastActivity = time.Now()

		// Process the message
		var message Message
		if err := json.Unmarshal(msgBytes, &message); err != nil {
			log.Printf("Error unmarshalling message: %v", err)
			continue
		}

		// Handle different message types
		switch message.Type {
		case TypePong:
			// Client responded to ping
			continue

		case TypeAcknowledgment:
			// Handle acknowledgment
			if message.AckID != "" {
				delete(c.pendingAcks, message.AckID)
				c.receivedAcks[message.AckID] = true
			}

		case TypeControlPause, TypeControlResume, TypeControlCancel, TypeControlUpdateParams:
			// Handle control requests
			c.hub.control <- &ControlRequest{
				Client:     c,
				AnalysisID: c.analysisID,
				Action:     message.Type,
				Params:     message.Meta,
			}

		case TypeRoomMessage:
			// Handle room message
			if roomID, ok := message.Data.(string); ok && c.rooms[roomID] {
				// Forward the message to the room
				message.Timestamp = time.Now()
				message.SequenceID = c.hub.nextSequence()

				// Add sender info
				if message.Meta == nil {
					message.Meta = make(map[string]interface{})
				}
				message.Meta["sender_id"] = c.userID.String()
				message.Meta["sender_username"] = c.username

				c.hub.broadcastToRoom <- &BroadcastMessage{
					TargetID: roomID,
					Message:  message,
				}
			}
		}
	}
}

func (c *Client) StartWritePump() {
	go c.writePump()
}

func (c *Client) StartReadPump() {
	go c.readPump()
}

// JoinRoom requests to join a room
func (c *Client) JoinRoom(roomID string) {
	c.hub.joinRoom <- &RoomRequest{
		Client: c,
		RoomID: roomID,
	}
}

// LeaveRoom requests to leave a room
func (c *Client) LeaveRoom(roomID string) {
	c.hub.leaveRoom <- &RoomRequest{
		Client: c,
		RoomID: roomID,
	}
}

// SendControlRequest sends a control request
func (c *Client) SendControlRequest(action MessageType, params map[string]interface{}) {
	c.hub.control <- &ControlRequest{
		Client:     c,
		AnalysisID: c.analysisID,
		Action:     action,
		Params:     params,
	}
}

func NewClient(conn *websocket.Conn, analysisID uuid.UUID, userID uuid.UUID, username string, permissions map[string]bool, hub *Hub) *Client {
	return &Client{
		conn:         conn,
		analysisID:   analysisID,
		rooms:        make(map[string]bool),
		send:         make(chan []byte, 256),
		lastActivity: time.Now(),
		pendingAcks:  make(map[string]Message),
		receivedAcks: make(map[string]bool),
		ackTimeout:   30 * time.Second,
		userID:       userID,
		username:     username,
		permissions:  permissions,
		closeSignal:  make(chan struct{}),
		hub:          hub,
	}
}

func (h *Hub) RegisterClient(client *Client) {
	h.register <- client
}
