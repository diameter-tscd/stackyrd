package modules

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"stackyrd/config"
	"stackyrd/pkg/interfaces"
	"stackyrd/pkg/logger"
	"stackyrd/pkg/registry"
	"stackyrd/pkg/request"
	"stackyrd/pkg/response"

	"github.com/gin-gonic/gin"
)

type EncryptionService struct {
	enabled       bool
	algorithm     string
	encryptionKey []byte
}

func NewEncryptionService(enabled bool, cfg map[string]interface{}) *EncryptionService {
	algorithm := "aes-256-gcm"
	key := ""

	if cfg != nil {
		if alg, ok := cfg["algorithm"].(string); ok && alg != "" {
			algorithm = alg
		}
		if k, ok := cfg["key"].(string); ok && k != "" {
			key = k
		}
	}

	keyBytes := []byte(key)
	if len(keyBytes) < 32 {
		paddedKey := make([]byte, 32)
		copy(paddedKey, keyBytes)
		keyBytes = paddedKey
	} else if len(keyBytes) > 32 {
		keyBytes = keyBytes[:32]
	}

	return &EncryptionService{
		enabled:       enabled,
		algorithm:     algorithm,
		encryptionKey: keyBytes,
	}
}

func (s *EncryptionService) Name() string     { return "Encryption Service" }
func (s *EncryptionService) WireName() string { return "encryption-service" }
func (s *EncryptionService) Enabled() bool    { return s.enabled }
func (s *EncryptionService) Get() interface{} { return s }
func (s *EncryptionService) Endpoints() []string {
	return []string{"/encryption/encrypt", "/encryption/decrypt", "/encryption/status", "/encryption/key-rotate"}
}

func (s *EncryptionService) RegisterRoutes(g *gin.RouterGroup) {
	sub := g.Group("/encryption")
	sub.POST("/encrypt", s.EncryptData)
	sub.POST("/decrypt", s.DecryptData)
	sub.GET("/status", s.GetStatus)
	sub.POST("/key-rotate", s.RotateKey)
}

// Request/Response structs
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
	CurrentKey   string `json:"current_key"`
	KeyLength    int    `json:"key_length"`
	RotateKeys   bool   `json:"rotate_keys"`
	LastRotation int64  `json:"last_rotation"`
}

type KeyRotateRequest struct {
	NewKey string `json:"new_key" validate:"required,min=16,max=64"`
}

// Encryption/Decryption functions
func (s *EncryptionService) encrypt(data []byte) (string, error) {
	block, err := aes.NewCipher(s.encryptionKey)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %v", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %v", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %v", err)
	}

	encrypted := gcm.Seal(nonce, nonce, data, nil)
	return base64.StdEncoding.EncodeToString(encrypted), nil
}

func (s *EncryptionService) decrypt(encryptedData string) ([]byte, error) {
	data, err := base64.StdEncoding.DecodeString(encryptedData)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64: %v", err)
	}

	block, err := aes.NewCipher(s.encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %v", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %v", err)
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, errors.New("encrypted data too short")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	decrypted, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt: %v", err)
	}

	return decrypted, nil
}

// Handlers
func (s *EncryptionService) EncryptData(c *gin.Context) {
	var req EncryptRequest
	if err := request.Bind(c, &req); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}

	contentType := req.ContentType
	if contentType == "" {
		contentType = "text/plain"
	}

	encrypted, err := s.encrypt([]byte(req.Data))
	if err != nil {
		response.InternalServerError(c, fmt.Sprintf("Encryption failed: %v", err))
		return
	}

	resp := EncryptResponse{
		EncryptedData: encrypted,
		Algorithm:     s.algorithm,
		Timestamp:     time.Now().Unix(),
		ContentType:   contentType,
	}

	response.Success(c, resp, "Data encrypted successfully")
}

func (s *EncryptionService) DecryptData(c *gin.Context) {
	var req DecryptRequest
	if err := request.Bind(c, &req); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}

	contentType := req.ContentType
	if contentType == "" {
		contentType = "text/plain"
	}

	decrypted, err := s.decrypt(req.EncryptedData)
	if err != nil {
		response.BadRequest(c, fmt.Sprintf("Decryption failed: %v", err))
		return
	}

	resp := DecryptResponse{
		DecryptedData: string(decrypted),
		Algorithm:     s.algorithm,
		Timestamp:     time.Now().Unix(),
		ContentType:   contentType,
	}

	response.Success(c, resp, "Data decrypted successfully")
}

func (s *EncryptionService) GetStatus(c *gin.Context) {
	currentKeyPreview := fmt.Sprintf("%s...", hex.EncodeToString(s.encryptionKey[:4]))

	resp := StatusResponse{
		Enabled:      s.enabled,
		Algorithm:    s.algorithm,
		CurrentKey:   currentKeyPreview,
		KeyLength:    len(s.encryptionKey),
		RotateKeys:   false,
		LastRotation: time.Now().Unix(),
	}

	response.Success(c, resp, "Encryption service status")
}

func (s *EncryptionService) RotateKey(c *gin.Context) {
	var req KeyRotateRequest
	if err := request.Bind(c, &req); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}

	if len(req.NewKey) < 16 {
		response.BadRequest(c, "New key must be at least 16 characters long")
		return
	}

	newKeyBytes := []byte(req.NewKey)
	if len(newKeyBytes) < 32 {
		paddedKey := make([]byte, 32)
		copy(paddedKey, newKeyBytes)
		s.encryptionKey = paddedKey
	} else if len(newKeyBytes) > 32 {
		s.encryptionKey = newKeyBytes[:32]
	} else {
		s.encryptionKey = newKeyBytes
	}

	if strings.Contains(req.NewKey, "-") {
		s.algorithm = "aes-256-gcm-custom"
	}

	response.Success(c, map[string]string{
		"message":         "Encryption key rotated successfully",
		"new_key_preview": fmt.Sprintf("%s...", hex.EncodeToString(s.encryptionKey[:4])),
	}, "Key rotation successful")
}

// Helper functions
func (s *EncryptionService) EncryptJSON(data interface{}) (string, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("failed to marshal JSON: %v", err)
	}
	return s.encrypt(jsonData)
}

func (s *EncryptionService) DecryptJSON(encryptedData string, target interface{}) error {
	decrypted, err := s.decrypt(encryptedData)
	if err != nil {
		return fmt.Errorf("failed to decrypt: %v", err)
	}
	return json.Unmarshal(decrypted, target)
}

// Auto-registration function
func init() {
	registry.RegisterService("encryption_service", func(config *config.Config, logger *logger.Logger, deps *registry.Dependencies) interfaces.Service {
		encryptionConfig := map[string]interface{}{
			"algorithm": config.Encryption.Algorithm,
			"key":       config.Encryption.Key,
		}
		return NewEncryptionService(config.Encryption.Enabled, encryptionConfig)
	})
}
