package media

import (
	"fmt"
	"mime"
	"path/filepath"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"petconnect/server/internal/platform/httpx"
	"petconnect/server/internal/platform/storage"
)

type Handler struct {
	store          *storage.Store
	publicBaseURL  string
	maxUploadBytes int64
}

func New(store *storage.Store, publicBaseURL string, maxUploadBytes int64) *Handler {
	return &Handler{
		store:          store,
		publicBaseURL:  strings.TrimRight(publicBaseURL, "/"),
		maxUploadBytes: maxUploadBytes,
	}
}

func (h *Handler) Register(protected fiber.Router, public fiber.Router) {
	protected.Post("/media/uploads", h.upload)
	public.Get("/media/uploads/*", h.download)
}

func (h *Handler) upload(c *fiber.Ctx) error {
	userID, err := httpx.UserID(c)
	if err != nil {
		return httpx.Problem(c, fiber.StatusUnauthorized, "authentication_required", "Authentication is required.")
	}
	fileHeader, err := c.FormFile("file")
	if err != nil {
		return httpx.Problem(c, fiber.StatusBadRequest, "file_required", "A multipart file field named 'file' is required.")
	}
	if fileHeader.Size <= 0 || fileHeader.Size > h.maxUploadBytes {
		return httpx.Problem(c, fiber.StatusRequestEntityTooLarge, "invalid_file_size", fmt.Sprintf("Files must be between 1 byte and %d bytes.", h.maxUploadBytes))
	}

	contentType := strings.ToLower(strings.TrimSpace(fileHeader.Header.Get(fiber.HeaderContentType)))
	extension, mediaType, ok := acceptedMedia(contentType)
	if !ok {
		return httpx.Problem(c, fiber.StatusUnsupportedMediaType, "unsupported_media_type", "Upload a JPEG, PNG, WebP, MP4, WebM, QuickTime video, or common audio file.")
	}

	file, err := fileHeader.Open()
	if err != nil {
		return httpx.Problem(c, fiber.StatusBadRequest, "invalid_file", "The uploaded file could not be opened.")
	}
	defer file.Close()

	key := fmt.Sprintf("uploads/%s/%s%s", userID, uuid.NewString(), extension)
	if err := h.store.Put(c.UserContext(), key, contentType, fileHeader.Size, file); err != nil {
		return httpx.Problem(c, fiber.StatusInternalServerError, "upload_failed", "The media upload failed. Please retry.")
	}
	path := "/media/" + key
	return httpx.Created(c, fiber.Map{
		"url":          h.publicBaseURL + path,
		"path":         path,
		"media_type":   mediaType,
		"content_type": contentType,
		"size":         fileHeader.Size,
	})
}

func (h *Handler) download(c *fiber.Ctx) error {
	key := strings.TrimPrefix(c.Params("*"), "/")
	if key == "" || strings.Contains(key, "..") || filepath.IsAbs(key) {
		return httpx.Problem(c, fiber.StatusBadRequest, "invalid_media_key", "The media key is invalid.")
	}
	object, info, err := h.store.Get(c.UserContext(), "uploads/"+key)
	if err != nil {
		return httpx.Problem(c, fiber.StatusNotFound, "media_not_found", "The media file was not found.")
	}
	c.Set(fiber.HeaderContentType, info.ContentType)
	c.Set(fiber.HeaderCacheControl, "public, max-age=31536000, immutable")
	return c.SendStream(object, int(info.Size))
}

func acceptedMedia(contentType string) (extension string, mediaType string, ok bool) {
	allowed := map[string]string{
		"image/jpeg":      "image",
		"image/png":       "image",
		"image/webp":      "image",
		"video/mp4":       "video",
		"video/webm":      "video",
		"video/quicktime": "video",
		"audio/mpeg":      "audio",
		"audio/mp4":       "audio",
		"audio/webm":      "audio",
		"audio/ogg":       "audio",
	}
	mediaType, ok = allowed[contentType]
	if !ok {
		return "", "", false
	}
	extensions, _ := mime.ExtensionsByType(contentType)
	if len(extensions) == 0 {
		return "", "", false
	}
	return extensions[0], mediaType, true
}
