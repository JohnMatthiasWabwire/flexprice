package hubspot

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/flexprice/flexprice/internal/domain/connection"
	ierr "github.com/flexprice/flexprice/internal/errors"
	"github.com/flexprice/flexprice/internal/httpclient"
	"github.com/flexprice/flexprice/internal/logger"
	"github.com/flexprice/flexprice/internal/security"
	"github.com/flexprice/flexprice/internal/types"
)

const (
	HubSpotAPIBaseURL = "https://api.hubapi.com"
)

// HubSpotClient defines the interface for HubSpot API operations
type HubSpotClient interface {
	GetHubSpotConfig(ctx context.Context) (*HubSpotConfig, error)
	GetDecryptedHubSpotConfig(conn *connection.Connection) (*HubSpotConfig, error)
	VerifyWebhookSignatureV3(method string, uri string, requestBody []byte, timestamp string, signature string, clientSecret string) bool
	GetDeal(ctx context.Context, dealID string) (*DealResponse, error)
	GetContact(ctx context.Context, contactID string) (*ContactResponse, error)
	GetDealAssociations(ctx context.Context, dealID string) (*AssociationResponse, error)
	HasHubSpotConnection(ctx context.Context) bool

	// Invoice operations
	CreateInvoice(ctx context.Context, req *InvoiceCreateRequest) (*InvoiceResponse, error)
	UpdateInvoice(ctx context.Context, invoiceID string, properties InvoiceProperties) (*InvoiceResponse, error)
	CreateLineItem(ctx context.Context, req *LineItemCreateRequest) (*LineItemResponse, error)
	AssociateLineItemToInvoice(ctx context.Context, lineItemID, invoiceID string) error
	AssociateInvoiceToContact(ctx context.Context, invoiceID, contactID string) error

	// Deal operations
	UpdateDeal(ctx context.Context, dealID string, properties map[string]string) (*DealUpdateResponse, error)
	CreateDealLineItem(ctx context.Context, req *DealLineItemCreateRequest) (*DealLineItemResponse, error)
}

// Client handles HubSpot API client setup and configuration
type Client struct {
	connectionRepo    connection.Repository
	encryptionService security.EncryptionService
	logger            *logger.Logger
	httpClient        httpclient.Client
}

// NewClient creates a new HubSpot client
func NewClient(
	connectionRepo connection.Repository,
	encryptionService security.EncryptionService,
	logger *logger.Logger,
) HubSpotClient {
	return &Client{
		connectionRepo:    connectionRepo,
		encryptionService: encryptionService,
		logger:            logger,
		httpClient:        httpclient.NewDefaultClient(),
	}
}

// HubSpotConfig holds decrypted HubSpot configuration
type HubSpotConfig struct {
	AccessToken  string
	ClientSecret string
	AppID        string
}

// GetHubSpotConfig retrieves and decrypts HubSpot configuration for the current environment
func (c *Client) GetHubSpotConfig(ctx context.Context) (*HubSpotConfig, error) {
	// Get HubSpot connection for this environment
	conn, err := c.connectionRepo.GetByProvider(ctx, types.SecretProviderHubSpot)
	if err != nil {
		return nil, ierr.NewError("failed to get HubSpot connection").
			WithHint("HubSpot connection not configured for this environment").
			Mark(ierr.ErrNotFound)
	}

	hubspotConfig, err := c.GetDecryptedHubSpotConfig(conn)
	if err != nil {
		return nil, ierr.NewError("failed to get HubSpot configuration").
			WithHint("Invalid HubSpot configuration").
			Mark(ierr.ErrValidation)
	}

	return hubspotConfig, nil
}

// GetDecryptedHubSpotConfig decrypts and returns HubSpot configuration
func (c *Client) GetDecryptedHubSpotConfig(conn *connection.Connection) (*HubSpotConfig, error) {
	// Decrypt the connection metadata if it's encrypted
	decryptedMetadata, err := c.decryptConnectionMetadata(conn)
	if err != nil {
		return nil, err
	}

	// Extract HubSpot configuration from decrypted metadata
	hubspotConfig := &HubSpotConfig{}

	if accessToken, exists := decryptedMetadata["access_token"]; exists {
		hubspotConfig.AccessToken = accessToken
	}

	if clientSecret, exists := decryptedMetadata["client_secret"]; exists {
		hubspotConfig.ClientSecret = clientSecret
	}

	if appID, exists := decryptedMetadata["app_id"]; exists {
		hubspotConfig.AppID = appID
	}

	return hubspotConfig, nil
}

