package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"testing"

	"github.com/lealre/movies-backend/internal/models"
	"github.com/lealre/movies-backend/internal/services/comments"
	"github.com/stretchr/testify/require"
)

func addComment(t *testing.T, newComment comments.NewComment, innerToken string) *http.Response {
	jsonData, err := json.Marshal(newComment)
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost,
		testServer.URL+"/comments",
		bytes.NewBuffer(jsonData),
	)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+innerToken)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)

	return resp
}

// addCommentAndGetResult posts a comment, asserts it was created and returns the
// decoded body. A comment is always group-scoped, so groupId is not optional.
func addCommentAndGetResult(t *testing.T, groupId, titleId, comment string, season *int, token string) comments.Comment {
	t.Helper()

	resp := addComment(t, comments.NewComment{
		GroupId: groupId,
		TitleId: titleId,
		Comment: comment,
		Season:  season,
	}, token)
	defer resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode, "expected the comment to be created")

	var created comments.Comment
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&created), "error decoding the created comment")
	return created
}

// getCommentsForGroupTitle decodes a successful GET
// /groups/{groupId}/titles/{titleId}/comments response.
func getCommentsForGroupTitle(t *testing.T, groupId, titleId, token string) []comments.Comment {
	t.Helper()

	resp := getCommentsFromApi(t, groupId, titleId, token)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode, "expected the group-scoped comments read to succeed")

	var body comments.AllCommentsFromTitle
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body), "error decoding the comments response")
	return body.Comments
}

func getCommentsFromApi(t *testing.T, groupId, titleId, innerToken string) *http.Response {
	req, err := http.NewRequest(http.MethodGet,
		testServer.URL+"/groups/"+groupId+"/titles/"+titleId+"/comments",
		nil,
	)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+innerToken)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)

	return resp
}

func updateCommentFromApi(t *testing.T, groupId, titleId, commentId, comment, innerToken string, season *int) *http.Response {
	jsonData, err := json.Marshal(comments.UpdateCommentRequest{
		Comment: comment,
		Season:  season,
	})
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPatch,
		testServer.URL+"/groups/"+groupId+"/titles/"+titleId+"/comments/"+commentId,
		bytes.NewBuffer(jsonData),
	)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+innerToken)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)

	return resp
}

func deleteCommentFromApi(t *testing.T, groupId, titleId, commentId, innerToken string) *http.Response {
	req, err := http.NewRequest(http.MethodDelete,
		testServer.URL+"/groups/"+groupId+"/titles/"+titleId+"/comments/"+commentId,
		nil,
	)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+innerToken)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)

	return resp
}

func deleteCommentSeasonFromApi(t *testing.T, groupId, titleId, commentId, innerToken string, season int) *http.Response {
	req, err := http.NewRequest(http.MethodDelete,
		testServer.URL+"/groups/"+groupId+"/titles/"+titleId+"/comments/"+commentId+"/seasons/"+strconv.Itoa(season),
		nil,
	)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+innerToken)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)

	return resp
}

func getCommentFromDB(t *testing.T, commentId string) models.Comment {
	ctx := context.Background()

	var c models.Comment
	err := testPool.QueryRow(ctx,
		"SELECT id, title_id, user_id, group_id, comment, created_at, updated_at FROM comments WHERE id = $1", commentId).
		Scan(&c.Id, &c.TitleId, &c.UserId, &c.GroupId, &c.Comment, &c.CreatedAt, &c.UpdatedAt)
	require.NoError(t, err, "error querying a comment from db")

	rows, err := testQueries.GetCommentSeasons(ctx, commentId)
	require.NoError(t, err)
	if len(rows) > 0 {
		m := make(models.SeasonsComments, len(rows))
		for _, row := range rows {
			m[row.Season] = models.SeasonCommentItem{Comment: row.Comment, AddedAt: row.AddedAt.Time, UpdatedAt: row.UpdatedAt.Time}
		}
		c.SeasonsComments = &m
	}
	return c
}

func getCommentsFromDB(t *testing.T, titleId string) []models.Comment {
	ctx := context.Background()

	rows, err := testPool.Query(ctx,
		"SELECT id, title_id, user_id, group_id, comment, created_at, updated_at FROM comments WHERE title_id = $1 ORDER BY group_id, user_id, id", titleId)
	require.NoError(t, err, "error querying comments from db")
	defer rows.Close()

	var commentIds []string
	comments := []models.Comment{}
	for rows.Next() {
		var c models.Comment
		require.NoError(t, rows.Scan(&c.Id, &c.TitleId, &c.UserId, &c.GroupId, &c.Comment, &c.CreatedAt, &c.UpdatedAt))
		comments = append(comments, c)
		commentIds = append(commentIds, c.Id)
	}
	require.NoError(t, rows.Err())

	for i, commentId := range commentIds {
		seasonRows, err := testQueries.GetCommentSeasons(ctx, commentId)
		require.NoError(t, err)
		if len(seasonRows) > 0 {
			m := make(models.SeasonsComments, len(seasonRows))
			for _, row := range seasonRows {
				m[row.Season] = models.SeasonCommentItem{Comment: row.Comment, AddedAt: row.AddedAt.Time, UpdatedAt: row.UpdatedAt.Time}
			}
			comments[i].SeasonsComments = &m
		}
	}

	return comments
}
