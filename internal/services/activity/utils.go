package activity

import (
	"errors"
	"net/http"
)

var ErrInvalidSeq = errors.New("seq must be a positive number")

// ErrInvalidTicket covers an unknown, already-redeemed, or expired stream
// ticket alike: activity.TicketStore.Redeem collapses all three into a single
// (userId, ok) result, and the caller (the SSE handler in a later task) has
// no way to tell them apart either, so there is nothing more specific to
// report to the client than "not authorized."
var ErrInvalidTicket = errors.New("ticket is invalid, expired, or already used")

var ErrorMap = map[error]int{
	ErrInvalidSeq:    http.StatusBadRequest,
	ErrInvalidTicket: http.StatusUnauthorized,
}