// decryptConnectionMetadata decrypts the connection encrypted secret data
func (c *Client) decryptConnectionMetadata(conn *connection.Connection) (types.Metadata, error) {
	// Check if the connection has encrypted secret data
	if conn.EncryptedSecretData.HubSpot == nil {
		return types.Metadata{}, nil
	}

	// For HubSpot connections, decrypt the structured metadata
	if conn.ProviderType == types.SecretProviderHubSpot {
		if conn.EncryptedSecretData.HubSpot == nil {
			return types.Metadata{}, nil
		}

		// Decrypt each field
		accessToken, err := c.encryptionService.Decrypt(conn.EncryptedSecretData.HubSpot.AccessToken)
		if err != nil {
			c.logger.Errorw("failed to decrypt access token", "connection_id", conn.ID, "error", err)
			return nil, ierr.NewError("failed to decrypt access token").Mark(ierr.ErrInternal)
		}

		clientSecret, err := c.encryptionService.Decrypt(conn.EncryptedSecretData.HubSpot.ClientSecret)
		if err != nil {
			c.logger.Errorw("failed to decrypt client secret", "connection_id", conn.ID, "error", err)
			return nil, ierr.NewError("failed to decrypt client secret").Mark(ierr.ErrInternal)
		}

		decryptedMetadata := types.Metadata{
			"access_token":  accessToken,
			"client_secret": clientSecret,
			"app_id":        conn.EncryptedSecretData.HubSpot.AppID,
		}

		return decryptedMetadata, nil
	}

	return types.Metadata{}, nil
}

// VerifyWebhookSignatureV3 verifies the HubSpot webhook signature (v3 format)
// v3 format: Base64(HMAC-SHA256(clientSecret, method + uri + body + timestamp))
func (c *Client) VerifyWebhookSignatureV3(method string, uri string, requestBody []byte, timestamp string, signature string, clientSecret string) bool {
	if signature == "" {
		return false
	}

	// Build the source string: method + uri + body + timestamp
	sourceString := method + uri + string(requestBody) + timestamp

	// Compute HMAC SHA256 of the source string
	mac := hmac.New(sha256.New, []byte(clientSecret))
	mac.Write([]byte(sourceString))
	computedMAC := mac.Sum(nil)

	// HubSpot v3 sends Base64-encoded signature
	computedSignature := base64.StdEncoding.EncodeToString(computedMAC)

	// Use constant-time comparison to prevent timing attacks
	isValid := hmac.Equal([]byte(computedSignature), []byte(signature))

	if !isValid {
		c.logger.Warnw("webhook signature verification failed",
			"source_string_length", len(sourceString))
	}

	return isValid
}

// GetDeal fetches a deal from HubSpot by ID with associated contacts
func (c *Client) GetDeal(ctx context.Context, dealID string) (*DealResponse, error) {
	config, err := c.GetHubSpotConfig(ctx)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/crm/v3/objects/deals/%s?associations=contacts&properties=hs_acv,hs_arr,hs_mrr,hs_tcv,amount,dealname,dealstage", HubSpotAPIBaseURL, dealID)

	req := &httpclient.Request{
		Method: "GET",
		URL:    url,
		Headers: map[string]string{
			"Authorization": "Bearer " + config.AccessToken,
			"Content-Type":  "application/json",
		},
	}

	resp, err := c.httpClient.Send(ctx, req)
	if err != nil {
		return nil, ierr.NewError("failed to fetch deal from HubSpot").
			WithHint("HubSpot API error").
			Mark(ierr.ErrHTTPClient)
	}

	if resp.StatusCode != http.StatusOK {
		c.logger.Errorw("hubspot api error",
			"status", resp.StatusCode,
			"body", string(resp.Body),
			"deal_id", dealID)
		return nil, ierr.NewError("failed to fetch deal from HubSpot").
			WithHint(fmt.Sprintf("HubSpot API returned status %d", resp.StatusCode)).
			Mark(ierr.ErrHTTPClient)
	}

	var deal DealResponse
	if err := json.Unmarshal(resp.Body, &deal); err != nil {
		return nil, ierr.NewError("failed to decode deal response").Mark(ierr.ErrInternal)
	}

	return &deal, nil
}

