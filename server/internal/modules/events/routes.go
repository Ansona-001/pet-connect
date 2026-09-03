package events

import (
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RegisterRoutes mounts event endpoints on an authenticated /v1 router.
func RegisterRoutes(router fiber.Router, db *pgxpool.Pool) {
	handler := &Handler{db: db}
	router.Get("/events", handler.list)
	router.Post("/events", handler.create)
	router.Put("/events/:eventId/rsvps/me", handler.setRSVP)
	router.Delete("/events/:eventId/rsvps/me", handler.removeRSVP)
}
