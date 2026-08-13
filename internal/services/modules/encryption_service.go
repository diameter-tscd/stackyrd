package modules

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"time"

	"stackyrd/config"
	"stackyrd/pkg/interfaces"
	"stackyrd/pkg/logger"
	"stackyrd/pkg/registry"
	"stackyrd/pkg/request"
	"stackyrd/pkg/response"

	"github.com/labstack/echo/v4"
	"github.com/samber/oops"
)

type EncryptionService struct {
	enabled       bool
	algorithm     string
	encryptionKey []byte
	keyMu         sync.RWMutex
	lastRotation  int64
	logger        *logger.Logger
}

func NewEncryptionService(enabled bool, cfg map[string]any, log ...*logger.Logger) *EncryptionService {
	algorithm := "aes-256-gcm"
	key := ""

	var l *logger.Logger
	if len(log) > 0 {
		l = log[0]
	}

	if cfg != nil {
		if alg, ok := cfg["algorithm"].(string); ok && alg != "" {
			algorithm = alg
		}
		if k, ok := cfg["key"].(string); ok && k != "" {
			key = k
		}
	}

	// Derive a full-strength 32-byte AES key. Never zero-pad or truncate user
	// input: sha256 stretches any passphrase to 256 bits of key material. If no
	// key is configured, fall back to a fresh random key so ciphertext is not
	// trivially decryptable with a known constant.
	var keyBytes []byte
	if key != "" {
		sum := sha256.Sum256([]byte(key))
		keyBytes = sum[:]
	} else {
		keyBytes = make([]byte, 32)
		if _, err := rand.Read(keyBytes); err != nil {
			panic(fmt.Sprintf("failed to generate encryption key: %v", err))
		}
		if l != nil {
			l.Warn("Encryption service using a random in-memory key — configure encryption.key to persist")
		}
	}

	return &EncryptionService{
		enabled:       enabled,
		algorithm:     algorithm,
		encryptionKey: keyBytes,
		logger:        l,
	}
}

func (s *EncryptionService) Name() string     { return "Encryption Service" }
func (s *EncryptionService) WireName() string { return "encryption-service" }
func (s *EncryptionService) Enabled() bool    { return s.enabled }
func (s *EncryptionService) Get() any { return s }
func (s *EncryptionService) Endpoints() []string {
	return []string{"/encryption/encrypt", "/encryption/decrypt", "/encryption/status", "/encryption/key-rotate"}
}

func (s *EncryptionService) RegisterRoutes(g *echo.Group) {
	sub := g.Group("/encryption")
	sub.POST("/encrypt", s.EncryptData)
	sub.POST("/decrypt", s.DecryptData)
	sub.GET("/status", s.GetStatus)
	sub.POST("/key-rotate", s.RotateKey)
}

type EncryptRequest struct {
	Data        string `json:"data" validate:"required"`
	ContentType string `json:"content_type,omitempty"`
}

type EncryptResponse struct {
	EncryptedData string `json:"encrypted_data"`
	Algorithm     string `json:"algorithm"`
	Timestamp     int64  `json:"timestamp"`
	ContentType   string `json:"content_type,omitempty"`
}

type DecryptRequest struct {
	EncryptedData string `json:"encrypted_data" validate:"required"`
	ContentType   string `json:"content_type,omitempty"`
}

type DecryptResponse struct {
	DecryptedData string `json:"decrypted_data"`
	Algorithm     string `json:"algorithm"`
	Timestamp     int64  `json:"timestamp"`
	ContentType   string `json:"content_type,omitempty"`
}

type StatusResponse struct {
	Enabled      bool   `json:"enabled"`
	Algorithm    string `json:"algorithm"`
	RotateKeys   bool   `json:"rotate_keys"`
	LastRotation int64  `json:"last_rotation"`
}

type KeyRotateRequest struct {
	NewKey string `json:"new_key" validate:"required,min=32,max=128"`
}

func (s *EncryptionService) encrypt(data []byte) (string, error) {
	s.keyMu.RLock()
	key := s.encryptionKey
	s.keyMu.RUnlock()

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", oops.In("encryption-service").Tags("aes", "cipher").Wrapf(err, "failed to create cipher")
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", oops.In("encryption-service").Tags("aes", "gcm").Wrapf(err, "failed to create gcm")
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", oops.In("encryption-service").Tags("aes", "nonce").Wrapf(err, "failed to generate nonce")
	}

	encrypted := gcm.Seal(nonce, nonce, data, nil)
	return base64.StdEncoding.EncodeToString(encrypted), nil
}

