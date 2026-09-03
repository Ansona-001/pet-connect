package realtime

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"petconnect/server/internal/platform/authjwt"
	"petconnect/server/internal/platform/httpx"
)

const (
	websocketWriteTimeout = 10 * time.Second
	websocketPongTimeout  = 70 * time.Second
	websocketPingInterval = 25 * time.Second
	websocketReadLimit    = 16 * 1024
)

type Server struct {
	pool           *pgxpool.Pool
	tickets        *TicketStore
	hub            *Hub
	tokens         *authjwt.Manager
	allowedOrigins []string
	upgrade        fiber.Handler
	clock          func() time.Time
}

func NewServer(pool *pgxpool.Pool, tickets *TicketStore, hub *Hub, tokens *authjwt.Manager, allowedOrigins []string) *Server {
	server := &Server{
		pool: pool, tickets: tickets, hub: hub, tokens: tokens,
		allowedOrigins: append([]string(nil), allowedOrigins...), clock: time.Now,
	}
	server.upgrade = websocket.New(server.handleConnection)
	return server
}

// Register mounts the ticket endpoint on an authenticated /v1 router and the
// WebSocket endpoint on the public /v1 router. The ticket itself authenticates
// the WebSocket upgrade.
func (s *Server) Register(authenticated, public fiber.Router) {
	s.RegisterTicketRoute(authenticated)
	s.RegisterSocketRoute(public)
}

func (s *Server) RegisterTicketRoute(authenticated fiber.Router) {
	authenticated.Post("/realtime/tickets", s.IssueTicket)
}

func (s *Server) RegisterSocketRoute(public fiber.Router) {
	public.Get("/realtime", s.Upgrade)
}

func (s *Server) IssueTicket(c *fiber.Ctx) error {
	userID, err := httpx.UserID(c)
	if err != nil {
		return httpx.Problem(c, fiber.StatusUnauthorized, "authentication_required", "Authentication is required.")
	}
	raw := bearerToken(c.Get(fiber.HeaderAuthorization))
	claims, err := s.tokens.Parse(raw)
	if err != nil || claims.ExpiresAt == nil || claims.Subject != userID.String() {
		return httpx.Problem(c, fiber.StatusUnauthorized, "invalid_access_token", "The access token is invalid or expired.")
	}
	sessionID, err := uuid.Parse(claims.SessionID)
	if err != nil {
		return httpx.Problem(c, fiber.StatusUnauthorized, "invalid_access_token", "The access token session is invalid.")
	}
	identity := Identity{
		UserID: userID, SessionID: sessionID, AccessExpiresAt: claims.ExpiresAt.Time,
	}
	active, err := s.sessionActive(c.UserContext(), identity)
	if err != nil {
		return httpx.Problem(c, fiber.StatusServiceUnavailable, "realtime_unavailable", "Realtime connection tickets are temporarily unavailable.")
	}
	if !active {
		return httpx.Problem(c, fiber.StatusUnauthorized, "session_revoked", "The authenticated session is no longer active.")
	}
	ticket, err := s.tickets.Issue(c.UserContext(), identity)
	if err != nil {
		return httpx.Problem(c, fiber.StatusServiceUnavailable, "realtime_unavailable", "Realtime connection tickets are temporarily unavailable.")
	}
	return httpx.Created(c, ticket)
}

func (s *Server) Upgrade(c *fiber.Ctx) error {
	if !websocket.IsWebSocketUpgrade(c) {
		return httpx.Problem(c, fiber.StatusUpgradeRequired, "websocket_upgrade_required", "A WebSocket upgrade is required.")
	}
	if !originAllowed(c.Get(fiber.HeaderOrigin), s.allowedOrigins) {
		return httpx.Problem(c, fiber.StatusForbidden, "origin_forbidden", "The WebSocket origin is not allowed.")
	}
	identity, err := s.tickets.Consume(c.UserContext(), strings.TrimSpace(c.Query("ticket")))
	if err != nil {
		status := fiber.StatusUnauthorized
		code := "invalid_realtime_ticket"
		message := "The realtime ticket is invalid or expired."
		if errors.Is(err, ErrTicketStore) {
			status, code, message = fiber.StatusServiceUnavailable, "realtime_unavailable", "Realtime is temporarily unavailable."
		}
		return httpx.Problem(c, status, code, message)
	}
	active, err := s.sessionActive(c.UserContext(), identity)
	if err != nil {
		return httpx.Problem(c, fiber.StatusServiceUnavailable, "realtime_unavailable", "Realtime is temporarily unavailable.")
	}
	if !active {
		return httpx.Problem(c, fiber.StatusUnauthorized, "session_revoked", "The authenticated session is no longer active.")
	}
	c.Locals("realtime_user_id", identity.UserID.String())
	c.Locals("realtime_session_id", identity.SessionID.String())
	c.Locals("realtime_access_expires_at", identity.AccessExpiresAt.Format(time.RFC3339Nano))
	return s.upgrade(c)
}

