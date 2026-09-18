package matching

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	pool      *pgxpool.Pool
	publisher EventPublisher
}

func NewService(pool *pgxpool.Pool, publisher EventPublisher) *Service {
	return &Service{pool: pool, publisher: publisher}
}

func (s *Service) Candidates(ctx context.Context, userID, sourcePetID uuid.UUID, filter CandidateFilter) ([]Candidate, error) {
	if err := s.ensurePetOwner(ctx, userID, sourcePetID); err != nil {
		return nil, err
	}
	filter.PetType = strings.TrimSpace(filter.PetType)
	filter.Breed = strings.TrimSpace(filter.Breed)
	filter.Gender = strings.TrimSpace(filter.Gender)
	if filter.MaxDistanceKM < 0 {
		return nil, fmt.Errorf("%w: max distance cannot be negative", ErrInvalidSwipe)
	}
	if filter.Limit <= 0 {
		filter.Limit = 20
	}
	if filter.Limit > 50 {
		filter.Limit = 50
	}

	rows, err := s.pool.Query(ctx, `
		SELECT target.id,
		       target.owner_id,
		       owner.name,
		       target.name,
		       target.pet_type,
		       target.breed,
		       target.age_label,
		       target.gender,
		       target.bio,
		       target.personality,
		       target.interests,
		       target.primary_image_url,
		       target.is_verified,
		       COALESCE(ST_Distance(source.location, target.location) / 1000.0, -1) AS distance_km
		FROM pets source
		JOIN pets target ON target.id <> source.id
		JOIN users owner ON owner.id = target.owner_id AND owner.deleted_at IS NULL
		WHERE source.id = $1
		  AND source.owner_id = $2
		  AND source.deleted_at IS NULL
		  AND target.owner_id <> $2
		  AND target.deleted_at IS NULL
		  AND target.status = 'active'
		  AND ($3 = '' OR lower(target.pet_type) = lower($3))
		  AND ($4 = '' OR lower(target.breed) = lower($4))
		  AND ($5 = '' OR lower(target.gender) = lower($5))
		  AND ($6::double precision = 0 OR (
		        source.location IS NOT NULL
		        AND target.location IS NOT NULL
		        AND ST_DWithin(source.location, target.location, $6 * 1000.0)
		      ))
		  AND NOT EXISTS (
		        SELECT 1
		        FROM pet_swipes swipe
		        WHERE swipe.source_pet_id = source.id
		          AND swipe.target_pet_id = target.id
		      )
		  -- Enforced both directions — see internal/modules/social's feed
		  -- query for the same pattern and reasoning.
		  AND NOT EXISTS (
		        SELECT 1 FROM blocks bl
		        WHERE (bl.blocker_user_id = $2 AND bl.blocked_user_id = target.owner_id)
		           OR (bl.blocker_user_id = target.owner_id AND bl.blocked_user_id = $2)
		      )
		ORDER BY
		  CASE WHEN source.location IS NULL OR target.location IS NULL THEN 1 ELSE 0 END,
		  ST_Distance(source.location, target.location),
		  target.created_at DESC,
		  target.id
		LIMIT $7`, sourcePetID, userID, filter.PetType, filter.Breed, filter.Gender, filter.MaxDistanceKM, filter.Limit)
	if err != nil {
		return nil, fmt.Errorf("query match candidates: %w", err)
	}
	defer rows.Close()

	candidates := make([]Candidate, 0, filter.Limit)
	for rows.Next() {
		var candidate Candidate
		var distance float64
		if err := rows.Scan(
			&candidate.ID,
			&candidate.OwnerID,
			&candidate.OwnerName,
			&candidate.Name,
			&candidate.PetType,
			&candidate.Breed,
			&candidate.AgeLabel,
			&candidate.Gender,
			&candidate.Bio,
			&candidate.Personality,
			&candidate.Interests,
			&candidate.PrimaryImageURL,
			&candidate.IsVerified,
			&distance,
		); err != nil {
			return nil, fmt.Errorf("scan match candidate: %w", err)
		}
		if distance >= 0 {
			candidate.DistanceKM = &distance
		}
		candidates = append(candidates, candidate)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate match candidates: %w", err)
	}
	return candidates, nil
}

