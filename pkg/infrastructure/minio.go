package infrastructure

import (
	"context"
	"io"
	"stackyrd/config"
	"stackyrd/pkg/logger"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type MinIOManager struct {
	Client     *minio.Client
	BucketName string
	Connected  bool
	Pool       *WorkerPool // Async worker pool
}

// Name returns the display name of the component
func (m *MinIOManager) Name() string {
	return "MinIO"
}

func NewMinIOManager(cfg config.MinIOConfig) (*MinIOManager, error) {
	if !cfg.Enabled || cfg.Endpoint == "" {
		return &MinIOManager{Connected: false}, nil
	}

	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return &MinIOManager{Connected: false}, err
	}

	// Basic check
	_, err = client.ListBuckets(context.Background())
	if err != nil {
		return &MinIOManager{Connected: false}, err
	}

	// Initialize worker pool for async operations
	pool := NewWorkerPool(8) // Moderate pool for file operations
	pool.Start()

	return &MinIOManager{
		Client:     client,
		BucketName: cfg.BucketName,
		Connected:  true,
		Pool:       pool,
	}, nil
}

func (m *MinIOManager) GetStatus() map[string]interface{} {
	if m == nil || !m.Connected {
		return map[string]interface{}{
			"connected": false,
			"error":     "Not configured or connection failed",
		}
	}

	// Get bucket usage (approximate via listing, simplified for now)
	// In production, you might use Prometheus or MinIO admin API, but for simple stats:
	ctx := context.Background()
	exists, err := m.Client.BucketExists(ctx, m.BucketName)
	if err != nil || !exists {
		return map[string]interface{}{
			"connected":   true,
			"bucket_name": m.BucketName,
			"status":      "Bucket not found",
		}
	}

	// Count objects (up to 1000 for quick check)
	objectCh := m.Client.ListObjects(ctx, m.BucketName, minio.ListObjectsOptions{
		Recursive: true,
	})

	count := 0
	var size int64 = 0
	for obj := range objectCh {
		if obj.Err == nil {
			count++
			size += obj.Size
		}
		if count >= 1000 {
			break // Limit for performance
		}
	}

	return map[string]interface{}{
		"connected":     true,
		"bucket_name":   m.BucketName,
		"object_count":  count,
		"total_size_kb": size / 1024,
		"status":        "Healthy",
		"endpoint":      m.Client.EndpointURL().String(),
	}
}

// Async MinIO Operations

// UploadFileAsync asynchronously uploads a file to MinIO.
func (m *MinIOManager) UploadFileAsync(ctx context.Context, objectName string, reader io.Reader, objectSize int64, contentType string) *AsyncResult[minio.UploadInfo] {
	return ExecuteAsync(ctx, func(ctx context.Context) (minio.UploadInfo, error) {
		return m.Client.PutObject(ctx, m.BucketName, objectName, reader, objectSize, minio.PutObjectOptions{
			ContentType: contentType,
		})
	})
}

// GetObjectAsync asynchronously retrieves an object from MinIO.
func (m *MinIOManager) GetObjectAsync(ctx context.Context, objectName string) *AsyncResult[*minio.Object] {
	return ExecuteAsync(ctx, func(ctx context.Context) (*minio.Object, error) {
		return m.Client.GetObject(ctx, m.BucketName, objectName, minio.GetObjectOptions{})
	})
}

// DeleteObjectAsync asynchronously deletes an object from MinIO.
func (m *MinIOManager) DeleteObjectAsync(ctx context.Context, objectName string) *AsyncResult[struct{}] {
	return ExecuteAsync(ctx, func(ctx context.Context) (struct{}, error) {
		err := m.Client.RemoveObject(ctx, m.BucketName, objectName, minio.RemoveObjectOptions{})
		return struct{}{}, err
	})
}

