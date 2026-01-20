package kms

import (
	"context"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/aliyun/alibaba-cloud-sdk-go/services/kms"
)

// KMSClient 阿里云KMS客户端
type KMSClient struct {
	client    *kms.Client
	masterKeyID string
	regionID    string
}

// Config KMS配置
type Config struct {
	RegionID       string
	AccessKeyID     string
	AccessKeySecret string
	MasterKeyID     string
}

// NewClient 创建KMS客户端
func NewClient(cfg *Config) (*KMSClient, error) {
	client, err := kms.NewClientWithAccessKey(
		cfg.RegionID,
		cfg.AccessKeyID,
		cfg.AccessKeySecret,
	)
	if err != nil {
		return nil, fmt.Errorf("create KMS client failed: %w", err)
	}

	return &KMSClient{
		client:     client,
		masterKeyID: cfg.MasterKeyID,
		regionID:   cfg.RegionID,
	}, nil
}

// Encrypt 加密数据
func (c *KMSClient) Encrypt(plaintext []byte) ([]byte, error) {
	request := kms.CreateEncryptRequest()
	request.Scheme = "https"
	request.KeyId = c.masterKeyID
	request.Plaintext = base64.StdEncoding.EncodeToString(plaintext)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	response, err := c.client.EncertWithContext(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("KMS encrypt failed: %w", err)
	}

	if response == nil || response.CiphertextBlob == "" {
		return nil, fmt.Errorf("empty response from KMS")
	}

	ciphertext, err := base64.StdEncoding.DecodeString(response.CiphertextBlob)
	if err != nil {
		return nil, fmt.Errorf("decode ciphertext failed: %w", err)
	}

	return ciphertext, nil
}

// Decrypt 解密数据
func (c *KMSClient) Decrypt(hexCiphertext string) ([]byte, error) {
	ciphertext, err := hexDecode(hexCiphertext)
	if err != nil {
		return nil, fmt.Errorf("decode hex failed: %w", err)
	}

	request := kms.CreateDecryptRequest()
	request.Scheme = "https"
	request.CiphertextBlob = base64.StdEncoding.EncodeToString(ciphertext)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	response, err := c.client.DecryptWithContext(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("KMS decrypt failed: %w", err)
	}

	if response == nil || response.Plaintext == "" {
		return nil, fmt.Errorf("empty response from KMS")
	}

	plaintext, err := base64.StdEncoding.DecodeString(response.Plaintext)
	if err != nil {
		return nil, fmt.Errorf("decode plaintext failed: %w", err)
	}

	return plaintext, nil
}

// GenerateDataKey 生成数据密钥
func (c *KMSClient) GenerateDataKey() (plaintext, ciphertext []byte, err error) {
	request := kms.CreateGenerateDataKeyRequest()
	request.Scheme = "https"
	request.KeyId = c.masterKeyKeyID

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	response, err := c.client.GenerateDataKeyWithContext(ctx, request)
	if err != nil {
		return nil, nil, fmt.Errorf("generate data key failed: %w", err)
	}

	plaintextDEK, err := base64.StdEncoding.DecodeString(response.Plaintext)
	if err != nil {
		return nil, nil, err
	}

	ciphertextDEK, err := base64.StdEncoding.DecodeString(response.CiphertextBlob)
	if err != nil {
		return nil, nil, err
	}

	return plaintextDEK, ciphertextDEK, nil
}

// hexDecode 十六进制解码
func hexDecode(s string) ([]byte, error) {
	if len(s)%2 != 0 {
		s = "0" + s
	}

	b := make([]byte, len(s)/2)
	for i := 0; i < len(b); i++ {
		_, err := fmt.Sscanf(s[i*2:i*2+2], "%02x", &b[i])
		if err != nil {
			return nil, err
		}
	}
	return b, nil
}

func (c *KMSClient) masterKeyKeyID() string {
	return c.masterKeyID
}
