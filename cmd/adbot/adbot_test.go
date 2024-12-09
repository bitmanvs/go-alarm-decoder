package main

import (
	"io/ioutil"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	alarmdecoder "github.com/d4l3k/go-alarmdecoder"
	expo "github.com/oliveroneill/exponent-server-sdk-golang/sdk"
)

func TestDropOldEvents(t *testing.T) {
	now := time.Now()
	drop := now.Add(-1 * time.Hour)
	old := now.Add(-2 * time.Hour)
	events := []Event{
		{
			Time:    old,
			Message: alarmdecoder.Message{KeypadMessage: "old"},
		},
		{
			Time:    drop,
			Message: alarmdecoder.Message{KeypadMessage: "drop"},
		},
		{
			Time:    now,
			Message: alarmdecoder.Message{KeypadMessage: "now"},
		},
	}
	filtered := dropOldEvents(events, drop)
	if len(filtered) != 1 {
		t.Errorf("expected 1 events got: %+v", filtered)
	}
	if filtered[0].KeypadMessage != "now" {
		t.Errorf("expected event to be now: %+v", filtered)
	}
}

func TestStateSaveLoad(t *testing.T) {
	var s state
	if err := s.load("does/not/exist"); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(s, state{}) {
		t.Fatalf("should be empty isn't: %+v", s)
	}
	s.RegistrationTokens = map[string]expo.ExponentPushToken{
		"foo": expo.ExponentPushToken("bar"),
	}

	dir, err := ioutil.TempDir("", "TestStateSaveLoad")
	if err != nil {
		t.Fatal(err)
	}

	file := filepath.Join(dir, "foo.json")

	if err := s.save(file); err != nil {
		t.Fatal(err)
	}

	var loaded state
	if err := loaded.load(file); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(loaded, s) {
		t.Fatalf("loaded state doesn't match: %+v !+ %+v", loaded, s)
	}
}
