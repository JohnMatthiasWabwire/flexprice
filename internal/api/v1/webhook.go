package v1

import (
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/flexprice/flexprice/internal/config"
	"github.com/flexprice/flexprice/internal/integration"
	"github.com/flexprice/flexprice/internal/integration/stripe/webhook"
	"github.com/flexprice/flexprice/internal/interfaces"
	"github.com/flexprice/flexprice/internal/logger"
	"github.com/flexprice/flexprice/internal/postgres"
	"github.com/flexprice/flexprice/internal/svix"
	"github.com/flexprice/flexprice/internal/types"
	"github.com/gin-gonic/gin"
)

// WebhookHandler handles webhook-related endpoints
type WebhookHandler struct {
	config                          *config.Configuration
	svixClient                      *svix.Client
	logger                          *logger.Logger
	integrationFactory              *integration.Factory
	customerService                 interfaces.CustomerService
	paymentService                  interfaces.PaymentService
	invoiceService                  interfaces.InvoiceService
	planService                     interfaces.PlanService
	subscriptionService             interfaces.SubscriptionService
	entityIntegrationMappingService interfaces.EntityIntegrationMappingService
	db                              postgres.IClient
}

// NewWebhookHandler creates a new webhook handler
func NewWebhookHandler(
	cfg *config.Configuration,
	svixClient *svix.Client,
	logger *logger.Logger,
	integrationFactory *integration.Factory,
	customerService interfaces.CustomerService,
	paymentService interfaces.PaymentService,
	invoiceService interfaces.InvoiceService,
	planService interfaces.PlanService,
	subscriptionService interfaces.SubscriptionService,
	entityIntegrationMappingService interfaces.EntityIntegrationMappingService,
	db postgres.IClient,
) *WebhookHandler {
	return &WebhookHandler{
		config:                          cfg,
		svixClient:                      svixClient,
		logger:                          logger,
		integrationFactory:              integrationFactory,
		customerService:                 customerService,
		paymentService:                  paymentService,
		invoiceService:                  invoiceService,
		planService:                     planService,
		subscriptionService:             subscriptionService,
		entityIntegrationMappingService: entityIntegrationMappingService,
		db:                              db,
	}
}

// GetDashboardURL handles the GET /webhooks/dashboard endpoint
func (h *WebhookHandler) GetDashboardURL(c *gin.Context) {
	if !h.config.Webhook.Svix.Enabled {
		c.JSON(http.StatusOK, gin.H{
			"url":          "",
			"svix_enabled": false,
		})
		return
	}

	tenantID := types.GetTenantID(c.Request.Context())
	environmentID := types.GetEnvironmentID(c.Request.Context())

	// Get or create Svix application
	appID, err := h.svixClient.GetOrCreateApplication(c.Request.Context(), tenantID, environmentID)
	if err != nil {
		h.logger.Errorw("failed to get/create Svix application",
			"error", err,
			"tenant_id", tenantID,
			"environment_id", environmentID,
		)
		c.Error(err)
		return
	}

	// Get dashboard URL
	url, err := h.svixClient.GetDashboardURL(c.Request.Context(), appID)
	if err != nil {
		h.logger.Errorw("failed to get Svix dashboard URL",
			"error", err,
			"tenant_id", tenantID,
			"environment_id", environmentID,
			"app_id", appID,
		)
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"url":          url,
		"svix_enabled": true,
	})
}

