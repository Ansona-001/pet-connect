package media

import (
	"bytes"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"mime"
	"path/filepath"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/image/webp"

	"petconnect/server/internal/platform/httpx"
	"petconnect/server/internal/platform/storage"
)

const (
	// maxImageUploadBytes caps images well below maxUploadBytes (which
	// exists mainly to accommodate video) — a multi-tens-of-megabyte
	// "image" is never legitimate and only costs decode/re-encode
	// resources (Day 33's job) for no benefit.
	maxImageUploadBytes = 15 * 1024 * 1024
	// maxImageDimension guards the image decode/re-encode pipeline
	// against a decompression-bomb-style image: dimensions are read
	// from the header via image.DecodeConfig, which never allocates a
	// pixel buffer, so this check happens before anything downstream
	// would.
	maxImageDimension = 8000
	// sniffBytes is enough to read every magic-byte signature
	// sniffMatches checks — the longest is WebP's, at its 12th byte.
	sniffBytes = 32
)

type Handler struct {
	store          *storage.Store
	db             *pgxpool.Pool
	publicBaseURL  string
	maxUploadBytes int64
}

func New(store *storage.Store, db *pgxpool.Pool, publicBaseURL string, maxUploadBytes int64) *Handler {
	return &Handler{
		store:          store,
		db:             db,
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
	if mediaType == "image" && fileHeader.Size > maxImageUploadBytes {
		return httpx.Problem(c, fiber.StatusRequestEntityTooLarge, "invalid_file_size", fmt.Sprintf("Images must not exceed %d bytes.", maxImageUploadBytes))
	}

	file, err := fileHeader.Open()
	if err != nil {
		return httpx.Problem(c, fiber.StatusBadRequest, "invalid_file", "The uploaded file could not be opened.")
	}
	defer file.Close()

	// The client-supplied Content-Type header (what acceptedMedia just
	// checked) is entirely spoofable — a request can claim image/jpeg
	// for any bytes at all. sniffMatches reads the file's own leading
	// bytes to confirm they actually belong to the claimed format's
	// container family before anything is trusted further.
	header := make([]byte, sniffBytes)
	n, err := io.ReadFull(file, header)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return httpx.Problem(c, fiber.StatusBadRequest, "invalid_file", "The uploaded file could not be read.")
	}
	header = header[:n]
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return httpx.Problem(c, fiber.StatusBadRequest, "invalid_file", "The uploaded file could not be read.")
	}
	if !sniffMatches(contentType, header) {
		return httpx.Problem(c, fiber.StatusUnsupportedMediaType, "media_content_mismatch", "The file's contents do not match its declared type.")
	}

	var width, height *int
	if mediaType == "image" {
		config, decodeErr := decodeImageConfig(contentType, file)
		if decodeErr != nil {
			return httpx.Problem(c, fiber.StatusUnprocessableEntity, "invalid_image", "The image could not be read.")
		}
		if config.Width > maxImageDimension || config.Height > maxImageDimension {
			return httpx.Problem(c, fiber.StatusUnprocessableEntity, "image_too_large", fmt.Sprintf("Images must not exceed %d pixels in either dimension.", maxImageDimension))
		}
		width, height = &config.Width, &config.Height
		if _, err := file.Seek(0, io.SeekStart); err != nil {
			return httpx.Problem(c, fiber.StatusBadRequest, "invalid_file", "The uploaded file could not be read.")
		}
	}

	key := fmt.Sprintf("uploads/%s/%s%s", userID, uuid.NewString(), extension)
	if err := h.store.Put(c.UserContext(), key, contentType, fileHeader.Size, file); err != nil {
		return httpx.Problem(c, fiber.StatusInternalServerError, "upload_failed", "The media upload failed. Please retry.")
	}

	// A durable record of the upload (migrations/000007_durable_media_
	// schema.sql) — a failure here leaves an orphaned object in storage
	// with no media row, which is exactly what the later orphaned-media
	// cleanup job exists to reconcile, rather than something worth
	// rolling the already-succeeded storage upload back for.
	var mediaID uuid.UUID
	if err := h.db.QueryRow(c.UserContext(), `
		INSERT INTO media (owner_user_id, media_type, storage_path, content_type, byte_size, width, height)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id`,
		userID, mediaType, key, contentType, fileHeader.Size, width, height,
	).Scan(&mediaID); err != nil {
		return httpx.Problem(c, fiber.StatusInternalServerError, "upload_failed", "The media upload failed. Please retry.")
	}

	path := "/media/" + key
	return httpx.Created(c, fiber.Map{
		"id":           mediaID,
		"url":          h.publicBaseURL + path,
		"path":         path,
		"media_type":   mediaType,
		"content_type": contentType,
		"size":         fileHeader.Size,
		"width":        width,
		"height":       height,
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

// sniffMatches confirms header — the file's own leading bytes — actually
// belongs to contentType's container family, rather than trusting the
// client's (spoofable) declared Content-Type outright. audio/mp4 shares
// its ISO-BMFF "ftyp" box with video/mp4, and audio/webm shares its EBML
// signature with video/webm: neither pair can be told apart from a few
// leading bytes without parsing internal track boxes, so those two are
// verified only down to "a valid container of the right family," not
// the specific audio/video subtype claimed.
func sniffMatches(contentType string, header []byte) bool {
	switch contentType {
	case "image/jpeg":
		return bytes.HasPrefix(header, []byte{0xFF, 0xD8, 0xFF})
	case "image/png":
		return bytes.HasPrefix(header, []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A})
	case "image/webp":
		return len(header) >= 12 &&
			bytes.Equal(header[0:4], []byte("RIFF")) &&
			bytes.Equal(header[8:12], []byte("WEBP"))
	case "video/mp4", "audio/mp4":
		return len(header) >= 8 && bytes.Equal(header[4:8], []byte("ftyp"))
	case "video/quicktime":
		if len(header) < 8 {
			return false
		}
		switch string(header[4:8]) {
		case "ftyp", "moov", "free", "mdat", "wide", "skip":
			return true
		default:
			return false
		}
	case "video/webm", "audio/webm":
		return bytes.HasPrefix(header, []byte{0x1A, 0x45, 0xDF, 0xA3})
	case "audio/mpeg":
		if bytes.HasPrefix(header, []byte("ID3")) {
			return true
		}
		return len(header) >= 2 && header[0] == 0xFF && header[1]&0xE0 == 0xE0
	case "audio/ogg":
		return bytes.HasPrefix(header, []byte("OggS"))
	default:
		return false
	}
}

// decodeImageConfig reads only the image header (never a full pixel
// buffer) to recover its declared dimensions, dispatching to the
// decoder registered for contentType — image.DecodeConfig covers
// image/jpeg and image/png via this file's blank imports; WebP has no
// standard-library decoder, hence the golang.org/x/image dependency.
func decodeImageConfig(contentType string, r io.Reader) (image.Config, error) {
	if contentType == "image/webp" {
		return webp.DecodeConfig(r)
	}
	config, _, err := image.DecodeConfig(r)
	return config, err
}
