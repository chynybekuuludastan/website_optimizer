package handlers

import (
	"encoding/json"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"

	ws "github.com/chynybekuuludastan/website_optimizer/internal/api/websocket"
	"github.com/chynybekuuludastan/website_optimizer/internal/models"
	"github.com/chynybekuuludastan/website_optimizer/internal/repository"
)

// WebSocketHandler handles WebSocket connections
type WebSocketHandler struct {
	Hub          *ws.Hub
	AnalysisRepo repository.AnalysisRepository
	UserRepo     repository.UserRepository
}

// NewWebSocketHandler creates a new WebSocket handler
func NewWebSocketHandler(hub *ws.Hub, analysisRepo repository.AnalysisRepository, userRepo repository.UserRepository) *WebSocketHandler {
	return &WebSocketHandler{
		Hub:          hub,
		AnalysisRepo: analysisRepo,
		UserRepo:     userRepo,
	}
}

// HandleAnalysisWebSocket handles WebSocket connections for analysis updates
func (h *WebSocketHandler) HandleAnalysisWebSocket(c *websocket.Conn) {
	// Get analysis ID from URL
	id := c.Params("id")
	analysisID, err := uuid.Parse(id)
	if err != nil {
		errMsg := ws.Message{
			Type:      ws.TypeAnalysisError,
			Status:    "error",
			Timestamp: time.Now(),
			Data: map[string]interface{}{
				"message": "Invalid analysis ID format",
				"error":   err.Error(),
			},
		}
		msgJSON, _ := json.Marshal(errMsg)
		c.WriteMessage(websocket.TextMessage, msgJSON)
		c.Close()
		return
	}

	// Verify that analysis exists
	var analysis models.Analysis
	err = h.AnalysisRepo.FindByID(analysisID, &analysis)
	if err != nil {
		errMsg := ws.Message{
			Type:      ws.TypeAnalysisError,
			Status:    "error",
			Timestamp: time.Now(),
			Data: map[string]interface{}{
				"message": "Analysis not found",
				"error":   err.Error(),
			},
		}
		msgJSON, _ := json.Marshal(errMsg)
		c.WriteMessage(websocket.TextMessage, msgJSON)
		c.Close()
		return
	}

	// Get user ID from context if available
	var userID uuid.UUID
	if userIDRaw := c.Locals("userID"); userIDRaw != nil {
		if id, ok := userIDRaw.(uuid.UUID); ok {
			userID = id
		}
	}

	// Get username if available
	if userID != uuid.Nil {
		var user models.User
		h.UserRepo.FindByID(userID, &user)
	}

	// Set up permissions based on user role
	permissions := make(map[string]bool)
	if userID != uuid.Nil {
		// Default permissions
		permissions["view"] = true

		// Check if user is analysis owner or admin
		if analysis.UserID == userID {
			permissions["control"] = true
			permissions["delete"] = true
		}

		// If we have user data, check their role for admin permissions
		if userIDRaw := c.Locals("userID"); userIDRaw != nil {
			if role := c.Locals("role"); role != nil {
				if roleStr, ok := role.(string); ok && roleStr == "admin" {
					permissions["control"] = true
					permissions["delete"] = true
					permissions["admin"] = true
				}
			}
		}
	}

	// Register control handlers for this analysis
	h.registerControlHandlers(analysisID)

	// Handle the WebSocket connection with our improved hub
	h.Hub.HandleConnection(c, analysisID)
}

