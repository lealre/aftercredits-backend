package activity

import "time"

type Event struct {
	Id        string         `json:"id"`
	Seq       int64          `json:"seq"`
	GroupId   string         `json:"groupId"`
	GroupName string         `json:"groupName"`
	ActorId   string         `json:"actorId"`
	ActorName string         `json:"actorName"`
	Kind      string         `json:"kind"`
	TitleId   *string        `json:"titleId,omitempty"`
	TitleName *string        `json:"titleName,omitempty"`
	Payload   map[string]any `json:"payload"`
	CreatedAt time.Time      `json:"createdAt"`
	// Read is per-reader: whether the caller of this request has read this
	// event, so the UI can render one row read while its neighbours stay
	// unread. It is never omitempty — a client cannot tell an absent field
	// from a false one, and "unread" is the state that has to be visible.
	Read bool `json:"read"`
}

// Feed is the feed's own envelope. generics.Page is deliberately not reused:
// Page/Size/TotalPages/TotalResults are offset concepts that mean nothing for a
// keyset cursor, and TotalResults would cost a count(*) over the whole log on
// every poll.
type Feed struct {
	Events     []Event `json:"events"`
	NextBefore *int64  `json:"nextBefore"`
	HasMore    bool    `json:"hasMore"`
}

type UnreadCount struct {
	Unread int64 `json:"unread"`
}
