package ent

import (
	"context"

	"github.com/flexprice/flexprice/ent"
	"github.com/flexprice/flexprice/ent/connection"
	"github.com/flexprice/flexprice/ent/predicate"
	"github.com/flexprice/flexprice/internal/cache"
	domainConnection "github.com/flexprice/flexprice/internal/domain/connection"
	"github.com/flexprice/flexprice/internal/dsl"
	ierr "github.com/flexprice/flexprice/internal/errors"
	"github.com/flexprice/flexprice/internal/logger"
	"github.com/flexprice/flexprice/internal/postgres"
	"github.com/flexprice/flexprice/internal/types"
)

type connectionRepository struct {
	client    postgres.IClient
	log       *logger.Logger
	queryOpts ConnectionQueryOptions
	cache     cache.Cache
}

func NewConnectionRepository(client postgres.IClient, log *logger.Logger, cache cache.Cache) domainConnection.Repository {
	return &connectionRepository{
		client:    client,
		log:       log,
		queryOpts: ConnectionQueryOptions{},
		cache:     cache,
	}
}

func (r *connectionRepository) Create(ctx context.Context, c *domainConnection.Connection) error {
	client := r.client.Querier(ctx)

	r.log.Debugw("creating connection",
		"connection_id", c.ID,
		"tenant_id", c.TenantID,
		"name", c.Name,
	)

	// Start a span for this repository operation
	span := StartRepositorySpan(ctx, "connection", "create", map[string]interface{}{
		"connection_id": c.ID,
		"name":          c.Name,
	})
	defer FinishSpan(span)

	// Set environment ID from context if not already set
	if c.EnvironmentID == "" {
		c.EnvironmentID = types.GetEnvironmentID(ctx)
	}

	// Convert structured metadata to map format for database storage
	metadataMap := convertConnectionMetadataToMap(c.Metadata)

	connection, err := client.Connection.Create().
		SetID(c.ID).
		SetTenantID(c.TenantID).
		SetName(c.Name).
		SetProviderType(connection.ProviderType(c.ProviderType)).
		SetMetadata(metadataMap).
		SetStatus(string(c.Status)).
		SetCreatedAt(c.CreatedAt).
		SetUpdatedAt(c.UpdatedAt).
		SetCreatedBy(c.CreatedBy).
		SetUpdatedBy(c.UpdatedBy).
		SetEnvironmentID(c.EnvironmentID).
		Save(ctx)

	if err != nil {
		SetSpanError(span, err)

		if ent.IsConstraintError(err) {
			return ierr.WithError(err).
				WithHint("Failed to create connection").
				WithReportableDetails(map[string]any{
					"name":          c.Name,
					"provider_type": c.ProviderType,
				}).
				Mark(ierr.ErrAlreadyExists)
		}
		return ierr.WithError(err).
			WithHint("Failed to create connection").
			Mark(ierr.ErrDatabase)
	}

	SetSpanSuccess(span)
	*c = *domainConnection.FromEnt(connection)
	return nil
}

func (r *connectionRepository) Get(ctx context.Context, id string) (*domainConnection.Connection, error) {
	// Start a span for this repository operation
	span := StartRepositorySpan(ctx, "connection", "get", map[string]interface{}{
		"connection_id": id,
	})
	defer FinishSpan(span)

	// Try to get from cache first
	if cachedConnection := r.GetCache(ctx, id); cachedConnection != nil {
		return cachedConnection, nil
	}

	client := r.client.Querier(ctx)
	r.log.Debugw("getting connection", "connection_id", id)

	c, err := client.Connection.Query().
		Where(
			connection.ID(id),
			connection.TenantID(types.GetTenantID(ctx)),
			connection.EnvironmentID(types.GetEnvironmentID(ctx)),
		).
		Only(ctx)

	if err != nil {
		SetSpanError(span, err)

		if ent.IsNotFound(err) {
			return nil, ierr.WithError(err).
				WithHintf("Connection with ID %s was not found", id).
				WithReportableDetails(map[string]any{
					"connection_id": id,
				}).
				Mark(ierr.ErrNotFound)
		}
		return nil, ierr.WithError(err).
			WithHint("Failed to get connection").
			Mark(ierr.ErrDatabase)
	}

	SetSpanSuccess(span)
	domainConn := domainConnection.FromEnt(c)

	// Cache the result
	r.SetCache(ctx, domainConn)

	return domainConn, nil
}

