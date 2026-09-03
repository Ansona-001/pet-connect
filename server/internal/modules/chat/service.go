package chat

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	pool      *pgxpool.Pool
	publisher EventPublisher
}

func NewService(pool *pgxpool.Pool, publisher EventPublisher) *Service {
	return &Service{pool: pool, publisher: publisher}
}

func (s *Service) ListChats(ctx context.Context, userID uuid.UUID, limit int) ([]Chat, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}
	rows, err := s.pool.Query(ctx, `
		SELECT chat.id,
		       match.id,
		       match.matched_at,
		       own_pet.id,
		       own_pet.name,
		       other_pet.id,
		       other_pet.name,
		       other_pet.pet_type,
		       other_pet.breed,
		       other_pet.primary_image_url,
		       other_owner.id,
		       other_owner.name,
		       member.last_read_at,
		       last_message.id IS NOT NULL,
		       COALESCE(last_message.id, '00000000-0000-0000-0000-000000000000'::uuid),
		       COALESCE(last_message.sender_user_id, '00000000-0000-0000-0000-000000000000'::uuid),
		       COALESCE(last_message.sender_pet_id, '00000000-0000-0000-0000-000000000000'::uuid),
		       COALESCE(last_message.client_message_id, '00000000-0000-0000-0000-000000000000'::uuid),
		       COALESCE(last_message.message_type, ''),
		       COALESCE(last_message.body, ''),
		       COALESCE(last_message.media_url, ''),
		       COALESCE(last_message.created_at, chat.created_at),
		       last_message.edited_at,
		       COALESCE(unread.count, 0)
		FROM chat_members member
		JOIN chats chat ON chat.id = member.chat_id
		JOIN matches match ON match.id = chat.match_id AND match.status <> 'blocked'
		JOIN pets own_pet
		  ON own_pet.owner_id = $1
		 AND own_pet.id IN (match.pet_low_id, match.pet_high_id)
		JOIN pets other_pet
		  ON other_pet.id = CASE
		       WHEN own_pet.id = match.pet_low_id THEN match.pet_high_id
		       ELSE match.pet_low_id
		     END
		JOIN users other_owner ON other_owner.id = other_pet.owner_id
		LEFT JOIN LATERAL (
		  SELECT message.*
		  FROM messages message
		  WHERE message.chat_id = chat.id AND message.deleted_at IS NULL
		  ORDER BY message.created_at DESC, message.id DESC
		  LIMIT 1
		) last_message ON true
		LEFT JOIN LATERAL (
		  SELECT count(*) AS count
		  FROM messages message
		  WHERE message.chat_id = chat.id
		    AND message.deleted_at IS NULL
		    AND message.sender_user_id <> $1
		    AND message.created_at > COALESCE(member.last_read_at, '-infinity'::timestamptz)
		) unread ON true
		WHERE member.user_id = $1
		  AND own_pet.deleted_at IS NULL
		  AND other_pet.deleted_at IS NULL
		  AND other_owner.deleted_at IS NULL
		ORDER BY COALESCE(last_message.created_at, chat.created_at) DESC, chat.id DESC
		LIMIT $2`, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("list chats: %w", err)
	}
	defer rows.Close()

	chats := make([]Chat, 0, limit)
	for rows.Next() {
		var item Chat
		var hasLastMessage bool
		var last Message
		var senderPetID uuid.UUID
		if err := rows.Scan(
			&item.ID,
			&item.MatchID,
			&item.MatchedAt,
			&item.OwnPetID,
			&item.OwnPetName,
			&item.OtherPetID,
			&item.OtherPetName,
			&item.OtherPetType,
			&item.OtherPetBreed,
			&item.OtherPetImage,
			&item.OtherOwnerID,
			&item.OtherOwnerName,
			&item.LastReadAt,
			&hasLastMessage,
			&last.ID,
			&last.SenderUserID,
			&senderPetID,
			&last.ClientMessageID,
			&last.MessageType,
			&last.Body,
			&last.MediaURL,
			&last.CreatedAt,
			&last.EditedAt,
			&item.UnreadCount,
		); err != nil {
			return nil, fmt.Errorf("scan chat: %w", err)
		}
		if hasLastMessage {
			last.ChatID = item.ID
			if senderPetID != uuid.Nil {
				last.SenderPetID = &senderPetID
			}
			item.LastMessage = &last
		}
		chats = append(chats, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate chats: %w", err)
	}
	return chats, nil
}

