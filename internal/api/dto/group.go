package dto

import (
	"context"
	"time"

	"github.com/flexprice/flexprice/internal/domain/group"
	"github.com/flexprice/flexprice/internal/types"
	"github.com/flexprice/flexprice/internal/validator"
)

// CreateGroupRequest represents the request to create a group
type CreateGroupRequest struct {
	Name       string   `json:"name" validate:"required"`
	EntityType string   `json:"entity_type" validate:"required"`
	EntityIDs  []string `json:"entity_ids,omitempty"`
	LookupKey  string   `json:"lookup_key" validate:"required"`
}

func (r *CreateGroupRequest) Validate() error {
	if err := validator.ValidateRequest(r); err != nil {
		return err
	}

	// Validate entity type
	entityType := types.GroupEntityType(r.EntityType)
	if err := entityType.Validate(); err != nil {
		return err
	}

	return nil
}

func (r *CreateGroupRequest) ToGroup(ctx context.Context) (*group.Group, error) {
	entityType := types.GroupEntityType(r.EntityType)
	return &group.Group{
		ID:            types.GenerateUUIDWithPrefix(types.UUID_PREFIX_GROUP),
		Name:          r.Name,
		EntityType:    entityType,
		EnvironmentID: types.GetEnvironmentID(ctx),
		LookupKey:     r.LookupKey,
		BaseModel:     types.GetDefaultBaseModel(ctx),
	}, nil
}

// UpdateGroupRequest represents the request to update a group
type UpdateGroupRequest struct {
	Name      *string  `json:"name,omitempty"`
	EntityIDs []string `json:"entity_ids,omitempty"`
}

func (r *UpdateGroupRequest) Validate() error {
	return validator.ValidateRequest(r)
}

// GroupResponse represents the group response
type GroupResponse struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	EntityType string    `json:"entity_type"`
	EntityIDs  []string  `json:"entity_ids"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// NewGroupResponse creates a new group response
func NewGroupResponse(group *group.Group, entityIDs []string) *GroupResponse {
	return &GroupResponse{
		ID:         group.ID,
		Name:       group.Name,
		EntityType: string(group.EntityType),
		EntityIDs:  entityIDs,
		Status:     string(group.Status),
		CreatedAt:  group.CreatedAt,
		UpdatedAt:  group.UpdatedAt,
	}
}

// ListGroupsResponse represents the response for listing groups
type ListGroupsResponse struct {
	Groups []*GroupResponse `json:"groups"`
	Total  int              `json:"total"`
}
