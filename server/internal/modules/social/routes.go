package social

import (
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RegisterRoutes mounts social endpoints on an authenticated router rooted at
// /v1. Authentication must run before these handlers so httpx.UserID is set.
func RegisterRoutes(router fiber.Router, db *pgxpool.Pool) {
	handler := &Handler{db: db}

	router.Get("/feed", handler.listFeed)
	router.Post("/posts", handler.createPost)
	router.Put("/posts/:postId/like", handler.likePost)
	router.Delete("/posts/:postId/like", handler.unlikePost)
	router.Put("/posts/:postId/save", handler.savePost)
	router.Delete("/posts/:postId/save", handler.unsavePost)
	router.Get("/posts/:postId/comments", handler.listComments)
	router.Post("/posts/:postId/comments", handler.createComment)

	router.Get("/stories", handler.listStories)
	router.Post("/stories", handler.createStory)

	router.Get("/reels", handler.listReels)
	router.Post("/reels", handler.createReel)
}
