package s3

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	ierr "github.com/flexprice/flexprice/internal/errors"
)

// ExportFormat represents the export file format
type ExportFormat string

const (
	ExportFormatCSV     ExportFormat = "csv"
	ExportFormatJSON    ExportFormat = "json"
	ExportFormatParquet ExportFormat = "parquet"
)

// ExportRequest represents a request to export data to S3
type ExportRequest struct {
	FileName    string       // File name (without path)
	Data        []byte       // Data to export
	Format      ExportFormat // File format
	EntityType  string       // Entity type being exported (e.g., "feature_usage", "invoice")
	Timestamp   time.Time    // Timestamp for the export
	Compress    bool         // Whether to compress the data
	ContentType string       // Content type (optional, will be inferred if empty)
}

// ExportResponse represents the response after exporting data to S3
type ExportResponse struct {
	FileURL        string    // S3 URL of the uploaded file
	Bucket         string    // S3 bucket name
	Key            string    // S3 object key
	FileSizeBytes  int64     // Size of the file in bytes
	CompressedSize int64     // Size after compression (if compressed)
	UploadedAt     time.Time // Time of upload
}

// UploadFile uploads a file to S3
func (c *s3Client) UploadFile(ctx context.Context, request *ExportRequest) (*ExportResponse, error) {
	if request == nil {
		return nil, ierr.NewError("export request is nil").
			WithHint("Export request is required").
			Mark(ierr.ErrValidation)
	}

	// Validate request
	if err := c.validateExportRequest(request); err != nil {
		return nil, err
	}

	// Generate S3 key
	key := c.generateObjectKey(request)

	// Prepare data
	data := request.Data
	originalSize := int64(len(data))
	compressedSize := originalSize

	// Compress data if requested and compression is enabled
	if request.Compress && c.config.Compression == "gzip" {
		compressedData, err := c.compressData(data)
		if err != nil {
			return nil, err
		}
		data = compressedData
		compressedSize = int64(len(data))
		c.logger.Info("Data compressed",
			"original_size", originalSize,
			"compressed_size", compressedSize,
			"compression_ratio", fmt.Sprintf("%.2f%%", float64(compressedSize)/float64(originalSize)*100),
		)
	}

	// Determine content type
	contentType := request.ContentType
	if contentType == "" {
		contentType = c.getContentType(request.Format, request.Compress)
	}

	// Prepare upload input
	uploadInput := &s3.PutObjectInput{
		Bucket:      aws.String(c.config.Bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String(contentType),
	}

	// Add server-side encryption
	if c.config.Encryption == "AES256" {
		uploadInput.ServerSideEncryption = types.ServerSideEncryptionAes256
	} else if c.config.Encryption == "aws:kms" {
		uploadInput.ServerSideEncryption = types.ServerSideEncryptionAwsKms
	}

	// Upload to S3
	_, err := c.s3Client.PutObject(ctx, uploadInput)
	if err != nil {
		return nil, ierr.WithError(err).
			WithHint("failed to upload file to S3").
			WithMessagef("bucket: %s, key: %s", c.config.Bucket, key).
			Mark(ierr.ErrHTTPClient)
	}

	// Generate file URL
	fileURL := c.generateFileURL(key)

	c.logger.Info("File uploaded to S3 successfully",
		"bucket", c.config.Bucket,
		"key", key,
		"file_size", compressedSize,
		"entity_type", request.EntityType,
	)

	return &ExportResponse{
		FileURL:        fileURL,
		Bucket:         c.config.Bucket,
		Key:            key,
		FileSizeBytes:  originalSize,
		CompressedSize: compressedSize,
		UploadedAt:     time.Now(),
	}, nil
}

// UploadCSV uploads CSV data to S3
func (c *s3Client) UploadCSV(ctx context.Context, fileName string, data []byte, entityType string) (*ExportResponse, error) {
	request := &ExportRequest{
		FileName:   fileName,
		Data:       data,
		Format:     ExportFormatCSV,
		EntityType: entityType,
		Timestamp:  time.Now(),
		Compress:   c.config.Compression == "gzip",
	}
	return c.UploadFile(ctx, request)
}

// UploadJSON uploads JSON data to S3
func (c *s3Client) UploadJSON(ctx context.Context, fileName string, data []byte, entityType string) (*ExportResponse, error) {
	request := &ExportRequest{
		FileName:   fileName,
		Data:       data,
		Format:     ExportFormatJSON,
		EntityType: entityType,
		Timestamp:  time.Now(),
		Compress:   c.config.Compression == "gzip",
	}
	return c.UploadFile(ctx, request)
}

// validateExportRequest validates the export request
func (c *s3Client) validateExportRequest(request *ExportRequest) error {
	if request.FileName == "" {
		return ierr.NewError("file name is required").
			WithHint("File name must be provided").
			Mark(ierr.ErrValidation)
	}
	if len(request.Data) == 0 {
		return ierr.NewError("data is required").
			WithHint("Data must be provided").
			Mark(ierr.ErrValidation)
	}
	if request.EntityType == "" {
		return ierr.NewError("entity type is required").
			WithHint("Entity type must be provided").
			Mark(ierr.ErrValidation)
	}

	// Check max file size
	maxSizeBytes := int64(c.config.MaxFileSizeMB * 1024 * 1024)
	if int64(len(request.Data)) > maxSizeBytes {
		return ierr.NewErrorf("file size exceeds maximum allowed size of %d MB", c.config.MaxFileSizeMB).
			WithHintf("Reduce the file size or increase the max_file_size_mb limit").
			Mark(ierr.ErrValidation)
	}

	return nil
}

// generateObjectKey generates the S3 object key for the export
func (c *s3Client) generateObjectKey(request *ExportRequest) string {
	// Format: {key_prefix}/{entity_type}/{filename}.{extension}
	extension := string(request.Format)
	if request.Compress && c.config.Compression == "gzip" {
		extension = extension + ".gz"
	}

	fileName := request.FileName
	if fileName == "" {
		// Fallback if filename not provided (shouldn't happen normally)
		timestamp := request.Timestamp.Format("20060102_150405")
		fileName = fmt.Sprintf("%s_%s", request.EntityType, timestamp)
	}

	key := fmt.Sprintf("%s/%s/%s.%s",
		c.config.KeyPrefix,
		request.EntityType,
		fileName,
		extension,
	)

	// Remove leading slash if present
	if key[0] == '/' {
		key = key[1:]
	}

	return key
}

// generateFileURL generates the file URL
func (c *s3Client) generateFileURL(key string) string {
	if c.config.EndpointURL != "" {
		return fmt.Sprintf("%s/%s/%s", c.config.EndpointURL, c.config.Bucket, key)
	}
	return fmt.Sprintf("s3://%s/%s", c.config.Bucket, key)
}

// compressData compresses data using gzip
func (c *s3Client) compressData(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	writer := gzip.NewWriter(&buf)

	_, err := writer.Write(data)
	if err != nil {
		return nil, ierr.WithError(err).
			WithHint("failed to compress data").
			Mark(ierr.ErrSystem)
	}

	if err := writer.Close(); err != nil {
		return nil, ierr.WithError(err).
			WithHint("failed to close gzip writer").
			Mark(ierr.ErrSystem)
	}

	return buf.Bytes(), nil
}

// getContentType returns the content type for the given format
func (c *s3Client) getContentType(format ExportFormat, compressed bool) string {
	if compressed {
		return "application/gzip"
	}

	switch format {
	case ExportFormatCSV:
		return "text/csv"
	case ExportFormatJSON:
		return "application/json"
	case ExportFormatParquet:
		return "application/octet-stream"
	default:
		return "application/octet-stream"
	}
}
