package main

import (
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
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

	if _, err := tmpFile.Seek(0, io.SeekStart); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't reset temporary file pointer", err)
		return
	}

	key, err := generateAssetKey()
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to generate filename", err)
		return
	}
	pui := s3.PutObjectInput{
		Bucket:      aws.String(cfg.s3Bucket),
		Key:         aws.String(key),
		Body:        tmpFile,
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

	respondWithJSON(w, http.StatusOK, video)
}