// @Summary Handle Stripe webhook events
// @Description Process incoming Stripe webhook events for payment status updates and customer creation
// @Tags Webhooks
// @Accept json
// @Produce json
// @Param tenant_id path string true "Tenant ID"
// @Param environment_id path string true "Environment ID"
// @Param Stripe-Signature header string true "Stripe webhook signature"
// @Success 200 {object} map[string]interface{} "Webhook processed successfully"
// @Failure 400 {object} map[string]interface{} "Bad request - missing parameters or invalid signature"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /webhooks/stripe/{tenant_id}/{environment_id} [post]
func (h *WebhookHandler) HandleStripeWebhook(c *gin.Context) {
	tenantID := c.Param("tenant_id")
	environmentID := c.Param("environment_id")

	if tenantID == "" || environmentID == "" {
		h.logger.Errorw("missing tenant_id or environment_id in webhook URL")
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "tenant_id and environment_id are required",
		})
		return
	}

	// Read the raw request body
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		h.logger.Errorw("failed to read request body", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Failed to read request body",
		})
		return
	}

	// Get Stripe signature from headers
	signature := c.GetHeader("Stripe-Signature")
	if signature == "" {
		h.logger.Errorw("missing Stripe-Signature header")
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Missing Stripe-Signature header",
		})
		return
	}

	// Set context with tenant and environment IDs
	ctx := types.SetTenantID(c.Request.Context(), tenantID)
	ctx = types.SetEnvironmentID(ctx, environmentID)
	c.Request = c.Request.WithContext(ctx)

	// Get Stripe integration
	stripeIntegration, err := h.integrationFactory.GetStripeIntegration(ctx)
	if err != nil {
		h.logger.Errorw("failed to get Stripe integration", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Stripe integration not available",
		})
		return
	}

	// Get Stripe client and configuration
	_, stripeConfig, err := stripeIntegration.Client.GetStripeClient(ctx)
	if err != nil {
		h.logger.Errorw("failed to get Stripe client and configuration",
			"error", err,
			"environment_id", environmentID,
		)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Stripe connection not configured for this environment",
		})
		return
	}

	// Verify webhook secret is configured
	if stripeConfig.WebhookSecret == "" {
		h.logger.Errorw("webhook secret not configured for Stripe connection",
			"environment_id", environmentID,
		)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Webhook secret not configured",
		})
		return
	}

	// Log webhook processing (without sensitive data)
	h.logger.Debugw("processing webhook",
		"environment_id", environmentID,
		"tenant_id", tenantID,
		"payload_length", len(body),
	)

	// Parse and verify the webhook event using new integration
	event, err := stripeIntegration.PaymentSvc.ParseWebhookEvent(body, signature, stripeConfig.WebhookSecret)
	if err != nil {
		h.logger.Errorw("failed to parse/verify Stripe webhook event", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Failed to verify webhook signature or parse event",
		})
		return
	}

	// Create service dependencies for webhook handler
	serviceDeps := &webhook.ServiceDependencies{
		CustomerService:                 h.customerService,
		PaymentService:                  h.paymentService,
		InvoiceService:                  h.invoiceService,
		PlanService:                     h.planService,
		SubscriptionService:             h.subscriptionService,
		EntityIntegrationMappingService: h.entityIntegrationMappingService,
		DB:                              h.db,
	}

	// Handle the webhook event using new integration
	err = stripeIntegration.WebhookHandler.HandleWebhookEvent(ctx, event, environmentID, serviceDeps)
	if err != nil {
		h.logger.Errorw("failed to handle webhook event", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to process webhook event",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Webhook processed successfully",
	})
}

