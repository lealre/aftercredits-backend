package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/lealre/movies-backend/internal/api"
	"github.com/lealre/movies-backend/internal/generics"
	"github.com/lealre/movies-backend/internal/models"
	"github.com/lealre/movies-backend/internal/services/groups"
	"github.com/lealre/movies-backend/internal/services/users"
	"github.com/stretchr/testify/require"
)

func getGroup(t *testing.T, groupId string) models.Group {
	ctx := context.Background()

	row, err := testQueries.GetGroupRowAnyById(ctx, groupId)
	require.NoError(t, err, "error querying a group from db")

	memberIds, err := testQueries.GetGroupMemberIds(ctx, groupId)
	require.NoError(t, err)

	titleRows, err := testQueries.GetGroupTitleRows(ctx, groupId)
	require.NoError(t, err)
	seasonRows, err := testQueries.GetGroupTitleSeasonRows(ctx, groupId)
	require.NoError(t, err)

	seasonsByTitle := map[string]models.SeasonsWatched{}
	for _, s := range seasonRows {
		if seasonsByTitle[s.TitleID] == nil {
			seasonsByTitle[s.TitleID] = models.SeasonsWatched{}
		}
		seasonsByTitle[s.TitleID][s.Season] = models.SeasonWatchedItem{
			Watched:   s.Watched,
			WatchedAt: timestamptzPtr(s.WatchedAt),
			AddedAt:   s.AddedAt.Time,
			UpdatedAt: s.UpdatedAt.Time,
		}
	}

	titles := models.GroupTitles{}
	for _, tr := range titleRows {
		var sw *models.SeasonsWatched
		if m, ok := seasonsByTitle[tr.TitleID]; ok {
			sw = &m
		}
		titles[tr.TitleID] = models.GroupTitleItem{
			TitleId:        tr.TitleID,
			SeasonsWatched: sw,
			Watched:        tr.Watched,
			AddedAt:        tr.AddedAt.Time,
			UpdatedAt:      tr.UpdatedAt.Time,
			WatchedAt:      timestamptzPtr(tr.WatchedAt),
		}
	}

	return models.Group{
		Id: row.ID, Name: row.Name, Description: row.Description, OwnerId: row.OwnerID,
		Users: memberIds, Titles: titles,
		CreatedAt: row.CreatedAt.Time, UpdatedAt: row.UpdatedAt.Time,
		Deleted: row.Deleted, DeletedAt: timestamptzPtr(row.DeletedAt),
	}
}

// timestamptzPtr converts a nullable pgtype.Timestamptz to *time.Time.
func timestamptzPtr(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	v := t.Time
	return &v
}

func createGroup(t *testing.T, newGroup groups.CreateGroupRequest, userToken string) groups.GroupResponse {
	jsonData, err := json.Marshal(newGroup)
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost,
		testServer.URL+"/groups",
		bytes.NewBuffer(jsonData),
	)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+userToken)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	respGroup, err := client.Do(req)

	require.NoError(t, err)
	defer respGroup.Body.Close()
	require.Equal(t, http.StatusCreated, respGroup.StatusCode)

	var group groups.GroupResponse
	require.NoError(t, json.NewDecoder(respGroup.Body).Decode(&group))

	return group
}

func addUserToGroup(t *testing.T, addUserBody groups.AddUserToGroupRequest, groupId, token string) {
	jsonData, err := json.Marshal(addUserBody)
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost,
		testServer.URL+"/groups/"+groupId+"/users",
		bytes.NewBuffer(jsonData),
	)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	respGroup, err := client.Do(req)
	require.NoError(t, err)

	defer respGroup.Body.Close()
	require.Equal(t, http.StatusOK, respGroup.StatusCode)
}