func (s *Service) ListMessages(ctx context.Context, userID, chatID uuid.UUID, cursor string, limit int) (MessagePage, error) {
	if err := s.ensureMembership(ctx, s.pool, userID, chatID, false); err != nil {
		return MessagePage{}, err
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}

	var rows pgx.Rows
	var err error
	if strings.TrimSpace(cursor) == "" {
		rows, err = s.pool.Query(ctx, messageListSQL+`
			ORDER BY message.created_at DESC, message.id DESC
			LIMIT $2`, chatID, limit+1)
	} else {
		cursorTime, cursorID, decodeErr := decodeCursor(cursor)
		if decodeErr != nil {
			return MessagePage{}, fmt.Errorf("%w: invalid cursor", ErrInvalidMessage)
		}
		rows, err = s.pool.Query(ctx, messageListSQL+`
			AND (message.created_at, message.id) < ($2, $3)
			ORDER BY message.created_at DESC, message.id DESC
			LIMIT $4`, chatID, cursorTime, cursorID, limit+1)
	}
	if err != nil {
		return MessagePage{}, fmt.Errorf("list messages: %w", err)
	}
	defer rows.Close()

	items := make([]Message, 0, limit+1)
	for rows.Next() {
		message, err := scanMessage(rows)
		if err != nil {
			return MessagePage{}, err
		}
		items = append(items, message)
	}
	if err := rows.Err(); err != nil {
		return MessagePage{}, fmt.Errorf("iterate messages: %w", err)
	}
	page := MessagePage{Items: items}
	if len(items) > limit {
		page.Items = items[:limit]
		last := page.Items[len(page.Items)-1]
		page.NextCursor = encodeCursor(last.CreatedAt, last.ID)
	}
	return page, nil
}

const messageListSQL = `
	SELECT message.id,
	       message.chat_id,
	       message.sender_user_id,
	       COALESCE(message.sender_pet_id, '00000000-0000-0000-0000-000000000000'::uuid),
	       message.client_message_id,
	       message.message_type,
	       message.body,
	       message.media_url,
	       message.created_at,
	       message.edited_at
	FROM messages message
	WHERE message.chat_id = $1
	  AND message.deleted_at IS NULL`

