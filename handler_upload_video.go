package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
		return
	}

	userID, err := cfg.authenticate(r)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't authenticate", err)
		return
	}

	fmt.Println("uploading video", videoID, "by user", userID)

	// Limit to 1GB
	formFile, header, err := readMultipartForm(r, "video", 10<<30)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't parse multipart form", err)
		return
	}
	defer formFile.Close()

	mediaType, _, err := mime.ParseMediaType(header.Header.Get("Content-Type"))
	if err != nil || mediaType != "video/mp4" {
		respondWithError(w, http.StatusBadRequest, "Invalid Content-Type", err)
		return
	}

	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't read metadata", err)
		return
	}

	if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "That's not your video", nil)
		return
	}

	tmpFile, err := os.CreateTemp("", "tubley-temp-upload.mp4")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't create temporary file", err)
		return
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()
	if _, err := io.Copy(tmpFile, formFile); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't write temporary file", err)
		return
	}

	// Determine aspect ratio and convert to folder prefix
	aspect, err := getVideoAspectRatio(tmpFile.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't get aspect ratio", err)
		return
	}
	var prefix string
	switch aspect {
	case "16:9":
		prefix = "landscape"
	case "9:16":
		prefix = "portrait"
	default:
		prefix = "other"
	}

	log.Print("Processing for fast start\n")
	optimisedPath, err := processVideoForFastStart(tmpFile.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't process video for fast start", err)
		return
	}
	defer os.Remove(optimisedPath)

	log.Printf("Open optimised video: %s\n", optimisedPath)
	optimisedFile, err := os.Open(optimisedPath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't open optimised file", err)
		return
	}
	defer optimisedFile.Close()

	key, err := generateAssetKey()
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to generate filename", err)
		return
	}

	key = fmt.Sprintf("%s/%s", prefix, key)

	log.Printf("Uploading %s with key %s\n", optimisedFile.Name(), key)
	pui := s3.PutObjectInput{
		Bucket:      aws.String(cfg.s3Bucket),
		Key:         aws.String(key),
		Body:        optimisedFile,
		ContentType: aws.String(mediaType),
	}
	_, err = cfg.s3Client.PutObject(context.TODO(), &pui)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error uploading", err)
		return
	}

	url := cfg.keyToS3URL(key)
	video.VideoURL = &url
	video.UpdatedAt = time.Now()
	if err := cfg.db.UpdateVideo(video); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Video update failed", err)
		return
	}

	log.Printf("Upload %s done\n", key)
	respondWithJSON(w, http.StatusOK, video)
}

// Get video aspect ratio string (16:9, 9:16 or other)
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

// Optimises mp4 by relocating moov atom to start of file
func processVideoForFastStart(filePath string) (string, error) {
	outPath := filePath + ".processing"
	cmd := exec.Command("ffmpeg", "-i", filePath, "-c", "copy", "-movflags", "faststart", "-f", "mp4", outPath)
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("couldn't run ffmpeg: %s", err)
	}
	return outPath, nil
}