func addTitleToGroup(t *testing.T, newTitle groups.AddTitleToGroupRequest, token string) api.DefaultResponse {
	jsonData, err := json.Marshal(newTitle)
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost,
		testServer.URL+"/groups/titles",
		bytes.NewBuffer(jsonData),
	)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	respGroupAddTitle, err := client.Do(req)

	require.NoError(t, err)
	defer respGroupAddTitle.Body.Close()
	require.Equal(t, http.StatusOK, respGroupAddTitle.StatusCode)

	var respGroupTitlesBody api.DefaultResponse
	require.NoError(t, json.NewDecoder(respGroupAddTitle.Body).Decode(&respGroupTitlesBody))

	return respGroupTitlesBody

}

// addTitleToGroupResponse calls POST /groups/titles and returns the raw
// response, for callers that need to assert on a failing request (addTitleToGroup
// asserts 200 and decodes the body, so it cannot be reused for that).
func addTitleToGroupResponse(t *testing.T, newTitle groups.AddTitleToGroupRequest, token string) *http.Response {
	t.Helper()

	jsonData, err := json.Marshal(newTitle)
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost,
		testServer.URL+"/groups/titles",
		bytes.NewBuffer(jsonData),
	)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	return resp
}

// deleteTitleFromGroupResponse calls DELETE /groups/{groupId}/titles/{titleId}
// and returns the raw response for the caller to assert on.
func deleteTitleFromGroupResponse(t *testing.T, groupId, titleId, token string) *http.Response {
	t.Helper()

	req, err := http.NewRequest(http.MethodDelete,
		testServer.URL+"/groups/"+groupId+"/titles/"+titleId,
		nil,
	)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	return resp
}

func patchGroupTitleWatched(t *testing.T, groupId string, pathBody []byte, token string) groups.GroupTitle {
	req, err := http.NewRequest(http.MethodPatch,
		testServer.URL+"/groups/"+groupId+"/titles",
		bytes.NewBuffer(pathBody),
	)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	respGroupSetWatched, err := client.Do(req)
	require.NoError(t, err)
	defer respGroupSetWatched.Body.Close()
	require.Equal(t, http.StatusOK, respGroupSetWatched.StatusCode)
	var resp groups.GroupTitle
	require.NoError(t, json.NewDecoder(respGroupSetWatched.Body).Decode(&resp))
	return resp
}

func getGroupFromApi(t *testing.T, groupId, token string) *http.Response {
	req, err := http.NewRequest(http.MethodGet, testServer.URL+"/groups/"+groupId, nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)
	return resp
}

func updateGroupFromApi(t *testing.T, groupId string, body groups.UpdateGroupRequest, token string) *http.Response {
	jsonData, err := json.Marshal(body)
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPatch, testServer.URL+"/groups/"+groupId, bytes.NewBuffer(jsonData))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)
	return resp
}

func deleteGroupFromApi(t *testing.T, groupId, token string) *http.Response {
	req, err := http.NewRequest(http.MethodDelete, testServer.URL+"/groups/"+groupId, nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)
	return resp
}

func removeUserFromGroupApi(t *testing.T, groupId, userId, token string) *http.Response {
	req, err := http.NewRequest(http.MethodDelete, testServer.URL+"/groups/"+groupId+"/users/"+userId, nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)
	return resp
}

// getGroupTitlesResponse calls GET /groups/{id}/titles with a raw query string
// (no leading "?") and returns the response for the caller to assert on.
func getGroupTitlesResponse(t *testing.T, groupId, query, token string) *http.Response {
	t.Helper()

	url := testServer.URL + "/groups/" + groupId + "/titles"
	if query != "" {
		url += "?" + query
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	return resp
}

// getGroupTitlesPage decodes a successful group-titles response.
func getGroupTitlesPage(t *testing.T, groupId, query, token string) generics.Page[groups.GroupTitleDetail] {
	t.Helper()

	resp := getGroupTitlesResponse(t, groupId, query, token)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var page generics.Page[groups.GroupTitleDetail]
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&page))
	return page
}

