package main

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadThumbnail(w http.ResponseWriter, r *http.Request) {
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

	fmt.Println("uploading thumbnail for video", videoID, "by user", userID)

	formFile, header, err := readMultipartForm(r, "thumbnail", 10<<20)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't parse multipart form", err)
		return
	}
	defer formFile.Close()

	mediaType, _, err := mime.ParseMediaType(header.Header.Get("Content-Type"))
	if err != nil || (mediaType != "image/jpeg" && mediaType != "image/png") {
		respondWithError(w, http.StatusBadRequest, "Invalid Content-Type", err)
		return
	}
	ext := strings.Split(mediaType, "/")[1]

	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't read metadata", err)
		return
	}

	if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "That's not your video", nil)
		return
	}

	key, err := generateAssetKey()
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to generate filename", err)
		return
	}

	filename := fmt.Sprintf("%s.%s", key, ext)
	path := filepath.Join(cfg.assetsRoot, filename)
	outFile, err := os.Create(path)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Create video file failed", err)
		return
	}
	defer outFile.Close()
	if _, err := io.Copy(outFile, formFile); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Video save failed", err)
		return
	}

	url := fmt.Sprintf("http://localhost:%s/assets/%s", cfg.port, filename)
	video.ThumbnailURL = &url
	video.UpdatedAt = time.Now()
	if err := cfg.db.UpdateVideo(video); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Video update failed", err)
		return
	}

	respondWithJSON(w, http.StatusOK, video)
}
