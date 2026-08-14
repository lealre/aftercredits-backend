package tests

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/lealre/movies-backend/internal/auth"
	"github.com/lealre/movies-backend/internal/models"
	"github.com/lealre/movies-backend/internal/services/activity"
	"github.com/lealre/movies-backend/internal/services/groups"
	"github.com/lealre/movies-backend/internal/services/users"
	"github.com/stretchr/testify/require"
)

// activityStreamTestTimeout bounds every stream connection opened in this
// file: a stream that never responds, or a read that never delivers, fails
// the subtest it belongs to instead of hanging the whole suite.
const activityStreamTestTimeout = 10 * time.Second

// activityRow is one row of the activity log, read straight from the database
// so emit-site tests do not depend on the feed API existing yet.
type activityRow struct {
	Kind      string
	GroupId   string
	ActorId   string
	ActorName string
	TitleName *string
	Payload   map[string]any
}

// getActivityRows returns every recorded event, oldest first.
func getActivityRows(t *testing.T) []activityRow {
	t.Helper()
	rows, err := testPool.Query(context.Background(),
		`SELECT kind, group_id, actor_id, actor_name, title_name, payload
		 FROM activity_events ORDER BY seq`)
	require.NoError(t, err, "failed to read the activity log")
	defer rows.Close()

	out := []activityRow{}
	for rows.Next() {
		var r activityRow
		require.NoError(t, rows.Scan(&r.Kind, &r.GroupId, &r.ActorId, &r.ActorName, &r.TitleName, &r.Payload))
		out = append(out, r)
	}
	require.NoError(t, rows.Err())
	return out
}

// lastActivityPayload returns the payload of the most recently recorded event,
// asserting its kind first — every watched-payload case ends the same way, in
// "the PATCH I just sent produced this payload".
func lastActivityPayload(t *testing.T, kind string) map[string]any {
	t.Helper()

	rows := getActivityRows(t)
	require.NotEmpty(t, rows, "expected at least one recorded event")
	last := rows[len(rows)-1]
	require.Equal(t, kind, last.Kind, "the last recorded event must be a %s", kind)
	return last.Payload
}

// activityPayloadTime reads a timestamp out of an event payload.
//
// It insists the value is an RFC 3339 string rather than accepting whatever
// parses: the payload's whole purpose is to be rendered by a browser, and
// Date.parse only reliably handles that format. Passing back a time.Time lets
// callers compare instants instead of text, so a round trip through
// timestamptz that changes the zone offset but not the moment still passes.
func activityPayloadTime(t *testing.T, payload map[string]any, key string) time.Time {
	t.Helper()

	raw, ok := payload[key]
	require.True(t, ok, "the payload must carry %q, got %v", key, payload)
	text, ok := raw.(string)
	require.True(t, ok, "%q must be a string, got %T", key, raw)

	parsed, err := time.Parse(time.RFC3339, text)
	require.NoError(t, err, "%q must be an RFC 3339 timestamp a browser can parse, got %q", key, text)
	return parsed
}

// activityWatchedFixture builds the common ground every watched-payload case
// starts from: a user, a group, and one title of theirs already in it. It
// returns the title's id, the group's id and the caller's token, plus the id of
// the title, so a case can go straight to the transition it is about.
func activityWatchedFixture(t *testing.T, username string, series bool) (groupId, titleId, token string) {
	t.Helper()

	_, token = addUser(t, users.NewUserRequest{Username: username, Password: "pass"})
	group := createGroup(t, groups.CreateGroupRequest{Name: "feed group"}, token)

	fixture := loadTitlesFixture
	if series {
		fixture = loadTVSeriesTitlesFixture
	}
	catalogue := fixture(t)
	seedTitles(t, catalogue)
	title := catalogue[0]

	addTitleToGroup(t, groups.AddTitleToGroupRequest{
		URL:     fmt.Sprintf("https://www.imdb.com/title/%s/", title.ID),
		GroupId: group.Id,
	}, token)

	return group.Id, title.ID, token
}

// countActivityRows reports how many activity_events rows exist, for the
// plug-out test, which only needs to know the log stayed empty.
func countActivityRows(t *testing.T) int {
	t.Helper()
	var n int
	require.NoError(t, testPool.QueryRow(context.Background(),
		"SELECT count(*) FROM activity_events").Scan(&n),
		"failed to count the activity log")
	return n
}

