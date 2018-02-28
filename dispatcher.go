package main

import (
	"fmt"
	"sync"

	"encoding/json"
	"firebase.google.com/go"
	"firebase.google.com/go/messaging"
	scribble "github.com/nanobox-io/golang-scribble"
	"golang.org/x/net/context"
	"google.golang.org/api/option"
)

const (
	usercol       = "users"
	configcol     = "config"
	automationcol = "automations"
	jsonDir       = "json"
	oauthFile     = "oauth.json"
)

type action struct {
	Device     string   `json:"device"`
	Method     string   `json:"method"`
	Parameters []string `json:"parameters"` // Types of arguments
	Arguments  []string `json:"arguments"`  // Actual data
}

type automation struct {
	Me        string
	Locations map[string]string
	Actions   []action
}

type config struct {
	Data interface{} `json:"data"`
}

var db *scribble.Driver

var userMap map[string]string
var userMapMutex sync.RWMutex

var userLoc map[string]string
var userLocMutex sync.RWMutex

var automations map[string][]automation
var automationsMutex sync.RWMutex

var cfg config
var cfgMutex sync.RWMutex

var firebaseClient *messaging.Client

func init() {
	var err error
	db, err = scribble.New(jsonDir, nil)
	if err != nil {
		panic(err)
	}

	// Initialise usermap from db
	err = db.Read(usercol, usercol, &userMap)
	if err != nil {
		userMap = make(map[string]string)
		er := db.Write(usercol, usercol, userMap)
		if er != nil {
			fmt.Println(err)
			panic(er)
		}
	}
	// Initialise automations from db
	err = db.Read(automationcol, automationcol, &automations)
	if err != nil {
		automations = make(map[string][]automation)
		er := db.Write(automationcol, automationcol, automations)
		if er != nil {
			fmt.Println(err)
			panic(er)
		}
	}
	// Initialise config
	err = db.Read(configcol, configcol, &cfg)
	if err != nil {
		er := db.Write(configcol, configcol, cfg)
		if er != nil {
			fmt.Println(err)
			panic(er)
		}
	}

	opt := option.WithCredentialsFile(oauthFile)
	app, err := firebase.NewApp(context.Background(), nil, opt)
	if err != nil {
		panic(err)
	}

	firebaseClient, err = app.Messaging(context.Background())
	if err != nil {
		panic(err)
	}
}

func updateUserloc(name string, loc string) {
	userMapMutex.RLock()
	if _, ok := userMap[name]; !ok {
		return // Silently return for non-existent users
	}
	userMapMutex.RUnlock()

	changed := false
	userLocMutex.Lock()
	current, ok := userLoc[name]
	if !ok || current != loc {
		changed = true
		userLoc[name] = loc
	}
	userLocMutex.Unlock()

	if changed {
		checkAutomation(name)
	}
}

func checkAutomation(name string) {
	// Check the automations for matched conditions and trigger if required
	userLocMutex.RLock()
	automationsMutex.RLock()
	for user, autos := range automations { // Iterates over automation lists
		for _, auto := range autos { // Iterates over automations
			if _, ok := auto.Locations[name]; ok && verifyLocations(&auto, userLoc) { // Eligible for trigger
				go triggerAction(user, auto)
			}
		}
	}
	automationsMutex.RUnlock()
	userLocMutex.RUnlock()
}

func verifyLocations(auto *automation, loc map[string]string) bool { // Assumes a read lock is held by calling function
	for person, l := range auto.Locations { // Validates other location conditions
		checkLoc, ok2 := loc[person]
		if !ok2 || checkLoc != l {
			return false
		}
	}

	return true
}

func triggerAction(name string, auto automation) {
	token, ok := userMap[name]
	if !ok {
		return
	}


	b, err := json.Marshal(auto)
	if err != nil {
		fmt.Println(err)
		return
	}

	m := &messaging.Message{
		Data: map[string]string{
			"actions": string(b),
		},
		Token: token,
	}

	res, err := firebaseClient.Send(context.Background(), m)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(res)
}
