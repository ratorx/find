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
	"time"
)

const (
	usercol       = "users"
	configcol     = "config"
	automationcol = "automations"
	jsonDir       = "json"
	oauthFile     = "oauth.json"
	buffer = 1
	checkDelay = 100
)

type action struct {
	Device     string   `json:"device"`
	Method     string   `json:"method"`
	Parameters []string `json:"parameters"` // Types of arguments
	Arguments  []string `json:"arguments"`  // Actual data
}

func (a action) String() string {
	return fmt.Sprintf("Call %s on device %s with arguments %v", a.Method, a.Device, a.Arguments)
}

type automation struct {
	Me           string // Identifier for user who created this - should be an entry in userMap
	Locations    map[string]string
	Actions      []action
	LeaveActions []action
	called       bool
}

type config struct {
	Data interface{} `json:"data"`
}

var db *scribble.Driver

var userMap map[string]string // Stores user names to identifiers
var userMapMutex sync.RWMutex

var userLoc map[string]string
var userLocMutex sync.Mutex

var automations map[string][]automation
var automationsMutex sync.RWMutex

var cfg config
var cfgMutex sync.RWMutex

var firebaseClient *messaging.Client

var checkAuto chan struct{}

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

	// Initialise userLoc
	userLoc = make(map[string]string)

	opt := option.WithCredentialsFile(oauthFile)
	app, err := firebase.NewApp(context.Background(), nil, opt)
	if err != nil {
		panic(err)
	}

	firebaseClient, err = app.Messaging(context.Background())
	if err != nil {
		panic(err)
	}

	checkAuto = make(chan struct{}, buffer)
}

func updateUserloc(name string, loc string) {
	if _, ok := userMap[name]; !ok {
		return // Silently return for non-existent users
	}

	changed := false
	userLocMutex.Lock()
	current, ok := userLoc[name]
	if !ok || current != loc {
		changed = true
		userLoc[name] = loc
	}
	userLocMutex.Unlock()

	if changed {
		select {
		case checkAuto <- struct{}{}:
			fmt.Println("Check automation requested")
		default:
			fmt.Println("Queue full")
		}
	}
}

func checkAutomation() {
	for {
		<-checkAuto
		automationsMutex.RLock()
		userLocMutex.Lock()
		fmt.Println("Mutex acquired - checking automation")
		for user := range automations { // Iterates over automation lists
			for i := range automations[user] { // Iterates over automations
				ok, enterac := verifyLocations(&automations[user][i], userLoc)
				fmt.Println(ok, enterac, userLoc)
				if ok && enterac {
					go triggerAction(user, automations[user][i].Actions)
				} else if ok && !enterac {
					go triggerAction(user, automations[user][i].LeaveActions)
				}
			}
		}
		userLocMutex.Unlock()
		automationsMutex.RUnlock()
		fmt.Println("Mutex unlocked")
		time.Sleep(checkDelay * time.Millisecond)
	}
}


func verifyLocations(auto *automation, loc map[string]string) (bool, bool) { // Assumes a read lock is held by calling function
	fmt.Println("Verifying Locations")
	// 1st bool is trigger indicator, 2nd bool is enter (true) or leave  (false) actions
	for person, l := range auto.Locations { // Validates other location conditions
		checkLoc, ok2 := loc[person]
		if !ok2 || checkLoc != l {
			if auto.called {
				auto.called = false
				return true, false
			}
			return false, false
		}
	}

	if !auto.called {
		auto.called = true
		return true, true
	}

	return false, true
}

func triggerAction(name string, actions []action) {
	userMapMutex.RLock()
	token, ok := userMap[name]
	if !ok {
		return
	}
	userMapMutex.RUnlock()

	if len(actions) == 0 {
		fmt.Println("No actions specified - skip sending push notification")
		return
	}

	b, err := json.Marshal(actions)
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

	_, err = firebaseClient.Send(context.Background(), m)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println("Automation message send success")
	fmt.Println(string(b))
}