func (s *Service) SendMessage(ctx context.Context, userID, chatID uuid.UUID, request SendMessageRequest) (SendMessageResult, error) {
	request.MessageType = strings.ToLower(strings.TrimSpace(request.MessageType))
	request.Body = strings.TrimSpace(request.Body)
	request.MediaURL = strings.TrimSpace(request.MediaURL)
	if err := validateMessage(request); err != nil {
		return SendMessageResult{}, err
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return SendMessageResult{}, fmt.Errorf("begin message transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := s.ensureMembership(ctx, tx, userID, chatID, true); err != nil {
		return SendMessageResult{}, err
	}
	if request.SenderPetID != nil {
		var allowed bool
		if err := tx.QueryRow(ctx, `
			SELECT EXISTS(
			  SELECT 1
			  FROM chats chat
			  JOIN matches match ON match.id = chat.match_id
			  JOIN pets pet ON pet.id = $3
			  WHERE chat.id = $1
			    AND pet.owner_id = $2
			    AND pet.id IN (match.pet_low_id, match.pet_high_id)
			    AND pet.deleted_at IS NULL
			)`, chatID, userID, *request.SenderPetID).Scan(&allowed); err != nil {
			return SendMessageResult{}, fmt.Errorf("validate sender pet: %w", err)
		}
		if !allowed {
			return SendMessageResult{}, ErrSenderPetForbidden
		}
	}

	message, created, err := insertOrLoadMessage(ctx, tx, userID, chatID, request)
	if err != nil {
		return SendMessageResult{}, err
	}
	recipients, err := chatMemberIDs(ctx, tx, chatID)
	if err != nil {
		return SendMessageResult{}, err
	}
	if created {
		for _, recipientID := range recipients {
			if recipientID == userID {
				continue
			}
			if _, err := tx.Exec(ctx, `
				INSERT INTO notifications(user_id, notification_type, payload)
				VALUES($1, 'message', jsonb_build_object(
				  'chat_id', $2::text,
				  'message_id', $3::text,
				  'sender_user_id', $4::text
				))`, recipientID, chatID, message.ID, userID); err != nil {
				return SendMessageResult{}, fmt.Errorf("create message notification: %w", err)
			}
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return SendMessageResult{}, fmt.Errorf("commit message transaction: %w", err)
	}
	if created && s.publisher != nil {
		_ = s.publisher.PublishToUsers(recipients, "message.created", message)
	}
	return SendMessageResult{Message: message, Created: created}, nil
}

func (s *Service) MarkRead(ctx context.Context, userID, chatID, messageID uuid.UUID) (ReadCursor, error) {
	if err := s.ensureMembership(ctx, s.pool, userID, chatID, false); err != nil {
		return ReadCursor{}, err
	}
	var readAt time.Time
	err := s.pool.QueryRow(ctx, `
		UPDATE chat_members member
		SET last_read_at = GREATEST(
		  COALESCE(member.last_read_at, '-infinity'::timestamptz),
		  message.created_at
		)
		FROM messages message
		WHERE member.chat_id = $1
		  AND member.user_id = $2
		  AND message.id = $3
		  AND message.chat_id = member.chat_id
		  AND message.deleted_at IS NULL
		RETURNING member.last_read_at`, chatID, userID, messageID).Scan(&readAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ReadCursor{}, ErrMessageNotFound
	}
	if err != nil {
		return ReadCursor{}, fmt.Errorf("advance read cursor: %w", err)
	}
	cursor := ReadCursor{ChatID: chatID, MessageID: messageID, ReadAt: readAt}
	if s.publisher != nil {
		if recipients, memberErr := chatMemberIDs(ctx, s.pool, chatID); memberErr == nil {
			_ = s.publisher.PublishToUsers(recipients, "chat.read", struct {
				ReadCursor
				UserID uuid.UUID `json:"user_id"`
			}{ReadCursor: cursor, UserID: userID})
		}
	}
	return cursor, nil
}

type queryer interface {
	QueryRow(context.Context, string, ...any) pgx.Row
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func (s *Service) ensureMembership(ctx context.Context, db queryer, userID, chatID uuid.UUID, requireActive bool) error {
	var status string
	err := db.QueryRow(ctx, `
		SELECT match.status
		FROM chat_members member
		JOIN chats chat ON chat.id = member.chat_id
		JOIN matches match ON match.id = chat.match_id
		WHERE member.chat_id = $1 AND member.user_id = $2`, chatID, userID).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrChatNotFound
	}
	if err != nil {
		return fmt.Errorf("verify chat membership: %w", err)
	}
	if status == "blocked" || (requireActive && status != "active") {
		return ErrChatInactive
	}
	return nil
}

func insertOrLoadMessage(ctx context.Context, tx pgx.Tx, userID, chatID uuid.UUID, request SendMessageRequest) (Message, bool, error) {
	var senderPet any
	if request.SenderPetID != nil {
		senderPet = *request.SenderPetID
	}
	row := tx.QueryRow(ctx, `
		INSERT INTO messages(
		  chat_id, sender_user_id, sender_pet_id, client_message_id,
		  message_type, body, media_url
		)
		VALUES($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (chat_id, sender_user_id, client_message_id) DO NOTHING
		RETURNING id, chat_id, sender_user_id,
		          COALESCE(sender_pet_id, '00000000-0000-0000-0000-000000000000'::uuid),
		          client_message_id, message_type, body, media_url, created_at, edited_at`,
		chatID, userID, senderPet, request.ClientMessageID,
		request.MessageType, request.Body, request.MediaURL)
	message, err := scanMessage(row)
	if err == nil {
		return message, true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Message{}, false, fmt.Errorf("insert message: %w", err)
	}

	message, err = scanMessage(tx.QueryRow(ctx, `
		SELECT id, chat_id, sender_user_id,
		       COALESCE(sender_pet_id, '00000000-0000-0000-0000-000000000000'::uuid),
		       client_message_id, message_type, body, media_url, created_at, edited_at
		FROM messages
		WHERE chat_id = $1 AND sender_user_id = $2 AND client_message_id = $3`, chatID, userID, request.ClientMessageID))
	if err != nil {
		return Message{}, false, fmt.Errorf("load idempotent message: %w", err)
	}
	if message.MessageType != request.MessageType || message.Body != request.Body || message.MediaURL != request.MediaURL || !sameOptionalUUID(message.SenderPetID, request.SenderPetID) {
		return Message{}, false, ErrIdempotencyConflict
	}
	return message, false, nil
}

type rowScanner interface {
	Scan(...any) error
}

func scanMessage(row rowScanner) (Message, error) {
	var message Message
	var senderPetID uuid.UUID
	if err := row.Scan(
		&message.ID,
		&message.ChatID,
		&message.SenderUserID,
		&senderPetID,
		&message.ClientMessageID,
		&message.MessageType,
		&message.Body,
		&message.MediaURL,
		&message.CreatedAt,
		&message.EditedAt,
	); err != nil {
		return Message{}, err
	}
	if senderPetID != uuid.Nil {
		message.SenderPetID = &senderPetID
	}
	return message, nil
}

func chatMemberIDs(ctx context.Context, db queryer, chatID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := db.Query(ctx, `SELECT user_id FROM chat_members WHERE chat_id = $1 ORDER BY user_id`, chatID)
	if err != nil {
		return nil, fmt.Errorf("list chat members: %w", err)
	}
	defer rows.Close()
	result := make([]uuid.UUID, 0, 2)
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan chat member: %w", err)
		}
		result = append(result, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate chat members: %w", err)
	}
	return result, nil
}

func validateMessage(request SendMessageRequest) error {
	if request.ClientMessageID == uuid.Nil {
		return fmt.Errorf("%w: client_message_id is required", ErrInvalidMessage)
	}
	if !utf8.ValidString(request.Body) || !utf8.ValidString(request.MediaURL) {
		return fmt.Errorf("%w: text must be valid UTF-8", ErrInvalidMessage)
	}
	if utf8.RuneCountInString(request.Body) > 5000 || len(request.MediaURL) > 2048 {
		return fmt.Errorf("%w: message is too large", ErrInvalidMessage)
	}
	switch request.MessageType {
	case "text":
		if request.Body == "" || request.MediaURL != "" {
			return fmt.Errorf("%w: text messages require body and cannot include media", ErrInvalidMessage)
		}
	case "image", "video", "audio":
		if request.MediaURL == "" {
			return fmt.Errorf("%w: media message requires media_url", ErrInvalidMessage)
		}
	case "location":
		if request.Body == "" || request.MediaURL != "" {
			return fmt.Errorf("%w: location message requires body and cannot include media", ErrInvalidMessage)
		}
	default:
		return fmt.Errorf("%w: unsupported message type", ErrInvalidMessage)
	}
	return nil
}

func encodeCursor(createdAt time.Time, id uuid.UUID) string {
	value := createdAt.UTC().Format(time.RFC3339Nano) + "|" + id.String()
	return base64.RawURLEncoding.EncodeToString([]byte(value))
}

func decodeCursor(value string) (time.Time, uuid.UUID, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return time.Time{}, uuid.Nil, err
	}
	parts := strings.Split(string(decoded), "|")
	if len(parts) != 2 {
		return time.Time{}, uuid.Nil, errors.New("invalid message cursor")
	}
	createdAt, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return time.Time{}, uuid.Nil, err
	}
	id, err := uuid.Parse(parts[1])
	if err != nil {
		return time.Time{}, uuid.Nil, err
	}
	return createdAt, id, nil
}

func sameOptionalUUID(first, second *uuid.UUID) bool {
	if first == nil || second == nil {
		return first == nil && second == nil
	}
	return *first == *second
}
