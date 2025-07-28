package dto

import (
	"github.com/flexprice/flexprice/internal/domain/connection"
	"github.com/flexprice/flexprice/internal/types"
)

// CreateConnectionRequest represents the request to create a connection
type CreateConnectionRequest struct {
	Name           string                 `json:"name" validate:"required,max=255"`
	Description    string                 `json:"description,omitempty" validate:"max=1000"`
	ConnectionCode string                 `json:"connection_code" validate:"required,max=100"`
	ProviderType   types.SecretProvider   `json:"provider_type" validate:"required"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
	SecretID       string                 `json:"secret_id,omitempty" validate:"max=50"`
}

// UpdateConnectionRequest represents the request to update a connection
type UpdateConnectionRequest struct {
	Name           string                 `json:"name,omitempty" validate:"omitempty,max=255"`
	Description    string                 `json:"description,omitempty" validate:"max=1000"`
	ConnectionCode string                 `json:"connection_code,omitempty" validate:"omitempty,max=100"`
	ProviderType   types.SecretProvider   `json:"provider_type,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
	SecretID       string                 `json:"secret_id,omitempty" validate:"max=50"`
}

// ConnectionResponse represents the response for connection operations
type ConnectionResponse struct {
	ID             string                 `json:"id"`
	Name           string                 `json:"name"`
	Description    string                 `json:"description"`
	ConnectionCode string                 `json:"connection_code"`
	ProviderType   types.SecretProvider   `json:"provider_type"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
	SecretID       string                 `json:"secret_id,omitempty"`
	EnvironmentID  string                 `json:"environment_id"`
	TenantID       string                 `json:"tenant_id"`
	Status         types.Status           `json:"status"`
	CreatedAt      string                 `json:"created_at"`
	UpdatedAt      string                 `json:"updated_at"`
	CreatedBy      string                 `json:"created_by"`
	UpdatedBy      string                 `json:"updated_by"`
}

// ListConnectionsResponse represents the response for listing connections
type ListConnectionsResponse struct {
	Connections []ConnectionResponse `json:"connections"`
	Total       int                  `json:"total"`
	Limit       int                  `json:"limit"`
	Offset      int                  `json:"offset"`
}

// ToConnection converts CreateConnectionRequest to domain Connection
func (req *CreateConnectionRequest) ToConnection() *connection.Connection {
	return &connection.Connection{
		Name:           req.Name,
		Description:    req.Description,
		ConnectionCode: req.ConnectionCode,
		ProviderType:   req.ProviderType,
		Metadata:       req.Metadata,
		SecretID:       req.SecretID,
	}
}

// ToConnectionResponse converts domain Connection to ConnectionResponse
func ToConnectionResponse(conn *connection.Connection) *ConnectionResponse {
	if conn == nil {
		return nil
	}

	return &ConnectionResponse{
		ID:             conn.ID,
		Name:           conn.Name,
		Description:    conn.Description,
		ConnectionCode: conn.ConnectionCode,
		ProviderType:   conn.ProviderType,
		Metadata:       conn.Metadata,
		SecretID:       conn.SecretID,
		EnvironmentID:  conn.EnvironmentID,
		TenantID:       conn.TenantID,
		Status:         conn.Status,
		CreatedAt:      conn.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:      conn.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		CreatedBy:      conn.CreatedBy,
		UpdatedBy:      conn.UpdatedBy,
	}
}

// ToConnectionResponses converts multiple domain Connections to ConnectionResponses
func ToConnectionResponses(connections []*connection.Connection) []ConnectionResponse {
	var responses []ConnectionResponse
	for _, conn := range connections {
		if resp := ToConnectionResponse(conn); resp != nil {
			responses = append(responses, *resp)
		}
	}
	return responses
}