// GetContact fetches a contact from HubSpot by ID
func (c *Client) GetContact(ctx context.Context, contactID string) (*ContactResponse, error) {
	config, err := c.GetHubSpotConfig(ctx)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/crm/v3/objects/contacts/%s", HubSpotAPIBaseURL, contactID)

	req := &httpclient.Request{
		Method: "GET",
		URL:    url,
		Headers: map[string]string{
			"Authorization": "Bearer " + config.AccessToken,
			"Content-Type":  "application/json",
		},
	}

	resp, err := c.httpClient.Send(ctx, req)
	if err != nil {
		return nil, ierr.NewError("failed to fetch contact from HubSpot").
			WithHint("HubSpot API error").
			Mark(ierr.ErrHTTPClient)
	}

	if resp.StatusCode != http.StatusOK {
		c.logger.Errorw("hubspot api error",
			"status", resp.StatusCode,
			"body", string(resp.Body),
			"contact_id", contactID)
		return nil, ierr.NewError("failed to fetch contact from HubSpot").
			WithHint(fmt.Sprintf("HubSpot API returned status %d", resp.StatusCode)).
			Mark(ierr.ErrHTTPClient)
	}

	var contact ContactResponse
	if err := json.Unmarshal(resp.Body, &contact); err != nil {
		return nil, ierr.NewError("failed to decode contact response").Mark(ierr.ErrInternal)
	}

	return &contact, nil
}

// GetDealAssociations fetches associated contacts for a deal
func (c *Client) GetDealAssociations(ctx context.Context, dealID string) (*AssociationResponse, error) {
	config, err := c.GetHubSpotConfig(ctx)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/crm/v3/objects/deals/%s/associations/contacts", HubSpotAPIBaseURL, dealID)

	req := &httpclient.Request{
		Method: "GET",
		URL:    url,
		Headers: map[string]string{
			"Authorization": "Bearer " + config.AccessToken,
			"Content-Type":  "application/json",
		},
	}

	resp, err := c.httpClient.Send(ctx, req)
	if err != nil {
		return nil, ierr.NewError("failed to fetch associations from HubSpot").
			WithHint("HubSpot API error").
			Mark(ierr.ErrHTTPClient)
	}

	if resp.StatusCode != http.StatusOK {
		c.logger.Errorw("hubspot api error",
			"status", resp.StatusCode,
			"body", string(resp.Body),
			"deal_id", dealID)
		return nil, ierr.NewError("failed to fetch associations from HubSpot").
			WithHint(fmt.Sprintf("HubSpot API returned status %d", resp.StatusCode)).
			Mark(ierr.ErrHTTPClient)
	}

	var associations AssociationResponse
	if err := json.Unmarshal(resp.Body, &associations); err != nil {
		return nil, ierr.NewError("failed to decode associations response").Mark(ierr.ErrInternal)
	}

	return &associations, nil
}

// CreateInvoice creates a draft invoice in HubSpot
func (c *Client) CreateInvoice(ctx context.Context, req *InvoiceCreateRequest) (*InvoiceResponse, error) {
	config, err := c.GetHubSpotConfig(ctx)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/crm/v3/objects/invoices", HubSpotAPIBaseURL)

	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, ierr.NewError("failed to marshal invoice request").Mark(ierr.ErrInternal)
	}

	httpReq := &httpclient.Request{
		Method: http.MethodPost,
		URL:    url,
		Headers: map[string]string{
			"Authorization": fmt.Sprintf("Bearer %s", config.AccessToken),
			"Content-Type":  "application/json",
		},
		Body: reqBody,
	}

	resp, err := c.httpClient.Send(ctx, httpReq)
	if err != nil {
		// Check if it's an HTTP error with status code and response body
		if httpErr, ok := httpclient.IsHTTPError(err); ok {
			c.logger.Errorw("HubSpot API error creating invoice",
				"status_code", httpErr.StatusCode,
				"response_body", string(httpErr.Response),
				"url", url,
				"request_body", string(reqBody))
			return nil, ierr.NewError("failed to create invoice in HubSpot").
				WithHint(fmt.Sprintf("HubSpot API returned status %d: %s", httpErr.StatusCode, string(httpErr.Response))).
				Mark(ierr.ErrHTTPClient)
		}
		// Generic HTTP client error
		c.logger.Errorw("http client error creating invoice",
			"error", err,
			"url", url,
			"request_body", string(reqBody))
		return nil, ierr.NewError("failed to create invoice in HubSpot").
			WithHint("Check HubSpot API connectivity and access token").
			Mark(ierr.ErrHTTPClient)
	}

	if resp.StatusCode != http.StatusCreated {
		c.logger.Errorw("hubspot create invoice error",
			"status", resp.StatusCode,
			"body", string(resp.Body),
			"request_body", string(reqBody))
		return nil, ierr.NewError("failed to create invoice in HubSpot").
			WithHint(fmt.Sprintf("HubSpot API returned status %d", resp.StatusCode)).
			Mark(ierr.ErrHTTPClient)
	}

	var invoice InvoiceResponse
	if err := json.Unmarshal(resp.Body, &invoice); err != nil {
		return nil, ierr.NewError("failed to decode invoice response").Mark(ierr.ErrInternal)
	}

	return &invoice, nil
}

