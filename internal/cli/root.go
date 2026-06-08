package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
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
	humanOutput  bool
	searchQuery  string
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
	Long: fmt.Sprintf(`MCP CLI-Ent v%s: Use MCP tools without loading them into agent context window.

Use "mcp-cli-ent --help verbose" for detailed information.`,
		version.Version),
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
	autoInstallAlias()

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
	// Load configuration
	configPath := GetConfigPath()
	cfg, err := LoadConfiguration(configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "No configuration found - run 'mcp-cli-ent create-config'")
		return nil
	}

	enabledServers := cfg.GetEnabledServers()
	if len(enabledServers) == 0 {
		fmt.Fprintln(os.Stderr, "No enabled MCP servers found")
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
			fmt.Fprintf(os.Stderr, "Error: failed to create client factory: %v\n", err)
			fmt.Fprintln(os.Stderr, "Run 'mcp-cli-ent list-servers' to see available servers")
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
					fmt.Fprintf(os.Stderr, "%s: (failed to connect: %v)\n", name, err)
					return
				}

				tools, err := mcpClient.ListTools(ctx)
				_ = mcpClient.Close()
				if err != nil {
					fmt.Fprintf(os.Stderr, "%s: (failed to list tools: %v)\n", name, err)
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

	// Filter by search query if provided
	if searchQuery != "" {
		for serverName, tools := range toolsByServer {
			var filtered []mcp.Tool
			for _, tool := range tools {
				if toolMatches(tool, searchQuery) {
					filtered = append(filtered, tool)
				}
			}
			if len(filtered) == 0 {
				delete(toolsByServer, serverName)
			} else {
				toolsByServer[serverName] = filtered
			}
		}
		// Recount
		totalTools = 0
		for _, tools := range toolsByServer {
			totalTools += len(tools)
		}
	}

	if totalTools == 0 {
		if humanOutput {
			if searchQuery != "" {
				fmt.Printf("No tools matching '%s' found\n", searchQuery)
			} else {
				fmt.Println("No tools found on any server")
			}
		} else {
			if searchQuery != "" {
				return encodeErrorJSON("no_match", "No tools matching '%s' found", searchQuery)
			}
			return encodeErrorJSON("no_tools", "No tools found on any server")
		}
		return nil
	}

	// Output: JSON by default, --human for terminal
	// Build sorted server keys for deterministic output in both modes
	var sortedServers []string
	for serverName := range enabledServers {
		sortedServers = append(sortedServers, serverName)
	}
	sort.Strings(sortedServers)

	if humanOutput {
		fmt.Printf("MCP CLI-Ent v%s\n\n", version.Version)
		fmt.Println("Usage:")
		fmt.Println("mcp-cli-ent call <server_name> <tool_name> <params>")
		fmt.Println()

		for _, serverName := range sortedServers {
			serverConfig := enabledServers[serverName]
			tools, ok := toolsByServer[serverName]
			if !ok || len(tools) == 0 {
				continue
			}

			displayName := fmt.Sprintf("<%s>", serverName)
			if serverConfig.Description != "" {
				fmt.Printf("%s (%s) [%d]\n", displayName, serverConfig.Description, len(tools))
			} else {
				fmt.Printf("%s [%d]\n", displayName, len(tools))
			}

			printToolsHuman(tools, serverName, verbose)
			fmt.Println()
		}

		fmt.Printf("Total: %d tools across %d servers\n\n", totalTools, len(toolsByServer))
		fmt.Println("For full specific MCP server details:")
		fmt.Println("  mcp-cli-ent list-tools <server_name>")
		fmt.Println("\nUse --verbose for expanded tool details")
		return nil
	}

	// JSON output (default): compact index — name + description only
	// Full details via: mcp-cli-ent list-tools <server>
	result := make(map[string][]indexTool)

	for _, serverName := range sortedServers {
		tools, ok := toolsByServer[serverName]
		if !ok || len(tools) == 0 {
			continue
		}
		for _, tool := range tools {
			result[serverName] = append(result[serverName], indexTool{
				Name:        tool.Name,
				Description: tool.Description,
			})
		}
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}

// BuildExampleArgs creates example JSON arguments based on tool schema.
// It prioritizes required parameters to keep output concise, and formats
// properties with correct JSON types (string -> "...", integer -> 0, boolean -> true, etc.).
func BuildExampleArgs(tool *mcp.Tool) string {
	if tool == nil || tool.InputSchema == nil {
		return "'{}'"
	}

	properties, ok := tool.InputSchema["properties"].(map[string]interface{})
	if !ok || len(properties) == 0 {
		return "'{}'"
	}

	// Extract required properties
	var requiredList []string
	if reqs, ok := tool.InputSchema["required"].([]interface{}); ok {
		for _, req := range reqs {
			if reqStr, ok := req.(string); ok {
				requiredList = append(requiredList, reqStr)
			}
		}
	}

	// Determine which properties to show
	var keys []string
	if len(requiredList) > 0 {
		// Only show required properties by default to keep context window small
		keys = requiredList
		sort.Strings(keys)
		if len(keys) > 4 {
			keys = keys[:4]
		}
	} else {
		// No required parameters, show up to 4 optional properties
		for name := range properties {
			keys = append(keys, name)
		}
		sort.Strings(keys)
		if len(keys) > 4 {
			keys = keys[:4]
		}
	}

	var examples []string
	for _, name := range keys {
		prop, ok := properties[name].(map[string]interface{})
		if !ok {
			examples = append(examples, fmt.Sprintf(`"%s":"..."`, name))
			continue
		}

		propType, _ := prop["type"].(string)
		var valStr string
		switch propType {
		case "string":
			valStr = `"..."`
		case "number", "integer":
			valStr = "0"
		case "boolean":
			valStr = "true"
		case "array":
			valStr = "[]"
		case "object":
			valStr = "{}"
		default:
			valStr = `"..."`
		}
		examples = append(examples, fmt.Sprintf(`"%s":%s`, name, valStr))
	}

	if len(examples) == 0 {
		return "'{}'"
	}

	return fmt.Sprintf("'{%s}'", strings.Join(examples, ", "))
}

// toolMatches checks if a tool matches a search query (case-insensitive substring on name and description)
func toolMatches(tool mcp.Tool, query string) bool {
	q := strings.ToLower(query)
	if strings.Contains(strings.ToLower(tool.Name), q) {
		return true
	}
	if tool.Description != "" && strings.Contains(strings.ToLower(tool.Description), q) {
		return true
	}
	return false
}

// printToolsHuman prints tools in human-readable format.
// Default: terse 1-line-per-tool ("name: description").
// With verbose: expanded 4-line format (desc, params, call).
func printToolsHuman(tools []mcp.Tool, serverName string, isVerbose bool) {
	for _, tool := range tools {
		if isVerbose {
			fmt.Printf("  • %s\n", tool.Name)
			if tool.Description != "" {
				fmt.Printf("    desc: %s\n", tool.Description)
			}
			if tool.InputSchema != nil {
				names := extractParamNames(tool.InputSchema)
				if len(names) > 0 {
					fmt.Printf("    params: %s\n", strings.Join(names, ", "))
				}
			}
			exampleArgs := BuildExampleArgs(&tool)
			fmt.Printf("    call: %s\n\n", buildCallString(serverName, tool.Name, exampleArgs))
		} else {
			// Terse: single line per tool
			if tool.Description != "" {
				fmt.Printf("  %s: %s\n", tool.Name, tool.Description)
			} else {
				fmt.Printf("  %s\n", tool.Name)
			}
		}
	}
}

// buildCallString constructs the mcp-cli-ent call command string
func buildCallString(serverName, toolName, exampleArgs string) string {
	if exampleArgs == "" || exampleArgs == "'{}'" {
		return fmt.Sprintf("mcp-cli-ent call %s %s", serverName, toolName)
	}
	return fmt.Sprintf("mcp-cli-ent call %s %s %s", serverName, toolName, exampleArgs)
}

// extractParamNames extracts parameter names from a tool's input schema
func extractParamNames(schema map[string]interface{}) []string {
	if schema == nil {
		return nil
	}
	properties, ok := schema["properties"].(map[string]interface{})
	if !ok || len(properties) == 0 {
		return nil
	}
	names := make([]string, 0, len(properties))
	for name := range properties {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// JSONTool represents a tool in structured JSON output.
// Shared by list-tools (single server) and root command (all servers).
type JSONTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Params      []string               `json:"params,omitempty"`
	Call        string                 `json:"call"`
	Schema      map[string]interface{} `json:"schema,omitempty"`
}

// indexTool is a compact tool entry for the bare-invocation discovery index.
type indexTool struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// encodeErrorJSON writes a structured JSON error to stdout.
func encodeErrorJSON(code, format string, args ...interface{}) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(map[string]interface{}{
		"error":             true,
		"error_code":        code,
		"error_description": fmt.Sprintf(format, args...),
	})
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
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output (full schema in JSON, expanded details in --human)")
	rootCmd.PersistentFlags().IntVar(&timeout, "timeout", 30, "request timeout in seconds")
	rootCmd.PersistentFlags().BoolVar(&refreshCache, "refresh", false, "force refresh of tools cache (alias: --clear-cache)")
	rootCmd.PersistentFlags().BoolVar(&clearCache, "clear-cache", false, "clear tools cache (alias: --refresh)")
	rootCmd.PersistentFlags().BoolVar(&humanOutput, "human", false, "human-readable terminal output (default is JSON)")
	rootCmd.PersistentFlags().StringVar(&searchQuery, "search", "", "filter tools by name or description (case-insensitive)")

	// Bind flags to viper
	_ = viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose"))
	_ = viper.BindPFlag("timeout", rootCmd.PersistentFlags().Lookup("timeout"))
	_ = viper.BindPFlag("refresh", rootCmd.PersistentFlags().Lookup("refresh"))
	_ = viper.BindPFlag("clear-cache", rootCmd.PersistentFlags().Lookup("clear-cache"))
	_ = viper.BindPFlag("human", rootCmd.PersistentFlags().Lookup("human"))
	_ = viper.BindPFlag("search", rootCmd.PersistentFlags().Lookup("search"))
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
