package chat

import (
	"errors"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"petconnect/server/internal/platform/httpx"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// Register mounts chat routes on an already authenticated /v1 router.
func (h *Handler) Register(router fiber.Router) {
	router.Get("/chats", h.listChats)
	router.Get("/chats/:chatId/messages", h.listMessages)
	router.Post("/chats/:chatId/messages", h.sendMessage)
	router.Put("/chats/:chatId/read", h.markRead)
}

func RegisterRoutes(router fiber.Router, service *Service) {
	NewHandler(service).Register(router)
}

func (h *Handler) listChats(c *fiber.Ctx) error {
	userID, err := httpx.UserID(c)
	if err != nil {
		return httpx.Problem(c, fiber.StatusUnauthorized, "authentication_required", "Authentication is required.")
	}
	limit, err := parseLimit(c.Query("limit"), 50)
	if err != nil {
		return httpx.Problem(c, fiber.StatusBadRequest, "invalid_limit", "The result limit is invalid.")
	}
	items, err := h.service.ListChats(c.UserContext(), userID, limit)
	if err != nil {
		return chatProblem(c, err)
	}
	return httpx.OK(c, fiber.Map{"items": items})
}

func (h *Handler) listMessages(c *fiber.Ctx) error {
	userID, chatID, ok := chatIdentity(c)
	if !ok {
		return nil
	}
	limit, err := parseLimit(c.Query("limit"), 50)
	if err != nil {
		return httpx.Problem(c, fiber.StatusBadRequest, "invalid_limit", "The result limit is invalid.")
	}
	page, err := h.service.ListMessages(c.UserContext(), userID, chatID, c.Query("cursor"), limit)
	if err != nil {
		return chatProblem(c, err)
	}
	return httpx.OK(c, page)
}

type sendPayload struct {
	SenderPetID     string `json:"sender_pet_id"`
	ClientMessageID string `json:"client_message_id"`
	MessageType     string `json:"message_type"`
	Body            string `json:"body"`
	MediaURL        string `json:"media_url"`
}

func (h *Handler) sendMessage(c *fiber.Ctx) error {
	userID, chatID, ok := chatIdentity(c)
	if !ok {
		return nil
	}
	var payload sendPayload
	if err := c.BodyParser(&payload); err != nil {
		return httpx.Problem(c, fiber.StatusBadRequest, "invalid_json", "The request body is invalid.")
	}
	clientMessageID, err := uuid.Parse(strings.TrimSpace(payload.ClientMessageID))
	if err != nil {
		return httpx.Problem(c, fiber.StatusBadRequest, "invalid_client_message_id", "client_message_id must be a UUID.")
	}
	var senderPetID *uuid.UUID
	if strings.TrimSpace(payload.SenderPetID) != "" {
		id, err := uuid.Parse(strings.TrimSpace(payload.SenderPetID))
		if err != nil {
			return httpx.Problem(c, fiber.StatusBadRequest, "invalid_sender_pet_id", "sender_pet_id must be a UUID.")
		}
		senderPetID = &id
	}
	result, err := h.service.SendMessage(c.UserContext(), userID, chatID, SendMessageRequest{
		SenderPetID: senderPetID, ClientMessageID: clientMessageID,
		MessageType: payload.MessageType, Body: payload.Body, MediaURL: payload.MediaURL,
	})
	if err != nil {
		return chatProblem(c, err)
	}
	if result.Created {
		return httpx.Created(c, result.Message)
	}
	return httpx.OK(c, result.Message)
}

type readPayload struct {
	MessageID string `json:"message_id"`
}

func (h *Handler) markRead(c *fiber.Ctx) error {
	userID, chatID, ok := chatIdentity(c)
	if !ok {
		return nil
	}
	var payload readPayload
	if err := c.BodyParser(&payload); err != nil {
		return httpx.Problem(c, fiber.StatusBadRequest, "invalid_json", "The request body is invalid.")
	}
	messageID, err := uuid.Parse(strings.TrimSpace(payload.MessageID))
	if err != nil {
		return httpx.Problem(c, fiber.StatusBadRequest, "invalid_message_id", "message_id must be a UUID.")
	}
	cursor, err := h.service.MarkRead(c.UserContext(), userID, chatID, messageID)
	if err != nil {
		return chatProblem(c, err)
	}
	return httpx.OK(c, cursor)
}

func chatIdentity(c *fiber.Ctx) (uuid.UUID, uuid.UUID, bool) {
	userID, err := httpx.UserID(c)
	if err != nil {
		_ = httpx.Problem(c, fiber.StatusUnauthorized, "authentication_required", "Authentication is required.")
		return uuid.Nil, uuid.Nil, false
	}
	chatID, err := httpx.UUIDParam(c, "chatId")
	if err != nil {
		_ = httpx.Problem(c, fiber.StatusBadRequest, "invalid_chat_id", "The chat id is invalid.")
		return uuid.Nil, uuid.Nil, false
	}
	return userID, chatID, true
}

func chatProblem(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, ErrChatNotFound):
		return httpx.Problem(c, fiber.StatusNotFound, "chat_not_found", "The chat was not found.")
	case errors.Is(err, ErrMessageNotFound):
		return httpx.Problem(c, fiber.StatusNotFound, "message_not_found", "The message was not found in this chat.")
	case errors.Is(err, ErrChatInactive):
		return httpx.Problem(c, fiber.StatusForbidden, "chat_inactive", "Messages cannot be sent to this chat.")
	case errors.Is(err, ErrSenderPetForbidden):
		return httpx.Problem(c, fiber.StatusForbidden, "sender_pet_forbidden", "The sender pet is not part of this match.")
	case errors.Is(err, ErrIdempotencyConflict):
		return httpx.Problem(c, fiber.StatusConflict, "idempotency_conflict", "The client message id was already used for different message input.")
	case errors.Is(err, ErrInvalidMessage):
		return httpx.Problem(c, fiber.StatusBadRequest, "invalid_message", "The message is invalid.")
	default:
		return httpx.Problem(c, fiber.StatusInternalServerError, "chat_failed", "The chat request could not be completed.")
	}
}

func parseLimit(value string, fallback int) (int, error) {
	if strings.TrimSpace(value) == "" {
		return fallback, nil
	}
	return strconv.Atoi(value)
}