// UpdateInvoice updates an existing invoice in HubSpot
func (c *Client) UpdateInvoice(ctx context.Context, invoiceID string, properties InvoiceProperties) (*InvoiceResponse, error) {
	config, err := c.GetHubSpotConfig(ctx)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/crm/v3/objects/invoices/%s", HubSpotAPIBaseURL, invoiceID)

	reqBody, err := json.Marshal(map[string]interface{}{
		"properties": properties,
	})
	if err != nil {
		return nil, ierr.NewError("failed to marshal invoice update request").Mark(ierr.ErrInternal)
	}

	httpReq := &httpclient.Request{
		Method: http.MethodPatch,
		URL:    url,
		Headers: map[string]string{
			"Authorization": fmt.Sprintf("Bearer %s", config.AccessToken),
			"Content-Type":  "application/json",
		},
		Body: reqBody,
	}

	resp, err := c.httpClient.Send(ctx, httpReq)
	if err != nil {
		// Check if it's an HTTP error with status code
		if httpErr, ok := httpclient.IsHTTPError(err); ok {
			c.logger.Errorw("HubSpot API error updating invoice",
				"status_code", httpErr.StatusCode,
				"response_body", string(httpErr.Response),
				"url", url,
				"invoice_id", invoiceID,
				"request_body", string(reqBody))
			return nil, ierr.NewError("failed to update invoice in HubSpot").
				WithHint(fmt.Sprintf("HubSpot API returned status %d: %s", httpErr.StatusCode, string(httpErr.Response))).
				Mark(ierr.ErrHTTPClient)
		}

		// Generic error (network/timeout)
		c.logger.Errorw("http client error updating invoice",
			"error", err,
			"url", url,
			"invoice_id", invoiceID,
			"request_body", string(reqBody))
		return nil, ierr.NewError("failed to update invoice in HubSpot").
			WithHint("Check HubSpot API connectivity").
			Mark(ierr.ErrHTTPClient)
	}

	if resp.StatusCode != http.StatusOK {
		c.logger.Errorw("hubspot update invoice error",
			"status", resp.StatusCode,
			"body", string(resp.Body),
			"invoice_id", invoiceID,
			"request_body", string(reqBody))
		return nil, ierr.NewError("failed to update invoice in HubSpot").
			WithHint(fmt.Sprintf("HubSpot API returned status %d", resp.StatusCode)).
			Mark(ierr.ErrHTTPClient)
	}

	var invoice InvoiceResponse
	if err := json.Unmarshal(resp.Body, &invoice); err != nil {
		return nil, ierr.NewError("failed to decode invoice response").Mark(ierr.ErrInternal)
	}

	return &invoice, nil
}

