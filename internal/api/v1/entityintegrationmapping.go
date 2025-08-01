package v1

import (
	"net/http"
	"strconv"

	"github.com/flexprice/flexprice/internal/api/dto"
	ierr "github.com/flexprice/flexprice/internal/errors"
	"github.com/flexprice/flexprice/internal/logger"
	"github.com/flexprice/flexprice/internal/service"
	"github.com/flexprice/flexprice/internal/types"
	"github.com/gin-gonic/gin"
)

// EntityIntegrationMappingHandler handles entity integration mapping API requests
type EntityIntegrationMappingHandler struct {
	EntityIntegrationMappingService service.EntityIntegrationMappingService
	logger                          *logger.Logger
}

// NewEntityIntegrationMappingHandler creates a new entity integration mapping handler
func NewEntityIntegrationMappingHandler(
	entityIntegrationMappingService service.EntityIntegrationMappingService,
	logger *logger.Logger,
) *EntityIntegrationMappingHandler {
	return &EntityIntegrationMappingHandler{
		EntityIntegrationMappingService: entityIntegrationMappingService,
		logger:                          logger,
	}
}

// CreateEntityIntegrationMapping godoc
// @Summary Create entity integration mapping
// @Description Create a new entity integration mapping
// @Tags Entity Integration Mappings
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param entity_integration_mapping body dto.CreateEntityIntegrationMappingRequest true "Entity integration mapping data"
// @Success 201 {object} dto.EntityIntegrationMappingResponse
// @Failure 400 {object} ierr.ErrorResponse
// @Failure 401 {object} ierr.ErrorResponse
// @Failure 409 {object} ierr.ErrorResponse
// @Failure 500 {object} ierr.ErrorResponse
// @Router /entity-integration-mappings [post]
func (h *EntityIntegrationMappingHandler) CreateEntityIntegrationMapping(c *gin.Context) {
	var req dto.CreateEntityIntegrationMappingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Errorw("failed to bind create entity integration mapping request", "error", err)
		c.Error(ierr.WithError(err).
			WithHint("Invalid request body").
			Mark(ierr.ErrValidation))
		return
	}

	mapping, err := h.EntityIntegrationMappingService.CreateEntityIntegrationMapping(c.Request.Context(), req)
	if err != nil {
		h.logger.Errorw("failed to create entity integration mapping", "error", err)
		c.Error(err)
		return
	}

	c.JSON(http.StatusCreated, mapping)
}

// GetEntityIntegrationMapping godoc
// @Summary Get entity integration mapping
// @Description Retrieve a specific entity integration mapping by ID
// @Tags Entity Integration Mappings
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param id path string true "Entity integration mapping ID"
// @Success 200 {object} dto.EntityIntegrationMappingResponse
// @Failure 400 {object} ierr.ErrorResponse
// @Failure 401 {object} ierr.ErrorResponse
// @Failure 404 {object} ierr.ErrorResponse
// @Failure 500 {object} ierr.ErrorResponse
// @Router /entity-integration-mappings/{id} [get]
func (h *EntityIntegrationMappingHandler) GetEntityIntegrationMapping(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.Error(ierr.NewError("entity integration mapping ID is required").
			WithHint("Entity integration mapping ID is required").
			Mark(ierr.ErrValidation))
		return
	}

	mapping, err := h.EntityIntegrationMappingService.GetEntityIntegrationMapping(c.Request.Context(), id)
	if err != nil {
		h.logger.Errorw("failed to get entity integration mapping", "error", err, "id", id)
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, mapping)
}

// UpdateEntityIntegrationMapping godoc
// @Summary Update entity integration mapping
// @Description Update an existing entity integration mapping
// @Tags Entity Integration Mappings
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param id path string true "Entity integration mapping ID"
// @Param entity_integration_mapping body dto.UpdateEntityIntegrationMappingRequest true "Entity integration mapping data"
// @Success 200 {object} dto.EntityIntegrationMappingResponse
// @Failure 400 {object} ierr.ErrorResponse
// @Failure 401 {object} ierr.ErrorResponse
// @Failure 404 {object} ierr.ErrorResponse
// @Failure 500 {object} ierr.ErrorResponse
// @Router /entity-integration-mappings/{id} [put]
func (h *EntityIntegrationMappingHandler) UpdateEntityIntegrationMapping(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.Error(ierr.NewError("entity integration mapping ID is required").
			WithHint("Entity integration mapping ID is required").
			Mark(ierr.ErrValidation))
		return
	}

	var req dto.UpdateEntityIntegrationMappingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Errorw("failed to bind update entity integration mapping request", "error", err)
		c.Error(ierr.WithError(err).
			WithHint("Invalid request body").
			Mark(ierr.ErrValidation))
		return
	}

	mapping, err := h.EntityIntegrationMappingService.UpdateEntityIntegrationMapping(c.Request.Context(), id, req)
	if err != nil {
		h.logger.Errorw("failed to update entity integration mapping", "error", err, "id", id)
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, mapping)
}

// DeleteEntityIntegrationMapping godoc
// @Summary Delete entity integration mapping
// @Description Delete an entity integration mapping
// @Tags Entity Integration Mappings
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param id path string true "Entity integration mapping ID"
// @Success 204 "No Content"
// @Failure 400 {object} ierr.ErrorResponse
// @Failure 401 {object} ierr.ErrorResponse
// @Failure 404 {object} ierr.ErrorResponse
// @Failure 500 {object} ierr.ErrorResponse
// @Router /entity-integration-mappings/{id} [delete]
func (h *EntityIntegrationMappingHandler) DeleteEntityIntegrationMapping(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.Error(ierr.NewError("entity integration mapping ID is required").
			WithHint("Entity integration mapping ID is required").
			Mark(ierr.ErrValidation))
		return
	}

	err := h.EntityIntegrationMappingService.DeleteEntityIntegrationMapping(c.Request.Context(), id)
	if err != nil {
		h.logger.Errorw("failed to delete entity integration mapping", "error", err, "id", id)
		c.Error(err)
		return
	}

	c.Status(http.StatusNoContent)
}

