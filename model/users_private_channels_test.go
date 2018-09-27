package model

import (
	"github.com/satori/go.uuid"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUsersPrivateChannel_TableName(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "users_private_channels", (&UsersPrivateChannel{}).TableName())
}

func TestAddPrivateChannelMember(t *testing.T) {
	assert, require, user, _ := beforeTest(t)

	channel := &Channel{
		ID:        CreateUUID(),
		CreatorID: user.ID,
		UpdaterID: user.ID,
		Name:      "Private-Channel",
		IsPublic:  false,
	}
	require.NoError(db.Create(channel).Error)

	po := mustMakeUser(t, "po")

	assert.NoError(AddPrivateChannelMember(channel.GetCID(), user.GetUID()))
	assert.NoError(AddPrivateChannelMember(channel.GetCID(), po.GetUID()))

	channelList, err := GetChannelList(user.GetUID())
	if assert.NoError(err) {
		assert.Len(channelList, 1+3)
	}

	channelList, err = GetChannelList(uuid.Nil)
	if assert.NoError(err) {
		assert.Len(channelList, 0+3)
	}
}

func TestGetPrivateChannelMembers(t *testing.T) {
	assert, _, _, _ := beforeTest(t)

	user1 := mustMakeUser(t, "private-1")
	user2 := mustMakeUser(t, "private-2")
	channel := mustMakePrivateChannel(t, "privatechannel-1", []uuid.UUID{user1.GetUID(), user2.GetUID()})

	member, err := GetPrivateChannelMembers(channel.GetCID())
	assert.NoError(err)
	assert.Len(member, 2)
}

func TestIsUserPrivatateChannelMember(t *testing.T) {
	assert, _, user, _ := beforeTest(t)

	user1 := mustMakeUser(t, "private-1")
	user2 := mustMakeUser(t, "private-2")
	channel := mustMakePrivateChannel(t, "privatechannel-1", []uuid.UUID{user1.GetUID(), user2.GetUID()})

	ok, err := IsUserPrivateChannelMember(channel.GetCID(), user1.GetUID())
	if assert.NoError(err) {
		assert.True(ok)
	}
	ok, err = IsUserPrivateChannelMember(channel.GetCID(), user2.GetUID())
	if assert.NoError(err) {
		assert.True(ok)
	}
	ok, err = IsUserPrivateChannelMember(channel.GetCID(), user.GetUID())
	if assert.NoError(err) {
		assert.False(ok)
	}

}