// seedTwoActivityTitles seeds the movie fixture and returns its first two ids,
// for feed tests that only need "a title" and "a second title" to add to a
// group without caring which ones they are.
func seedTwoActivityTitles(t *testing.T) (testTitleId, secondTestTitleId string) {
	t.Helper()

	titles := loadTitlesFixture(t)
	require.GreaterOrEqual(t, len(titles), 2, "the movie fixture must carry at least two titles")
	seedTitles(t, titles)
	return titles[0].ID, titles[1].ID
}

// seededTitleIds seeds n distinct, synthetic movie titles directly (via
// newSortableMovieTitle/seedTitles, the same helpers the sort/pagination tests
// use) and returns their ids. The committed fixture only carries 5 movies + 2
// TV series, short of what a cursor-pagination test needs, so this builds as
// many distinct titles as required instead of depending on fixture size.
func seededTitleIds(t *testing.T, n int) []string {
	t.Helper()

	titles := make([]models.Title, 0, n)
	ids := make([]string, 0, n)
	for i := range n {
		title := newSortableMovieTitle(
			fmt.Sprintf("tt988%05d", i),
			fmt.Sprintf("Activity Feed Title %d", i),
			2000+i%5,
			float64(5+i%4),
			100*(i%5),
			nil,
		)
		titles = append(titles, title)
		ids = append(ids, title.ID)
	}
	seedTitles(t, titles)
	return ids
}

// getActivityFeedResponse calls GET /activity with a raw query string
// (including the leading "?", or "" for none) and returns the response for the
// caller to assert on.
func getActivityFeedResponse(t *testing.T, token, query string) *http.Response {
	t.Helper()

	req, err := http.NewRequest(http.MethodGet, testServer.URL+"/activity"+query, nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	return resp
}

// getActivityFeed decodes a successful GET /activity response.
func getActivityFeed(t *testing.T, token, query string) activity.Feed {
	t.Helper()

	resp := getActivityFeedResponse(t, token, query)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var feed activity.Feed
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&feed))
	return feed
}

// getActivityFeedRawBody returns the response body as a string. Needed because
// a JSON `null` and a `[]` both decode into an empty Go slice, so the
// difference is only observable before decoding.
func getActivityFeedRawBody(t *testing.T, token, query string) string {
	t.Helper()

	resp := getActivityFeedResponse(t, token, query)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return string(body)
}

// getActivityUnreadCountResponse calls GET /activity/unread-count and returns
// the response for the caller to assert on.
func getActivityUnreadCountResponse(t *testing.T, token string) *http.Response {
	t.Helper()

	req, err := http.NewRequest(http.MethodGet, testServer.URL+"/activity/unread-count", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	return resp
}

// getActivityUnreadCount decodes a successful GET /activity/unread-count
// response.
func getActivityUnreadCount(t *testing.T, token string) int64 {
	t.Helper()

	resp := getActivityUnreadCountResponse(t, token)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var count activity.UnreadCount
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&count))
	return count.Unread
}

// activityFeedFixture is what seedActivityFeed hands back: the group, the two
// tokens, the ids of the events the reader can see — numbered oldest first, so
// a case can talk about "event 3" the way the UI's list does — and one title
// that is seeded but not yet in the group, for cases that need to record a
// further event after doing something.
type activityFeedFixture struct {
	groupId      string
	actorToken   string
	readerToken  string
	eventIds     []string
	spareTitleId string
}

// seedActivityFeed puts an actor and a reader in one group and has the actor
// add n titles to it, which is n events in the reader's feed and none in the
// actor's.
func seedActivityFeed(t *testing.T, n int) activityFeedFixture {
	t.Helper()

	_, actorToken := addUser(t, users.NewUserRequest{Username: "actor", Password: "pass"})
	reader, readerToken := addUser(t, users.NewUserRequest{Username: "reader", Password: "pass"})
	group := createGroup(t, groups.CreateGroupRequest{Name: "shared"}, actorToken)
	addUserToGroup(t, groups.AddUserToGroupRequest{UserId: reader.Id}, group.Id, actorToken)

	// One more title than events: seededTitleIds is deterministic, so a case
	// cannot simply seed another one later without colliding with these.
	titleIds := seededTitleIds(t, n+1)
	for _, titleId := range titleIds[:n] {
		addTitleToGroup(t, groups.AddTitleToGroupRequest{
			URL:     fmt.Sprintf("https://www.imdb.com/title/%s/", titleId),
			GroupId: group.Id,
		}, actorToken)
	}

	feed := getActivityFeed(t, readerToken, "?limit=50")
	require.Len(t, feed.Events, n, "the fixture must record exactly one event per title added")

	// The feed is newest first; the ids come back oldest first so event 1 is
	// the oldest, matching how the cases are written.
	ids := make([]string, 0, n)
	for i := len(feed.Events) - 1; i >= 0; i-- {
		ids = append(ids, feed.Events[i].Id)
	}
	return activityFeedFixture{
		groupId:      group.Id,
		actorToken:   actorToken,
		readerToken:  readerToken,
		eventIds:     ids,
		spareTitleId: titleIds[n],
	}
}

