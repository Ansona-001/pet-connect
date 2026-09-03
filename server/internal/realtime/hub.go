package realtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const defaultBrokerChannel = "petconnect:realtime:events"

type Hub struct {
	redis      redis.UniversalClient
	channel    string
	instanceID uuid.UUID

	mu          sync.RWMutex
	connections map[uuid.UUID]map[*connection]struct{}
	pubsub      *redis.PubSub
	closed      bool
}

func NewHub(client redis.UniversalClient, channel string) *Hub {
	if channel == "" {
		channel = defaultBrokerChannel
	}
	return &Hub{
		redis: client, channel: channel, instanceID: uuid.New(),
		connections: make(map[uuid.UUID]map[*connection]struct{}),
	}
}

// Start subscribes this API instance to cross-instance realtime events.
// It must be called once during application startup; a nil Redis client leaves
// the hub in local-only mode for tests.
func (h *Hub) Start(ctx context.Context) error {
	if h.redis == nil {
		return nil
	}
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		return errors.New("realtime hub is closed")
	}
	if h.pubsub != nil {
		h.mu.Unlock()
		return nil
	}
	pubsub := h.redis.Subscribe(ctx, h.channel)
	h.pubsub = pubsub
	h.mu.Unlock()

	if _, err := pubsub.Receive(ctx); err != nil {
		h.mu.Lock()
		if h.pubsub == pubsub {
			h.pubsub = nil
		}
		h.mu.Unlock()
		_ = pubsub.Close()
		return fmt.Errorf("subscribe realtime broker: %w", err)
	}
	go h.consume(ctx, pubsub)
	return nil
}

func (h *Hub) Close() error {
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		return nil
	}
	h.closed = true
	pubsub := h.pubsub
	h.pubsub = nil
	connections := make([]*connection, 0)
	for _, userConnections := range h.connections {
		for client := range userConnections {
			connections = append(connections, client)
		}
	}
	h.connections = make(map[uuid.UUID]map[*connection]struct{})
	h.mu.Unlock()
	for _, client := range connections {
		client.close()
	}
	if pubsub != nil {
		return pubsub.Close()
	}
	return nil
}

// PublishToUsers satisfies the matching and chat publisher contracts. Durable
// state is always fetched from PostgreSQL after reconnect; Redis pub/sub is an
// online delivery accelerator, not the source of truth.
func (h *Hub) PublishToUsers(userIDs []uuid.UUID, eventType string, payload any) error {
	users := uniqueUsers(userIDs)
	if len(users) == 0 || eventType == "" {
		return nil
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode realtime event data: %w", err)
	}
	event := Event{ID: uuid.New(), Type: eventType, OccurredAt: time.Now().UTC(), Data: data}
	encodedEvent, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("encode realtime event: %w", err)
	}
	h.deliverLocal(users, encodedEvent)
	if h.redis == nil {
		return nil
	}
	envelope, err := json.Marshal(brokerEnvelope{OriginInstanceID: h.instanceID, UserIDs: users, Event: event})
	if err != nil {
		return fmt.Errorf("encode realtime broker event: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := h.redis.Publish(ctx, h.channel, envelope).Err(); err != nil {
		return fmt.Errorf("publish realtime broker event: %w", err)
	}
	return nil
}

func (h *Hub) register(client *connection) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return errors.New("realtime hub is closed")
	}
	clients := h.connections[client.identity.UserID]
	if clients == nil {
		clients = make(map[*connection]struct{})
		h.connections[client.identity.UserID] = clients
	}
	clients[client] = struct{}{}
	return nil
}

func (h *Hub) unregister(client *connection) {
	h.mu.Lock()
	clients := h.connections[client.identity.UserID]
	if clients != nil {
		delete(clients, client)
		if len(clients) == 0 {
			delete(h.connections, client.identity.UserID)
		}
	}
	h.mu.Unlock()
	client.close()
}

func (h *Hub) DisconnectSession(sessionID uuid.UUID) {
	h.mu.Lock()
	clients := make([]*connection, 0)
	for userID, userConnections := range h.connections {
		for client := range userConnections {
			if client.identity.SessionID == sessionID {
				delete(userConnections, client)
				clients = append(clients, client)
			}
		}
		if len(userConnections) == 0 {
			delete(h.connections, userID)
		}
	}
	h.mu.Unlock()
	for _, client := range clients {
		client.close()
	}
}

func (h *Hub) consume(ctx context.Context, pubsub *redis.PubSub) {
	channel := pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case message, ok := <-channel:
			if !ok {
				return
			}
			var envelope brokerEnvelope
			if err := json.Unmarshal([]byte(message.Payload), &envelope); err != nil || envelope.OriginInstanceID == h.instanceID {
				continue
			}
			encoded, err := json.Marshal(envelope.Event)
			if err != nil {
				continue
			}
			h.deliverLocal(uniqueUsers(envelope.UserIDs), encoded)
		}
	}
}

func (h *Hub) deliverLocal(userIDs []uuid.UUID, event []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.closed {
		return
	}
	for _, userID := range userIDs {
		for client := range h.connections[userID] {
			client.enqueue(event)
		}
	}
}

func uniqueUsers(input []uuid.UUID) []uuid.UUID {
	seen := make(map[uuid.UUID]struct{}, len(input))
	result := make([]uuid.UUID, 0, len(input))
	for _, id := range input {
		if id == uuid.Nil {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result
}

type connection struct {
	identity Identity
	send     chan []byte
	done     chan struct{}
	once     sync.Once
}

func newConnection(identity Identity) *connection {
	return &connection{identity: identity, send: make(chan []byte, 128), done: make(chan struct{})}
}

func (c *connection) enqueue(message []byte) {
	select {
	case <-c.done:
		return
	default:
	}
	copyOfMessage := append([]byte(nil), message...)
	select {
	case c.send <- copyOfMessage:
	default:
		// REST cursors recover durable events for a slow client. Dropping here
		// prevents a single connection from blocking every publisher.
	}
}

func (c *connection) close() {
	c.once.Do(func() { close(c.done) })
}
