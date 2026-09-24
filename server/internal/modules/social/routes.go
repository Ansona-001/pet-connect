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
	router.Patch("/posts/:postId", handler.patchPost)
	router.Delete("/posts/:postId", handler.deletePost)
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

	// Public pet profile media tabs (brief Milestone 2) — one pet's own
	// posts/reels/stories, subject to that pet's own visibility rules
	// (see internal/platform/visibility), not the caller's global feed.
	router.Get("/pets/:petId/posts", handler.listPetPosts)
	router.Get("/pets/:petId/reels", handler.listPetReels)
	router.Get("/pets/:petId/stories", handler.listPetStories)

	// Follow/unfollow (brief Milestone 2) — idempotent in both directions,
	// notifying the pet's owner only on the follow's actual creation.
	router.Put("/pets/:petId/follow", handler.followPet)
	router.Delete("/pets/:petId/follow", handler.unfollowPet)
}
