package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/mcp-cli-ent/mcp-cli/internal/config"
	"github.com/mcp-cli-ent/mcp-cli/pkg/version"
)

var (
	cfgFile string
	verbose bool
	timeout int
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "mcp-cli-ent",
	Short: "MCP server client",
	Long: `MCP CLI-Ent: Call MCP tools without loading them into agent context.

Use "mcp-cli-ent --help verbose" for detailed information.`,
	Version: version.Version,
	RunE: func(cmd *cobra.Command, args []string) error {
		// If no command was specified, show help with available servers
		if len(args) == 0 {
			return showRootHelpWithServers(cmd)
		}
		return nil
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() error {
	return rootCmd.Execute()
}

// showRootHelpWithServers displays the root help along with available MCP servers
func showRootHelpWithServers(cmd *cobra.Command) error {
	// First show the standard help
	if err := cmd.Help(); err != nil {
		return err
	}

	// Then show available servers
	fmt.Println()
	fmt.Println("Available MCP Servers:")

	// Try to load configuration and list enabled servers
	configPath := GetConfigPath()
	cfg, err := LoadConfiguration(configPath)
	if err != nil {
		fmt.Println("  (No configuration found - run 'mcp-cli-ent create-config' to create one)")
		return nil
	}

	enabledServers := cfg.GetEnabledServers()
	if len(enabledServers) == 0 {
		fmt.Println("  (No enabled MCP servers found)")
		return nil
	}

	for name, serverConfig := range enabledServers {
		if serverConfig.Description != "" {
			fmt.Printf("  • %s | %s\n", name, serverConfig.Description)
		} else {
			fmt.Printf("  • %s\n", name)
		}
	}

	fmt.Println()
	return nil
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "configuration file path (default is mcp_servers.json)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().IntVar(&timeout, "timeout", 30, "request timeout in seconds")

	// Bind flags to viper
	_ = viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose"))
	_ = viper.BindPFlag("timeout", rootCmd.PersistentFlags().Lookup("timeout"))
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	// Ensure config directory and files exist on first run
	if err := config.EnsureConfigDirectory(); err != nil && verbose {
		fmt.Fprintf(os.Stderr, "Warning: failed to ensure config directory: %v\n", err)
	}

	if cfgFile != "" {
		// Use config file from the flag
		viper.SetConfigFile(cfgFile)
	} else {
		// Find config file
		viper.SetConfigName("mcp_servers")
		viper.SetConfigType("json")
		viper.AddConfigPath(".")
		viper.AddConfigPath("$HOME/.config")
		viper.AddConfigPath("$HOME")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		if verbose {
			fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
		}
	}
}
