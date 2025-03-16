package handlers

import (
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
}

// NewWebSocketHandler creates a new WebSocket handler
func NewWebSocketHandler(hub *ws.Hub, analysisRepo repository.AnalysisRepository) *WebSocketHandler {
	return &WebSocketHandler{
		Hub:          hub,
		AnalysisRepo: analysisRepo,
	}
}

// HandleAnalysisWebSocket handles WebSocket connections for analysis updates
func (h *WebSocketHandler) HandleAnalysisWebSocket(c *websocket.Conn) {
	// Get analysis ID from URL
	id := c.Params("id")
	analysisID, err := uuid.Parse(id)
	if err != nil {
		c.WriteJSON(map[string]interface{}{
			"type":  "error",
			"error": "Invalid analysis ID",
		})
		c.Close()
		return
	}

	// Verify that analysis exists
	var exists bool
	err = h.AnalysisRepo.Transaction(func(tx *gorm.DB) error {
		// Check if analysis exists
		var count int64
		if err := tx.Model(&models.Analysis{}).Where("id = ?", analysisID).Count(&count).Error; err != nil {
			return err
		}
		exists = count > 0
		return nil
	})

	if err != nil || !exists {
		c.WriteJSON(map[string]interface{}{
			"type":  "error",
			"error": "Analysis not found",
		})
		c.Close()
		return
	}

	// Handle the WebSocket connection
	h.Hub.HandleConnection(c, analysisID)
}
