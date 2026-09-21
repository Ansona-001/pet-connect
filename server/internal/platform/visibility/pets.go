// Package visibility resolves what an authenticated viewer may see about
// a pet, given its owner's block and privacy settings — shared by every
// endpoint that serves a single pet's public profile or media (pets.go's
// GET /pets/:petId, and social's pet-scoped posts/reels/stories tabs), so
// they all apply the exact same rule instead of maintaining copies that
// could quietly drift apart. This is deliberately its own package rather
// than a modules/pets export: sibling module packages (pets, social, ...)
// never import one another in this codebase, only internal/platform/*.
package visibility

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound means the pet doesn't exist, is soft-deleted/inactive to
// this viewer, or — indistinguishably, on purpose — its owner and the
// viewer have blocked each other. A block's existence must not be
// detectable from response shape (the same enumeration-safety reasoning
// as auth's generic errors and profile.go's block handling), so callers
// should respond identically (404) whether the pet never existed or a
// block is hiding it.
var ErrNotFound = errors.New("pet not visible to this viewer")

// PetAccess is what a viewer may see about petID.
type PetAccess struct {
	OwnerID uuid.UUID
	IsOwner bool
	// Restricted is true when the owner has a private account, the
	// viewer isn't the owner, and the viewer doesn't follow this pet —
	// callers should serve a teaser (identity fields only) rather than
	// the full profile/media in this case.
	Restricted bool
}

// ForPet resolves viewer's access to petID. Every caller should treat a
// returned ErrNotFound the same way regardless of the underlying reason.
func ForPet(ctx context.Context, db *pgxpool.Pool, petID, viewerID uuid.UUID) (PetAccess, error) {
	var ownerID uuid.UUID
	var isPrivate, blocked, following bool
	err := db.QueryRow(ctx, `
		SELECT p.owner_id, u.is_private,
		       EXISTS (
		         SELECT 1 FROM blocks b
		         WHERE (b.blocker_user_id = $2 AND b.blocked_user_id = p.owner_id)
		            OR (b.blocker_user_id = p.owner_id AND b.blocked_user_id = $2)
		       ),
		       EXISTS (SELECT 1 FROM follows f WHERE f.user_id = $2 AND f.pet_id = p.id)
		FROM pets p
		JOIN users u ON u.id = p.owner_id AND u.deleted_at IS NULL
		WHERE p.id = $1 AND p.deleted_at IS NULL
		  AND (p.owner_id = $2 OR p.status = 'active')`,
		petID, viewerID).Scan(&ownerID, &isPrivate, &blocked, &following)
	if errors.Is(err, pgx.ErrNoRows) {
		return PetAccess{}, ErrNotFound
	}
	if err != nil {
		return PetAccess{}, err
	}
	if blocked {
		return PetAccess{}, ErrNotFound
	}

	isOwner := ownerID == viewerID
	return PetAccess{
		OwnerID:    ownerID,
		IsOwner:    isOwner,
		Restricted: isPrivate && !isOwner && !following,
	}, nil
}
