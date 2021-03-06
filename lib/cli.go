package gondorcli

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"

	"golang.org/x/crypto/ssh/terminal"

	"github.com/eldarion-gondor/gondor-go/lib"
	"github.com/mitchellh/go-homedir"
	"github.com/pivotal-golang/bytefmt"
	"github.com/urfave/cli"
)

// VersionInfo is @@@
type VersionInfo struct {
	Version     string
	DownloadURL string
}

// CLI is a single representation of the command line
type CLI struct {
	Name         string
	LongName     string
	Version      string
	Author       string
	Email        string
	Usage        string
	EnvVarPrefix string

	Config *GlobalConfig

	api *gondor.Client
}

// Prepare is @@@
func (c *CLI) Prepare() {
	if c.Usage == "" {
		c.Usage = fmt.Sprintf("command-line tool for interacting with the %s", c.LongName)
	}
	if c.EnvVarPrefix == "" {
		c.EnvVarPrefix = strings.ToUpper(c.Name)
	}
	c.Config = &GlobalConfig{}
}

func (c *CLI) cmd(cmdFunc func(*CLI, *cli.Context)) func(*cli.Context) error {
	return func(ctx *cli.Context) error {
		configPath, err := homedir.Expand("~/.config/gondor")
		if err != nil {
			fatal(err.Error())
		}
		if err := LoadGlobalConfig(c, ctx, configPath); err != nil {
			fatal(err.Error())
		}
		// there are some cases when the command gets called within bash
		// autocomplete which can be a bad thing!
		for i := range ctx.Args() {
			if strings.Contains(ctx.Args()[i], "generate-bash-completion") {
				os.Exit(0)
			}
		}
		cmdFunc(c, ctx)
		return nil
	}
}

func (c *CLI) stdCmd(cmdFunc func(*CLI, *cli.Context)) func(*CLI, *cli.Context) {
	return func(c *CLI, ctx *cli.Context) {
		c.checkVersion()
		if !c.IsAuthenticated() {
			fatal(fmt.Sprintf("you are not authenticated. Run `%s login` to authenticate.", c.Name))
		}
		cmdFunc(c, ctx)
	}
}