func (s *Service) Swipe(ctx context.Context, userID uuid.UUID, request SwipeRequest) (SwipeResult, error) {
	if userID == uuid.Nil || request.SourcePetID == uuid.Nil || request.TargetPetID == uuid.Nil || request.ClientRequestID == uuid.Nil {
		return SwipeResult{}, fmt.Errorf("%w: identifiers are required", ErrInvalidSwipe)
	}
	if request.SourcePetID == request.TargetPetID || !request.Decision.Valid() {
		return SwipeResult{}, fmt.Errorf("%w: invalid target or decision", ErrInvalidSwipe)
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return SwipeResult{}, fmt.Errorf("begin swipe transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	pets, err := lockPetPair(ctx, tx, request.SourcePetID, request.TargetPetID)
	if err != nil {
		return SwipeResult{}, err
	}
	source, sourceOK := pets[request.SourcePetID]
	target, targetOK := pets[request.TargetPetID]
	if !sourceOK || !targetOK || source.deleted || target.deleted || source.status != "active" || target.status != "active" {
		return SwipeResult{}, ErrPetNotFound
	}
	if source.ownerID != userID {
		return SwipeResult{}, ErrForbidden
	}
	if source.ownerID == target.ownerID {
		return SwipeResult{}, fmt.Errorf("%w: cannot swipe another pet owned by the same user", ErrInvalidSwipe)
	}

	if existing, found, err := swipeByRequestID(ctx, tx, request.SourcePetID, request.ClientRequestID); err != nil {
		return SwipeResult{}, err
	} else if found {
		if existing.targetPetID != request.TargetPetID || existing.decision != request.Decision {
			return SwipeResult{}, ErrIdempotencyConflict
		}
		result, err := swipeResult(ctx, tx, existing.id, existing.decision, request.SourcePetID, request.TargetPetID)
		if err != nil {
			return SwipeResult{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return SwipeResult{}, fmt.Errorf("commit idempotent swipe: %w", err)
		}
		return result, nil
	}

	if existing, found, err := swipeByTarget(ctx, tx, request.SourcePetID, request.TargetPetID); err != nil {
		return SwipeResult{}, err
	} else if found {
		if existing.decision != request.Decision {
			return SwipeResult{}, ErrAlreadySwiped
		}
		result, err := swipeResult(ctx, tx, existing.id, existing.decision, request.SourcePetID, request.TargetPetID)
		if err != nil {
			return SwipeResult{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return SwipeResult{}, fmt.Errorf("commit duplicate swipe: %w", err)
		}
		return result, nil
	}

	var swipeID uuid.UUID
	if err := tx.QueryRow(ctx, `
		INSERT INTO pet_swipes(source_pet_id, target_pet_id, decision, client_request_id)
		VALUES($1, $2, $3, $4)
		RETURNING id`, request.SourcePetID, request.TargetPetID, request.Decision, request.ClientRequestID).Scan(&swipeID); err != nil {
		return SwipeResult{}, fmt.Errorf("insert pet swipe: %w", err)
	}

	result := SwipeResult{SwipeID: swipeID, Decision: string(request.Decision)}
	var notifyUsers []uuid.UUID
	if request.Decision.Interested() {
		var reciprocal bool
		if err := tx.QueryRow(ctx, `
			SELECT EXISTS(
			  SELECT 1
			  FROM pet_swipes
			  WHERE source_pet_id = $1
			    AND target_pet_id = $2
			    AND decision IN ('like', 'super_like')
			)`, request.TargetPetID, request.SourcePetID).Scan(&reciprocal); err != nil {
			return SwipeResult{}, fmt.Errorf("check reciprocal swipe: %w", err)
		}
		if reciprocal {
			match, created, err := createOrLoadMatch(ctx, tx, source, target)
			if err != nil {
				return SwipeResult{}, err
			}
			if match.Status == "active" {
				result.Matched = true
				result.Match = &match
				if created {
					notifyUsers = []uuid.UUID{source.ownerID, target.ownerID}
					for _, recipientID := range notifyUsers {
						if _, err := tx.Exec(ctx, `
							INSERT INTO notifications(user_id, notification_type, payload)
							VALUES($1, 'match', jsonb_build_object(
							  'match_id', $2::text,
							  'chat_id', $3::text,
							  'source_pet_id', $4::text,
							  'target_pet_id', $5::text
							))`, recipientID, match.ID, match.ChatID, source.id, target.id); err != nil {
							return SwipeResult{}, fmt.Errorf("create match notification: %w", err)
						}
					}
				}
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return SwipeResult{}, fmt.Errorf("commit swipe transaction: %w", err)
	}
	if len(notifyUsers) > 0 && s.publisher != nil {
		_ = s.publisher.PublishToUsers(notifyUsers, "match.created", result.Match)
	}
	return result, nil
}

func (s *Service) ListMatches(ctx context.Context, userID uuid.UUID, limit int) ([]Match, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}
	rows, err := s.pool.Query(ctx, `
		SELECT match.id,
		       chat.id,
		       match.status,
		       match.matched_at,
		       own_pet.id,
		       own_pet.name,
		       other_pet.id,
		       other_pet.name,
		       other_pet.pet_type,
		       other_pet.breed,
		       other_pet.age_label,
		       other_pet.gender,
		       other_pet.primary_image_url,
		       other_owner.id,
		       other_owner.name
		FROM matches match
		JOIN chats chat ON chat.match_id = match.id
		JOIN pets own_pet
		  ON own_pet.owner_id = $1
		 AND own_pet.id IN (match.pet_low_id, match.pet_high_id)
		JOIN pets other_pet
		  ON other_pet.id = CASE
		       WHEN match.pet_low_id = own_pet.id THEN match.pet_high_id
		       ELSE match.pet_low_id
		     END
		JOIN users other_owner ON other_owner.id = other_pet.owner_id
		WHERE match.status = 'active'
		  AND own_pet.deleted_at IS NULL
		  AND other_pet.deleted_at IS NULL
		  AND other_owner.deleted_at IS NULL
		ORDER BY match.matched_at DESC, match.id DESC
		LIMIT $2`, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("list matches: %w", err)
	}
	defer rows.Close()

	matches := make([]Match, 0, limit)
	for rows.Next() {
		var match Match
		if err := rows.Scan(
			&match.ID,
			&match.ChatID,
			&match.Status,
			&match.MatchedAt,
			&match.SourcePetID,
			&match.SourcePetName,
			&match.OtherPetID,
			&match.OtherPetName,
			&match.OtherPetType,
			&match.OtherPetBreed,
			&match.OtherPetAge,
			&match.OtherPetGender,
			&match.OtherPetImage,
			&match.OtherOwnerID,
			&match.OtherOwnerName,
		); err != nil {
			return nil, fmt.Errorf("scan match: %w", err)
		}
		matches = append(matches, match)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate matches: %w", err)
	}
	return matches, nil
}

func (s *Service) ensurePetOwner(ctx context.Context, userID, petID uuid.UUID) error {
	var ownerID uuid.UUID
	var status string
	if err := s.pool.QueryRow(ctx, `
		SELECT owner_id, status
		FROM pets
		WHERE id = $1 AND deleted_at IS NULL`, petID).Scan(&ownerID, &status); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrPetNotFound
		}
		return fmt.Errorf("load source pet: %w", err)
	}
	if ownerID != userID {
		return ErrForbidden
	}
	if status != "active" {
		return ErrPetNotFound
	}
	return nil
}

type lockedPet struct {
	id      uuid.UUID
	ownerID uuid.UUID
	name    string
	status  string
	deleted bool
}

func lockPetPair(ctx context.Context, tx pgx.Tx, first, second uuid.UUID) (map[uuid.UUID]lockedPet, error) {
	rows, err := tx.Query(ctx, `
		SELECT id, owner_id, name, status, deleted_at IS NOT NULL
		FROM pets
		WHERE id IN ($1, $2)
		ORDER BY id
		FOR UPDATE`, first, second)
	if err != nil {
		return nil, fmt.Errorf("lock swipe pets: %w", err)
	}
	defer rows.Close()
	result := make(map[uuid.UUID]lockedPet, 2)
	for rows.Next() {
		var pet lockedPet
		if err := rows.Scan(&pet.id, &pet.ownerID, &pet.name, &pet.status, &pet.deleted); err != nil {
			return nil, fmt.Errorf("scan locked pet: %w", err)
		}
		result[pet.id] = pet
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate locked pets: %w", err)
	}
	return result, nil
}

type storedSwipe struct {
	id          uuid.UUID
	targetPetID uuid.UUID
	decision    SwipeDecision
}

func swipeByRequestID(ctx context.Context, tx pgx.Tx, sourcePetID, requestID uuid.UUID) (storedSwipe, bool, error) {
	var swipe storedSwipe
	err := tx.QueryRow(ctx, `
		SELECT id, target_pet_id, decision
		FROM pet_swipes
		WHERE source_pet_id = $1 AND client_request_id = $2`, sourcePetID, requestID).
		Scan(&swipe.id, &swipe.targetPetID, &swipe.decision)
	if errors.Is(err, pgx.ErrNoRows) {
		return storedSwipe{}, false, nil
	}
	if err != nil {
		return storedSwipe{}, false, fmt.Errorf("load swipe idempotency key: %w", err)
	}
	return swipe, true, nil
}

func swipeByTarget(ctx context.Context, tx pgx.Tx, sourcePetID, targetPetID uuid.UUID) (storedSwipe, bool, error) {
	var swipe storedSwipe
	err := tx.QueryRow(ctx, `
		SELECT id, target_pet_id, decision
		FROM pet_swipes
		WHERE source_pet_id = $1 AND target_pet_id = $2`, sourcePetID, targetPetID).
		Scan(&swipe.id, &swipe.targetPetID, &swipe.decision)
	if errors.Is(err, pgx.ErrNoRows) {
		return storedSwipe{}, false, nil
	}
	if err != nil {
		return storedSwipe{}, false, fmt.Errorf("load existing target swipe: %w", err)
	}
	return swipe, true, nil
}

func swipeResult(ctx context.Context, tx pgx.Tx, swipeID uuid.UUID, decision SwipeDecision, sourcePetID, targetPetID uuid.UUID) (SwipeResult, error) {
	result := SwipeResult{SwipeID: swipeID, Decision: string(decision)}
	match, found, err := loadActiveMatch(ctx, tx, sourcePetID, targetPetID)
	if err != nil {
		return SwipeResult{}, err
	}
	if found {
		result.Matched = true
		result.Match = &match
	}
	return result, nil
}

func createOrLoadMatch(ctx context.Context, tx pgx.Tx, source, target lockedPet) (Match, bool, error) {
	var matchID uuid.UUID
	var matchedAt time.Time
	err := tx.QueryRow(ctx, `
		INSERT INTO matches(pet_low_id, pet_high_id)
		VALUES(LEAST($1::uuid, $2::uuid), GREATEST($1::uuid, $2::uuid))
		ON CONFLICT (pet_low_id, pet_high_id) DO NOTHING
		RETURNING id, matched_at`, source.id, target.id).Scan(&matchID, &matchedAt)
	created := err == nil
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return Match{}, false, fmt.Errorf("create reciprocal match: %w", err)
	}

	var status string
	if !created {
		if err := tx.QueryRow(ctx, `
			SELECT id, status, matched_at
			FROM matches
			WHERE pet_low_id = LEAST($1::uuid, $2::uuid)
			  AND pet_high_id = GREATEST($1::uuid, $2::uuid)`, source.id, target.id).
			Scan(&matchID, &status, &matchedAt); err != nil {
			return Match{}, false, fmt.Errorf("load reciprocal match: %w", err)
		}
	} else {
		status = "active"
	}

	var chatID uuid.UUID
	if status == "active" {
		if err := tx.QueryRow(ctx, `
			INSERT INTO chats(match_id)
			VALUES($1)
			ON CONFLICT (match_id) DO UPDATE SET match_id = EXCLUDED.match_id
			RETURNING id`, matchID).Scan(&chatID); err != nil {
			return Match{}, false, fmt.Errorf("create match chat: %w", err)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO chat_members(chat_id, user_id)
			VALUES($1, $2), ($1, $3)
			ON CONFLICT DO NOTHING`, chatID, source.ownerID, target.ownerID); err != nil {
			return Match{}, false, fmt.Errorf("create match chat members: %w", err)
		}
	}

	if status == "active" {
		match, found, err := loadActiveMatch(ctx, tx, source.id, target.id)
		if err != nil {
			return Match{}, false, err
		}
		if !found {
			return Match{}, false, fmt.Errorf("load newly created match: %w", pgx.ErrNoRows)
		}
		return match, created, nil
	}
	return Match{
		ID: matchID, ChatID: chatID, Status: status, MatchedAt: matchedAt,
		SourcePetID: source.id, SourcePetName: source.name,
		OtherPetID: target.id, OtherPetName: target.name, OtherOwnerID: target.ownerID,
	}, created, nil
}

func loadActiveMatch(ctx context.Context, tx pgx.Tx, sourcePetID, targetPetID uuid.UUID) (Match, bool, error) {
	var match Match
	err := tx.QueryRow(ctx, `
		SELECT match.id, chat.id, match.status, match.matched_at,
		       source.id, source.name,
		       target.id, target.name, target.pet_type, target.breed,
		       target.age_label, target.gender, target.primary_image_url,
		       owner.id, owner.name
		FROM matches match
		JOIN chats chat ON chat.match_id = match.id
		JOIN pets source ON source.id = $1
		JOIN pets target ON target.id = $2
		JOIN users owner ON owner.id = target.owner_id
		WHERE match.pet_low_id = LEAST($1::uuid, $2::uuid)
		  AND match.pet_high_id = GREATEST($1::uuid, $2::uuid)
		  AND match.status = 'active'`, sourcePetID, targetPetID).Scan(
		&match.ID, &match.ChatID, &match.Status, &match.MatchedAt,
		&match.SourcePetID, &match.SourcePetName,
		&match.OtherPetID, &match.OtherPetName, &match.OtherPetType, &match.OtherPetBreed,
		&match.OtherPetAge, &match.OtherPetGender, &match.OtherPetImage,
		&match.OtherOwnerID, &match.OtherOwnerName,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Match{}, false, nil
	}
	if err != nil {
		return Match{}, false, fmt.Errorf("load active match: %w", err)
	}
	return match, true, nil
}