// getGroupTitlesRawBody returns the response body as a string. Needed because a
// JSON `null` and a `[]` both decode into an empty Go slice, so the difference
// is only observable before decoding.
func getGroupTitlesRawBody(t *testing.T, groupId, query, token string) string {
	t.Helper()

	resp := getGroupTitlesResponse(t, groupId, query, token)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return string(body)
}

// getGroupTitleDetail returns the GroupTitleDetail for a single title from
// GET /groups/{id}/titles. The group-titles list is the read path that carries
// GroupRatings, so it is where a cross-group rating leak becomes observable.
func getGroupTitleDetail(t *testing.T, groupId, titleId, token string) groups.GroupTitleDetail {
	t.Helper()

	page := getGroupTitlesPage(t, groupId, "", token)
	for _, detail := range page.Content {
		if detail.Id == titleId {
			return detail
		}
	}

	require.FailNowf(t, "title missing from group titles page",
		"expected title %s to be listed for group %s, got %v", titleId, groupId, groupTitleIds(page))
	return groups.GroupTitleDetail{}
}

// getGroupTitleResponse calls GET /groups/{groupId}/titles/{titleId} and
// returns the raw response for the caller to assert on.
func getGroupTitleResponse(t *testing.T, groupId, titleId, token string) *http.Response {
	t.Helper()

	req, err := http.NewRequest(http.MethodGet,
		testServer.URL+"/groups/"+groupId+"/titles/"+titleId, nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	return resp
}

// getGroupTitleById decodes a successful single-title response.
func getGroupTitleById(t *testing.T, groupId, titleId, token string) groups.GroupTitleDetail {
	t.Helper()

	resp := getGroupTitleResponse(t, groupId, titleId, token)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var detail groups.GroupTitleDetail
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&detail))
	return detail
}

// getGroupTitleRawBody returns the single-title response body as a string, for
// the assertions a decoded struct cannot make: a `null` and a `[]` both decode
// into an empty Go slice (CONVENTIONS §5).
func getGroupTitleRawBody(t *testing.T, groupId, titleId, token string) string {
	t.Helper()

	resp := getGroupTitleResponse(t, groupId, titleId, token)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return string(body)
}

// groupTitleRawFromList returns the raw JSON of one entry of the group-titles
// list — the element the single-title endpoint has to reproduce exactly.
//
// It is deliberately taken before decoding: the drift worth catching between
// the two responses includes a field that is `null` in one and `[]` in the
// other, which no decoded comparison can see.
func groupTitleRawFromList(t *testing.T, groupId, titleId, token string) string {
	t.Helper()

	var page struct {
		Content []json.RawMessage `json:"Content"`
	}
	body := getGroupTitlesRawBody(t, groupId, "size=100&page=1", token)
	require.NoError(t, json.Unmarshal([]byte(body), &page), "failed to decode the group titles page envelope")

	listed := make([]string, 0, len(page.Content))
	for _, entry := range page.Content {
		var identified struct {
			Id string `json:"id"`
		}
		require.NoError(t, json.Unmarshal(entry, &identified), "failed to decode a group titles entry")
		if identified.Id == titleId {
			return string(entry)
		}
		listed = append(listed, identified.Id)
	}

	require.FailNowf(t, "title missing from the group titles page",
		"expected title %s to be listed for group %s, got %v", titleId, groupId, listed)
	return ""
}

// twoGroupFixture is the world the group-scoping tests share: one user who
// belongs to two groups that both carry the same movie and the same TV series,
// plus a second user who belongs to group B only. Ratings and comments are
// group-scoped facts, so this is the smallest world in which a cross-group leak
// is observable at all.
type twoGroupFixture struct {
	user       users.UserResponse
	token      string
	otherUser  users.UserResponse
	otherToken string
	groupA     groups.GroupResponse
	groupB     groups.GroupResponse
	movie      models.Title
	tvSeries   models.Title
}

