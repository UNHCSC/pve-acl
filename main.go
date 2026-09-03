package main

import (
	"context"
	"time"

	"github.com/UNHCSC/organesson/app"
	"github.com/UNHCSC/organesson/auth"
	"github.com/UNHCSC/organesson/config"
	"github.com/UNHCSC/organesson/db"
	jobscheduler "github.com/UNHCSC/organesson/scheduler"
	"github.com/gofiber/fiber/v2"
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

	if err = db.EnsureInitialSetup(); err != nil {
		log.Panicf("Failed to complete initial setup: %v\n", err)
	}
	if _, err = db.RecoverAbandonedJobs(time.Now().UTC()); err != nil {
		log.Panicf("Failed to recover abandoned jobs: %v\n", err)
	}

	if err = auth.Init(log); err != nil {
		log.Panicf("Failed to initialize auth: %v\n", err)
	}

	var schedulerService *jobscheduler.Service
	var schedulerDatabaseFile string

	schedulerDatabaseFile = jobscheduler.ResolveDatabaseFile(config.Config.Database.File, config.Config.Scheduler.DatabaseFile)
	if schedulerService, err = jobscheduler.Init(schedulerDatabaseFile); err != nil {
		log.Panicf("Failed to initialize scheduler: %v\n", err)
	}
	defer schedulerService.Close()

	go func() {
		var runErr error
		if runErr = schedulerService.Run(context.Background()); runErr != nil {
			log.Errorf("Scheduler stopped: %v\n", runErr)
		}
	}()

	var fiberApp *fiber.App
	if fiberApp, err = app.InitAndListen(log); err != nil {
		log.Panicf("Failed to initialize app: %v\n", err)
	} else {
		if err = app.StartApp(fiberApp); err != nil {
			log.Panicf("Failed to start app: %v\n", err)
		}
	}
}
