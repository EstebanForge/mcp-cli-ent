package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/mcp-cli-ent/mcp-cli/internal/config"
	"github.com/mcp-cli-ent/mcp-cli/internal/mcp"
	"github.com/mcp-cli-ent/mcp-cli/pkg/version"
)

var (
	cfgFile      string
	verbose      bool
	timeout      int
	refreshCache bool
	clearCache   bool
)

// ToolsCacheEntry represents a cached tool listing for a server
type ToolsCacheEntry struct {
	Tools      []mcp.Tool `json:"tools"`
	LastUpdate time.Time  `json:"lastUpdate"`
}

// ToolsCache represents the full cache structure
type ToolsCache struct {
	Servers map[string]ToolsCacheEntry `json:"servers"`
}

const (
	CacheFileName = "tools_cache.json"
	CacheTTL      = 30 * 24 * time.Hour // Cache expires after 30 days
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
	// Override the help function to include available servers
	originalHelpFunc := rootCmd.HelpFunc()
	rootCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		originalHelpFunc(cmd, args) // Show standard help
		if err := showAvailableServers(cmd); err != nil {
			fmt.Printf("Warning: Failed to load servers: %v\n", err)
		}
	})
	return rootCmd.Execute()
}

// GetCachePath returns the path to the tools cache file
func GetCachePath() (string, error) {
	configDir, err := config.GetConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, CacheFileName), nil
}

// LoadToolsFromCache loads cached tool listings
func LoadToolsFromCache() (*ToolsCache, error) {
	cachePath, err := GetCachePath()
	if err != nil {
		return nil, err
	}

	// Check if cache file exists
	if _, err := os.Stat(cachePath); os.IsNotExist(err) {
		return nil, nil // No cache yet, not an error
	}

	data, err := os.ReadFile(cachePath)
	if err != nil {
		return nil, err
	}

	var cache ToolsCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, err
	}

	// Check if cache is still valid
	now := time.Now()
	for serverName, entry := range cache.Servers {
		if now.Sub(entry.LastUpdate) > CacheTTL {
			// Cache expired for this server, remove it
			delete(cache.Servers, serverName)
		}
	}

	return &cache, nil
}

// SaveToolsToCache saves tool listings to cache
func SaveToolsToCache(cache *ToolsCache) error {
	cachePath, err := GetCachePath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(cachePath, data, 0644)
}

