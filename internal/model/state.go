package model

import "time"

type State struct {
	Lastfired map[string]time.Time
}