func (r *connectionRepository) GetByEnvironmentAndProvider(ctx context.Context, environmentID string, provider types.SecretProvider) (*domainConnection.Connection, error) {
	// Start a span for this repository operation
	span := StartRepositorySpan(ctx, "connection", "get_by_env_provider", map[string]interface{}{
		"environment_id": environmentID,
		"provider_type":  provider,
	})
	defer FinishSpan(span)

	r.log.Debugw("getting connection by environment and provider",
		"environment_id", environmentID,
		"provider_type", provider)

	// Create a filter to get connections by environment and provider
	filter := &types.ConnectionFilter{
		ProviderType: provider,
	}

	// Use the List function internally
	connections, err := r.List(ctx, filter)
	if err != nil {
		SetSpanError(span, err)
		return nil, err
	}

	// Filter by environment ID since the List function applies environment filter automatically
	// but we need to match the specific environment ID
	var matchingConnection *domainConnection.Connection
	for _, conn := range connections {
		if conn.EnvironmentID == environmentID {
			matchingConnection = conn
			break
		}
	}

	if matchingConnection == nil {
		SetSpanError(span, ierr.ErrNotFound)
		return nil, ierr.NewError("connection not found").
			WithHintf("Connection with environment ID %s and provider %s was not found", environmentID, provider).
			WithReportableDetails(map[string]any{
				"environment_id": environmentID,
				"provider_type":  provider,
			}).
			Mark(ierr.ErrNotFound)
	}

	SetSpanSuccess(span)

	// Cache the result
	r.SetCache(ctx, matchingConnection)

	return matchingConnection, nil
}

func (r *connectionRepository) List(ctx context.Context, filter *types.ConnectionFilter) ([]*domainConnection.Connection, error) {
	client := r.client.Querier(ctx)

	query := client.Connection.Query()
	query = r.queryOpts.ApplyTenantFilter(ctx, query)
	query = r.queryOpts.ApplyEnvironmentFilter(ctx, query)
	query = r.queryOpts.ApplyStatusFilter(query, string(types.StatusPublished))

	query, err := r.queryOpts.applyEntityQueryOptions(ctx, filter, query)
	if err != nil {
		return nil, err
	}

	connections, err := query.All(ctx)
	if err != nil {
		return nil, ierr.WithError(err).
			WithHint("Failed to list connections").
			Mark(ierr.ErrDatabase)
	}

	var result []*domainConnection.Connection
	for _, c := range connections {
		result = append(result, domainConnection.FromEnt(c))
	}

	return result, nil
}

// convertConnectionMetadataToMap converts structured metadata to map format for database storage
func convertConnectionMetadataToMap(metadata types.ConnectionMetadata) map[string]interface{} {
	switch metadata.Type {
	case types.ConnectionMetadataTypeStripe:
		if metadata.Stripe != nil {
			return map[string]interface{}{
				"publishable_key": metadata.Stripe.PublishableKey,
				"secret_key":      metadata.Stripe.SecretKey,
				"webhook_secret":  metadata.Stripe.WebhookSecret,
				"account_id":      metadata.Stripe.AccountID,
			}
		}
	case types.ConnectionMetadataTypeRazorpay:
		if metadata.Razorpay != nil {
			return map[string]interface{}{
				"key_id":         metadata.Razorpay.KeyID,
				"key_secret":     metadata.Razorpay.KeySecret,
				"webhook_secret": metadata.Razorpay.WebhookSecret,
				"account_id":     metadata.Razorpay.AccountID,
			}
		}
	case types.ConnectionMetadataTypePayPal:
		if metadata.PayPal != nil {
			return map[string]interface{}{
				"client_id":     metadata.PayPal.ClientID,
				"client_secret": metadata.PayPal.ClientSecret,
				"webhook_id":    metadata.PayPal.WebhookID,
				"account_id":    metadata.PayPal.AccountID,
			}
		}
	case types.ConnectionMetadataTypeGeneric:
		if metadata.Generic != nil {
			return metadata.Generic.Data
		}
	}
	return make(map[string]interface{})
}

func (r *connectionRepository) Count(ctx context.Context, filter *types.ConnectionFilter) (int, error) {
	client := r.client.Querier(ctx)

	query := client.Connection.Query()
	query = r.queryOpts.ApplyTenantFilter(ctx, query)
	query = r.queryOpts.ApplyEnvironmentFilter(ctx, query)
	query = r.queryOpts.ApplyStatusFilter(query, string(types.StatusPublished))

	query, err := r.queryOpts.applyEntityQueryOptions(ctx, filter, query)
	if err != nil {
		return 0, err
	}

	count, err := query.Count(ctx)
	if err != nil {
		return 0, ierr.WithError(err).
			WithHint("Failed to count connections").
			Mark(ierr.ErrDatabase)
	}

	return count, nil
}

