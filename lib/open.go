package gondorcli

import (
	"fmt"

	"github.com/skratchdot/open-golang/open"
	"github.com/urfave/cli"
)

func openCmd(c *CLI, ctx *cli.Context) {
	usage := func(msg string) {
		fmt.Printf("Usage: %s open [--instance] <service-name>\n", c.Name)
		fatal(msg)
	}
	if len(ctx.Args()) == 0 {
		usage("too few arguments")
	}
	api := c.GetAPIClient(ctx)
	instance := c.GetInstance(ctx, nil)
	service, err := api.Services.Get(*instance.URL, ctx.Args()[0])
	if err != nil {
		fatal(err.Error())
	}
	open.Run(fmt.Sprintf("https://%s/", *service.WebURL))
}