// UpdateDeal updates a HubSpot deal with the given properties
func (c *Client) UpdateDeal(ctx context.Context, dealID string, properties map[string]string) (*DealUpdateResponse, error) {
	config, err := c.GetHubSpotConfig(ctx)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/crm/v3/objects/deals/%s", HubSpotAPIBaseURL, dealID)

	reqBody, err := json.Marshal(&DealUpdateRequest{
		Properties: properties,
	})
	if err != nil {
		return nil, ierr.NewError("failed to marshal deal update request").Mark(ierr.ErrInternal)
	}

	httpReq := &httpclient.Request{
		Method: http.MethodPatch,
		URL:    url,
		Headers: map[string]string{
			"Authorization": fmt.Sprintf("Bearer %s", config.AccessToken),
			"Content-Type":  "application/json",
		},
		Body: reqBody,
	}

	resp, err := c.httpClient.Send(ctx, httpReq)
	if err != nil {
		if httpErr, ok := httpclient.IsHTTPError(err); ok {
			c.logger.Errorw("HubSpot API error updating deal",
				"status_code", httpErr.StatusCode,
				"response_body", string(httpErr.Response),
				"url", url,
				"deal_id", dealID)
			return nil, ierr.NewError("failed to update deal in HubSpot").
				WithHint(fmt.Sprintf("HubSpot API returned status %d: %s", httpErr.StatusCode, string(httpErr.Response))).
				Mark(ierr.ErrHTTPClient)
		}

		c.logger.Errorw("http client error updating deal",
			"error", err,
			"url", url,
			"deal_id", dealID)
		return nil, ierr.NewError("failed to update deal in HubSpot").
			WithHint("Check HubSpot API connectivity").
			Mark(ierr.ErrHTTPClient)
	}

	if resp.StatusCode != http.StatusOK {
		c.logger.Errorw("hubspot update deal error",
			"status", resp.StatusCode,
			"body", string(resp.Body),
			"deal_id", dealID)
		return nil, ierr.NewError("failed to update deal in HubSpot").
			WithHint(fmt.Sprintf("HubSpot API returned status %d", resp.StatusCode)).
			Mark(ierr.ErrHTTPClient)
	}

	var deal DealUpdateResponse
	if err := json.Unmarshal(resp.Body, &deal); err != nil {
		return nil, ierr.NewError("failed to decode deal response").Mark(ierr.ErrInternal)
	}

	return &deal, nil
}

// CreateLineItem creates a line item in HubSpot
func (c *Client) CreateLineItem(ctx context.Context, req *LineItemCreateRequest) (*LineItemResponse, error) {
	config, err := c.GetHubSpotConfig(ctx)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/crm/v3/objects/line_items", HubSpotAPIBaseURL)

	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, ierr.NewError("failed to marshal line item request").Mark(ierr.ErrInternal)
	}

	httpReq := &httpclient.Request{
		Method: http.MethodPost,
		URL:    url,
		Headers: map[string]string{
			"Authorization": fmt.Sprintf("Bearer %s", config.AccessToken),
			"Content-Type":  "application/json",
		},
		Body: reqBody,
	}

	resp, err := c.httpClient.Send(ctx, httpReq)
	if err != nil {
		c.logger.Errorw("http client error creating line item",
			"error", err,
			"url", url,
			"request_body", string(reqBody))
		return nil, ierr.NewError("failed to create line item in HubSpot").
			WithHint("Check HubSpot API connectivity").
			Mark(ierr.ErrHTTPClient)
	}

	if resp.StatusCode != http.StatusCreated {
		c.logger.Errorw("hubspot create line item error",
			"status", resp.StatusCode,
			"body", string(resp.Body),
			"request_body", string(reqBody))
		return nil, ierr.NewError("failed to create line item in HubSpot").
			WithHint(fmt.Sprintf("HubSpot API returned status %d", resp.StatusCode)).
			Mark(ierr.ErrHTTPClient)
	}

	var lineItem LineItemResponse
	if err := json.Unmarshal(resp.Body, &lineItem); err != nil {
		return nil, ierr.NewError("failed to decode line item response").Mark(ierr.ErrInternal)
	}

	return &lineItem, nil
}

// AssociateLineItemToInvoice associates a line item with an invoice
func (c *Client) AssociateLineItemToInvoice(ctx context.Context, lineItemID, invoiceID string) error {
	config, err := c.GetHubSpotConfig(ctx)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/crm/v4/objects/line_items/%s/associations/default/invoices/%s",
		HubSpotAPIBaseURL, lineItemID, invoiceID)

	httpReq := &httpclient.Request{
		Method: http.MethodPut,
		URL:    url,
		Headers: map[string]string{
			"Authorization": fmt.Sprintf("Bearer %s", config.AccessToken),
		},
	}

	resp, err := c.httpClient.Send(ctx, httpReq)
	if err != nil {
		return ierr.NewError("failed to associate line item to invoice").
			WithHint("Check HubSpot API connectivity").
			Mark(ierr.ErrHTTPClient)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		c.logger.Errorw("hubspot associate line item error",
			"status", resp.StatusCode,
			"body", string(resp.Body),
			"line_item_id", lineItemID,
			"invoice_id", invoiceID)
		return ierr.NewError("failed to associate line item to invoice").
			WithHint(fmt.Sprintf("HubSpot API returned status %d", resp.StatusCode)).
			Mark(ierr.ErrHTTPClient)
	}

	return nil
}

