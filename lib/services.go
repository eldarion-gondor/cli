package gondorcli

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/eldarion-gondor/gondor-go/lib"
	"github.com/olekukonko/tablewriter"
	"github.com/urfave/cli"
)

func servicesCreateCmd(c *CLI, ctx *cli.Context) {
	usage := func(msg string) {
		fmt.Printf("Usage: %s services create [--name,--version,--instance] <service-kind>\n", c.Name)
		fatal(msg)
	}
	if len(ctx.Args()) == 0 {
		usage("too few arguments")
	}
	api := c.GetAPIClient(ctx)
	instance := c.GetInstance(ctx, nil)
	name := ctx.String("name")
	service := gondor.Service{
		Instance: instance.URL,
		Name:     &name,
		Kind:     &ctx.Args()[0],
	}
	if ctx.String("version") != "" {
		version := ctx.String("version")
		service.Version = &version
	}
	if err := api.Services.Create(&service); err != nil {
		fatal(err.Error())
	}
	success(fmt.Sprintf("%s service has been created.", *service.Kind))
}

func servicesListCmd(c *CLI, ctx *cli.Context) {
	api := c.GetAPIClient(ctx)
	instance := c.GetInstance(ctx, nil)
	services, err := api.Services.List(&*instance.URL)
	if err != nil {
		fatal(err.Error())
	}
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Name", "Kind", "Size", "Replicas", "Web URL", "State"})
	for i := range services {
		service := services[i]
		var webURL string
		if service.WebURL != nil {
			webURL = *service.WebURL
		}
		table.Append([]string{
			*service.Name,
			*service.Kind,
			*service.Size,
			strconv.Itoa(*service.Replicas),
			webURL,
			*service.State,
		})
	}
	table.Render()
}

func servicesDeleteCmd(c *CLI, ctx *cli.Context) {
	usage := func(msg string) {
		fmt.Printf("Usage: %s services delete <name>\n", c.Name)
		fatal(msg)
	}
	if len(ctx.Args()) == 0 {
		usage("too few arguments")
	}
	api := c.GetAPIClient(ctx)
	instance := c.GetInstance(ctx, nil)
	name := ctx.Args()[0]
	service, err := api.Services.Get(*instance.URL, name)
	if err != nil {
		fatal(err.Error())
	}
	if err := api.Services.Delete(*service.URL); err != nil {
		fatal(err.Error())
	}
	success(fmt.Sprintf("%s service has been deleted.", name))
}

func servicesEnvCmd(c *CLI, ctx *cli.Context) {
	usage := func(msg string) {
		fmt.Printf("Usage: %s services env <name>\n", c.Name)
		fatal(msg)
	}
	if len(ctx.Args()) == 0 {
		usage("too few arguments")
	}
	api := c.GetAPIClient(ctx)
	instance := c.GetInstance(ctx, nil)
	name := ctx.Args()[0]
	service, err := api.Services.Get(*instance.URL, name)
	if err != nil {
		fatal(err.Error())
	}
	var createMode bool
	var displayEnvVars, desiredEnvVars []*gondor.EnvironmentVariable
	if len(ctx.Args()) >= 2 {
		createMode = true
		for i := range ctx.Args() {
			arg := ctx.Args()[i]
			if strings.Contains(arg, "=") {
				parts := strings.SplitN(arg, "=", 2)
				envVar := gondor.EnvironmentVariable{
					Service: service.URL,
					Key:     &parts[0],
					Value:   &parts[1],
				}
				desiredEnvVars = append(desiredEnvVars, &envVar)
			}
		}
	}
	if !createMode {
		displayEnvVars, err = api.EnvVars.ListByService(*service.URL)
		for i := range displayEnvVars {
			envVar := displayEnvVars[i]
			fmt.Printf("%s=%s\n", *envVar.Key, *envVar.Value)
		}
	} else {
		if err := api.EnvVars.Create(desiredEnvVars); err != nil {
			fatal(err.Error())
		}
		for i := range desiredEnvVars {
			fmt.Printf("%s=%s\n", *desiredEnvVars[i].Key, *desiredEnvVars[i].Value)
		}
	}
}

