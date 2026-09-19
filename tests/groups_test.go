package tests

import (
	"fmt"
	"testing"

	"github.com/lealre/movies-backend/internal/services/groups"
	"github.com/lealre/movies-backend/internal/services/users"
	"github.com/stretchr/testify/require"
)

// Who added a title is recorded end to end, and it is the member who actually
// added it rather than whoever owns the group.
//
// That distinction is the whole test. The store is handed a user id, and the
// nearest wrong id to reach for is the group's owner — with the owner adding
// everything, a mapper that used group.OwnerId would pass every other test in
// this package.
func TestGroupTitles_RecordWhoAddedThem(t *testing.T) {
	resetDB(t)

	owner, tokenOwner := addUser(t, users.NewUserRequest{
		Username: "owner_adds", Password: "testpass", Email: "owner_adds@test.dev",
	})
	member, tokenMember := addUser(t, users.NewUserRequest{
		Username: "member_adds", Password: "testpass", Email: "member_adds@test.dev",
	})

	group := createGroup(t, groups.CreateGroupRequest{Name: "who added what"}, tokenOwner)
	addUserToGroup(t, groups.AddUserToGroupRequest{UserId: member.Id}, group.Id, tokenOwner)

	titles := loadTitlesFixture(t)
	seedTitles(t, titles)
	title := titles[0]

	// The MEMBER adds it, not the owner.
	addTitleToGroup(t, groups.AddTitleToGroupRequest{
		URL:     fmt.Sprintf("https://www.imdb.com/title/%s/", title.ID),
		GroupId: group.Id,
	}, tokenMember)

	page := getGroupTitlesPage(t, group.Id, "", tokenOwner)
	require.Len(t, page.Content, 1)

	addedBy := page.Content[0].AddedBy
	require.NotNil(t, addedBy, "the API must report who added the title")
	require.Equal(t, member.Id, addedBy.Id, "must be the member who added it, not the group owner")
	require.Equal(t, "member_adds", addedBy.Username)
	require.NotEqual(t, owner.Id, addedBy.Id)

	// The single-title read is a separate SQL statement, so it can drift from
	// the list. It must report the same author.
	detail := getGroupTitleDetail(t, group.Id, title.ID, tokenOwner)
	require.NotNil(t, detail.AddedBy, "the single-title endpoint must report it too")
	require.Equal(t, member.Id, detail.AddedBy.Id)
	require.Equal(t, "member_adds", detail.AddedBy.Username)
}