// setupTwoGroups builds the twoGroupFixture world. Callers are expected to have
// called resetDB(t) first.
func setupTwoGroups(t *testing.T) twoGroupFixture {
	t.Helper()

	user, token := addUser(t, users.NewUserRequest{
		Username: "twogroupsowner",
		Password: "testpass",
	})
	otherUser, otherToken := addUser(t, users.NewUserRequest{
		Username: "twogroupsmember",
		Password: "testpass",
	})

	groupA := createGroup(t, groups.CreateGroupRequest{Name: "group A"}, token)
	groupB := createGroup(t, groups.CreateGroupRequest{Name: "group B"}, token)

	// otherUser is a member of group B only, so everything they write is a
	// group B fact and must never surface in group A.
	addUserToGroup(t, groups.AddUserToGroupRequest{UserId: otherUser.Id}, groupB.Id, token)

	movieTitles := loadTitlesFixture(t)
	tvSeriesTitles := loadTVSeriesTitlesFixture(t)
	seedTitles(t, append(movieTitles, tvSeriesTitles...))
	movie := movieTitles[0]
	tvSeries := tvSeriesTitles[0]

	// Both groups carry both titles: the same title in two groups is exactly the
	// situation the (user, title, group) key exists to keep apart.
	for _, group := range []groups.GroupResponse{groupA, groupB} {
		for _, title := range []models.Title{movie, tvSeries} {
			addTitleToGroup(t, groups.AddTitleToGroupRequest{
				URL:     fmt.Sprintf("https://www.imdb.com/title/%s/", title.ID),
				GroupId: group.Id,
			}, token)
		}
	}

	return twoGroupFixture{
		user:       user,
		token:      token,
		otherUser:  otherUser,
		otherToken: otherToken,
		groupA:     groupA,
		groupB:     groupB,
		movie:      movie,
		tvSeries:   tvSeries,
	}
}

// groupTitleIds reduces a page to its title ids, in order. Order is the point
// of the sorting subtests, so it is asserted explicitly rather than as a set.
func groupTitleIds(page generics.Page[groups.GroupTitleDetail]) []string {
	ids := make([]string, 0, len(page.Content))
	for _, detail := range page.Content {
		ids = append(ids, detail.Id)
	}
	return ids
}

// tiedTitlesFixture is a group whose titles deliberately collide on every
// column the titles sort whitelist can order by, so paging over it exercises a
// sort that is only total if it ends in a tie-break.
type tiedTitlesFixture struct {
	token    string
	group    groups.GroupResponse
	titleIds []string
}

// tiedSortKeys is every orderBy value a group-titles request accepts — the
// whole whitelist, title-side and group-side alike. All of them now sort
// through one CASE-based ORDER BY in SQL (GetGroupTitlesPage) under the same
// LIMIT/OFFSET, so all of them depend on the same trailing t.id ASC tie-break
// to page correctly, and all of them belong here. The group-side three
// (watched, watchedAt, addedAt) used to take a separate in-Go branch and were
// excluded for that reason; that branch is gone.
//
// Keep this in sync with groupTitlesOrderKeys in internal/postgres/groups.go.
var tiedSortKeys = []string{
	"", "primaryTitle", "imdbRating", "startYear", "type", "voteCount", "updatedAt",
	"watched", "watchedAt", "addedAt",
}