// registerControlHandlers registers handlers for control actions
func (h *WebSocketHandler) registerControlHandlers(analysisID uuid.UUID) {
	// Handler for pause action
	h.Hub.RegisterControlHandler(ws.TypeControlPause, func(req *ws.ControlRequest) interface{} {
		// Check permissions
		if !req.Client.GetPermissions()["control"] {
			return map[string]interface{}{
				"success": false,
				"message": "Permission denied",
			}
		}

		// Update analysis status to paused
		err := h.AnalysisRepo.Transaction(func(tx *gorm.DB) error {
			// Create a temporary repository with the transaction
			repo := repository.NewAnalysisRepository(tx)
			return repo.UpdateStatus(analysisID, "paused")
		})

		if err != nil {
			return map[string]interface{}{
				"success": false,
				"message": "Failed to pause analysis",
				"error":   err.Error(),
			}
		}

		// Broadcast pause message to all clients
		h.Hub.BroadcastToAnalysis(analysisID, ws.Message{
			Type:     ws.TypeControlResponse,
			Status:   "paused",
			Category: ws.CategoryAll,
			Data: map[string]interface{}{
				"analysis_id": analysisID.String(),
				"status":      "paused",
				"action":      "pause",
				"user_id":     req.Client.GetUserID().String(),
				"username":    req.Client.GetUsername(),
				"timestamp":   time.Now(),
			},
		})

		return map[string]interface{}{
			"success": true,
			"message": "Analysis paused successfully",
		}
	})

	// Handler for resume action
	h.Hub.RegisterControlHandler(ws.TypeControlResume, func(req *ws.ControlRequest) interface{} {
		// Check permissions
		if !req.Client.GetPermissions()["control"] {
			return map[string]interface{}{
				"success": false,
				"message": "Permission denied",
			}
		}

		// Update analysis status to running
		err := h.AnalysisRepo.Transaction(func(tx *gorm.DB) error {
			// Create a temporary repository with the transaction
			repo := repository.NewAnalysisRepository(tx)
			return repo.UpdateStatus(analysisID, "running")
		})

		if err != nil {
			return map[string]interface{}{
				"success": false,
				"message": "Failed to resume analysis",
				"error":   err.Error(),
			}
		}

		// Broadcast resume message to all clients
		h.Hub.BroadcastToAnalysis(analysisID, ws.Message{
			Type:     ws.TypeControlResponse,
			Status:   "running",
			Category: ws.CategoryAll,
			Data: map[string]interface{}{
				"analysis_id": analysisID.String(),
				"status":      "running",
				"action":      "resume",
				"user_id":     req.Client.GetUserID().String(),
				"username":    req.Client.GetUsername(),
				"timestamp":   time.Now(),
			},
		})

		return map[string]interface{}{
			"success": true,
			"message": "Analysis resumed successfully",
		}
	})

	// Handler for cancel action
	h.Hub.RegisterControlHandler(ws.TypeControlCancel, func(req *ws.ControlRequest) interface{} {
		// Check permissions
		if !req.Client.GetPermissions()["control"] {
			return map[string]interface{}{
				"success": false,
				"message": "Permission denied",
			}
		}

		// Update analysis status to cancelled
		err := h.AnalysisRepo.Transaction(func(tx *gorm.DB) error {
			// Create a temporary repository with the transaction
			repo := repository.NewAnalysisRepository(tx)
			return repo.UpdateStatus(analysisID, "cancelled")
		})

		if err != nil {
			return map[string]interface{}{
				"success": false,
				"message": "Failed to cancel analysis",
				"error":   err.Error(),
			}
		}

		// Broadcast cancel message to all clients
		h.Hub.BroadcastToAnalysis(analysisID, ws.Message{
			Type:     ws.TypeControlResponse,
			Status:   "cancelled",
			Category: ws.CategoryAll,
			Data: map[string]interface{}{
				"analysis_id": analysisID.String(),
				"status":      "cancelled",
				"action":      "cancel",
				"user_id":     req.Client.GetUserID().String(),
				"username":    req.Client.GetUsername(),
				"timestamp":   time.Now(),
			},
		})

		return map[string]interface{}{
			"success": true,
			"message": "Analysis cancelled successfully",
		}
	})

	// Handler for updating parameters
	h.Hub.RegisterControlHandler(ws.TypeControlUpdateParams, func(req *ws.ControlRequest) interface{} {
		// Check permissions
		if !req.Client.GetPermissions()["control"] {
			return map[string]interface{}{
				"success": false,
				"message": "Permission denied",
			}
		}

		// Get parameters from request
		params := req.Params
		if params == nil {
			return map[string]interface{}{
				"success": false,
				"message": "No parameters provided",
			}
		}

		// Update analysis metadata
		err := h.AnalysisRepo.Transaction(func(tx *gorm.DB) error {
			// Get current analysis
			var analysis models.Analysis
			if err := tx.First(&analysis, analysisID).Error; err != nil {
				return err
			}

			// Update metadata with new parameters
			var metadata map[string]interface{}
			if err := json.Unmarshal(analysis.Metadata, &metadata); err != nil {
				metadata = make(map[string]interface{})
			}

			// Add parameters to metadata
			for k, v := range params {
				metadata[k] = v
			}

			// Convert back to JSON
			metadataJSON, err := json.Marshal(metadata)
			if err != nil {
				return err
			}

			// Update analysis
			return tx.Model(&models.Analysis{}).
				Where("id = ?", analysisID).
				Update("metadata", metadataJSON).Error
		})

		if err != nil {
			return map[string]interface{}{
				"success": false,
				"message": "Failed to update parameters",
				"error":   err.Error(),
			}
		}

		// Broadcast parameter update to all clients
		h.Hub.BroadcastToAnalysis(analysisID, ws.Message{
			Type:     ws.TypeControlResponse,
			Status:   "parameters_updated",
			Category: ws.CategoryAll,
			Data: map[string]interface{}{
				"analysis_id": analysisID.String(),
				"action":      "update_params",
				"params":      params,
				"user_id":     req.Client.GetUserID().String(),
				"username":    req.Client.GetUsername(),
				"timestamp":   time.Now(),
			},
		})

		return map[string]interface{}{
			"success": true,
			"message": "Analysis parameters updated successfully",
			"params":  params,
		}
	})
}