func (s *EncryptionService) decrypt(encryptedData string) ([]byte, error) {
	data, err := base64.StdEncoding.DecodeString(encryptedData)
	if err != nil {
		return nil, oops.In("encryption-service").Tags("decode").Wrapf(err, "failed to decode base64")
	}

	s.keyMu.RLock()
	key := s.encryptionKey
	s.keyMu.RUnlock()

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, oops.In("encryption-service").Tags("aes", "cipher").Wrapf(err, "failed to create cipher")
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, oops.In("encryption-service").Tags("aes", "gcm").Wrapf(err, "failed to create gcm")
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, oops.In("encryption-service").Tags("data", "decryption").Code("encrypted_data_too_short").Public("Encrypted data too short").Errorf("encrypted data too short")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	decrypted, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, oops.In("encryption-service").Tags("aes", "gcm").Wrapf(err, "failed to decrypt")
	}

	return decrypted, nil
}

func (s *EncryptionService) EncryptData(c echo.Context) error {
	var req EncryptRequest
	if err := request.Bind(c, &req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	contentType := req.ContentType
	if contentType == "" {
		contentType = "text/plain"
	}

	encrypted, err := s.encrypt([]byte(req.Data))
	if err != nil {
		if s.logger != nil {
			s.logger.Error("Encryption failed", err)
		}
		return response.InternalServerError(c, "Encryption failed")
	}

	resp := EncryptResponse{
		EncryptedData: encrypted,
		Algorithm:     s.algorithm,
		Timestamp:     time.Now().Unix(),
		ContentType:   contentType,
	}

	return response.Success(c, resp, "Data encrypted successfully")
}

func (s *EncryptionService) DecryptData(c echo.Context) error {
	var req DecryptRequest
	if err := request.Bind(c, &req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	contentType := req.ContentType
	if contentType == "" {
		contentType = "text/plain"
	}

	decrypted, err := s.decrypt(req.EncryptedData)
	if err != nil {
		if s.logger != nil {
			s.logger.Error("Decryption failed", err)
		}
		return response.BadRequest(c, "Decryption failed")
	}

	resp := DecryptResponse{
		DecryptedData: string(decrypted),
		Algorithm:     s.algorithm,
		Timestamp:     time.Now().Unix(),
		ContentType:   contentType,
	}

	return response.Success(c, resp, "Data decrypted successfully")
}

func (s *EncryptionService) GetStatus(c echo.Context) error {
	s.keyMu.RLock()
	lastRotation := s.lastRotation
	s.keyMu.RUnlock()

	resp := StatusResponse{
		Enabled:      s.enabled,
		Algorithm:    s.algorithm,
		RotateKeys:   lastRotation != 0,
		LastRotation: lastRotation,
	}

	return response.Success(c, resp, "Encryption service status")
}

func (s *EncryptionService) RotateKey(c echo.Context) error {
	var req KeyRotateRequest
	if err := request.Bind(c, &req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	// Reject weak keys instead of silently padding/truncating them.
	if len(req.NewKey) < 32 {
		return response.BadRequest(c, "New key must be at least 32 characters long")
	}

	sum := sha256.Sum256([]byte(req.NewKey))
	s.keyMu.Lock()
	s.encryptionKey = sum[:]
	s.lastRotation = time.Now().Unix()
	s.keyMu.Unlock()

	return response.Success(c, map[string]string{
		"message": "Encryption key rotated successfully",
	}, "Key rotation successful")
}

func (s *EncryptionService) EncryptJSON(data any) (string, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return "", oops.In("encryption-service").Tags("json").Wrapf(err, "failed to marshal JSON")
	}
	return s.encrypt(jsonData)
}

func (s *EncryptionService) DecryptJSON(encryptedData string, target any) error {
	decrypted, err := s.decrypt(encryptedData)
	if err != nil {
		return oops.In("encryption-service").Tags("json").Wrapf(err, "failed to decrypt")
	}
	return json.Unmarshal(decrypted, target)
}

func init() {
	registry.RegisterService("encryption_service", func(config *config.Config, logger *logger.Logger) interfaces.Service {
		encryptionConfig := map[string]any{
			"algorithm": config.Encryption.Algorithm,
			"key":       config.Encryption.Key,
		}
		return NewEncryptionService(config.Encryption.Enabled, encryptionConfig, logger)
	})
}