// Run is @@@
func (c *CLI) Run() {
	app := cli.NewApp()
	app.Name = c.Name
	app.Version = c.Version
	app.Author = c.Author
	app.Email = c.Email
	app.Usage = c.Usage
	app.EnableBashCompletion = true
	app.Flags = []cli.Flag{}
	if c.Config.Cloud == nil {
		app.Flags = append(app.Flags, cli.StringFlag{
			Name:   "cloud",
			Value:  "",
			Usage:  "cloud used for this invocation",
			EnvVar: fmt.Sprintf("%s_CLOUD", c.EnvVarPrefix),
		})
	}
	app.Flags = append(app.Flags,
		cli.StringFlag{
			Name:   "cluster",
			Value:  "",
			Usage:  "cluster used for this invocation",
			EnvVar: fmt.Sprintf("%s_CLUSTER", c.EnvVarPrefix),
		},
		cli.StringFlag{
			Name:   "resource-group",
			Value:  "",
			Usage:  "resource group used for this invocation",
			EnvVar: fmt.Sprintf("%s_RESOURCE_GROUP", c.EnvVarPrefix),
		},
		cli.StringFlag{
			Name:   "site",
			Value:  "",
			Usage:  "site used for this invocation",
			EnvVar: fmt.Sprintf("%s_SITE", c.EnvVarPrefix),
		},
		cli.BoolFlag{
			Name:  "log-http",
			Usage: "log HTTP interactions",
		},
	)
	app.Action = func(ctx *cli.Context) error {
		c.checkVersion()
		cli.ShowAppHelp(ctx)
		return nil
	}
	app.Commands = []cli.Command{
		{
			Name:   "login",
			Usage:  fmt.Sprintf("authenticate with the %s", c.LongName),
			Action: c.cmd(loginCmd),
		},
		{
			Name:   "logout",
			Usage:  fmt.Sprintf("invalidate any existing credentials with the %s", c.LongName),
			Action: c.cmd(logoutCmd),
		},
		{
			Name:   "upgrade",
			Usage:  fmt.Sprintf("upgrade the client to latest version supported by the %s", c.LongName),
			Action: c.cmd(upgradeCmd),
		},
		{
			Name:  "resource-groups",
			Usage: "manage resource groups",
			Action: c.cmd(func(c *CLI, ctx *cli.Context) {
				cli.ShowSubcommandHelp(ctx)
			}),
			Subcommands: []cli.Command{
				{
					Name:   "list",
					Usage:  "show resource groups to which you belong",
					Action: c.cmd(c.stdCmd(resourceGroupListCmd)),
				},
			},
		},
		{
			Name:  "keypairs",
			Usage: "manage keypairs",
			Action: c.cmd(func(c *CLI, ctx *cli.Context) {
				cli.ShowSubcommandHelp(ctx)
			}),
			Subcommands: []cli.Command{
				{
					Name:   "list",
					Usage:  "List keypairs",
					Action: c.cmd(c.stdCmd(keypairsListCmd)),
				},
				{
					Name:  "create",
					Usage: "create a keypair",
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:  "name",
							Value: "",
							Usage: "name of keypair",
						},
					},
					Action: c.cmd(c.stdCmd(keypairsCreateCmd)),
				},
				{
					Name:  "attach",
					Usage: "attach keypair to service",
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:  "instance",
							Value: "",
							Usage: "instance label",
						},
						cli.StringFlag{
							Name:  "keypair",
							Value: "",
							Usage: "name of keypair",
						},
						cli.StringFlag{
							Name:  "service",
							Value: "",
							Usage: "service path",
						},
					},
					Action: c.cmd(c.stdCmd(keypairsAttachCmd)),
				},
				{
					Name:  "detach",
					Usage: "detach keypair from service",
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:  "instance",
							Value: "",
							Usage: "instance label",
						},
						cli.StringFlag{
							Name:  "service",
							Value: "",
							Usage: "service name",
						},
					},
					Action: c.cmd(c.stdCmd(keypairsDetachCmd)),
					BashComplete: func(ctx *cli.Context) {
					},
				},
				{
					Name:   "delete",
					Usage:  "delete a keypair by name",
					Action: c.cmd(c.stdCmd(keypairsDeleteCmd)),
					BashComplete: func(ctx *cli.Context) {
						if len(ctx.Args()) > 0 {
							return
						}
						api := c.GetAPIClient(ctx)
						resourceGroup := c.GetResourceGroup(ctx)
						keypairs, err := api.KeyPairs.List(&*resourceGroup.URL)
						if err != nil {
							return
						}
						for i := range keypairs {
							fmt.Println(*keypairs[i].Name)
						}
					},
				},
			},
		},
		{
			Name:  "sites",
			Usage: "manage sites",
			Action: c.cmd(func(c *CLI, ctx *cli.Context) {
				cli.ShowSubcommandHelp(ctx)
			}),
			Subcommands: []cli.Command{
				{
					Name:   "list",
					Usage:  "show sites in the resource group",
					Action: c.cmd(c.stdCmd(sitesListCmd)),
				},
				{
					Name:  "init",
					Usage: "create a site, production instance and write config",
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:  "name",
							Value: "",
							Usage: "optional name for site",
						},
					},
					Action: c.cmd(c.stdCmd(sitesInitCmd)),
				},
				{
					Name:   "create",
					Usage:  "create a site in the resource group",
					Action: c.cmd(c.stdCmd(sitesCreateCmd)),
				},
				{
					Name:   "delete",
					Usage:  "delete a site in the resource group",
					Action: c.cmd(c.stdCmd(sitesDeleteCmd)),
					BashComplete: func(ctx *cli.Context) {
						if len(ctx.Args()) > 0 {
							return
						}
						api := c.GetAPIClient(ctx)
						resourceGroup := c.GetResourceGroup(ctx)
						sites, err := api.Sites.List(&*resourceGroup.URL)
						if err != nil {
							return
						}
						for i := range sites {
							fmt.Println(*sites[i].Name)
						}
					},
				},
				{
					Name:   "env",
					Usage:  "",
					Action: c.cmd(c.stdCmd(sitesEnvCmd)),
					BashComplete: func(ctx *cli.Context) {
						if len(ctx.Args()) > 0 {
							return
						}
						api := c.GetAPIClient(ctx)
						resourceGroup := c.GetResourceGroup(ctx)
						sites, err := api.Sites.List(&*resourceGroup.URL)
						if err != nil {
							return
						}
						for i := range sites {
							fmt.Println(*sites[i].Name)
						}
					},
				},
				{
					Name:  "users",
					Usage: "manage users",
					Action: c.cmd(func(c *CLI, ctx *cli.Context) {
						cli.ShowSubcommandHelp(ctx)
					}),
					Subcommands: []cli.Command{
						{
							Name:   "list",
							Usage:  "List users for the site",
							Action: c.cmd(c.stdCmd(sitesUsersListCmd)),
						},
						{
							Name:   "add",
							Usage:  "Add a user to site with a given role",
							Action: c.cmd(c.stdCmd(sitesUsersAddCmd)),
							Flags: []cli.Flag{
								cli.StringFlag{
									Name:  "role",
									Value: "dev",
									Usage: "desired role for user",
								},
							},
						},
					},
				},
			},
		},
		{
			Name:  "instances",
			Usage: "manage instances",
			Action: c.cmd(func(c *CLI, ctx *cli.Context) {
				cli.ShowSubcommandHelp(ctx)
			}),
			Subcommands: []cli.Command{
				{
					Name:  "create",
					Usage: "create new instance",
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:  "kind",
							Value: "",
							Usage: "kind of instance",
						},
					},
					Action: c.cmd(c.stdCmd(instancesCreateCmd)),
				},
				{
					Name:   "list",
					Usage:  "",
					Action: c.cmd(c.stdCmd(instancesListCmd)),
				},
				{
					Name:   "delete",
					Usage:  "",
					Action: c.cmd(c.stdCmd(instancesDeleteCmd)),
					BashComplete: func(ctx *cli.Context) {
						if len(ctx.Args()) > 0 {
							return
						}
						api := c.GetAPIClient(ctx)
						site := c.GetSite(ctx)
						instances, err := api.Instances.List(&*site.URL)
						if err != nil {
							fatal(err.Error())
						}
						for i := range instances {
							fmt.Println(*instances[i].Label)
						}
					},
				},
				{
					Name:  "env",
					Usage: "",
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:  "instance",
							Value: "",
							Usage: "instance label",
						},
					},
					Action: c.cmd(c.stdCmd(instancesEnvCmd)),
					BashComplete: func(ctx *cli.Context) {
						if len(ctx.Args()) > 0 {
							return
						}
						api := c.GetAPIClient(ctx)
						site := c.GetSite(ctx)
						instances, err := api.Instances.List(&*site.URL)
						if err != nil {
							fatal(err.Error())
						}
						for i := range instances {
							fmt.Println(*instances[i].Label)
						}
					},
				},
			},
		},
		{
			Name:  "services",
			Usage: "manage services",
			Action: c.cmd(func(c *CLI, ctx *cli.Context) {
				cli.ShowSubcommandHelp(ctx)
			}),
			Subcommands: []cli.Command{
				{
					Name:  "create",
					Usage: "create new service",
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:  "name",
							Value: "",
							Usage: "name of the service",
						},
						cli.StringFlag{
							Name:  "version",
							Value: "",
							Usage: "version for the new service",
						},
						cli.StringFlag{
							Name:  "instance",
							Value: "",
							Usage: "instance label",
						},
					},
					Action: c.cmd(c.stdCmd(servicesCreateCmd)),
				},
				{
					Name:  "list",
					Usage: "",
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:  "instance",
							Value: "",
							Usage: "instance label",
						},
					},
					Action: c.cmd(c.stdCmd(servicesListCmd)),
				},
				{
					Name:  "config",
					Usage: "",
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:  "instance",
							Value: "",
							Usage: "instance label",
						},
					},
					Subcommands: []cli.Command{
						{
							Name:   "get",
							Usage:  "",
							Action: c.cmd(c.stdCmd(servicesConfigGetCmd)),
						},
						{
							Name:   "set",
							Usage:  "",
							Action: c.cmd(c.stdCmd(servicesConfigSetCmd)),
						},
					},
				},
				{
					Name:  "delete",
					Usage: "",
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:  "instance",
							Value: "",
							Usage: "instance label",
						},
					},
					Action: c.cmd(c.stdCmd(servicesDeleteCmd)),
					BashComplete: func(ctx *cli.Context) {
						if len(ctx.Args()) > 0 {
							return
						}
						api := c.GetAPIClient(ctx)
						instance := c.GetInstance(ctx, nil)
						services, err := api.Services.List(&*instance.URL)
						if err != nil {
							fatal(err.Error())
						}
						for i := range services {
							fmt.Println(*services[i].Name)
						}
					},
				},
				{
					Name:  "env",
					Usage: "",
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:  "instance",
							Value: "",
							Usage: "instance label",
						},
					},
					Action: c.cmd(c.stdCmd(servicesEnvCmd)),
					BashComplete: func(ctx *cli.Context) {
						if len(ctx.Args()) > 0 {
							return
						}
						api := c.GetAPIClient(ctx)
						instance := c.GetInstance(ctx, nil)
						services, err := api.Services.List(&*instance.URL)
						if err != nil {
							fatal(err.Error())
						}
						for i := range services {
							fmt.Println(*services[i].Name)
						}
					},
				},
				{
					Name:  "scale",
					Usage: "scale up/down a service on an instance",
					Flags: []cli.Flag{
						cli.IntFlag{
							Name:  "replicas",
							Usage: "desired number of replicas",
						},
						cli.StringFlag{
							Name:  "instance",
							Value: "",
							Usage: "instance label",
						},
					},
					Action: c.cmd(c.stdCmd(servicesScaleCmd)),
					BashComplete: func(ctx *cli.Context) {
						if len(ctx.Args()) > 0 {
							return
						}
						api := c.GetAPIClient(ctx)
						instance := c.GetInstance(ctx, nil)
						services, err := api.Services.List(&*instance.URL)
						if err != nil {
							fatal(err.Error())
						}
						for i := range services {
							fmt.Println(*services[i].Name)
						}
					},
				},
				{
					Name:  "restart",
					Usage: "restart a service on a given instance",
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:  "instance",
							Value: "",
							Usage: "instance label",
						},
					},
					Action: c.cmd(c.stdCmd(servicesRestartCmd)),
					BashComplete: func(ctx *cli.Context) {
						if len(ctx.Args()) > 0 {
							return
						}
						api := c.GetAPIClient(ctx)
						instance := c.GetInstance(ctx, nil)
						services, err := api.Services.List(&*instance.URL)
						if err != nil {
							fatal(err.Error())
						}
						for i := range services {
							fmt.Println(*services[i].Name)
						}
					},
				},
			},
		},
		{
			Name:  "run",
			Usage: "run a one-off process",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "instance",
					Value: "",
					Usage: "instance label",
				},
			},
			Action: c.cmd(c.stdCmd(runCmd)),
			BashComplete: func(ctx *cli.Context) {
				if len(ctx.Args()) > 0 {
					return
				}
				api := c.GetAPIClient(ctx)
				instance := c.GetInstance(ctx, nil)
				services, err := api.Services.List(&*instance.URL)
				if err != nil {
					fatal(err.Error())
				}
				for i := range services {
					fmt.Println(*services[i].Name)
				}
			},
		},
		{
			Name:  "deploy",
			Usage: "create a new release and deploy",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "instance",
					Value: "",
					Usage: "instance label",
				},
			},
			Action: c.cmd(c.stdCmd(deployCmd)),
		},
		{
			Name:  "hosts",
			Usage: "manage hosts for an instance",
			Action: c.cmd(func(c *CLI, ctx *cli.Context) {
				cli.ShowSubcommandHelp(ctx)
			}),
			Subcommands: []cli.Command{
				{
					Name:  "list",
					Usage: "List hosts for an instance",
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:  "instance",
							Value: "",
							Usage: "instance label",
						},
					},
					Action: c.cmd(c.stdCmd(hostsListCmd)),
				},
				{
					Name:  "create",
					Usage: "Create a host for an instance",
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:  "instance",
							Value: "",
							Usage: "instance label",
						},
					},
					Action: c.cmd(c.stdCmd(hostsCreateCmd)),
				},
				{
					Name:  "delete",
					Usage: "Delete a host from an instance",
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:  "instance",
							Value: "",
							Usage: "instance label",
						},
					},
					Action: c.cmd(c.stdCmd(hostsDeleteCmd)),
				},
			},
		},
		{
			Name:  "scheduled-tasks",
			Usage: "manage scheduled tasks for an instance",
			Action: c.cmd(func(c *CLI, ctx *cli.Context) {
				cli.ShowSubcommandHelp(ctx)
			}),
			Subcommands: []cli.Command{
				{
					Name:  "list",
					Usage: "List scheduled tasks for an instance",
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:  "instance",
							Value: "",
							Usage: "instance label",
						},
					},
					Action: c.cmd(c.stdCmd(scheduledTasksListCmd)),
				},
				{
					Name:  "create",
					Usage: "Create a scheduled task for an instance",
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:  "instance",
							Value: "",
							Usage: "instance label",
						},
						cli.StringFlag{
							Name:  "name",
							Value: "",
							Usage: "scheduled task name",
						},
						cli.StringFlag{
							Name:  "timezone",
							Value: "UTC",
							Usage: "scheduled task timezone (default: UTC)",
						},
						cli.StringFlag{
							Name:  "schedule",
							Value: "",
							Usage: "scheduled task schedule (cron syntax)",
						},
					},
					Action: c.cmd(c.stdCmd(scheduledTasksCreateCmd)),
				},
				{
					Name:  "delete",
					Usage: "Delete a scheduled task from an instance",
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:  "instance",
							Value: "",
							Usage: "instance label",
						},
					},
					Action: c.cmd(c.stdCmd(scheduledTasksDeleteCmd)),
				},
			},
		},
		{
			Name:   "open",
			Usage:  "open instance URL in browser",
			Action: c.cmd(c.stdCmd(openCmd)),
			BashComplete: func(ctx *cli.Context) {
				if len(ctx.Args()) > 0 {
					return
				}
				api := c.GetAPIClient(ctx)
				instance := c.GetInstance(ctx, nil)
				services, err := api.Services.List(&*instance.URL)
				if err != nil {
					fatal(err.Error())
				}
				for i := range services {
					if *services[i].Kind == "web" {
						fmt.Println(*services[i].Name)
					}
				}
			},
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "instance",
					Value: "",
					Usage: "instance label",
				},
			},
		},
		{
			Name:  "logs",
			Usage: "view logs for an instance or service",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "instance",
					Value: "",
					Usage: "instance label",
				},
				cli.IntFlag{
					Name:  "lines",
					Value: 20,
					Usage: "number of lines to query",
				},
			},
			Action: c.cmd(c.stdCmd(logsCmd)),
			BashComplete: func(ctx *cli.Context) {
				if len(ctx.Args()) > 0 {
					return
				}
				api := c.GetAPIClient(ctx)
				instance := c.GetInstance(ctx, nil)
				services, err := api.Services.List(&*instance.URL)
				if err != nil {
					fatal(err.Error())
				}
				for i := range services {
					fmt.Println(*services[i].Name)
				}
			},
		},
		{
			Name:  "metrics",
			Usage: "view metrics for a given service",
			Action: c.cmd(func(c *CLI, ctx *cli.Context) {
				api := c.GetAPIClient(ctx)
				site := c.GetSite(ctx)
				if len(ctx.Args()) != 1 {
					fatal("missing service")
				}
				parts := strings.Split(ctx.Args()[0], "/")
				instanceLabel := parts[0]
				serviceName := parts[1]
				instance, err := api.Instances.Get(*site.URL, instanceLabel)
				if err != nil {
					fatal(err.Error())
				}
				service, err := api.Services.Get(*instance.URL, serviceName)
				if err != nil {
					fatal(err.Error())
				}
				series, err := api.Metrics.List(*service.URL)
				if err != nil {
					fatal(err.Error())
				}
				for i := range series {
					s := series[i]
					fmt.Printf("%s = ", s.Name)
					for j := range s.Points {
						value := s.Points[j][2]
						switch *s.Name {
						case "filesystem/limit_bytes_gauge", "filesystem/usage_bytes_gauge", "memory/usage_bytes_gauge", "memory/working_set_bytes_gauge":
							fmt.Printf("%s ", bytefmt.ByteSize(uint64(value)))
							break
						default:
							fmt.Printf("%d ", value)
						}
					}
					fmt.Println("")
				}
			}),
		},
	}
	app.Run(os.Args)
}

