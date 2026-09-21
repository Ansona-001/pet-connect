package visibility

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// UserAccess is what a viewer may see about a user's own profile-level
// resources (their followers/following lists, today — see
// modules/profile). Mirrors PetAccess's shape and reasoning, but there is
// no follow-based exception here the way PetAccess has one: following one
// of a private owner's pets doesn't unlock the owner's own profile-level
// lists, only that one pet's profile/media (see profile.getPublicProfile,
// which applies this same is_private-only rule inline for the profile
// endpoint itself).
type UserAccess struct {
	IsSelf     bool
	Restricted bool
}

// ForUser resolves viewer's access to targetUserID. Returns ErrNotFound
// (the same sentinel PetAccess uses) both when the user doesn't exist and
// when a block in either direction is hiding them — deliberately
// indistinguishable, so a block's existence can't be detected from
// response shape.
func ForUser(ctx context.Context, db *pgxpool.Pool, targetUserID, viewerID uuid.UUID) (UserAccess, error) {
	var isPrivate bool
	err := db.QueryRow(ctx, `
		SELECT u.is_private
		FROM users u
		WHERE u.id = $1 AND u.deleted_at IS NULL
		  AND NOT EXISTS (
		    SELECT 1 FROM blocks b
		    WHERE (b.blocker_user_id = $2 AND b.blocked_user_id = u.id)
		       OR (b.blocker_user_id = u.id AND b.blocked_user_id = $2)
		  )`, targetUserID, viewerID).Scan(&isPrivate)
	if errors.Is(err, pgx.ErrNoRows) {
		return UserAccess{}, ErrNotFound
	}
	if err != nil {
		return UserAccess{}, err
	}

	isSelf := targetUserID == viewerID
	return UserAccess{IsSelf: isSelf, Restricted: isPrivate && !isSelf}, nil
}
