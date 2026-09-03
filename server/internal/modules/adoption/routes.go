package adoption

import (
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RegisterRoutes mounts adoption endpoints on an authenticated /v1 router.
func RegisterRoutes(router fiber.Router, db *pgxpool.Pool) {
	handler := &Handler{db: db}
	router.Get("/adoptions", handler.list)
	router.Post("/adoptions", handler.create)
	router.Put("/adoptions/:listingId/save", handler.save)
	router.Delete("/adoptions/:listingId/save", handler.unsave)
}
