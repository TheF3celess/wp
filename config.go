package main

import (
	"time"

	"github.com/gosuri/uilive"
)

var (
	total    = 0
	progress = 0
	goods    = 0

	WordPress      = 0
	bad            = 0
	tries          = 0
	writer         = uilive.New()
	updateInterval = 100 * time.Millisecond
)

const (
	passFile = "top-300-es.txt"
)

type infos struct {
	domain   string
	username []string
}
