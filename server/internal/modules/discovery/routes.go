package discovery

import (
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RegisterRoutes mounts discovery endpoints on an authenticated /v1 router.
func RegisterRoutes(router fiber.Router, db *pgxpool.Pool) {
	handler := &Handler{db: db}
	router.Get("/discovery/pets", handler.nearbyPets)
}