// setupTiedTitlesGroup seeds count movies and puts all of them in one group.
// Values repeat on a short cycle per column, so every sort key has large groups
// of rows that compare equal:
//
//   - type:         one value, so all count rows tie
//   - startYear:    2 values, primaryTitle: 3, rating: 4, voteCount: 5
//   - updatedAt:    NULL on every other title, one shared timestamp otherwise —
//     NULL rows tie with each other too
//
// and, on the group_titles side:
//
//   - watched:      true on every 4th entry, false on the rest — two big ties
//   - watchedAt:    one shared timestamp on the watched entries, NULL on the
//     rest, so entries tie on a value and on NULL
//   - addedAt:      2 shared timestamps, alternating
//
// The cycle lengths are coprime-ish on purpose: no two columns partition the
// set the same way, so no column accidentally acts as another's tie-break.
//
// Callers are expected to have called resetDB(t) first.
func setupTiedTitlesGroup(t *testing.T, count int) tiedTitlesFixture {
	t.Helper()

	_, token := addUser(t, users.NewUserRequest{
		Username: "tiedtitlesowner",
		Password: "testpass",
	})
	group := createGroup(t, groups.CreateGroupRequest{Name: "tied titles group"}, token)

	updatedAt := time.Date(2025, 3, 4, 5, 6, 7, 0, time.UTC)

	titles := make([]models.Title, 0, count)
	titleIds := make([]string, 0, count)
	for i := range count {
		var titleUpdatedAt *time.Time
		if i%2 == 1 {
			titleUpdatedAt = &updatedAt
		}
		title := newSortableMovieTitle(
			fmt.Sprintf("tt90%05d", i),
			fmt.Sprintf("Tied Movie %d", i%3),
			2000+i%2,
			float64(5+i%4),
			100*(i%5),
			titleUpdatedAt,
		)
		titles = append(titles, title)
		titleIds = append(titleIds, title.ID)
	}
	seedTitles(t, titles)

	// Seeded titles already exist, so this only creates the group_titles rows
	// (the endpoint never reaches the title provider).
	for _, titleId := range titleIds {
		addTitleToGroup(t, groups.AddTitleToGroupRequest{
			URL:     "https://www.imdb.com/title/" + titleId + "/",
			GroupId: group.Id,
		}, token)
	}
	setTiedGroupTitleColumns(t, group.Id, titleIds)

	return tiedTitlesFixture{token: token, group: group, titleIds: titleIds}
}

// setTiedGroupTitleColumns overwrites the three group_titles columns the sort
// whitelist can order by (watched, watched_at, added_at) with deliberately
// repeating values, so those sort keys are as thoroughly tied as the
// title-side ones.
//
// It has to write them directly: add-title-to-group stamps watched=false and
// watched_at=NULL on every entry (already ties, but only one value each) and
// added_at from time.Now(), which is DISTINCT per row — under a distinct
// column the order is total with or without a tie-break, so paging by addedAt
// would pass whether or not the fix is in place. That is the vacuous pass
// CONVENTIONS §8 warns about, which is exactly what this helper exists to
// prevent.
func setTiedGroupTitleColumns(t *testing.T, groupId string, titleIds []string) {
	t.Helper()

	watchedAt := time.Date(2025, 6, 7, 8, 9, 10, 0, time.UTC)
	addedAt := []time.Time{
		time.Date(2024, 2, 3, 0, 0, 0, 0, time.UTC),
		time.Date(2024, 5, 6, 0, 0, 0, 0, time.UTC),
	}

	for i, titleId := range titleIds {
		watched := i%4 == 0
		// Only a watched entry carries a watchedAt, mirroring the invariant
		// the watched-update endpoint enforces.
		var entryWatchedAt *time.Time
		if watched {
			entryWatchedAt = &watchedAt
		}

		tag, err := testPool.Exec(context.Background(),
			`UPDATE group_titles SET watched = $3, watched_at = $4, added_at = $5
			 WHERE group_id = $1 AND title_id = $2`,
			groupId, titleId, watched, entryWatchedAt, addedAt[i%2])
		require.NoError(t, err, "failed to set the tied group-title columns for %s", titleId)
		require.EqualValues(t, 1, tag.RowsAffected(),
			"expected to update exactly one group_titles row for %s", titleId)
	}
}