// @Summary Handle HubSpot webhook events
// @Description Process incoming HubSpot webhook events for deal closed won and customer creation
// @Tags Webhooks
// @Accept json
// @Produce json
// @Param tenant_id path string true "Tenant ID"
// @Param environment_id path string true "Environment ID"
// @Param X-HubSpot-Signature header string true "HubSpot webhook signature"
// @Success 200 {object} map[string]interface{} "Webhook received (always returns 200)"
// @Router /webhooks/hubspot/{tenant_id}/{environment_id} [post]
func (h *WebhookHandler) HandleHubSpotWebhook(c *gin.Context) {
	// Always return 200 OK to HubSpot to prevent retries
	// We log errors internally but don't expose them to HubSpot
	defer func() {
		c.JSON(http.StatusOK, gin.H{
			"message": "Webhook received",
		})
	}()

	tenantID := c.Param("tenant_id")
	environmentID := c.Param("environment_id")

	if tenantID == "" || environmentID == "" {
		h.logger.Errorw("missing tenant_id or environment_id in webhook URL",
			"tenant_id", tenantID,
			"environment_id", environmentID)
		return
	}

	// Read the raw request body
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		h.logger.Errorw("failed to read request body", "error", err)
		return
	}

	// Get HubSpot v3 signature and timestamp headers
	signature := c.GetHeader("X-HubSpot-Signature-v3")
	timestamp := c.GetHeader("X-HubSpot-Request-Timestamp")

	if signature == "" {
		h.logger.Errorw("missing X-HubSpot-Signature-v3 header")
		return
	}

	if timestamp == "" {
		h.logger.Errorw("missing X-HubSpot-Request-Timestamp header")
		return
	}

	h.logger.Infow("received HubSpot webhook",
		"signature_length", len(signature),
		"timestamp", timestamp,
		"tenant_id", tenantID,
		"environment_id", environmentID)

	// Set context with tenant and environment IDs
	ctx := types.SetTenantID(c.Request.Context(), tenantID)
	ctx = types.SetEnvironmentID(ctx, environmentID)
	c.Request = c.Request.WithContext(ctx)

	// Get HubSpot integration
	hubspotIntegration, err := h.integrationFactory.GetHubSpotIntegration(ctx)
	if err != nil {
		h.logger.Errorw("failed to get HubSpot integration", "error", err)
		return
	}

	// Get HubSpot configuration
	hubspotConfig, err := hubspotIntegration.Client.GetHubSpotConfig(ctx)
	if err != nil {
		h.logger.Errorw("failed to get HubSpot configuration",
			"error", err,
			"environment_id", environmentID)
		return
	}

	// Verify webhook secret is configured
	if hubspotConfig.ClientSecret == "" {
		h.logger.Errorw("client secret not configured for HubSpot connection",
			"environment_id", environmentID)
		return
	}

	// Validate timestamp (reject if older than 5 minutes)
	timestampInt, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		h.logger.Errorw("invalid timestamp format", "timestamp", timestamp, "error", err)
		return
	}

	currentTime := time.Now().UnixMilli()
	maxAllowedTimestamp := int64(300000) // 5 minutes in milliseconds
	if currentTime-timestampInt > maxAllowedTimestamp {
		h.logger.Warnw("timestamp too old, rejecting webhook",
			"timestamp", timestampInt,
			"current_time", currentTime,
			"age_ms", currentTime-timestampInt)
		return
	}

	// Construct the full URL that HubSpot called
	// When behind a proxy (like ngrok), check X-Forwarded-Proto
	var scheme string
	if proto := c.GetHeader("X-Forwarded-Proto"); proto != "" {
		scheme = proto
	} else if c.Request.TLS != nil {
		scheme = "https"
	} else {
		scheme = "http"
	}
	fullURL := scheme + "://" + c.Request.Host + c.Request.URL.String()

	h.logger.Debugw("verifying v3 signature",
		"method", c.Request.Method,
		"full_url", fullURL,
		"timestamp", timestamp)

	// Verify webhook signature (v3)
	signatureValid := hubspotIntegration.Client.VerifyWebhookSignatureV3(
		c.Request.Method,
		fullURL,
		body,
		timestamp,
		signature,
		hubspotConfig.ClientSecret,
	)

	if !signatureValid {
		h.logger.Errorw("invalid webhook signature - rejecting")
		return
	}

	// Log webhook processing (without sensitive data)
	h.logger.Infow("processing HubSpot webhook",
		"environment_id", environmentID,
		"tenant_id", tenantID,
		"payload_length", len(body))

	// Parse webhook payload
	events, err := hubspotIntegration.WebhookHandler.ParseWebhookPayload(body)
	if err != nil {
		h.logger.Errorw("failed to parse HubSpot webhook payload", "error", err)
		return
	}

	// Create service dependencies for webhook handler
	serviceDeps := &interfaces.ServiceDependencies{
		CustomerService:                 h.customerService,
		PaymentService:                  h.paymentService,
		InvoiceService:                  h.invoiceService,
		PlanService:                     h.planService,
		SubscriptionService:             h.subscriptionService,
		EntityIntegrationMappingService: h.entityIntegrationMappingService,
		DB:                              h.db,
	}

	// Handle the webhook events
	err = hubspotIntegration.WebhookHandler.HandleWebhookEvent(ctx, events, environmentID, serviceDeps)
	if err != nil {
		h.logger.Errorw("failed to handle HubSpot webhook event", "error", err)
		return
	}

	h.logger.Infow("successfully processed HubSpot webhook",
		"environment_id", environmentID,
		"event_count", len(events))
}
