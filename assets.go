package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
)

func (cfg *apiConfig) ensureAssetsDir() error {
	if _, err := os.Stat(cfg.assetsRoot); os.IsNotExist(err) {
		return os.Mkdir(cfg.assetsRoot, 0755)
	}
	return nil
}

// Generate 32 bytes of randomness and convert to "unique" base64 key string
func generateAssetKey() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}

func (cfg *apiConfig) keyToS3URL(key string) string {
	return fmt.Sprintf("https://%s/%s",
		cfg.s3CfDistribution,
		key)
}
