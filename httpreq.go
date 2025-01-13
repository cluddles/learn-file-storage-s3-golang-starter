package main

import (
	"fmt"
	"mime/multipart"
	"net/http"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

// Gets JWT token from header and validates, returning userID (or error)
func (cfg *apiConfig) authenticate(r *http.Request) (uuid.UUID, error) {
	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		return uuid.Nil, fmt.Errorf("couldn't find JWT: %s", err)
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		return uuid.Nil, fmt.Errorf("couldn't validate JWT: %s", err)
	}

	return userID, nil
}

func readMultipartForm(r *http.Request, name string, sizeLimit int64) (multipart.File, *multipart.FileHeader, error) {
	if err := r.ParseMultipartForm(sizeLimit); err != nil {
		return nil, nil, fmt.Errorf("couldn't parse multipart form: %s", err)
	}

	file, header, err := r.FormFile(name)
	if err != nil {
		return nil, nil, fmt.Errorf("couldn't read multipart form file: %s", err)
	}
	return file, header, nil

}
