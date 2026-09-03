package community

import (
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RegisterRoutes mounts community endpoints on an authenticated /v1 router.
func RegisterRoutes(router fiber.Router, db *pgxpool.Pool) {
	handler := &Handler{db: db}
	router.Get("/communities", handler.list)
	router.Post("/communities", handler.create)
	router.Put("/communities/:communityId/members/me", handler.join)
	router.Delete("/communities/:communityId/members/me", handler.leave)
}
