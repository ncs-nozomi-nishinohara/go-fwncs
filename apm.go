package fwncs

import (
	"os"
	"strings"
)

type APM int

const (
	Unknown APM = iota
	Elastic
	Newrelic
)

func GetApmName() APM {
	switch strings.ToLower(os.Getenv("APM_NAME")) {
	case "elastic":
		return Elastic
	case "newrelic":
		return Newrelic
	default:
		return Unknown
	}
}
