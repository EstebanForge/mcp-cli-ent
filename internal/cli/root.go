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
	Use:   "mcp-cli",
	Short: "CLI tool for MCP servers",
	Long: `mcp-cli is a standalone CLI tool for interacting with MCP (Model Context Protocol) servers.
It supports both HTTP and stdio-based servers, with configuration compatible with Claude Code and VSCode.

Use mcp-cli to call MCP tools without loading them into Claude Code's context.`,
	Version: version.Version,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() error {
	return rootCmd.Execute()
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
