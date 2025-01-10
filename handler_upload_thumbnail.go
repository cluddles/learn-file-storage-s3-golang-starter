package main

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadThumbnail(w http.ResponseWriter, r *http.Request) {
	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
		return
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't find JWT", err)
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't validate JWT", err)
		return
	}

	fmt.Println("uploading thumbnail for video", videoID, "by user", userID)

	const maxMemory = 10 << 20
	if err := r.ParseMultipartForm(maxMemory); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't parse multipart form", err)
		return
	}

	file, header, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't read form file", err)
		return
	}

	mediaType := header.Header.Get("Content-Type")
	if mediaType == "" {
		respondWithError(w, http.StatusBadRequest, "Invalid Content-Type", err)
		return
	}

	data, err := io.ReadAll(file)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't read bytes", err)
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

	thumbnail := thumbnail{
		data:      data,
		mediaType: mediaType,
	}
	videoThumbnails[videoID] = thumbnail

	url := fmt.Sprintf("http://localhost:%s/api/thumbnails/%s", cfg.port, videoIDString)
	video.ThumbnailURL = &url
	video.UpdatedAt = time.Now()
	if err := cfg.db.UpdateVideo(video); err != nil {
		delete(videoThumbnails, videoID)
		respondWithError(w, http.StatusInternalServerError, "Video update failed", err)
		return
	}

	respondWithJSON(w, http.StatusOK, video)
}
