package models

import "time"

// ActivityEvent is one recorded action in a group: who did what, to which
// title, when. It is append-only — nothing updates or deletes a row once
// written.
//
// ActorName and TitleName are denormalized copies taken at record time so the
// row stays readable after the actor leaves the group or the title is removed
// from the catalogue. Seq is the ordering and cursor key; it is unique, so
// ordering by it alone is total.
//
// Read is the one field that is not a property of the event: it says whether
// the reader who asked for it has read it, so the same row is Read for one
// member and unread for another. It is filled by the read paths only — writes
// ignore it, and an event freshly pushed to the stream is unread by definition.
type ActivityEvent struct {
	Id        string
	Seq       int64
	GroupId   string
	GroupName string
	ActorId   string
	ActorName string
	Kind      string
	TitleId   *string
	TitleName *string
	Payload   map[string]any
	CreatedAt time.Time
	Read      bool
}