func (s *Server) handleConnection(socket *websocket.Conn) {
	identity, err := socketIdentity(socket)
	if err != nil || !identity.AccessExpiresAt.After(s.clock().UTC()) {
		_ = socket.Close()
		return
	}
	client := newConnection(identity)
	if err := s.hub.register(client); err != nil {
		_ = socket.Close()
		return
	}
	defer func() {
		s.hub.unregister(client)
		_ = socket.Close()
	}()

	socket.SetReadLimit(websocketReadLimit)
	_ = socket.SetReadDeadline(minTime(s.clock().Add(websocketPongTimeout), identity.AccessExpiresAt))
	socket.SetPongHandler(func(string) error {
		return socket.SetReadDeadline(minTime(s.clock().Add(websocketPongTimeout), identity.AccessExpiresAt))
	})
	client.enqueue(mustEvent("realtime.ready", struct {
		UserID    uuid.UUID `json:"user_id"`
		ExpiresAt time.Time `json:"access_expires_at"`
	}{UserID: identity.UserID, ExpiresAt: identity.AccessExpiresAt}))

	writerDone := make(chan struct{})
	go func() {
		defer close(writerDone)
		defer func() { _ = socket.Close() }()
		s.writeLoop(socket, client)
	}()
	s.readLoop(socket, client)
	client.close()
	<-writerDone
}

func (s *Server) writeLoop(socket *websocket.Conn, client *connection) {
	ticker := time.NewTicker(websocketPingInterval)
	defer ticker.Stop()
	for {
		select {
		case <-client.done:
			_ = socket.SetWriteDeadline(s.clock().Add(websocketWriteTimeout))
			_ = socket.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "connection closed"))
			return
		case message := <-client.send:
			_ = socket.SetWriteDeadline(s.clock().Add(websocketWriteTimeout))
			if err := socket.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
		case <-ticker.C:
			if !client.identity.AccessExpiresAt.After(s.clock().UTC()) {
				_ = socket.SetWriteDeadline(s.clock().Add(websocketWriteTimeout))
				_ = socket.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "access token expired"))
				return
			}
			_ = socket.SetWriteDeadline(s.clock().Add(websocketWriteTimeout))
			if err := socket.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (s *Server) readLoop(socket *websocket.Conn, client *connection) {
	lastTyping := time.Time{}
	for {
		messageType, body, err := socket.ReadMessage()
		if err != nil {
			return
		}
		if messageType != websocket.TextMessage {
			client.enqueue(mustEvent("realtime.error", fiber.Map{"code": "text_frames_only"}))
			continue
		}
		var frame clientFrame
		if err := json.Unmarshal(body, &frame); err != nil {
			client.enqueue(mustEvent("realtime.error", fiber.Map{"code": "invalid_event"}))
			continue
		}
		switch frame.Type {
		case "ping":
			client.enqueue(mustEvent("pong", fiber.Map{}))
		case "typing":
			if s.clock().Sub(lastTyping) < 300*time.Millisecond {
				continue
			}
			lastTyping = s.clock()
			s.publishTyping(client, frame)
		default:
			client.enqueue(mustEvent("realtime.error", fiber.Map{"code": "unsupported_event"}))
		}
	}
}

type clientFrame struct {
	Type     string `json:"type"`
	ChatID   string `json:"chat_id"`
	IsTyping bool   `json:"is_typing"`
}

