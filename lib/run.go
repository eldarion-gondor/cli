package gondorcli

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"github.com/urfave/cli"
)

func runCmd(c *CLI, ctx *cli.Context) {
	usage := func(msg string) {
		fmt.Printf("Usage: %s run [--instance] <service-name> -- <executable> <arg-or-option>...\n", c.Name)
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
	endpoint, err := service.Run(ctx.Args()[1:])
	if err != nil {
		fatal(err.Error())
	}
	re := remoteExec{
		endpoint:      endpoint,
		enableTty:     true,
		httpClient:    c.GetHTTPClient(ctx),
		tlsConfig:     c.GetTLSConfig(ctx),
		showAttaching: true,
		logger:        log.New(ioutil.Discard, "", log.LstdFlags),
	}
	os.Exit(re.execute())
}