// IsAuthenticated is @@@
func (c *CLI) IsAuthenticated() bool {
	return c.Config.loaded && c.Config.Identity != nil
}

// SetCloud is @@@
func (c *CLI) SetCloud(cloud *Cloud) {
	c.Config.Cloud = cloud
}

// SetCluster is @@@
func (c *CLI) SetCluster(cluster *Cluster) {
	c.Config.Cluster = cluster
}

// SetIdentity is @@@
func (c *CLI) SetIdentity(identity *Identity) {
	c.Config.Identity = identity
}

// GetAPIClient is @@@
func (c *CLI) GetAPIClient(ctx *cli.Context) *gondor.Client {
	if c.api == nil {
		LoadSiteConfig()
		if siteCfg.Cluster != "" {
			var siteCloud, siteCluster string
			if strings.Count(siteCfg.Cluster, "/") == 0 {
				siteCluster = siteCfg.Cluster
			} else {
				parts := strings.Split(siteCfg.Cluster, "/")
				siteCloud, siteCluster = parts[0], parts[1]
			}
			if siteCloud != "" {
				cloud, err := c.Config.GetCloudByName(siteCloud)
				if err != nil {
					fatal(err.Error())
				}
				c.SetCloud(cloud)
			}
			if siteCluster != "" {
				cluster, err := c.Config.Cloud.GetClusterByName(siteCluster)
				if err != nil {
					fatal(err.Error())
				}
				c.SetCluster(cluster)
			}
		}
		httpClient := c.GetHTTPClient(ctx)
		c.api = gondor.NewClient(c.Config.GetClientConfig(), httpClient)
		if ctx.GlobalBool("log-http") {
			c.api.EnableHTTPLogging(true)
		}
		c.api.SetClientVersion(fmt.Sprintf("%s %s", c.Name, c.Version))
	}
	return c.api
}