func (s *Server) publishTyping(client *connection, frame clientFrame) {
	chatID, err := uuid.Parse(strings.TrimSpace(frame.ChatID))
	if err != nil {
		client.enqueue(mustEvent("realtime.error", fiber.Map{"code": "invalid_chat_id"}))
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	rows, err := s.pool.Query(ctx, `
		SELECT recipient.user_id
		FROM chat_members sender
		JOIN chats chat ON chat.id = sender.chat_id
		JOIN matches match ON match.id = chat.match_id AND match.status = 'active'
		JOIN chat_members recipient ON recipient.chat_id = sender.chat_id
		WHERE sender.chat_id = $1
		  AND sender.user_id = $2
		  AND recipient.user_id <> $2`, chatID, client.identity.UserID)
	if err != nil {
		return
	}
	defer rows.Close()
	recipients := make([]uuid.UUID, 0, 1)
	for rows.Next() {
		var userID uuid.UUID
		if err := rows.Scan(&userID); err != nil {
			return
		}
		recipients = append(recipients, userID)
	}
	if rows.Err() != nil {
		return
	}
	if len(recipients) == 0 {
		client.enqueue(mustEvent("realtime.error", fiber.Map{"code": "chat_not_found"}))
		return
	}
	_ = s.hub.PublishToUsers(recipients, "chat.typing", struct {
		ChatID   uuid.UUID `json:"chat_id"`
		UserID   uuid.UUID `json:"user_id"`
		IsTyping bool      `json:"is_typing"`
	}{ChatID: chatID, UserID: client.identity.UserID, IsTyping: frame.IsTyping})
}

func (s *Server) sessionActive(ctx context.Context, identity Identity) (bool, error) {
	var active bool
	if err := s.pool.QueryRow(ctx, `
		SELECT EXISTS(
		  SELECT 1
		  FROM refresh_tokens
		  WHERE id = $1
		    AND user_id = $2
		    AND revoked_at IS NULL
		    AND expires_at > now()
		)`, identity.SessionID, identity.UserID).Scan(&active); err != nil {
		return false, err
	}
	return active, nil
}

func socketIdentity(socket *websocket.Conn) (Identity, error) {
	userID, err := uuid.Parse(stringLocal(socket.Locals("realtime_user_id")))
	if err != nil {
		return Identity{}, err
	}
	sessionID, err := uuid.Parse(stringLocal(socket.Locals("realtime_session_id")))
	if err != nil {
		return Identity{}, err
	}
	expiresAt, err := time.Parse(time.RFC3339Nano, stringLocal(socket.Locals("realtime_access_expires_at")))
	if err != nil {
		return Identity{}, err
	}
	return Identity{UserID: userID, SessionID: sessionID, AccessExpiresAt: expiresAt}, nil
}

func stringLocal(value any) string {
	text, _ := value.(string)
	return text
}

func bearerToken(header string) string {
	header = strings.TrimSpace(header)
	if len(header) < 8 || !strings.EqualFold(header[:7], "bearer ") {
		return ""
	}
	return strings.TrimSpace(header[7:])
}

func originAllowed(origin string, allowed []string) bool {
	origin = strings.TrimSpace(origin)
	if origin == "" {
		return true // Native WebSocket clients do not send Origin.
	}
	parsedOrigin, err := url.Parse(origin)
	if err != nil || (parsedOrigin.Scheme != "http" && parsedOrigin.Scheme != "https") || parsedOrigin.User != nil || parsedOrigin.Host == "" {
		return false
	}
	for _, candidate := range allowed {
		candidate = strings.TrimSpace(strings.TrimSuffix(candidate, "/"))
		if strings.EqualFold(candidate, strings.TrimSuffix(origin, "/")) {
			return true
		}
		if !strings.HasSuffix(candidate, ":*") {
			continue
		}
		wildcardURL, err := url.Parse(strings.TrimSuffix(candidate, ":*"))
		if err == nil && strings.EqualFold(wildcardURL.Scheme, parsedOrigin.Scheme) && strings.EqualFold(wildcardURL.Hostname(), parsedOrigin.Hostname()) {
			return true
		}
	}
	return false
}

func mustEvent(eventType string, payload any) []byte {
	data, _ := json.Marshal(payload)
	encoded, _ := json.Marshal(Event{ID: uuid.New(), Type: eventType, OccurredAt: time.Now().UTC(), Data: data})
	return encoded
}

func minTime(first, second time.Time) time.Time {
	if first.Before(second) {
		return first
	}
	return second
}