// walkGroupTitlePages pages through GET /groups/{id}/titles from page 1 to the
// reported TotalPages, exactly as a client walking the list does, and returns
// the title ids of each page. query carries the sort (and any filters) without
// size or page, which this helper supplies.
//
// It returns one slice per page rather than a flat list so a caller can tell a
// title repeated *within* a page from one that leaked across two pages.
func walkGroupTitlePages(t *testing.T, groupId string, size int, query, token string) [][]string {
	t.Helper()

	first := getGroupTitlesPage(t, groupId, fmt.Sprintf("size=%d&page=1&%s", size, query), token)
	pages := [][]string{groupTitleIds(first)}
	for page := 2; page <= first.TotalPages; page++ {
		next := getGroupTitlesPage(t, groupId, fmt.Sprintf("size=%d&page=%d&%s", size, page, query), token)
		require.Equal(t, first.TotalResults, next.TotalResults,
			"the total must not move while walking pages of %q", query)
		pages = append(pages, groupTitleIds(next))
	}
	return pages
}

// flattenPages concatenates the per-page ids returned by walkGroupTitlePages.
func flattenPages(pages [][]string) []string {
	var all []string
	for _, page := range pages {
		all = append(all, page...)
	}
	return all
}

// deleteTitleFromCatalogue removes a title from the titles table while leaving
// every group_titles row that points at it in place, producing an orphaned
// group entry. group_titles.title_id carries no foreign key to titles (see
// sql/schema/001_init.sql), so this is a state the production database can
// reach through the admin delete-title endpoint — not a fabricated one.
func deleteTitleFromCatalogue(t *testing.T, titleId string) {
	t.Helper()

	tag, err := testPool.Exec(context.Background(), "DELETE FROM titles WHERE id = $1", titleId)
	require.NoError(t, err, "failed to delete title %s from the catalogue", titleId)
	require.EqualValues(t, 1, tag.RowsAffected(), "expected to delete exactly one title row for %s", titleId)

	var groupEntries int
	require.NoError(t,
		testPool.QueryRow(context.Background(),
			"SELECT count(*) FROM group_titles WHERE title_id = $1", titleId).Scan(&groupEntries),
		"failed to count the group entries left behind for %s", titleId)
	require.NotZero(t, groupEntries, "the group entry for %s must survive the title deletion, or there is no orphan to test", titleId)
}

// setGroupTitleWatched marks a group title watched (or not) with an explicit
// watchedAt, so ordering tests have deterministic values to sort on.
func setGroupTitleWatched(t *testing.T, groupId, titleId string, watched bool, watchedAt *time.Time, token string) {
	t.Helper()

	body, err := json.Marshal(groups.UpdateGroupTitleWatchedRequest{
		TitleId:   titleId,
		Watched:   &watched,
		WatchedAt: &generics.FlexibleDate{Time: watchedAt},
	})
	require.NoError(t, err)
	patchGroupTitleWatched(t, groupId, body, token)
}

// applyWatchedUpdate sends one PATCH /groups/{id}/titles carrying exactly the
// fields set on req.
//
// setGroupTitleWatched always sends both watched and watchedAt, which collapses
// the very distinction the activity payload has to preserve: "mark it watched"
// and "change the date" are different requests, and only an omitted field says
// which one the caller meant.
func applyWatchedUpdate(t *testing.T, groupId string, req groups.UpdateGroupTitleWatchedRequest, token string) groups.GroupTitle {
	t.Helper()

	body, err := json.Marshal(req)
	require.NoError(t, err, "failed to encode the watched update for title %s", req.TitleId)
	return patchGroupTitleWatched(t, groupId, body, token)
}

// watchedFlag and watchedDate build the optional fields of an update: a nil
// field means "leave this alone", so tests need a way to set one without
// setting the other.
func watchedFlag(watched bool) *bool { return &watched }

func watchedDate(when time.Time) *generics.FlexibleDate {
	return &generics.FlexibleDate{Time: &when}
}