func (r *connectionRepository) Update(ctx context.Context, c *domainConnection.Connection) error {
	client := r.client.Querier(ctx)

	r.log.Debugw("updating connection",
		"connection_id", c.ID,
		"tenant_id", c.TenantID,
	)

	// Start a span for this repository operation
	span := StartRepositorySpan(ctx, "connection", "update", map[string]interface{}{
		"connection_id": c.ID,
	})
	defer FinishSpan(span)

	// Convert structured metadata to map format for database storage
	metadataMap := convertConnectionMetadataToMap(c.Metadata)

	connection, err := client.Connection.UpdateOneID(c.ID).
		Where(
			connection.TenantID(c.TenantID),
			connection.EnvironmentID(c.EnvironmentID),
		).
		SetName(c.Name).
		SetProviderType(connection.ProviderType(c.ProviderType)).
		SetMetadata(metadataMap).
		SetStatus(string(c.Status)).
		SetUpdatedAt(c.UpdatedAt).
		SetUpdatedBy(c.UpdatedBy).
		Save(ctx)

	if err != nil {
		SetSpanError(span, err)

		if ent.IsNotFound(err) {
			return ierr.WithError(err).
				WithHintf("Connection with ID %s was not found", c.ID).
				WithReportableDetails(map[string]any{
					"connection_id": c.ID,
				}).
				Mark(ierr.ErrNotFound)
		}

		return ierr.WithError(err).
			WithHint("Failed to update connection").
			Mark(ierr.ErrDatabase)
	}

	SetSpanSuccess(span)
	*c = *domainConnection.FromEnt(connection)

	// Update cache
	r.SetCache(ctx, c)

	return nil
}

func (r *connectionRepository) Delete(ctx context.Context, c *domainConnection.Connection) error {
	client := r.client.Querier(ctx)

	r.log.Debugw("deleting connection",
		"connection_id", c.ID,
		"tenant_id", c.TenantID,
	)

	// Start a span for this repository operation
	span := StartRepositorySpan(ctx, "connection", "delete", map[string]interface{}{
		"connection_id": c.ID,
	})
	defer FinishSpan(span)

	_, err := client.Connection.Update().
		Where(
			connection.ID(c.ID),
			connection.TenantID(c.TenantID),
			connection.EnvironmentID(c.EnvironmentID),
		).
		SetStatus(string(types.StatusArchived)).
		SetUpdatedAt(c.UpdatedAt).
		SetUpdatedBy(c.UpdatedBy).
		Save(ctx)

	if err != nil {
		SetSpanError(span, err)

		if ent.IsNotFound(err) {
			return ierr.WithError(err).
				WithHintf("Connection with ID %s was not found", c.ID).
				WithReportableDetails(map[string]any{
					"connection_id": c.ID,
				}).
				Mark(ierr.ErrNotFound)
		}
		return ierr.WithError(err).
			WithHint("Failed to delete connection").
			Mark(ierr.ErrDatabase)
	}

	SetSpanSuccess(span)
	r.DeleteCache(ctx, c)
	return nil
}

// ConnectionQuery type alias for better readability
type ConnectionQuery = *ent.ConnectionQuery

// ConnectionQueryOptions implements BaseQueryOptions for connection queries
type ConnectionQueryOptions struct{}

func (o ConnectionQueryOptions) ApplyTenantFilter(ctx context.Context, query ConnectionQuery) ConnectionQuery {
	return query.Where(connection.TenantIDEQ(types.GetTenantID(ctx)))
}

func (o ConnectionQueryOptions) ApplyEnvironmentFilter(ctx context.Context, query ConnectionQuery) ConnectionQuery {
	environmentID := types.GetEnvironmentID(ctx)
	if environmentID != "" {
		return query.Where(connection.EnvironmentIDEQ(environmentID))
	}
	return query
}

func (o ConnectionQueryOptions) ApplyStatusFilter(query ConnectionQuery, status string) ConnectionQuery {
	if status == "" {
		return query.Where(connection.StatusNotIn(string(types.StatusDeleted)))
	}
	return query.Where(connection.Status(status))
}

