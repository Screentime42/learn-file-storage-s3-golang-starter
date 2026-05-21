package main

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"

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

	// Fetch video meta data
	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "Video not found", err)
		return
	}

	// Ensure authed user
	if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "You do not own this video", err)
		return
	}

	fmt.Println("uploading thumbnail for video", videoID, "by user", userID)

	// Parse form data 
	const maxMemory = 10 << 20
	err = r.ParseMultipartForm(maxMemory)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't parse multipart form", err)
		return
	}

	file, header, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse form file", err)
		return
	}
	defer file.Close()

	ct := header.Header.Get("Content-Type")
	mediaType, _, err := mime.ParseMediaType(ct)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid content type", err)
		return
	}
	if mediaType != "image/jpeg" && mediaType != "image/png" {
		respondWithError(w, http.StatusUnsupportedMediaType, "Invalid file type", err)
		return
	}

	// Get file extension
	exts, _ := mime.ExtensionsByType(mediaType)
	if len(exts) == 0 {
		respondWithError(w, http.StatusBadRequest, "Unsupported file type", err)
		return
	}

	ext := exts[0]

	// Build file path
	fileName := videoID.String() + ext
	filePath := filepath.Join(cfg.assetsRoot, fileName)

	// Save file to disk
	dst, err := os.Create(filePath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to save file", err)
		return
	}
	defer dst.Close()

	// Copy uploaded bytes to new file
	_, err = io.Copy(dst,file)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to save file", err)
		return
	}

	// Update metadata
	thumbnailURL := "http://localhost:" + cfg.port + "/assets/" + fileName
	video.ThumbnailURL = &thumbnailURL

	// Save to db
	err = cfg.db.UpdateVideo(video)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to update video", err)
		return
	}
	
	
	respondWithJSON(w, http.StatusOK, video)
}
