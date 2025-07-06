package cmd

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const (
	// Configuration file constants
	ConfigFileName = "config.json"
)

type ShadowConfig struct {
	PlotDirectories    []string    `json:"plot_directories"`
	DirectoryServices  []string    `json:"directory_services"`
	ListenOn          string      `json:"listen_on"`
	MaxPeers          int         `json:"max_peers"`
	LogLevel          string      `json:"log_level"`
	LoggingDirectory  string      `json:"logging_directory"`
	ScratchDirectory  string      `json:"scratch_directory"`
	BlockchainDirectory string     `json:"blockchain_directory"`
	TimelordConfig    interface{} `json:"timelord_config,omitempty"`
	Version           int         `json:"version"`
	CreatedAt         string      `json:"created_at"`
	UpdatedAt         string      `json:"updated_at"`
}

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Configuration management for Shadowy",
	Long: `Manage Shadowy configuration including plot directories and settings.
Configuration is stored in JSON format in the same directory as wallets.`,
}

var addPlotDirCmd = &cobra.Command{
	Use:   "addplotdir [directory]",
	Short: "Add a directory to the plot search path",
	Long: `Add a directory to the list of directories that will be searched for plot files.
The directory must exist and be readable.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		directory := args[0]
		
		// Convert to absolute path
		absDir, err := filepath.Abs(directory)
		if err != nil {
			fmt.Printf("Error resolving directory path: %v\n", err)
			os.Exit(1)
		}
		
		// Check if directory exists
		if _, err := os.Stat(absDir); os.IsNotExist(err) {
			fmt.Printf("Error: Directory '%s' does not exist\n", absDir)
			os.Exit(1)
		}
		
		config, err := loadConfig()
		if err != nil {
			fmt.Printf("Error loading config: %v\n", err)
			os.Exit(1)
		}
		
		// Check if directory already exists in config
		if slices.Contains(config.PlotDirectories, absDir) {
			fmt.Printf("Directory '%s' is already in the plot search path\n", absDir)
			return
		}
		
		// Add directory to config
		config.PlotDirectories = append(config.PlotDirectories, absDir)
		config.UpdatedAt = getCurrentTimestamp()
		
		if err := saveConfig(config); err != nil {
			fmt.Printf("Error saving config: %v\n", err)
			os.Exit(1)
		}
		
		fmt.Printf("Added plot directory: %s\n", absDir)
		fmt.Printf("Total plot directories: %d\n", len(config.PlotDirectories))
	},
}

var rmPlotDirCmd = &cobra.Command{
	Use:   "rmplotdir [directory]",
	Short: "Remove a directory from the plot search path",
	Long: `Remove a directory from the list of directories searched for plot files.
The directory path must match exactly (use absolute paths for consistency).`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		directory := args[0]
		
		// Convert to absolute path for consistent matching
		absDir, err := filepath.Abs(directory)
		if err != nil {
			fmt.Printf("Error resolving directory path: %v\n", err)
			os.Exit(1)
		}
		
		config, err := loadConfig()
		if err != nil {
			fmt.Printf("Error loading config: %v\n", err)
			os.Exit(1)
		}
		
		// Find and remove directory from config
		initialLen := len(config.PlotDirectories)
		config.PlotDirectories = slices.DeleteFunc(config.PlotDirectories, func(dir string) bool {
			return dir == absDir
		})
		
		if len(config.PlotDirectories) == initialLen {
			fmt.Printf("Directory '%s' not found in plot search path\n", absDir)
			fmt.Printf("Current directories:\n")
			for i, dir := range config.PlotDirectories {
				fmt.Printf("  %d. %s\n", i+1, dir)
			}
			return
		}
		
		config.UpdatedAt = getCurrentTimestamp()
		
		if err := saveConfig(config); err != nil {
			fmt.Printf("Error saving config: %v\n", err)
			os.Exit(1)
		}
		
		fmt.Printf("Removed plot directory: %s\n", absDir)
		fmt.Printf("Remaining plot directories: %d\n", len(config.PlotDirectories))
	},
}

var setCmd = &cobra.Command{
	Use:   "set [key] [value]",
	Short: "Set a configuration value",
	Long: `Set configuration values for P2P networking and other settings.
Available keys:
  - listen_on: Network address to listen on (e.g., "0.0.0.0:8080")
  - max_peers: Maximum number of peer connections (integer)
  - log_level: Logging level (debug, info, warn, error)
  - logging_directory: Directory for log files
  - scratch_directory: Directory for temporary files
  - blockchain_directory: Directory for blockchain data storage`,
	Args: cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		key := args[0]
		value := args[1]
		
		config, err := loadConfig()
		if err != nil {
			fmt.Printf("Error loading config: %v\n", err)
			os.Exit(1)
		}
		
		if err := setConfigValue(config, key, value); err != nil {
			fmt.Printf("Error setting config: %v\n", err)
			os.Exit(1)
		}
		
		config.UpdatedAt = getCurrentTimestamp()
		
		if err := saveConfig(config); err != nil {
			fmt.Printf("Error saving config: %v\n", err)
			os.Exit(1)
		}
		
		fmt.Printf("Set %s = %s\n", key, value)
	},
}

var getCmd = &cobra.Command{
	Use:   "get [key]",
	Short: "Get a configuration value",
	Long: `Get configuration values for P2P networking and other settings.
Available keys:
  - listen_on: Network address to listen on
  - max_peers: Maximum number of peer connections
  - log_level: Logging level
  - logging_directory: Directory for log files
  - scratch_directory: Directory for temporary files
  - blockchain_directory: Directory for blockchain data storage
  - directory_services: List of directory service endpoints`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		key := args[0]
		
		config, err := loadConfig()
		if err != nil {
			fmt.Printf("Error loading config: %v\n", err)
			os.Exit(1)
		}
		
		value, err := getConfigValue(config, key)
		if err != nil {
			fmt.Printf("Error getting config: %v\n", err)
			os.Exit(1)
		}
		
		fmt.Printf("%s = %s\n", key, value)
	},
}

var listConfigCmd = &cobra.Command{
	Use:   "list",
	Short: "List all configuration settings",
	Long:  `Display all current configuration settings including P2P and plot directory settings.`,
	Run: func(cmd *cobra.Command, args []string) {
		config, err := loadConfig()
		if err != nil {
			fmt.Printf("Error loading config: %v\n", err)
			os.Exit(1)
		}
		
		fmt.Printf("Shadowy Configuration:\n\n")
		fmt.Printf("P2P Settings:\n")
		fmt.Printf("  listen_on:          %s\n", config.ListenOn)
		fmt.Printf("  max_peers:          %d\n", config.MaxPeers)
		fmt.Printf("  log_level:          %s\n", config.LogLevel)
		fmt.Printf("  directory_services: [%s]\n", strings.Join(config.DirectoryServices, ", "))
		
		fmt.Printf("\nDirectories:\n")
		fmt.Printf("  logging_directory:    %s\n", config.LoggingDirectory)
		fmt.Printf("  scratch_directory:    %s\n", config.ScratchDirectory)
		fmt.Printf("  blockchain_directory: %s\n", config.BlockchainDirectory)
		fmt.Printf("  plot_directories:     %d directories\n", len(config.PlotDirectories))
		for i, dir := range config.PlotDirectories {
			fmt.Printf("    %d. %s\n", i+1, dir)
		}
		
		fmt.Printf("\nMetadata:\n")
		fmt.Printf("  version:            %d\n", config.Version)
		fmt.Printf("  created_at:         %s\n", config.CreatedAt)
		fmt.Printf("  updated_at:         %s\n", config.UpdatedAt)
	},
}

var addDirServiceCmd = &cobra.Command{
	Use:   "adddirservice [url]",
	Short: "Add a directory service endpoint",
	Long: `Add a directory service endpoint to the list of services used for peer discovery.
The URL should be a valid HTTP/HTTPS endpoint.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		url := args[0]
		
		if err := validateURL(url); err != nil {
			fmt.Printf("Error: Invalid URL '%s': %v\n", url, err)
			os.Exit(1)
		}
		
		config, err := loadConfig()
		if err != nil {
			fmt.Printf("Error loading config: %v\n", err)
			os.Exit(1)
		}
		
		if slices.Contains(config.DirectoryServices, url) {
			fmt.Printf("Directory service '%s' already exists\n", url)
			return
		}
		
		config.DirectoryServices = append(config.DirectoryServices, url)
		config.UpdatedAt = getCurrentTimestamp()
		
		if err := saveConfig(config); err != nil {
			fmt.Printf("Error saving config: %v\n", err)
			os.Exit(1)
		}
		
		fmt.Printf("Added directory service: %s\n", url)
		fmt.Printf("Total directory services: %d\n", len(config.DirectoryServices))
	},
}

