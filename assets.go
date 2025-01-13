package main

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
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
	return fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s",
		cfg.s3Bucket,
		cfg.s3Region,
		key)
}

func getVideoAspectRatio(filePath string) (string, error) {
	cmd := exec.Command("ffprobe", "-v", "error", "-print_format", "json", "-show_streams", filePath)
	var buffer bytes.Buffer
	cmd.Stdout = &buffer
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("couldn't run ffprobe: %s", err)
	}
	var data map[string]interface{}
	if err := json.Unmarshal(buffer.Bytes(), &data); err != nil {
		return "", fmt.Errorf("error unmarshalling ffprobe output: %s", err)
	}

	// The exercise suggests getting the width, height and using math. Fiddly.
	// There's also a "display_aspect_ratio" field - we could just use that
	streams := data["streams"].([]interface{})
	firstStream := streams[0].(map[string]interface{})
	aspectRatio := firstStream["display_aspect_ratio"]
	if aspectRatio == "16:9" || aspectRatio == "9:16" {
		return aspectRatio.(string), nil
	}
	return "other", nil
}