func servicesScaleCmd(c *CLI, ctx *cli.Context) {
	usage := func(msg string) {
		fmt.Printf("Usage: %s services scale --replicas=N <name>\n", c.Name)
		fatal(msg)
	}
	if len(ctx.Args()) == 0 {
		usage("too few arguments")
	}
	api := c.GetAPIClient(ctx)
	instance := c.GetInstance(ctx, nil)
	name := ctx.Args()[0]
	replicas := ctx.Int("replicas")
	service, err := api.Services.Get(*instance.URL, name)
	if err != nil {
		fatal(err.Error())
	}
	if err := service.SetReplicas(replicas); err != nil {
		fatal(err.Error())
	}
	success(fmt.Sprintf("%s service has been scaled to %d replicas.", name, replicas))
}

func servicesRestartCmd(c *CLI, ctx *cli.Context) {
	usage := func(msg string) {
		fmt.Printf("Usage: %s services restart <name>\n", c.Name)
		fatal(msg)
	}
	if len(ctx.Args()) == 0 {
		usage("too few arguments")
	}
	api := c.GetAPIClient(ctx)
	instance := c.GetInstance(ctx, nil)
	name := ctx.Args()[0]
	service, err := api.Services.Get(*instance.URL, name)
	if err != nil {
		fatal(err.Error())
	}
	if err := service.Restart(); err != nil {
		fatal(err.Error())
	}
	success(fmt.Sprintf("%s service has been restarted.", name))
}

func servicesConfigGetCmd(c *CLI, ctx *cli.Context) {
	usage := func(msg string) {
		fmt.Printf("Usage: %s services config get <name> <attribute>\n", c.Name)
		fatal(msg)
	}
	if len(ctx.Args()) == 0 {
		usage("too few arguments")
	}
	attribute := ctx.Args().Get(1)
	if attribute == "" {
		usage("too few arguments")
	}
	api := c.GetAPIClient(ctx)
	instance := c.GetInstance(ctx, nil)
	name := ctx.Args()[0]
	service, err := api.Services.Get(*instance.URL, name)
	if err != nil {
		fatal(err.Error())
	}
	switch attribute {
	case "image":
		fmt.Println(*service.Image)
	case "size":
		fmt.Println(*service.Size)
	default:
		fatal(fmt.Sprintf("unknown service attribute %q", attribute))
	}
}

func servicesConfigSetCmd(c *CLI, ctx *cli.Context) {
	usage := func(msg string) {
		fmt.Printf("Usage: %s services config set <name> <attribute> <value>\n", c.Name)
		fatal(msg)
	}
	if len(ctx.Args()) == 0 {
		usage("too few arguments")
	}
	attribute := ctx.Args().Get(1)
	if attribute == "" {
		usage("too few arguments")
	}
	value := ctx.Args().Get(2)
	if attribute == "" {
		usage("too few arguments")
	}
	api := c.GetAPIClient(ctx)
	instance := c.GetInstance(ctx, nil)
	name := ctx.Args()[0]
	service, err := api.Services.Get(*instance.URL, name)
	if err != nil {
		fatal(err.Error())
	}
	var changed bool
	changedService := gondor.Service{
		URL: service.URL,
	}
	switch attribute {
	case "image":
		if *service.Image != value {
			changedService.Image = &value
			changed = true
		}
	case "size":
		if *service.Size != value {
			changedService.Size = &value
			changed = true
		}
	default:
		fatal(fmt.Sprintf("unknown service attribute %q", attribute))
	}
	if !changed {
		fmt.Printf("No changes detected.")
		os.Exit(0)
	}
	if err := api.Services.Update(changedService); err != nil {
		fatal(fmt.Sprintf("configuring service %q: %v", name, err))
	}
	success(fmt.Sprintf("%s service has been configured.", name))
}