// GetTLSConfig is @@@
func (c *CLI) GetTLSConfig(ctx *cli.Context) *tls.Config {
	var pool *x509.CertPool
	caCert, err := c.Config.Cluster.GetCertificateAuthority()
	if err != nil {
		// warn user
	} else {
		if caCert != nil {
			pool = x509.NewCertPool()
			pool.AddCert(caCert)
		}
	}
	return &tls.Config{
		RootCAs:            pool,
		InsecureSkipVerify: c.Config.Cluster.InsecureSkipVerify,
	}
}

// GetHTTPClient is @@@
func (c *CLI) GetHTTPClient(ctx *cli.Context) *http.Client {
	tr := &http.Transport{
		TLSClientConfig: c.GetTLSConfig(ctx),
	}
	return &http.Client{Transport: tr}
}

func (c *CLI) checkVersion() {
	var shouldCheck bool
	var outs io.Writer
	if terminal.IsTerminal(int(os.Stdout.Fd())) {
		outs = os.Stdout
		shouldCheck = true
	} else if terminal.IsTerminal(int(os.Stderr.Fd())) {
		outs = os.Stderr
		shouldCheck = true
	}
	if strings.Contains(c.Version, "dev") {
		shouldCheck = false
	}
	if shouldCheck {
		newVersion, err := c.CheckForUpgrade()
		if err != nil {
			fmt.Fprintf(outs, errize(fmt.Sprintf(
				"Failed checking for upgrade: %s\n",
				err.Error(),
			)))
		}
		if newVersion != nil {
			fmt.Fprintf(outs, heyYou(fmt.Sprintf(
				"You are using an older version (%s; latest: %s) of this client.\nTo upgrade run `%s upgrade`.\n",
				c.Version,
				newVersion.Version,
				c.Name,
			)))
		}
	}
}

