package domain

import "errors"

var (
	ErrHandNotFound          = errors.New("hand not found")
	ErrHandNotActive         = errors.New("hand is not active")
	ErrSubHandNotFound       = errors.New("sub-hand index not found")
	ErrSubHandNotActive      = errors.New("sub-hand is not active")
	ErrInvalidAction         = errors.New("invalid player action")
	ErrCannotDouble          = errors.New("double down only allowed on initial 2 cards")
	ErrCannotSplit           = errors.New("split only allowed on initial 2 cards of equal rank or value")
	ErrCannotSurrender       = errors.New("surrender only allowed on initial hand before other actions")
	ErrInvalidBetAmount      = errors.New("bet amount must be greater than zero")
	ErrInvalidUserID         = errors.New("invalid user id format")
	ErrUnauthorizedHand      = errors.New("hand does not belong to user")
	ErrDealerAlreadyResolved = errors.New("dealer turn already resolved")
)