// HandleGroupRoom handles WebSocket connections for team collaboration
func (h *WebSocketHandler) HandleGroupRoom(c *websocket.Conn) {
	// Get room ID from URL
	roomID := c.Params("room_id")
	if roomID == "" {
		errMsg := ws.Message{
			Type:      ws.TypeAnalysisError,
			Status:    "error",
			Timestamp: time.Now(),
			Data: map[string]interface{}{
				"message": "Room ID is required",
			},
		}
		msgJSON, _ := json.Marshal(errMsg)
		c.WriteMessage(websocket.TextMessage, msgJSON)
		c.Close()
		return
	}

	// Get user ID from context if available
	var userID uuid.UUID
	if userIDRaw := c.Locals("userID"); userIDRaw != nil {
		if id, ok := userIDRaw.(uuid.UUID); ok {
			userID = id
		}
	}

	// Get username if available
	username := "anonymous"
	if userID != uuid.Nil {
		var user models.User
		if err := h.UserRepo.FindByID(userID, &user); err == nil {
			username = user.Username
		}
	}

	// Create a fake analysis ID for the room - we'll use a UUID derived from the room name
	roomUUID := uuid.NewSHA1(uuid.NameSpaceOID, []byte("room:"+roomID))

	// Handle the connection
	conn := c
	client := ws.NewClient(conn, roomUUID, userID, username, map[string]bool{"view": true}, h.Hub)

	// Register the client
	h.Hub.RegisterClient(client)

	// Join the room
	client.JoinRoom(roomID)

	// Start the client's read and write pumps
	go client.StartWritePump()
	go client.StartReadPump()
}

// WebSocketTestHandler handles requests for the WebSocket test page
type WebSocketTestHandler struct{}

// NewWebSocketTestHandler creates a new WebSocket test handler
func NewWebSocketTestHandler() *WebSocketTestHandler {
	return &WebSocketTestHandler{}
}

// ServePage serves the WebSocket testing interface
func (h *WebSocketTestHandler) ServePage(c *fiber.Ctx) error {
	// Serve the WebSocket test HTML page
	return c.SendFile("./static/websocket-test.html")
}