// ListEntityIntegrationMappings godoc
// @Summary List entity integration mappings
// @Description Retrieve a list of entity integration mappings with optional filtering
// @Tags Entity Integration Mappings
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param entity_id query string false "Filter by FlexPrice entity ID"
// @Param entity_type query string false "Filter by entity type"
// @Param provider_type query string false "Filter by provider type"
// @Param provider_entity_id query string false "Filter by provider entity ID"
// @Param limit query int false "Number of results to return (default: 20, max: 100)"
// @Param offset query int false "Pagination offset (default: 0)"
// @Success 200 {object} dto.ListEntityIntegrationMappingsResponse
// @Failure 400 {object} ierr.ErrorResponse
// @Failure 401 {object} ierr.ErrorResponse
// @Failure 500 {object} ierr.ErrorResponse
// @Router /entity-integration-mappings [get]
func (h *EntityIntegrationMappingHandler) ListEntityIntegrationMappings(c *gin.Context) {
	// Parse query parameters
	filter := &types.EntityIntegrationMappingFilter{
		QueryFilter: &types.QueryFilter{},
	}

	if entityID := c.Query("entity_id"); entityID != "" {
		filter.EntityID = entityID
	}
	if entityType := c.Query("entity_type"); entityType != "" {
		filter.EntityType = types.IntegrationEntityType(entityType)
	}
	if providerType := c.Query("provider_type"); providerType != "" {
		filter.ProviderType = providerType
	}
	if providerEntityID := c.Query("provider_entity_id"); providerEntityID != "" {
		filter.ProviderEntityID = providerEntityID
	}

	// Parse pagination parameters
	limitStr := c.DefaultQuery("limit", "20")
	offsetStr := c.DefaultQuery("offset", "0")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 || limit > 100 {
		limit = 20
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}

	filter.Limit = &limit
	filter.Offset = &offset

	mappings, err := h.EntityIntegrationMappingService.GetEntityIntegrationMappings(c.Request.Context(), filter)
	if err != nil {
		h.logger.Errorw("failed to list entity integration mappings", "error", err)
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, mappings)
}

// GetEntityIntegrationMappingByEntityAndProvider godoc
// @Summary Get entity integration mapping by entity and provider
// @Description Retrieve an entity integration mapping by FlexPrice entity and provider
// @Tags Entity Integration Mappings
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param entity_id query string true "FlexPrice entity ID"
// @Param entity_type query string true "Entity type"
// @Param provider_type query string true "Provider type"
// @Success 200 {object} dto.EntityIntegrationMappingResponse
// @Failure 400 {object} ierr.ErrorResponse
// @Failure 401 {object} ierr.ErrorResponse
// @Failure 404 {object} ierr.ErrorResponse
// @Failure 500 {object} ierr.ErrorResponse
// @Router /entity-integration-mappings/by-entity-and-provider [get]
func (h *EntityIntegrationMappingHandler) GetEntityIntegrationMappingByEntityAndProvider(c *gin.Context) {
	entityID := c.Query("entity_id")
	entityType := c.Query("entity_type")
	providerType := c.Query("provider_type")

	if entityID == "" || entityType == "" || providerType == "" {
		c.Error(ierr.NewError("entity_id, entity_type, and provider_type are required").
			WithHint("entity_id, entity_type, and provider_type are required").
			Mark(ierr.ErrValidation))
		return
	}

	mapping, err := h.EntityIntegrationMappingService.GetByEntityAndProvider(c.Request.Context(), entityID, types.IntegrationEntityType(entityType), providerType)
	if err != nil {
		h.logger.Errorw("failed to get entity integration mapping by entity and provider",
			"error", err, "entity_id", entityID, "entity_type", entityType, "provider_type", providerType)
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, mapping)
}

// GetEntityIntegrationMappingByProviderEntity godoc
// @Summary Get entity integration mapping by provider entity
// @Description Retrieve an entity integration mapping by provider entity ID
// @Tags Entity Integration Mappings
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param provider_type query string true "Provider type"
// @Param provider_entity_id query string true "Provider entity ID"
// @Success 200 {object} dto.EntityIntegrationMappingResponse
// @Failure 400 {object} ierr.ErrorResponse
// @Failure 401 {object} ierr.ErrorResponse
// @Failure 404 {object} ierr.ErrorResponse
// @Failure 500 {object} ierr.ErrorResponse
// @Router /entity-integration-mappings/by-provider-entity [get]
func (h *EntityIntegrationMappingHandler) GetEntityIntegrationMappingByProviderEntity(c *gin.Context) {
	providerType := c.Query("provider_type")
	providerEntityID := c.Query("provider_entity_id")

	if providerType == "" || providerEntityID == "" {
		c.Error(ierr.NewError("provider_type and provider_entity_id are required").
			WithHint("provider_type and provider_entity_id are required").
			Mark(ierr.ErrValidation))
		return
	}

	mapping, err := h.EntityIntegrationMappingService.GetByProviderEntity(c.Request.Context(), providerType, providerEntityID)
	if err != nil {
		h.logger.Errorw("failed to get entity integration mapping by provider entity",
			"error", err, "provider_type", providerType, "provider_entity_id", providerEntityID)
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, mapping)
}