// showRootHelpWithServers displays available tools from all MCP servers with usage examples
func showRootHelpWithServers(cmd *cobra.Command) error {
	fmt.Printf("MCP CLI-Ent v%s\n\n", version.Version)

	// Load configuration
	configPath := GetConfigPath()
	cfg, err := LoadConfiguration(configPath)
	if err != nil {
		fmt.Println("No configuration found - run 'mcp-cli-ent create-config'")
		return nil
	}

	enabledServers := cfg.GetEnabledServers()
	if len(enabledServers) == 0 {
		fmt.Println("No enabled MCP servers found")
		return nil
	}

	// If clearCache or refreshCache is set, clear cache file
	// These flags are aliases - both trigger cache refresh
	if clearCache || refreshCache {
		cachePath, err := GetCachePath()
		if err == nil {
			_ = os.Remove(cachePath)
			fmt.Println("Cache cleared.")
		}
		// Force refresh after clearing
		refreshCache = true
		clearCache = true
	}

	cache, err := LoadToolsFromCache()
	if err != nil {
		cache = nil
	}
	useCache := cache != nil && !refreshCache && !clearCache

	var totalTools int
	var toolsByServer map[string][]mcp.Tool

	if useCache {
		// Check if all servers are in cache
		toolsByServer = make(map[string][]mcp.Tool)
		allCached := true
		for serverName := range enabledServers {
			if entry, ok := cache.Servers[serverName]; ok {
				toolsByServer[serverName] = entry.Tools
			} else {
				allCached = false
				break
			}
		}

		if !allCached {
			useCache = false
		} else {
			// Count tools from cache
			for _, tools := range toolsByServer {
				totalTools += len(tools)
			}
		}
	}

	if !useCache {
		// Discover tools from all servers
		factory, err := getSessionAwareClientFactory()
		if err != nil {
			fmt.Printf("Error: failed to create client factory: %v\n", err)
			fmt.Println("\nRun 'mcp-cli-ent list-servers' to see available servers")
			return nil
		}

		toolsByServer = make(map[string][]mcp.Tool)
		var wg sync.WaitGroup
		var mu sync.Mutex

		// Start workers for parallel discovery
		for serverName := range enabledServers {
			wg.Add(1)
			go func(name string) {
				defer wg.Done()
				ctx := context.Background()
				serverConfig := enabledServers[name]

				mcpClient, err := factory.CreateClient(name, serverConfig)
				if err != nil {
					fmt.Printf("%s: (failed to connect: %v)\n", name, err)
					return
				}

				tools, err := mcpClient.ListTools(ctx)
				_ = mcpClient.Close()
				if err != nil {
					fmt.Printf("%s: (failed to list tools: %v)\n", name, err)
					return
				}

				mu.Lock()
				toolsByServer[name] = tools
				mu.Unlock()
			}(serverName)
		}

		// Wait for all workers to complete
		wg.Wait()

		// Count total tools and build cache
		totalTools = 0
		newCache := &ToolsCache{Servers: make(map[string]ToolsCacheEntry)}
		for serverName, tools := range toolsByServer {
			totalTools += len(tools)
			newCache.Servers[serverName] = ToolsCacheEntry{
				Tools:      tools,
				LastUpdate: time.Now(),
			}
		}

		// Save to cache
		_ = SaveToolsToCache(newCache)
	}

	// Display tools
	for serverName, serverConfig := range enabledServers {
		tools, ok := toolsByServer[serverName]
		if !ok || len(tools) == 0 {
			continue
		}

		// Print server name with description if available
		if serverConfig.Description != "" {
			fmt.Printf("%s (%s):\n", serverName, serverConfig.Description)
		} else {
			fmt.Printf("%s:\n", serverName)
		}

		// Print each tool with usage example
		for _, tool := range tools {
			fmt.Printf("  • %s\n", tool.Name)
			if verbose && tool.Description != "" {
				// In verbose mode, show full description
				fmt.Printf("    desc: %s\n", tool.Description)
			}
			exampleArgs := BuildExampleArgs(&tool)
			if verbose {
				fmt.Printf("    call: mcp-cli-ent call-tool %s %s %s\n", serverName, tool.Name, exampleArgs)
			} else {
				fmt.Printf("    mcp-cli-ent call-tool %s %s %s\n", serverName, tool.Name, exampleArgs)
			}
		}
		fmt.Println()
	}

	if totalTools == 0 {
		fmt.Println("No tools found on any server")
		return nil
	}

	fmt.Printf("Total: %d tools across %d servers\n\n", totalTools, len(enabledServers))
	fmt.Println("For full details (descriptions + parameters):")
	fmt.Println("  mcp-cli-ent list-tools <server>")
	fmt.Println("\nUse --verbose for tool descriptions")

	return nil
}

// BuildExampleArgs creates example JSON arguments based on tool schema
func BuildExampleArgs(tool *mcp.Tool) string {
	if tool == nil || tool.InputSchema == nil {
		return "'{}'"
	}

	properties, ok := tool.InputSchema["properties"].(map[string]interface{})
	if !ok || len(properties) == 0 {
		return "'{}'"
	}

	// Limit to first 4 parameters to keep output concise
	maxParams := 4
	count := 0
	examples := make([]string, 0, min(len(properties), maxParams))
	for name := range properties {
		if count >= maxParams {
			examples = append(examples, "...")
			break
		}
		examples = append(examples, fmt.Sprintf(`"%s":"..."`, name))
		count++
	}

	return fmt.Sprintf("'{%s}'", strings.Join(examples, ", "))
}

// min returns the smaller of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// showAvailableServers displays a simple list of available MCP servers
func showAvailableServers(cmd *cobra.Command) error {
	// Load configuration
	configPath := GetConfigPath()
	cfg, err := LoadConfiguration(configPath)
	if err != nil {
		return nil // Skip if config not found
	}

	enabledServers := cfg.GetEnabledServers()
	if len(enabledServers) == 0 {
		return nil
	}

	fmt.Println()
	fmt.Println("Available MCP Servers:")
	for name, serverConfig := range enabledServers {
		if serverConfig.Description != "" {
			fmt.Printf("  • %s - %s\n", name, serverConfig.Description)
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
	rootCmd.PersistentFlags().BoolVar(&refreshCache, "refresh", false, "force refresh of tools cache (alias: --clear-cache)")
	rootCmd.PersistentFlags().BoolVar(&clearCache, "clear-cache", false, "clear tools cache (alias: --refresh)")

	// Bind flags to viper
	_ = viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose"))
	_ = viper.BindPFlag("timeout", rootCmd.PersistentFlags().Lookup("timeout"))
	_ = viper.BindPFlag("refresh", rootCmd.PersistentFlags().Lookup("refresh"))
	_ = viper.BindPFlag("clear-cache", rootCmd.PersistentFlags().Lookup("clear-cache"))
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
