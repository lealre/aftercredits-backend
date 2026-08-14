package activity

import "github.com/lealre/movies-backend/internal/models"

// MapDbEventToApiEvent hand-copies the domain event onto the wire type, the
// same shape as every other mapper in this codebase.
func MapDbEventToApiEvent(e models.ActivityEvent) Event {
	payload := e.Payload
	if payload == nil {
		payload = map[string]any{}
	}
	return Event{
		Id:        e.Id,
		Seq:       e.Seq,
		GroupId:   e.GroupId,
		GroupName: e.GroupName,
		ActorId:   e.ActorId,
		ActorName: e.ActorName,
		Kind:      e.Kind,
		TitleId:   e.TitleId,
		TitleName: e.TitleName,
		Payload:   payload,
		CreatedAt: e.CreatedAt,
		Read:      e.Read,
	}
}
