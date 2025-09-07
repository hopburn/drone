package boardwhite

import (
	"context"
	"testing"

	"github.com/boar-d-white-foundation/drone/db"
	"github.com/stretchr/testify/require"
	tele "gopkg.in/telebot.v3"
)

func TestAddUserToMeetup(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := db.NewBadgerDB(":memory:")
	err := database.Start(ctx)
	require.NoError(t, err)
	t.Cleanup(database.Stop)

	user1 := tele.User{ID: 1, Username: "user1"}
	user2 := tele.User{ID: 2, Username: "user2"}

	err = database.Do(ctx, func(tx db.Tx) error {
		return addUserToMeetup(tx, "park", user1)
	})
	require.NoError(t, err)

	// adding same user again should not create duplicates
	err = database.Do(ctx, func(tx db.Tx) error {
		return addUserToMeetup(tx, "park", user1)
	})
	require.NoError(t, err)

	// adding another user should append
	err = database.Do(ctx, func(tx db.Tx) error {
		return addUserToMeetup(tx, "park", user2)
	})
	require.NoError(t, err)

	err = database.Do(ctx, func(tx db.Tx) error {
		meetups, err := db.GetJsonDefault(tx, keyMeetupLocations, meetupStore{})
		require.NoError(t, err)
		users := meetups["park"]
		require.Len(t, users, 2)
		require.Equal(t, user1.ID, users[0].ID)
		require.Equal(t, user2.ID, users[1].ID)
		return nil
	})
	require.NoError(t, err)
}

func TestListMeetupLocations(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := db.NewBadgerDB(":memory:")
	err := database.Start(ctx)
	require.NoError(t, err)
	t.Cleanup(database.Stop)

	err = database.Do(ctx, func(tx db.Tx) error {
		if err := addUserToMeetup(tx, "park", tele.User{ID: 1}); err != nil {
			return err
		}
		return addUserToMeetup(tx, "office", tele.User{ID: 2})
	})
	require.NoError(t, err)

	var locations []string
	err = database.Do(ctx, func(tx db.Tx) error {
		locations, err = listMeetupLocations(tx)
		return err
	})
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"office", "park"}, locations)
}
