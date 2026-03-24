package main

import (
	"github.com/UNHCSC/pve-acl/auth"
	"github.com/UNHCSC/pve-acl/config"
	"github.com/UNHCSC/pve-acl/db"
	"github.com/z46-dev/golog"
)

var (
	log *golog.Logger = golog.New().Prefix("[MAIN]", golog.BoldBlue)
	err error
)

func main() {
	if err = config.Init("config.toml"); err != nil {
		log.Panicf("Failed to initialize config: %v\n", err)
	}

	if err = db.Init(log); err != nil {
		log.Panicf("Failed to initialize database: %v\n", err)
	}

	if err = auth.Init(log); err != nil {
		log.Panicf("Failed to initialize auth: %v\n", err)
	}
}
