package matching

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrPetNotFound         = errors.New("pet not found")
	ErrForbidden           = errors.New("pet does not belong to the authenticated user")
	ErrInvalidSwipe        = errors.New("invalid swipe")
	ErrAlreadySwiped       = errors.New("the target pet has already been swiped")
	ErrIdempotencyConflict = errors.New("the client request id was reused for different input")
)

type CandidateFilter struct {
	PetType       string
	Breed         string
	Gender        string
	MaxDistanceKM float64
	Limit         int
}

type Candidate struct {
	ID              uuid.UUID `json:"id"`
	OwnerID         uuid.UUID `json:"owner_id"`
	OwnerName       string    `json:"owner_name"`
	Name            string    `json:"name"`
	PetType         string    `json:"pet_type"`
	Breed           string    `json:"breed"`
	AgeLabel        string    `json:"age_label"`
	Gender          string    `json:"gender"`
	Bio             string    `json:"bio"`
	Personality     []string  `json:"personality"`
	Interests       []string  `json:"interests"`
	PrimaryImageURL string    `json:"primary_image_url"`
	DistanceKM      *float64  `json:"distance_km,omitempty"`
	IsVerified      bool      `json:"is_verified"`
}

type SwipeDecision string

const (
	SwipeSkip      SwipeDecision = "skip"
	SwipeLike      SwipeDecision = "like"
	SwipeSuperLike SwipeDecision = "super_like"
)

func (d SwipeDecision) Valid() bool {
	return d == SwipeSkip || d == SwipeLike || d == SwipeSuperLike
}

func (d SwipeDecision) Interested() bool {
	return d == SwipeLike || d == SwipeSuperLike
}

type SwipeRequest struct {
	SourcePetID     uuid.UUID
	TargetPetID     uuid.UUID
	Decision        SwipeDecision
	ClientRequestID uuid.UUID
}

type SwipeResult struct {
	SwipeID  uuid.UUID `json:"swipe_id"`
	Decision string    `json:"decision"`
	Matched  bool      `json:"matched"`
	Match    *Match    `json:"match,omitempty"`
}

type Match struct {
	ID             uuid.UUID `json:"id"`
	ChatID         uuid.UUID `json:"chat_id"`
	Status         string    `json:"status"`
	MatchedAt      time.Time `json:"matched_at"`
	SourcePetID    uuid.UUID `json:"source_pet_id"`
	SourcePetName  string    `json:"source_pet_name"`
	OtherPetID     uuid.UUID `json:"other_pet_id"`
	OtherPetName   string    `json:"other_pet_name"`
	OtherPetType   string    `json:"other_pet_type"`
	OtherPetBreed  string    `json:"other_pet_breed"`
	OtherPetAge    string    `json:"other_pet_age_label"`
	OtherPetGender string    `json:"other_pet_gender"`
	OtherPetImage  string    `json:"other_pet_image_url"`
	OtherOwnerID   uuid.UUID `json:"other_owner_id"`
	OtherOwnerName string    `json:"other_owner_name"`
}

type EventPublisher interface {
	PublishToUsers(userIDs []uuid.UUID, eventType string, payload any) error
}
