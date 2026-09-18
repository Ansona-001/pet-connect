package account

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"petconnect/server/internal/platform/httpx"
)

// exportedPet, exportedPost, etc. are deliberately local, minimal copies
// of each table's shape rather than imports from pets/social/adoption —
// matching this codebase's existing pattern of per-module response
// structs (see pets.pet, social.Post) instead of a shared cross-module
// type. An export is also a different contract than those modules' own
// API responses: it should stay stable even if a module's live response
// shape changes later.
type exportedPet struct {
	ID              uuid.UUID `json:"id"`
	Name            string    `json:"name"`
	PetType         string    `json:"pet_type"`
	Breed           string    `json:"breed"`
	AgeLabel        string    `json:"age_label"`
	Gender          string    `json:"gender"`
	Bio             string    `json:"bio"`
	PrimaryImageURL string    `json:"primary_image_url"`
	Status          string    `json:"status"`
	CreatedAt       time.Time `json:"created_at"`
}

type exportedPost struct {
	ID        uuid.UUID `json:"id"`
	PetID     uuid.UUID `json:"pet_id"`
	Kind      string    `json:"kind"`
	Caption   string    `json:"caption"`
	MediaURL  string    `json:"media_url"`
	CreatedAt time.Time `json:"created_at"`
}

type exportedComment struct {
	ID        uuid.UUID `json:"id"`
	PostID    uuid.UUID `json:"post_id"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}

type exportedStory struct {
	ID        uuid.UUID `json:"id"`
	PetID     uuid.UUID `json:"pet_id"`
	MediaURL  string    `json:"media_url"`
	CreatedAt time.Time `json:"created_at"`
}

type exportedAdoptionListing struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	PetType   string    `json:"pet_type"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

type exportedMatch struct {
	ID         uuid.UUID `json:"id"`
	OwnPetID   uuid.UUID `json:"own_pet_id"`
	OtherPetID uuid.UUID `json:"other_pet_id"`
	Status     string    `json:"status"`
	MatchedAt  time.Time `json:"matched_at"`
}

// exportData returns a data-portability export of the caller's own
// content (brief §13/§18: "data export/deletion"). Covers the account
// profile plus every table the owner directly authors or owns today.
//
// Not yet included, flagged here rather than silently omitted: chat
// message content. A user's own messages are inseparable from the other
// party's in the same conversation, and brief §3 ("Do not expose private
// chat content to moderators by default") signals this needs its own
// deliberate privacy design, not a quick addition to a general export —
// community memberships and event RSVPs are similarly left for a later
// pass once those modules have matured past their current partial state.
func (h *Handler) exportData(c *fiber.Ctx) error {
	userID, err := httpx.UserID(c)
	if err != nil {
		return httpx.Problem(c, fiber.StatusUnauthorized, "authentication_required", "Authentication is required.")
	}

	account, err := loadUser(c, h.db, userID)
	if err != nil {
		return internalProblem(c)
	}

	pets, err := queryExport(c, h.db, `
		SELECT id, name, pet_type, breed, age_label, gender, bio, primary_image_url, status, created_at
		FROM pets WHERE owner_id = $1 ORDER BY created_at`, userID,
		func(row pgx.Rows) (exportedPet, error) {
			var v exportedPet
			err := row.Scan(&v.ID, &v.Name, &v.PetType, &v.Breed, &v.AgeLabel, &v.Gender, &v.Bio, &v.PrimaryImageURL, &v.Status, &v.CreatedAt)
			return v, err
		})
	if err != nil {
		return internalProblem(c)
	}

	posts, err := queryExport(c, h.db, `
		SELECT id, pet_id, kind, caption, media_url, created_at
		FROM posts WHERE author_user_id = $1 ORDER BY created_at`, userID,
		func(row pgx.Rows) (exportedPost, error) {
			var v exportedPost
			err := row.Scan(&v.ID, &v.PetID, &v.Kind, &v.Caption, &v.MediaURL, &v.CreatedAt)
			return v, err
		})
	if err != nil {
		return internalProblem(c)
	}

	comments, err := queryExport(c, h.db, `
		SELECT id, post_id, body, created_at
		FROM comments WHERE user_id = $1 ORDER BY created_at`, userID,
		func(row pgx.Rows) (exportedComment, error) {
			var v exportedComment
			err := row.Scan(&v.ID, &v.PostID, &v.Body, &v.CreatedAt)
			return v, err
		})
	if err != nil {
		return internalProblem(c)
	}

	stories, err := queryExport(c, h.db, `
		SELECT id, pet_id, media_url, created_at
		FROM stories WHERE author_user_id = $1 ORDER BY created_at`, userID,
		func(row pgx.Rows) (exportedStory, error) {
			var v exportedStory
			err := row.Scan(&v.ID, &v.PetID, &v.MediaURL, &v.CreatedAt)
			return v, err
		})
	if err != nil {
		return internalProblem(c)
	}

	listings, err := queryExport(c, h.db, `
		SELECT id, name, pet_type, status, created_at
		FROM adoption_listings WHERE owner_user_id = $1 ORDER BY created_at`, userID,
		func(row pgx.Rows) (exportedAdoptionListing, error) {
			var v exportedAdoptionListing
			err := row.Scan(&v.ID, &v.Name, &v.PetType, &v.Status, &v.CreatedAt)
			return v, err
		})
	if err != nil {
		return internalProblem(c)
	}

	matches, err := queryExport(c, h.db, `
		SELECT m.id,
		       CASE WHEN p1.owner_id = $1 THEN m.pet_low_id ELSE m.pet_high_id END,
		       CASE WHEN p1.owner_id = $1 THEN m.pet_high_id ELSE m.pet_low_id END,
		       m.status, m.matched_at
		FROM matches m
		JOIN pets p1 ON p1.id = m.pet_low_id
		JOIN pets p2 ON p2.id = m.pet_high_id
		WHERE p1.owner_id = $1 OR p2.owner_id = $1
		ORDER BY m.matched_at`, userID,
		func(row pgx.Rows) (exportedMatch, error) {
			var v exportedMatch
			err := row.Scan(&v.ID, &v.OwnPetID, &v.OtherPetID, &v.Status, &v.MatchedAt)
			return v, err
		})
	if err != nil {
		return internalProblem(c)
	}

	return httpx.OK(c, fiber.Map{
		"exported_at":       time.Now().UTC(),
		"account":           account,
		"pets":              pets,
		"posts":             posts,
		"comments":          comments,
		"stories":           stories,
		"adoption_listings": listings,
		"matches":           matches,
	})
}

// queryExport runs sql (with userID as its one placeholder) and maps each
// row through scan, returning a non-nil empty slice — never null — when
// there are no rows, so every section of the export JSON is consistently
// an array.
func queryExport[T any](c *fiber.Ctx, db *pgxpool.Pool, sql string, userID uuid.UUID, scan func(pgx.Rows) (T, error)) ([]T, error) {
	rows, err := db.Query(c.UserContext(), sql, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := make([]T, 0)
	for rows.Next() {
		item, err := scan(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return results, nil
}