var rmDirServiceCmd = &cobra.Command{
	Use:   "rmdirservice [url]",
	Short: "Remove a directory service endpoint",
	Long: `Remove a directory service endpoint from the list of services used for peer discovery.
The URL must match exactly.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		url := args[0]
		
		config, err := loadConfig()
		if err != nil {
			fmt.Printf("Error loading config: %v\n", err)
			os.Exit(1)
		}
		
		initialLen := len(config.DirectoryServices)
		config.DirectoryServices = slices.DeleteFunc(config.DirectoryServices, func(service string) bool {
			return service == url
		})
		
		if len(config.DirectoryServices) == initialLen {
			fmt.Printf("Directory service '%s' not found\n", url)
			fmt.Printf("Current services:\n")
			for i, service := range config.DirectoryServices {
				fmt.Printf("  %d. %s\n", i+1, service)
			}
			return
		}
		
		config.UpdatedAt = getCurrentTimestamp()
		
		if err := saveConfig(config); err != nil {
			fmt.Printf("Error saving config: %v\n", err)
			os.Exit(1)
		}
		
		fmt.Printf("Removed directory service: %s\n", url)
		fmt.Printf("Remaining directory services: %d\n", len(config.DirectoryServices))
	},
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(addPlotDirCmd)
	configCmd.AddCommand(rmPlotDirCmd)
	configCmd.AddCommand(setCmd)
	configCmd.AddCommand(getCmd)
	configCmd.AddCommand(listConfigCmd)
	configCmd.AddCommand(addDirServiceCmd)
	configCmd.AddCommand(rmDirServiceCmd)
	
	// Add wallet-dir flag to all config commands
	configCmd.PersistentFlags().StringVar(&walletDir, "wallet-dir", "", 
		"Directory for config and wallet files (default: $HOME/.shadowy)")
}

// Configuration file management functions

func getConfigPath() string {
	return filepath.Join(getWalletDir(), ConfigFileName)
}

func loadConfig() (*ShadowConfig, error) {
	configPath := getConfigPath()
	
	// If config doesn't exist, create default
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		defaultConfig := &ShadowConfig{
			PlotDirectories:     []string{},
			DirectoryServices:   []string{},
			ListenOn:           "0.0.0.0:8080",
			MaxPeers:           50,
			LogLevel:           "info",
			LoggingDirectory:   filepath.Join(getWalletDir(), "logs"),
			ScratchDirectory:   filepath.Join(getWalletDir(), "scratch"),
			BlockchainDirectory: filepath.Join(getWalletDir(), "blockchain"),
			Version:            1,
			CreatedAt:          getCurrentTimestamp(),
			UpdatedAt:          getCurrentTimestamp(),
		}
		
		// Ensure config directory exists
		if err := ensureWalletDir(); err != nil {
			return nil, fmt.Errorf("failed to create config directory: %w", err)
		}
		
		// Save default config
		if err := saveConfig(defaultConfig); err != nil {
			return nil, fmt.Errorf("failed to create default config: %w", err)
		}
		
		return defaultConfig, nil
	}
	
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}
	
	var config ShadowConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}
	
	// Handle compatibility for configs without blockchain_directory
	if config.BlockchainDirectory == "" {
		config.BlockchainDirectory = filepath.Join(getWalletDir(), "blockchain")
		// Auto-save the updated config to include the new field
		config.UpdatedAt = getCurrentTimestamp()
		if err := saveConfig(&config); err != nil {
			// Non-fatal - continue with the default value
			fmt.Printf("Warning: could not update config with blockchain directory: %v\n", err)
		}
	}
	
	return &config, nil
}

func saveConfig(config *ShadowConfig) error {
	if err := ensureWalletDir(); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}
	
	configPath := getConfigPath()
	
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	
	if err := os.WriteFile(configPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}
	
	return nil
}

func getCurrentTimestamp() string {
	return time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
}

// Configuration value management functions

func setConfigValue(config *ShadowConfig, key, value string) error {
	switch key {
	case "listen_on":
		if err := validateListenAddress(value); err != nil {
			return fmt.Errorf("invalid listen address: %w", err)
		}
		config.ListenOn = value
	case "max_peers":
		peers, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("max_peers must be an integer: %w", err)
		}
		if peers < 1 || peers > 10000 {
			return fmt.Errorf("max_peers must be between 1 and 10000")
		}
		config.MaxPeers = peers
	case "log_level":
		if err := validateLogLevel(value); err != nil {
			return fmt.Errorf("invalid log level: %w", err)
		}
		config.LogLevel = value
	case "logging_directory":
		if err := validateDirectoryPath(value); err != nil {
			return fmt.Errorf("invalid logging directory: %w", err)
		}
		config.LoggingDirectory = value
	case "scratch_directory":
		if err := validateDirectoryPath(value); err != nil {
			return fmt.Errorf("invalid scratch directory: %w", err)
		}
		config.ScratchDirectory = value
	case "blockchain_directory":
		if err := validateDirectoryPath(value); err != nil {
			return fmt.Errorf("invalid blockchain directory: %w", err)
		}
		config.BlockchainDirectory = value
	default:
		return fmt.Errorf("unknown configuration key '%s'", key)
	}
	return nil
}

func getConfigValue(config *ShadowConfig, key string) (string, error) {
	switch key {
	case "listen_on":
		return config.ListenOn, nil
	case "max_peers":
		return strconv.Itoa(config.MaxPeers), nil
	case "log_level":
		return config.LogLevel, nil
	case "logging_directory":
		return config.LoggingDirectory, nil
	case "scratch_directory":
		return config.ScratchDirectory, nil
	case "blockchain_directory":
		return config.BlockchainDirectory, nil
	case "directory_services":
		return strings.Join(config.DirectoryServices, ", "), nil
	default:
		return "", fmt.Errorf("unknown configuration key '%s'", key)
	}
}

// Validation functions

func validateListenAddress(addr string) error {
	if addr == "" {
		return fmt.Errorf("address cannot be empty")
	}
	
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("invalid address format: %w", err)
	}
	
	// Validate port
	portNum, err := strconv.Atoi(port)
	if err != nil {
		return fmt.Errorf("invalid port: %w", err)
	}
	if portNum < 1 || portNum > 65535 {
		return fmt.Errorf("port must be between 1 and 65535")
	}
	
	// Validate host (basic check)
	if host != "0.0.0.0" && host != "127.0.0.1" && host != "localhost" {
		if net.ParseIP(host) == nil {
			return fmt.Errorf("invalid host address")
		}
	}
	
	return nil
}

func validateLogLevel(level string) error {
	validLevels := []string{"debug", "info", "warn", "error"}
	for _, valid := range validLevels {
		if level == valid {
			return nil
		}
	}
	return fmt.Errorf("log level must be one of: %s", strings.Join(validLevels, ", "))
}

func validateURL(url string) error {
	if url == "" {
		return fmt.Errorf("URL cannot be empty")
	}
	
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return fmt.Errorf("URL must start with http:// or https://")
	}
	
	return nil
}

func validateDirectoryPath(dir string) error {
	if dir == "" {
		return fmt.Errorf("directory path cannot be empty")
	}
	
	// Convert to absolute path for consistent handling
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("invalid directory path: %w", err)
	}
	
	// Check if parent directory exists (directory itself may not exist yet)
	parentDir := filepath.Dir(absDir)
	if parentDir != absDir { // Don't check root directory
		if _, err := os.Stat(parentDir); os.IsNotExist(err) {
			return fmt.Errorf("parent directory '%s' does not exist", parentDir)
		}
	}
	
	return nil
}