// AssociateInvoiceToContact associates an invoice with a contact
func (c *Client) AssociateInvoiceToContact(ctx context.Context, invoiceID, contactID string) error {
	config, err := c.GetHubSpotConfig(ctx)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/crm/v4/objects/invoices/%s/associations/default/contacts/%s",
		HubSpotAPIBaseURL, invoiceID, contactID)

	httpReq := &httpclient.Request{
		Method: http.MethodPut,
		URL:    url,
		Headers: map[string]string{
			"Authorization": fmt.Sprintf("Bearer %s", config.AccessToken),
		},
	}

	resp, err := c.httpClient.Send(ctx, httpReq)
	if err != nil {
		return ierr.NewError("failed to associate invoice to contact").
			WithHint("Check HubSpot API connectivity").
			Mark(ierr.ErrHTTPClient)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		c.logger.Errorw("hubspot associate invoice error",
			"status", resp.StatusCode,
			"body", string(resp.Body),
			"invoice_id", invoiceID,
			"contact_id", contactID)
		return ierr.NewError("failed to associate invoice to contact").
			WithHint(fmt.Sprintf("HubSpot API returned status %d", resp.StatusCode)).
			Mark(ierr.ErrHTTPClient)
	}

	return nil
}

// HasHubSpotConnection checks if the tenant has a HubSpot connection available
func (c *Client) HasHubSpotConnection(ctx context.Context) bool {
	conn, err := c.connectionRepo.GetByProvider(ctx, types.SecretProviderHubSpot)
	return err == nil && conn != nil && conn.Status == types.StatusPublished
}

// CreateDealLineItem creates a new line item in HubSpot and associates it with a deal
func (c *Client) CreateDealLineItem(ctx context.Context, req *DealLineItemCreateRequest) (*DealLineItemResponse, error) {
	config, err := c.GetHubSpotConfig(ctx)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/crm/v3/objects/line_items", HubSpotAPIBaseURL)

	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, ierr.NewError("failed to marshal line item create request").Mark(ierr.ErrInternal)
	}

	httpReq := &httpclient.Request{
		Method: http.MethodPost,
		URL:    url,
		Headers: map[string]string{
			"Authorization": fmt.Sprintf("Bearer %s", config.AccessToken),
			"Content-Type":  "application/json",
		},
		Body: reqBody,
	}

	resp, err := c.httpClient.Send(ctx, httpReq)
	if err != nil {
		if httpErr, ok := httpclient.IsHTTPError(err); ok {
			c.logger.Errorw("HubSpot API error creating line item",
				"status_code", httpErr.StatusCode,
				"response_body", string(httpErr.Response),
				"url", url)
			return nil, ierr.NewError("failed to create line item in HubSpot").
				WithHint(fmt.Sprintf("HubSpot API returned status %d: %s", httpErr.StatusCode, string(httpErr.Response))).
				Mark(ierr.ErrHTTPClient)
		}

		c.logger.Errorw("http client error creating line item",
			"error", err,
			"url", url)
		return nil, ierr.NewError("failed to create line item in HubSpot").
			WithHint("Check HubSpot API connectivity").
			Mark(ierr.ErrHTTPClient)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		c.logger.Errorw("hubspot create line item error",
			"status", resp.StatusCode,
			"body", string(resp.Body))
		return nil, ierr.NewError("failed to create line item in HubSpot").
			WithHint(fmt.Sprintf("HubSpot API returned status %d", resp.StatusCode)).
			Mark(ierr.ErrHTTPClient)
	}

	var lineItem DealLineItemResponse
	if err := json.Unmarshal(resp.Body, &lineItem); err != nil {
		return nil, ierr.NewError("failed to decode line item response").Mark(ierr.ErrInternal)
	}

	return &lineItem, nil
}