func (o ConnectionQueryOptions) ApplySortFilter(query ConnectionQuery, field string, order string) ConnectionQuery {
	if field != "" {
		if order == types.OrderDesc {
			query = query.Order(ent.Desc(o.GetFieldName(field)))
		} else {
			query = query.Order(ent.Asc(o.GetFieldName(field)))
		}
	}
	return query
}

func (o ConnectionQueryOptions) ApplyPaginationFilter(query ConnectionQuery, limit int, offset int) ConnectionQuery {
	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}
	return query
}

func (o ConnectionQueryOptions) GetFieldName(field string) string {
	switch field {
	case "created_at":
		return connection.FieldCreatedAt
	case "updated_at":
		return connection.FieldUpdatedAt
	case "name":
		return connection.FieldName
	case "provider_type":
		return connection.FieldProviderType
	case "status":
		return connection.FieldStatus
	default:
		//unknown field
		return ""
	}
}

func (o ConnectionQueryOptions) GetFieldResolver(field string) (string, error) {
	fieldName := o.GetFieldName(field)
	if fieldName == "" {
		return "", ierr.NewErrorf("unknown field name '%s' in connection query", field).
			Mark(ierr.ErrValidation)
	}
	return fieldName, nil
}

func (o ConnectionQueryOptions) applyEntityQueryOptions(_ context.Context, f *types.ConnectionFilter, query ConnectionQuery) (ConnectionQuery, error) {
	var err error
	if f == nil {
		return query, nil
	}

	if f.ProviderType != "" {
		query = query.Where(connection.ProviderTypeEQ(connection.ProviderType(f.ProviderType)))
	}

	if len(f.ConnectionIDs) > 0 {
		query = query.Where(connection.IDIn(f.ConnectionIDs...))
	}

	if f.Filters != nil {
		query, err = dsl.ApplyFilters[ConnectionQuery, predicate.Connection](
			query,
			f.Filters,
			o.GetFieldResolver,
			func(p dsl.Predicate) predicate.Connection { return predicate.Connection(p) },
		)
		if err != nil {
			return nil, err
		}
	}

	// Apply sorts using the generic function
	if f.Sort != nil {
		query, err = dsl.ApplySorts[ConnectionQuery, connection.OrderOption](
			query,
			f.Sort,
			o.GetFieldResolver,
			func(o dsl.OrderFunc) connection.OrderOption { return connection.OrderOption(o) },
		)
		if err != nil {
			return nil, err
		}
	}

	return query, nil
}

func (r *connectionRepository) SetCache(ctx context.Context, connection *domainConnection.Connection) {
	span := cache.StartCacheSpan(ctx, "connection", "set", map[string]interface{}{
		"connection_id": connection.ID,
	})
	defer cache.FinishSpan(span)

	tenantID := types.GetTenantID(ctx)
	environmentID := types.GetEnvironmentID(ctx)

	// Set ID based cache entry
	cacheKey := cache.GenerateKey(cache.PrefixConnection, tenantID, environmentID, connection.ID)

	r.cache.Set(ctx, cacheKey, connection, cache.ExpiryDefaultInMemory)

	r.log.Debugw("cache set", "key", cacheKey)
}

func (r *connectionRepository) GetCache(ctx context.Context, key string) *domainConnection.Connection {
	span := cache.StartCacheSpan(ctx, "connection", "get", map[string]interface{}{
		"connection_id": key,
	})
	defer cache.FinishSpan(span)

	cacheKey := cache.GenerateKey(cache.PrefixConnection, types.GetTenantID(ctx), types.GetEnvironmentID(ctx), key)
	if value, found := r.cache.Get(ctx, cacheKey); found {
		if connection, ok := value.(*domainConnection.Connection); ok {
			r.log.Debugw("cache hit", "key", cacheKey)
			return connection
		}
	}
	return nil
}

func (r *connectionRepository) DeleteCache(ctx context.Context, connection *domainConnection.Connection) {
	span := cache.StartCacheSpan(ctx, "connection", "delete", map[string]interface{}{
		"connection_id": connection.ID,
	})
	defer cache.FinishSpan(span)

	tenantID := types.GetTenantID(ctx)
	environmentID := types.GetEnvironmentID(ctx)

	// Delete ID-based cache
	cacheKey := cache.GenerateKey(cache.PrefixConnection, tenantID, environmentID, connection.ID)
	r.cache.Delete(ctx, cacheKey)
	r.log.Debugw("cache deleted", "key", cacheKey)
}
