package main

import "github.com/eldarion-gondor/gondor-go"

type ClusterMap map[string]*gondor.Config

var version string

func main() {
	cli := CLI{
		Name:    "gondor",
		Version: version,
		Author:  "Eldarion, Inc.",
		Email:   "development@eldarion.com",
		Usage:   "command-line tool for interacting with the Gondor API",
	}
	cli.Run()
}
