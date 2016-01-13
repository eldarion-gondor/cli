package main

import (
	"fmt"

	"github.com/codegangsta/cli"
	"github.com/skratchdot/open-golang/open"
)

func openCmd(ctx *cli.Context) {
	usage := func(msg string) {
		fmt.Println("Usage: g3a open [--instance] <service-name>")
		fatal(msg)
	}
	if len(ctx.Args()) == 0 {
		usage("too few arguments")
	}
	api := getAPIClient(ctx)
	instance := getInstance(ctx, api, nil)
	service, err := api.Services.Get(*instance.URL, ctx.Args()[0])
	if err != nil {
		fatal(err.Error())
	}
	open.Run(fmt.Sprintf("https://%s/", *service.WebURL))
}
