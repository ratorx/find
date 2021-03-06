package main

import (
	"testing"
	"github.com/stretchr/testify/require"
)

func TestUpdateUserLoc(t *testing.T) {
	userMap = make(map[string]string)
	userMap["user1"] = "fbid1"

	userLoc = make(map[string]string)
	userLoc["user1"] = "room1"

	updateUserloc("user1", "room2")

	require.Equal(t, userLoc["user1"], "room2")
}

func TestVerifyLocations(t *testing.T) {
	auto := automation{Me: "", Locations: map[string]string{"user1": "room1", "user2": "room2"}, Actions: []action{}, LeaveActions: []action{}}
	loc := map[string]string{"user1": "room1", "user2": "room2", "user3":"room1"}

	trigger, enterac := verifyLocations(&auto, loc)
	require.Equal(t, trigger, true, "Location entry conditions should be met")
	require.Equal(t, enterac, true, "Entry condtionas should be met")

	trigger, enterac = verifyLocations(&auto, loc)
	require.Equal(t, trigger, false, "Location entry conditions were met last time")
	require.Equal(t, enterac, true, "Entry condtionas should be met")

	loc["user2"] = "room1"

	trigger, enterac = verifyLocations(&auto, loc)
	require.Equal(t, trigger, true, "Location exit conditions met")
	require.Equal(t, enterac, false, "Exit condtions should be met")

	trigger, enterac = verifyLocations(&auto, loc)
	require.Equal(t, trigger, false, "Location exit conditions were met last time")
	require.Equal(t, enterac, false, "Exit condtions should be met")
}

func TestTriggerEntryAction(t *testing.T) {
	// Test is used for debugging android app, do not run as unit test
	t.Skip()
	const user = "ENTER USER HERE"
	const fbid = "ENTER CLIENTID HERE"
	userMap = make(map[string]string)
	userMap[user] = fbid
	triggerAction(user, automations[user][0].Actions)
}