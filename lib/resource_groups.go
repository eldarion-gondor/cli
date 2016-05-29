package gondorcli

import (
	"os"

	"github.com/olekukonko/tablewriter"
	"github.com/urfave/cli"
)

func resourceGroupListCmd(c *CLI, ctx *cli.Context) {
	api := c.GetAPIClient(ctx)
	resourceGroups, err := api.ResourceGroups.List()
	if err != nil {
		fatal(err.Error())
	}
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Name"})
	for i := range resourceGroups {
		resourceGroup := resourceGroups[i]
		table.Append([]string{
			*resourceGroup.Name,
		})
	}
	table.Render()
}