// markActivityEventReadResponse calls POST /activity/events/{id}/read and
// returns the response for the caller to assert on.
func markActivityEventReadResponse(t *testing.T, token, eventId string) *http.Response {
	t.Helper()

	req, err := http.NewRequest(http.MethodPost,
		testServer.URL+"/activity/events/"+eventId+"/read", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	return resp
}

// markActivityEventRead marks one event read and asserts it succeeded.
func markActivityEventRead(t *testing.T, token, eventId string) {
	t.Helper()

	resp := markActivityEventReadResponse(t, token, eventId)
	defer resp.Body.Close()
	require.Equal(t, http.StatusNoContent, resp.StatusCode)
}

// markAllActivityRead calls POST /activity/read-all and asserts it succeeded.
func markAllActivityRead(t *testing.T, token string) {
	t.Helper()

	req, err := http.NewRequest(http.MethodPost, testServer.URL+"/activity/read-all", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusNoContent, resp.StatusCode)
}

// activityReadStateById returns each event in token's feed by id with the read
// flag the API reported for it. Per-row read state is a set property — "this
// one moved and the others did not" — so cases assert on the whole map.
func activityReadStateById(t *testing.T, token string) map[string]bool {
	t.Helper()

	state := map[string]bool{}
	for _, event := range getActivityFeed(t, token, "?limit=50").Events {
		state[event.Id] = event.Read
	}
	return state
}

// buildGroupWithTitleAgainst registers a user, logs in, creates a group and
// adds one fixture title to it, all against baseURL rather than the shared
// testServer. It exists only for TestActivityFeedDisabled: that test builds
// its own server (the flag is read at construction time, so t.Setenv has to
// land before the server is built), which means it cannot reuse addUser,
// createGroup et al. — those all target the shared testServer.URL.
func buildGroupWithTitleAgainst(t *testing.T, baseURL string) (groupId, token string) {
	t.Helper()

	client := &http.Client{}

	registerBody, err := json.Marshal(users.NewUserRequest{Username: "offuser", Password: "pass"})
	require.NoError(t, err)
	registerResp, err := client.Post(baseURL+"/users", "application/json", bytes.NewBuffer(registerBody))
	require.NoError(t, err)
	defer registerResp.Body.Close()
	require.Equal(t, http.StatusCreated, registerResp.StatusCode)

	loginBody, err := json.Marshal(auth.LoginRequest{Username: "offuser", Password: "pass"})
	require.NoError(t, err)
	loginResp, err := client.Post(baseURL+"/login", "application/json", bytes.NewBuffer(loginBody))
	require.NoError(t, err)
	defer loginResp.Body.Close()
	require.Equal(t, http.StatusOK, loginResp.StatusCode)
	var login auth.LoginResponse
	require.NoError(t, json.NewDecoder(loginResp.Body).Decode(&login))

	groupBody, err := json.Marshal(groups.CreateGroupRequest{Name: "off group"})
	require.NoError(t, err)
	groupReq, err := http.NewRequest(http.MethodPost, baseURL+"/groups", bytes.NewBuffer(groupBody))
	require.NoError(t, err)
	groupReq.Header.Set("Authorization", "Bearer "+login.AccessToken)
	groupReq.Header.Set("Content-Type", "application/json")
	groupResp, err := client.Do(groupReq)
	require.NoError(t, err)
	defer groupResp.Body.Close()
	require.Equal(t, http.StatusCreated, groupResp.StatusCode)
	var group groups.GroupResponse
	require.NoError(t, json.NewDecoder(groupResp.Body).Decode(&group))

	// Any fixture id works: the off-server was built with the same
	// fixture-backed fake provider, so the fetch-and-insert-on-demand path in
	// titles.AddNewTitle covers it without needing testStore.AddTitle first.
	fixtureTitle := loadTitlesFixture(t)[0]
	titleBody, err := json.Marshal(groups.AddTitleToGroupRequest{
		URL:     "https://www.imdb.com/title/" + fixtureTitle.ID + "/",
		GroupId: group.Id,
	})
	require.NoError(t, err)
	titleReq, err := http.NewRequest(http.MethodPost, baseURL+"/groups/titles", bytes.NewBuffer(titleBody))
	require.NoError(t, err)
	titleReq.Header.Set("Authorization", "Bearer "+login.AccessToken)
	titleReq.Header.Set("Content-Type", "application/json")
	titleResp, err := client.Do(titleReq)
	require.NoError(t, err)
	defer titleResp.Body.Close()
	require.Equal(t, http.StatusOK, titleResp.StatusCode,
		"the mutating request under test must actually succeed, or the empty log proves nothing")

	return group.Id, login.AccessToken
}

// mintStreamTicket calls POST /activity/stream-ticket for token and returns
// the decoded ticket, asserting the mint succeeded.
func mintStreamTicket(t *testing.T, token string) activity.StreamTicket {
	t.Helper()

	req, err := http.NewRequest(http.MethodPost, testServer.URL+"/activity/stream-ticket", nil)
	require.NoError(t, err, "failed to build the ticket request")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err, "the ticket request failed")
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode, "minting a ticket must succeed for an authenticated caller")

	var ticket activity.StreamTicket
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&ticket), "the ticket response must be JSON")
	return ticket
}

