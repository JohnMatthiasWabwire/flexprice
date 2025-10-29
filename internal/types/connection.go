package types

import (
	ierr "github.com/flexprice/flexprice/internal/errors"
	"github.com/samber/lo"
)

// ConnectionMetadataType represents the type of connection metadata
type ConnectionMetadataType string

const (
	ConnectionMetadataTypeStripe  ConnectionMetadataType = "stripe"
	ConnectionMetadataTypeGeneric ConnectionMetadataType = "generic"
	ConnectionMetadataTypeS3      ConnectionMetadataType = "s3"
	ConnectionMetadataTypeHubSpot ConnectionMetadataType = "hubspot"
)

func (t ConnectionMetadataType) Validate() error {
	allowedTypes := []ConnectionMetadataType{
		ConnectionMetadataTypeStripe,
		ConnectionMetadataTypeGeneric,
		ConnectionMetadataTypeS3,
		ConnectionMetadataTypeHubSpot,
	}
	if !lo.Contains(allowedTypes, t) {
		return ierr.NewError("invalid connection metadata type").
			WithHint("Connection metadata type must be one of: stripe, generic, s3, hubspot").
			Mark(ierr.ErrValidation)
	}
	return nil
}

// StripeConnectionMetadata represents Stripe-specific connection metadata
type StripeConnectionMetadata struct {
	PublishableKey string `json:"publishable_key"`
	SecretKey      string `json:"secret_key"`
	WebhookSecret  string `json:"webhook_secret"`
	AccountID      string `json:"account_id,omitempty"`
}

// S3ConnectionMetadata represents S3-specific connection metadata (encrypted secrets only)
// This goes in the encrypted_secret_data column
type S3ConnectionMetadata struct {
	AWSAccessKeyID     string `json:"aws_access_key_id"`           // AWS access key (encrypted)
	AWSSecretAccessKey string `json:"aws_secret_access_key"`       // AWS secret access key (encrypted)
	AWSSessionToken    string `json:"aws_session_token,omitempty"` // AWS session token for temporary credentials (encrypted)
}

// Validate validates the S3 connection metadata
func (s *S3ConnectionMetadata) Validate() error {
	if s.AWSAccessKeyID == "" {
		return ierr.NewError("aws_access_key_id is required").
			WithHint("AWS access key ID is required").
			Mark(ierr.ErrValidation)
	}
	if s.AWSSecretAccessKey == "" {
		return ierr.NewError("aws_secret_access_key is required").
			WithHint("AWS secret access key is required").
			Mark(ierr.ErrValidation)
	}
	return nil
}

// HubSpotConnectionMetadata represents HubSpot-specific connection metadata
type HubSpotConnectionMetadata struct {
	AccessToken  string `json:"access_token"`     // Private App Access Token (encrypted)
	ClientSecret string `json:"client_secret"`    // Private App Client Secret for webhook verification (encrypted)
	AppID        string `json:"app_id,omitempty"` // HubSpot App ID (optional, not encrypted)
}

// Validate validates the HubSpot connection metadata
func (h *HubSpotConnectionMetadata) Validate() error {
	if h.AccessToken == "" {
		return ierr.NewError("access_token is required").
			WithHint("HubSpot access token is required").
			Mark(ierr.ErrValidation)
	}
	if h.ClientSecret == "" {
		return ierr.NewError("client_secret is required").
			WithHint("HubSpot client secret is required for webhook verification").
			Mark(ierr.ErrValidation)
	}
	return nil
}

// ConnectionSettings represents general connection settings
type ConnectionSettings struct {
	InvoiceSyncEnable *bool `json:"invoice_sync_enable,omitempty"`
}

// Validate validates the Stripe connection metadata
func (s *StripeConnectionMetadata) Validate() error {
	if s.PublishableKey == "" {
		return ierr.NewError("publishable_key is required").
			WithHint("Stripe publishable key is required").
			Mark(ierr.ErrValidation)
	}
	if s.SecretKey == "" {
		return ierr.NewError("secret_key is required").
			WithHint("Stripe secret key is required").
			Mark(ierr.ErrValidation)
	}
	if s.WebhookSecret == "" {
		return ierr.NewError("webhook_secret is required").
			WithHint("Stripe webhook secret is required").
			Mark(ierr.ErrValidation)
	}
	return nil
}

// GenericConnectionMetadata represents generic connection metadata
type GenericConnectionMetadata struct {
	Data map[string]interface{} `json:"data"`
}

// Validate validates the generic connection metadata
func (g *GenericConnectionMetadata) Validate() error {
	if g.Data == nil {
		return ierr.NewError("data is required").
			WithHint("Generic connection metadata data is required").
			Mark(ierr.ErrValidation)
	}
	return nil
}