// CheckForUpgrade is @@@
func (c *CLI) CheckForUpgrade() (*VersionInfo, error) {
	req, err := http.NewRequest("GET", "https://api.us2.gondor.io/v2/client/", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("X-Gondor-Client", fmt.Sprintf("%s %s", c.Name, c.Version))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	var versionJSON interface{}
	err = json.NewDecoder(resp.Body).Decode(&versionJSON)
	if err != nil {
		return nil, err
	}
	newVersion := versionJSON.(map[string]interface{})["version"].(string)
	downloadURL := versionJSON.(map[string]interface{})["download_url"].(map[string]interface{})[runtime.GOOS].(map[string]interface{})[runtime.GOARCH].(string)
	if newVersion != c.Version {
		return &VersionInfo{
			newVersion,
			downloadURL,
		}, nil
	}
	return nil, nil
}

func parseSiteIdentifier(value string) (string, string) {
	if value == "" {
		fatal("site not defined (either --site or in gondor.yml)")
	}
	if strings.Count(value, "/") != 1 {
		fatal(fmt.Sprintf("invalid site value: %q", value))
	}
	parts := strings.Split(value, "/")
	return parts[0], parts[1]
}

// GetResourceGroup is @@@
func (c *CLI) GetResourceGroup(ctx *cli.Context) *gondor.ResourceGroup {
	api := c.GetAPIClient(ctx)
	if ctx.GlobalString("resource-group") != "" {
		resourceGroup, err := api.ResourceGroups.GetByName(ctx.GlobalString("resource-group"))
		if err != nil {
			fatal(err.Error())
		}
		return resourceGroup
	}
	if err := LoadSiteConfig(); err == nil {
		resourceGroupName, _ := parseSiteIdentifier(siteCfg.Identifier)
		resourceGroup, err := api.ResourceGroups.GetByName(resourceGroupName)
		if err != nil {
			fatal(err.Error())
		}
		return resourceGroup
	} else if _, ok := err.(ErrConfigNotFound); !ok {
		fatal(fmt.Sprintf("failed to load gondor.yml\n%s", err.Error()))
	}
	user, err := api.AuthenticatedUser()
	if err != nil {
		fatal(err.Error())
	}
	if user.ResourceGroup == nil {
		fatal("you do not have a personal resource group.")
	}
	return user.ResourceGroup
}

// GetSite is @@@
func (c *CLI) GetSite(ctx *cli.Context) *gondor.Site {
	var err error
	var siteName string
	var resourceGroup *gondor.ResourceGroup
	api := c.GetAPIClient(ctx)
	siteFlag := ctx.GlobalString("site")
	if siteFlag != "" {
		if strings.Count(siteFlag, "/") == 1 {
			parts := strings.Split(siteFlag, "/")
			resourceGroup, err = api.ResourceGroups.GetByName(parts[0])
			if err != nil {
				fatal(err.Error())
			}
			siteName = parts[1]
		} else {
			resourceGroup = c.GetResourceGroup(ctx)
			siteName = siteFlag
		}
	} else {
		LoadSiteConfig()
		resourceGroup = c.GetResourceGroup(ctx)
		_, siteName = parseSiteIdentifier(siteCfg.Identifier)
	}
	site, err := api.Sites.Get(siteName, &*resourceGroup.URL)
	if err != nil {
		fatal(err.Error())
	}
	return site
}

// GetInstance is @@@
func (c *CLI) GetInstance(ctx *cli.Context, site *gondor.Site) *gondor.Instance {
	api := c.GetAPIClient(ctx)
	if site == nil {
		site = c.GetSite(ctx)
	}
	branch := siteCfg.vcs.Branch
	label := ctx.String("instance")
	if label == "" {
		if branch != "" {
			var ok bool
			label, ok = siteCfg.Branches[branch]
			if !ok {
				fatal(fmt.Sprintf("unable to map %q to an instance. Please provide --instance or map it to an instance in gondor.yml.", branch))
			}
		} else {
			fatal("instance not defined (missing --instance?).")
		}
	}
	instance, err := api.Instances.Get(*site.URL, label)
	if err != nil {
		fatal(err.Error())
	}
	return instance
}
