package main

import (
	"time"

	"github.com/gosuri/uilive"
)

var (
	total          = 0
	progress       = 0
	goods          = 0
	xPasswords     []string
	WordPress      = 0
	bad            = 0
	tries          = 0
	writer         = uilive.New()
	updateInterval = 5 * time.Second
)

const (
	passFile = "top-300-es.txt"
	wsURL    = "ws://localhost:8080"
)

type infos struct {
	domain   string
	username []string
}
