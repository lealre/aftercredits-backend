package activity

import (
	"errors"
	"net/http"
)

// ErrEventNotFound is the single answer for "you cannot mark that read": the
// id belongs to no event, to a group the caller is not in, or to an action the
// caller performed themselves. A 404 for all three is deliberate — a 403 for
// the second would confirm that an event with that id exists somewhere.
var ErrEventNotFound = errors.New("activity event not found")

// ErrInvalidTicket covers an unknown, already-redeemed, or expired stream
// ticket alike: activity.TicketStore.Redeem collapses all three into a single
// (userId, ok) result, and the caller (the SSE handler in a later task) has
// no way to tell them apart either, so there is nothing more specific to
// report to the client than "not authorized."
var ErrInvalidTicket = errors.New("ticket is invalid, expired, or already used")

var ErrorMap = map[error]int{
	ErrEventNotFound: http.StatusNotFound,
	ErrInvalidTicket: http.StatusUnauthorized,
}