// openActivityStreamWithTicket connects GET /activity/stream with an
// already-minted ticket and returns the raw response for the caller to assert
// on. The request carries a bounded context deadline, cancelled by cleanup —
// which also tears down the connection — so a stream that never responds
// fails the subtest instead of hanging the suite, and no stream goroutine
// outlives it.
func openActivityStreamWithTicket(t *testing.T, ticket string) *http.Response {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), activityStreamTestTimeout)
	t.Cleanup(cancel)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		testServer.URL+"/activity/stream?ticket="+ticket, nil)
	require.NoError(t, err, "failed to build the stream request")

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err, "the stream request failed")
	t.Cleanup(func() { resp.Body.Close() })

	return resp
}

// activityStream is one open, successfully-authenticated SSE connection: its
// response (for header assertions) and a buffered reader over its body (for
// reading frames).
type activityStream struct {
	resp *http.Response
	r    *bufio.Reader
}

// openActivityStream mints a ticket for token via the real endpoint and opens
// the stream with it, asserting the connection succeeded.
//
// Because StreamActivity registers the subscriber with the hub before it
// writes and flushes the response headers (internal/api/activity_stream_handler.go),
// by the time this call returns — the client's Do only returns once headers
// arrive — the caller is already subscribed. Any insert performed after this
// point is therefore guaranteed visible to it, with no extra synchronization.
func openActivityStream(t *testing.T, token string) *activityStream {
	t.Helper()

	ticket := mintStreamTicket(t, token)
	resp := openActivityStreamWithTicket(t, ticket.Ticket)
	require.Equal(t, http.StatusOK, resp.StatusCode, "a freshly minted ticket must open the stream")

	return &activityStream{resp: resp, r: bufio.NewReader(resp.Body)}
}

// readActivitySSEMessage reads one SSE message — everything up to the blank
// line that terminates it — as raw text, comments included. Bounded by the
// stream's own request context deadline (activityStreamTestTimeout), so a
// message that never arrives fails the test rather than hanging it.
func readActivitySSEMessage(t *testing.T, r *bufio.Reader) string {
	t.Helper()

	var message strings.Builder
	for {
		line, err := r.ReadString('\n')
		require.NoError(t, err, "the stream ended or timed out before a complete message arrived")
		if line == "\n" {
			return message.String()
		}
		message.WriteString(line)
	}
}

// readActivityFrame reads the next activity event, skipping the ":ping"
// keep-alive comments a live stream interleaves with real events (the
// production ping interval is 25s, well outside any bound used in this file,
// so none are expected — but skipping them costs nothing and removes the
// dependency on that timing).
func readActivityFrame(t *testing.T, r *bufio.Reader) string {
	t.Helper()

	for {
		message := readActivitySSEMessage(t, r)
		if !strings.HasPrefix(message, ":") {
			return message
		}
	}
}

