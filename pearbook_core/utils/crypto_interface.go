package utils

import (
	"encoding/hex"
	"fmt"
	"strings"
)

// CryptoService interface for cryptographic operations
type CryptoService interface {
	// VerifySignature verifies a signature against public key and data
	VerifySignature(publicKeyBytes, data, signature []byte) error

	// CreateOperationData creates canonical data for signing operations
	CreateOperationData(operation string, groupID, userID string, timestamp int64, extraData map[string]interface{}) []byte

	// DecodePublicKey decodes a public key string to bytes, handling mock keys for testing
	DecodePublicKey(publicKeyStr, userID string) ([]byte, error)
}

// RealCryptoService implements CryptoService with actual cryptographic operations
type CryptoServiceImpl struct{}

// NewCryptoService creates a new CryptoService
func NewCryptoService() *CryptoServiceImpl {
	return &CryptoServiceImpl{}
}

// VerifySignature verifies a signature against public key and data
func (r *CryptoServiceImpl) VerifySignature(publicKeyBytes, data, signature []byte) error {
	return VerifySignature(publicKeyBytes, data, signature)
}

// CreateOperationData creates canonical data for signing operations
func (r *CryptoServiceImpl) CreateOperationData(operation string, groupID, userID string, timestamp int64, extraData map[string]interface{}) []byte {
	return CreateOperationData(operation, groupID, userID, timestamp, extraData)
}

// DecodePublicKey decodes a hex-encoded public key to bytes
func (r *CryptoServiceImpl) DecodePublicKey(publicKeyStr, userID string) ([]byte, error) {
	return hex.DecodeString(publicKeyStr)
}

// MockCryptoService implements CryptoService with mock operations for testing
type MockCryptoService struct{}

// NewMockCryptoService creates a new MockCryptoService
func NewMockCryptoService() *MockCryptoService {
	return &MockCryptoService{}
}

// VerifySignature always returns nil (success) for mock signatures
func (m *MockCryptoService) VerifySignature(publicKeyBytes, data, signature []byte) error {
	// For testing purposes, allow mock signatures
	if len(signature) == 14 && string(signature) == "mock-signature" {
		return nil
	}
	// For other signatures, still perform basic validation
	if len(signature) != 64 {
		return fmt.Errorf("invalid signature length")
	}
	return nil
}

// CreateOperationData creates canonical data for signing operations
func (m *MockCryptoService) CreateOperationData(operation string, groupID, userID string, timestamp int64, extraData map[string]interface{}) []byte {
	return CreateOperationData(operation, groupID, userID, timestamp, extraData)
}

// DecodePublicKey decodes a public key string to bytes, allowing mock keys for testing
func (m *MockCryptoService) DecodePublicKey(publicKeyStr, userID string) ([]byte, error) {
	// Allow mock keys for testing
	if strings.HasPrefix(publicKeyStr, "mock-key-") {
		return []byte("mock-public-key-bytes-" + userID), nil
	}
	// For real keys, decode from hex
	return hex.DecodeString(publicKeyStr)
}