// ConnectionMetadata represents structured connection metadata
type ConnectionMetadata struct {
	Stripe   *StripeConnectionMetadata  `json:"stripe,omitempty"`
	S3       *S3ConnectionMetadata      `json:"s3,omitempty"`
	HubSpot  *HubSpotConnectionMetadata `json:"hubspot,omitempty"`
	Generic  *GenericConnectionMetadata `json:"generic,omitempty"`
	Settings *ConnectionSettings        `json:"settings,omitempty"`
}

// Validate validates the connection metadata based on provider type
func (c *ConnectionMetadata) Validate(providerType SecretProvider) error {
	switch providerType {
	case SecretProviderStripe:
		if c.Stripe == nil {
			return ierr.NewError("stripe metadata is required").
				WithHint("Stripe metadata is required for stripe provider").
				Mark(ierr.ErrValidation)
		}
		return c.Stripe.Validate()
	case SecretProviderS3:
		if c.S3 == nil {
			return ierr.NewError("s3 metadata is required").
				WithHint("S3 metadata is required for s3 provider").
				Mark(ierr.ErrValidation)
		}
		return c.S3.Validate()
	case SecretProviderHubSpot:
		if c.HubSpot == nil {
			return ierr.NewError("hubspot metadata is required").
				WithHint("HubSpot metadata is required for hubspot provider").
				Mark(ierr.ErrValidation)
		}
		return c.HubSpot.Validate()
	default:
		// For other providers or unknown types, use generic format
		if c.Generic == nil {
			return ierr.NewError("generic metadata is required").
				WithHint("Generic metadata is required for this provider type").
				Mark(ierr.ErrValidation)
		}
		return c.Generic.Validate()
	}
}

// ConnectionFilter represents filters for connection queries
type ConnectionFilter struct {
	*QueryFilter
	*TimeRangeFilter
	// filters allows complex filtering based on multiple fields

	Filters       []*FilterCondition `json:"filters,omitempty" form:"filters" validate:"omitempty"`
	Sort          []*SortCondition   `json:"sort,omitempty" form:"sort" validate:"omitempty"`
	ConnectionIDs []string           `json:"connection_ids,omitempty" form:"connection_ids" validate:"omitempty"`
	ProviderType  SecretProvider     `json:"provider_type,omitempty" form:"provider_type" validate:"omitempty"`
}

// NewConnectionFilter creates a new ConnectionFilter with default values
func NewConnectionFilter() *ConnectionFilter {
	return &ConnectionFilter{
		QueryFilter: NewDefaultQueryFilter(),
	}
}

// NewNoLimitConnectionFilter creates a new ConnectionFilter with no pagination limits
func NewNoLimitConnectionFilter() *ConnectionFilter {
	return &ConnectionFilter{
		QueryFilter: NewNoLimitQueryFilter(),
	}
}

// Validate validates the connection filter
func (f ConnectionFilter) Validate() error {
	if f.QueryFilter != nil {
		if err := f.QueryFilter.Validate(); err != nil {
			return err
		}
	}

	if f.TimeRangeFilter != nil {
		if err := f.TimeRangeFilter.Validate(); err != nil {
			return err
		}
	}

	if f.ProviderType != "" {
		if err := f.ProviderType.Validate(); err != nil {
			return err
		}
	}

	return nil
}

// GetLimit implements BaseFilter interface
func (f *ConnectionFilter) GetLimit() int {
	if f.QueryFilter == nil {
		return NewDefaultQueryFilter().GetLimit()
	}
	return f.QueryFilter.GetLimit()
}

// GetOffset implements BaseFilter interface
func (f *ConnectionFilter) GetOffset() int {
	if f.QueryFilter == nil {
		return NewDefaultQueryFilter().GetOffset()
	}
	return f.QueryFilter.GetOffset()
}

// GetSort implements BaseFilter interface
func (f *ConnectionFilter) GetSort() string {
	if f.QueryFilter == nil {
		return NewDefaultQueryFilter().GetSort()
	}
	return f.QueryFilter.GetSort()
}

// GetOrder implements BaseFilter interface
func (f *ConnectionFilter) GetOrder() string {
	if f.QueryFilter == nil {
		return NewDefaultQueryFilter().GetOrder()
	}
	return f.QueryFilter.GetOrder()
}

// GetStatus implements BaseFilter interface
func (f *ConnectionFilter) GetStatus() string {
	if f.QueryFilter == nil {
		return NewDefaultQueryFilter().GetStatus()
	}
	return f.QueryFilter.GetStatus()
}

// IsUnlimited implements BaseFilter interface
func (f *ConnectionFilter) IsUnlimited() bool {
	if f.QueryFilter == nil {
		return false
	}
	return f.QueryFilter.IsUnlimited()
}