// activityFrameData extracts an activity frame's data: line, still as raw
// JSON text — used for the byte-identical comparison against the feed, which
// must not go through a decode/re-encode round trip that could hide a
// difference.
func activityFrameData(t *testing.T, frame string) string {
	t.Helper()

	for line := range strings.SplitSeq(strings.TrimRight(frame, "\n"), "\n") {
		if after, found := strings.CutPrefix(line, "data: "); found {
			return after
		}
	}
	t.Fatalf("no data line in frame %q", frame)
	return ""
}

// activityFrameEvent decodes a frame's data: line into the typed DTO.
func activityFrameEvent(t *testing.T, frame string) activity.Event {
	t.Helper()

	var event activity.Event
	require.NoError(t, json.Unmarshal([]byte(activityFrameData(t, frame)), &event),
		"the data line must be the event DTO as JSON")
	return event
}

// noActivityMessageWithin reports whether nothing arrives on r within d — used
// to assert absence (the actor's own stream, or a non-member's, must receive
// nothing). The read runs on its own goroutine so a negative result never
// blocks past d; if something does arrive after the caller has moved on, the
// goroutine still exits on its own — the buffered channel absorbs the send —
// once the stream's cleanup closes the connection, so nothing here outlives
// its subtest.
func noActivityMessageWithin(t *testing.T, r *bufio.Reader, d time.Duration) bool {
	t.Helper()

	type result struct {
		line string
		err  error
	}
	ch := make(chan result, 1)
	go func() {
		line, err := r.ReadString('\n')
		ch <- result{line, err}
	}()

	select {
	case <-time.After(d):
		return true
	case res := <-ch:
		return res.err != nil
	}
}

// insertHeldActivityEvent inserts one activity event exactly the way
// InsertActivityEvents does it (a row, then a same-transaction pg_notify), but
// through a transaction the caller holds open rather than one that commits
// immediately.
//
// It exists to reproduce the design doc's INSERT-vs-COMMIT ordering caveat
// ("Ordering and cursors"): seq is assigned when the INSERT statement
// executes, not when its transaction commits, so a transaction that inserts
// earlier than another can still commit — and therefore notify and become
// visible — after it. The caller must Commit or Rollback the returned
// transaction.
func insertHeldActivityEvent(t *testing.T, ctx context.Context, groupId, actorId, actorName, kind string) (tx pgx.Tx, eventId string) {
	t.Helper()

	tx, err := testPool.Begin(ctx)
	require.NoError(t, err, "failed to begin the held transaction")

	eventId = uuid.NewString()
	_, err = tx.Exec(ctx,
		`INSERT INTO activity_events (id, group_id, actor_id, actor_name, kind, payload, created_at)
		 VALUES ($1, $2, $3, $4, $5, '{}', now())`,
		eventId, groupId, actorId, actorName, kind)
	require.NoError(t, err, "failed to insert the held event")

	// Same transaction as the insert, matching InsertActivityEvents: the
	// notification is only ever delivered if this transaction's commit
	// succeeds.
	_, err = tx.Exec(ctx, "SELECT pg_notify('activity_events', $1)", eventId)
	require.NoError(t, err, "failed to notify for the held event")

	return tx, eventId
}

// countActivityExceptionRows returns how many per-event read rows exist, across
// all users.
//
// The floor and the exception rows are two halves of one answer, so a case that
// only checks the badge cannot tell them apart: writing a row per event and
// raising the floor both clear it. Counting the rows is what distinguishes
// "mark all as read moved one integer" from "mark all as read wrote a row per
// event", which is the whole point of the floor.
func countActivityExceptionRows(t *testing.T) int {
	t.Helper()

	var n int
	err := testPool.QueryRow(context.Background(),
		`SELECT count(*) FROM activity_event_reads`).Scan(&n)
	require.NoError(t, err, "failed to count activity_event_reads")
	return n
}

// activityReadFloor returns the reader's floor seq, or 0 when they have none.
func activityReadFloor(t *testing.T, userId string) int64 {
	t.Helper()

	var seq int64
	err := testPool.QueryRow(context.Background(),
		`SELECT COALESCE(max(floor_seq), 0) FROM activity_read_floors WHERE user_id = $1`,
		userId).Scan(&seq)
	require.NoError(t, err, "failed to read the activity read floor")
	return seq
}