// ListObjectsAsync asynchronously lists objects in the bucket.
func (m *MinIOManager) ListObjectsAsync(ctx context.Context, prefix string, recursive bool) *AsyncResult[[]minio.ObjectInfo] {
	return ExecuteAsync(ctx, func(ctx context.Context) ([]minio.ObjectInfo, error) {
		var objects []minio.ObjectInfo

		objectCh := m.Client.ListObjects(ctx, m.BucketName, minio.ListObjectsOptions{
			Prefix:    prefix,
			Recursive: recursive,
		})

		for object := range objectCh {
			if object.Err != nil {
				return nil, object.Err
			}
			objects = append(objects, object)
		}

		return objects, nil
	})
}

// GetObjectInfoAsync asynchronously gets object information.
func (m *MinIOManager) GetObjectInfoAsync(ctx context.Context, objectName string) *AsyncResult[minio.ObjectInfo] {
	return ExecuteAsync(ctx, func(ctx context.Context) (minio.ObjectInfo, error) {
		return m.Client.StatObject(ctx, m.BucketName, objectName, minio.StatObjectOptions{})
	})
}

// Batch Operations

// UploadBatchAsync asynchronously uploads multiple files.
func (m *MinIOManager) UploadBatchAsync(ctx context.Context, uploads []struct {
	ObjectName  string
	Reader      io.Reader
	ObjectSize  int64
	ContentType string
}) *BatchAsyncResult[minio.UploadInfo] {
	operations := make([]AsyncOperation[minio.UploadInfo], len(uploads))

	for i, upload := range uploads {
		upload := upload // Capture loop variable
		operations[i] = func(ctx context.Context) (minio.UploadInfo, error) {
			return m.Client.PutObject(ctx, m.BucketName, upload.ObjectName, upload.Reader, upload.ObjectSize, minio.PutObjectOptions{
				ContentType: upload.ContentType,
			})
		}
	}

	return ExecuteBatchAsync(ctx, operations)
}

// DeleteBatchAsync asynchronously deletes multiple objects.
func (m *MinIOManager) DeleteBatchAsync(ctx context.Context, objectNames []string) *BatchAsyncResult[struct{}] {
	operations := make([]AsyncOperation[struct{}], len(objectNames))

	for i, objectName := range objectNames {
		objectName := objectName // Capture loop variable
		operations[i] = func(ctx context.Context) (struct{}, error) {
			err := m.Client.RemoveObject(ctx, m.BucketName, objectName, minio.RemoveObjectOptions{})
			return struct{}{}, err
		}
	}

	return ExecuteBatchAsync(ctx, operations)
}

// Sync Methods (for backward compatibility)

// UploadFile uploads a file synchronously (existing method for compatibility).
func (m *MinIOManager) UploadFile(ctx context.Context, objectName string, reader io.Reader, objectSize int64, contentType string) (minio.UploadInfo, error) {
	return m.Client.PutObject(ctx, m.BucketName, objectName, reader, objectSize, minio.PutObjectOptions{
		ContentType: contentType,
	})
}

// GetFileUrl generates a presigned URL for the object.
func (m *MinIOManager) GetFileUrl(objectName string) string {
	// Generate a presigned URL (expires in 7 days)
	url, err := m.Client.PresignedGetObject(context.Background(), m.BucketName, objectName, 7*24*time.Hour, nil)
	if err != nil {
		return ""
	}
	return url.String()
}

// Worker Pool Operations

// SubmitAsyncJob submits an async job to the worker pool.
func (m *MinIOManager) SubmitAsyncJob(job func()) {
	if m.Pool != nil {
		m.Pool.Submit(job)
	} else {
		// Fallback to direct execution if pool not available
		go job()
	}
}

// Close closes the MinIO manager and its worker pool.
func (m *MinIOManager) Close() error {
	if m.Pool != nil {
		m.Pool.Close()
	}
	return nil
}

func init() {
	RegisterComponent("minio", func(cfg *config.Config, l *logger.Logger) (InfrastructureComponent, error) {
		if !cfg.MinIO.Enabled {
			return nil, nil
		}
		return NewMinIOManager(cfg.MinIO)
	})
}
