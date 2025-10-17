package price

import (
	"context"

	"github.com/flexprice/flexprice/internal/types"
)

// Repository defines the interface for price persistence operations
type Repository interface {
	// Core operations
	Create(ctx context.Context, price *Price) error
	Get(ctx context.Context, id string) (*Price, error)
	GetByPlanID(ctx context.Context, planID string) ([]*Price, error)
	List(ctx context.Context, filter *types.PriceFilter) ([]*Price, error)
	Count(ctx context.Context, filter *types.PriceFilter) (int, error)
	ListAll(ctx context.Context, filter *types.PriceFilter) ([]*Price, error)
	Update(ctx context.Context, price *Price) error
	Delete(ctx context.Context, id string) error

	// Bulk operations
	CreateBulk(ctx context.Context, prices []*Price) error
	DeleteBulk(ctx context.Context, ids []string) error

	// Group-related operations (minimal set)
	CountByIDs(ctx context.Context, ids []string) (int, error)
	GetByGroupIDs(ctx context.Context, groupIDs []string) ([]*Price, error)
	CountNotInGroup(ctx context.Context, ids []string, excludeGroupID string) (int, error)
	UpdateGroupIDBulk(ctx context.Context, ids []string, groupID *string) error
	ClearGroupIDBulk(ctx context.Context, ids []string) error
}
