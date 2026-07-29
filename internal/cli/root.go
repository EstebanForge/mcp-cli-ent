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

	"github.com/mcp-cli-ent/mcp-cli/internal/client"
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

// ToolsCacheEntry represents a cached tool listing for a server.
// Failed entries act as a short-lived negative cache so a broken server
// does not force a full re-discovery on every invocation.
type ToolsCacheEntry struct {
	Tools      []mcp.Tool `json:"tools"`
	LastUpdate time.Time  `json:"lastUpdate"`
	Failed     bool       `json:"failed,omitempty"`
}

// ToolsCache represents the full cache structure
type ToolsCache struct {
	Servers map[string]ToolsCacheEntry `json:"servers"`
}

const (
	CacheFileName    = "tools_cache.json"
	CacheTTL         = 30 * 24 * time.Hour // Successful entries expire after 30 days
	NegativeCacheTTL = 5 * time.Minute     // Failed entries: retry the server after 5 min
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

// toolsIndex is the JSON output envelope for the tools index. meta carries
// per-server fetch errors so agents reading only stdout see why a server is
// missing; servers is the server->tools map.
type toolsIndex struct {
	Meta    *toolsIndexMeta        `json:"meta,omitempty"`
	Servers map[string][]indexTool `json:"servers"`
}

// toolsIndexMeta holds envelope metadata. errors maps a server name to the
// reason its tools could not be fetched (skip or failure).
type toolsIndexMeta struct {
	Errors map[string]string `json:"errors,omitempty"`
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

	// --refresh / --clear-cache are aliases. They force live discovery for
	// enabled servers, whose fresh results overwrite only their own cache
	// keys. The file itself is never deleted, so dormant/disabled servers
	// keep their cached entries (true merge semantics).
	if clearCache || refreshCache {
		refreshCache = true
		clearCache = true
	}

	// Load existing cache. We MERGE into it rather than overwrite, so a
	// transient failure on one server never evicts good entries for others.
	cache, err := LoadToolsFromCache()
	if err != nil || cache == nil {
		cache = &ToolsCache{Servers: make(map[string]ToolsCacheEntry)}
	}
	if cache.Servers == nil {
		cache.Servers = make(map[string]ToolsCacheEntry)
	}

	toolsByServer := make(map[string][]mcp.Tool)
	errorsByServer := make(map[string]string) // per-server fetch errors, surfaced in JSON meta
	var toDiscover []string
	now := time.Now()

	// Partition enabled servers: serve valid cache hits immediately, queue
	// the rest (missing, expired, or failed-past-TTL) for live discovery.
	for serverName := range enabledServers {
		entry, cached := cache.Servers[serverName]
		if cached && !refreshCache && !clearCache {
			ttl := CacheTTL
			if entry.Failed {
				ttl = NegativeCacheTTL
			}
			if now.Sub(entry.LastUpdate) <= ttl {
				// Still valid. Negative entries yield no tools but are
				// surfaced so a failing server does not vanish silently.
				if entry.Failed {
					retry := (ttl - now.Sub(entry.LastUpdate)).Round(time.Second)
					msg := fmt.Sprintf("skipped: recently failed; retry in %s or use --refresh", retry)
					if humanOutput || verbose {
						fmt.Fprintf(os.Stderr, "%s: (%s)\n", serverName, msg)
					}
					errorsByServer[serverName] = msg
				} else {
					toolsByServer[serverName] = entry.Tools
				}
				continue
			}
		}
		toDiscover = append(toDiscover, serverName)
	}

	// Discover only what we could not serve from cache. Each worker gets its
	// own deadline so one hanging server cannot stall the whole invocation.
	if len(toDiscover) > 0 {
		factory, err := getSessionAwareClientFactory()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to create client factory: %v\n", err)
			fmt.Fprintln(os.Stderr, "Run 'mcp-cli-ent list-servers' to see available servers")
			return nil
		}

		var wg sync.WaitGroup
		var mu sync.Mutex
		for _, name := range toDiscover {
			wg.Add(1)
			go func(name string) {
				defer wg.Done()
				serverConfig := enabledServers[name]

				// Honor a server-specific timeout if set, else the global flag.
				serverTimeout := timeout
				if serverConfig.Timeout > 0 {
					serverTimeout = serverConfig.Timeout
				}
				ctx, cancel := context.WithTimeout(context.Background(), time.Duration(serverTimeout)*time.Second)
				defer cancel()

				mcpClient, err := factory.CreateClient(name, serverConfig)
				if err != nil {
					msg := fmt.Sprintf("failed to connect: %v", err)
					if humanOutput || verbose {
						fmt.Fprintf(os.Stderr, "%s: (%s)\n", name, msg)
					}
					mu.Lock()
					errorsByServer[name] = msg
					cache.Servers[name] = ToolsCacheEntry{Failed: true, LastUpdate: time.Now()}
					mu.Unlock()
					return
				}

				tools, err := mcpClient.ListTools(ctx)
				_ = mcpClient.Close()
				if err != nil {
					msg := fmt.Sprintf("failed to list tools: %v", err)
					if humanOutput || verbose {
						fmt.Fprintf(os.Stderr, "%s: (%s)\n", name, msg)
					}
					mu.Lock()
					errorsByServer[name] = msg
					cache.Servers[name] = ToolsCacheEntry{Failed: true, LastUpdate: time.Now()}
					mu.Unlock()
					return
				}

				mu.Lock()
				toolsByServer[name] = tools
				cache.Servers[name] = ToolsCacheEntry{Tools: tools, LastUpdate: time.Now()}
				mu.Unlock()
			}(name)
		}
		wg.Wait()

		// Persist the merged cache: fresh successes + retained hits + new
		// negative entries. A sibling failure never drops a good entry.
		if err := SaveToolsToCache(cache); err != nil && verbose {
			fmt.Fprintf(os.Stderr, "Warning: failed to save tools cache: %v\n", err)
		}
	}

	totalTools := 0
	for _, tools := range toolsByServer {
		totalTools += len(tools)
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
			return nil
		}
		// JSON mode: emit the bare error envelope only when there are no fetch
		// errors to report. With per-server errors, fall through to render the
		// meta envelope (servers may be empty) so consumers see why each failed.
		if len(errorsByServer) == 0 {
			if searchQuery != "" {
				return encodeErrorJSON("no_match", "No tools matching '%s' found", searchQuery)
			}
			return encodeErrorJSON("no_tools", "No tools found on any server")
		}
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

	// JSON output (default): compact index under a meta envelope. Per-server
	// fetch errors are carried in meta.errors so agents reading only stdout see
	// why a server is missing, not just its absence.
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

	index := toolsIndex{Servers: result}
	if len(errorsByServer) > 0 {
		index.Meta = &toolsIndexMeta{Errors: errorsByServer}
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(index)
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
	// Propagate the --verbose flag to the client package so diagnostic logs
	// (e.g. era-detection fallback) stay silent in the default machine/JSON path.
	cobra.OnInitialize(func() { client.Verbose = verbose })

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
