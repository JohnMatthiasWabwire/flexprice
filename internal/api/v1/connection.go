package v1

import (
	"net/http"

	"github.com/flexprice/flexprice/internal/api/dto"
	ierr "github.com/flexprice/flexprice/internal/errors"
	"github.com/flexprice/flexprice/internal/logger"
	"github.com/flexprice/flexprice/internal/service"
	"github.com/flexprice/flexprice/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
)

type ConnectionHandler struct {
	service service.ConnectionService
	log     *logger.Logger
}

func NewConnectionHandler(
	service service.ConnectionService,
	log *logger.Logger,
) *ConnectionHandler {
	return &ConnectionHandler{
		service: service,
		log:     log,
	}
}

// @Summary Create a connection
// @Description Create a new integration connection
// @Tags Connections
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param connection body dto.CreateConnectionRequest true "Connection"
// @Success 201 {object} dto.ConnectionResponse
// @Failure 400 {object} ierr.ErrorResponse
// @Failure 500 {object} ierr.ErrorResponse
// @Router /connections [post]
func (h *ConnectionHandler) CreateConnection(c *gin.Context) {
	var req dto.CreateConnectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(ierr.WithError(err).
			WithHint("Invalid request format").
			Mark(ierr.ErrValidation))
		return
	}

	resp, err := h.service.CreateConnection(c.Request.Context(), req)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusCreated, resp)
}

// @Summary Get a connection
// @Description Get a connection by ID
// @Tags Connections
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param id path string true "Connection ID"
// @Success 200 {object} dto.ConnectionResponse
// @Failure 400 {object} ierr.ErrorResponse
// @Failure 404 {object} ierr.ErrorResponse
// @Failure 500 {object} ierr.ErrorResponse
// @Router /connections/{id} [get]
func (h *ConnectionHandler) GetConnection(c *gin.Context) {
	id := c.Param("id")

	resp, err := h.service.GetConnection(c.Request.Context(), id)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

// @Summary Get connections
// @Description Get a list of connections
// @Tags Connections
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param filter query types.ConnectionFilter false "Filter"
// @Success 200 {object} dto.ListConnectionsResponse
// @Failure 400 {object} ierr.ErrorResponse
// @Failure 500 {object} ierr.ErrorResponse
// @Router /connections [get]
func (h *ConnectionHandler) GetConnections(c *gin.Context) {
	var filter types.ConnectionFilter
	if err := c.ShouldBindQuery(&filter); err != nil {
		c.Error(ierr.WithError(err).
			WithHint("Invalid filter parameters").
			Mark(ierr.ErrValidation))
		return
	}

	if filter.GetLimit() == 0 {
		filter.Limit = lo.ToPtr(types.GetDefaultFilter().Limit)
	}

	resp, err := h.service.GetConnections(c.Request.Context(), &filter)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

// @Summary Update a connection
// @Description Update a connection by ID
// @Tags Connections
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param id path string true "Connection ID"
// @Param connection body dto.UpdateConnectionRequest true "Connection"
// @Success 200 {object} dto.ConnectionResponse
// @Failure 400 {object} ierr.ErrorResponse
// @Failure 404 {object} ierr.ErrorResponse
// @Failure 500 {object} ierr.ErrorResponse
// @Router /connections/{id} [put]
func (h *ConnectionHandler) UpdateConnection(c *gin.Context) {
	id := c.Param("id")

	var req dto.UpdateConnectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(ierr.WithError(err).
			WithHint("Invalid request format").
			Mark(ierr.ErrValidation))
		return
	}

	resp, err := h.service.UpdateConnection(c.Request.Context(), id, req)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

// @Summary Delete a connection
// @Description Delete a connection by ID
// @Tags Connections
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param id path string true "Connection ID"
// @Success 204
// @Failure 400 {object} ierr.ErrorResponse
// @Failure 404 {object} ierr.ErrorResponse
// @Failure 500 {object} ierr.ErrorResponse
// @Router /connections/{id} [delete]
func (h *ConnectionHandler) DeleteConnection(c *gin.Context) {
	id := c.Param("id")

	err := h.service.DeleteConnection(c.Request.Context(), id)
	if err != nil {
		c.Error(err)
		return
	}

	c.Status(http.StatusNoContent)
}

// @Summary Get a connection by code
// @Description Get a connection by connection code
// @Tags Connections
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param connection_code path string true "Connection Code"
// @Success 200 {object} dto.ConnectionResponse
// @Failure 400 {object} ierr.ErrorResponse
// @Failure 404 {object} ierr.ErrorResponse
// @Failure 500 {object} ierr.ErrorResponse
// @Router /connections/code/{connection_code} [get]
func (h *ConnectionHandler) GetConnectionByCode(c *gin.Context) {
	connectionCode := c.Param("connection_code")
	if connectionCode == "" {
		c.Error(ierr.NewError("connection code is required").
			WithHint("Connection code is required").
			Mark(ierr.ErrValidation))
		return
	}

	resp, err := h.service.GetConnectionByCode(c.Request.Context(), connectionCode)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

// @Summary List connections by filter
// @Description List connections by filter
// @Tags Connections
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param filter body types.ConnectionFilter true "Filter"
// @Success 200 {object} dto.ListConnectionsResponse
// @Failure 400 {object} ierr.ErrorResponse
// @Failure 500 {object} ierr.ErrorResponse
// @Router /connections/search [post]
func (h *ConnectionHandler) ListConnectionsByFilter(c *gin.Context) {
	var filter types.ConnectionFilter
	if err := c.ShouldBindJSON(&filter); err != nil {
		c.Error(ierr.WithError(err).
			WithHint("Invalid filter parameters").
			Mark(ierr.ErrValidation))
		return
	}

	if filter.GetLimit() == 0 {
		filter.Limit = lo.ToPtr(types.GetDefaultFilter().Limit)
	}

	resp, err := h.service.GetConnections(c.Request.Context(), &filter)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, resp)
}
