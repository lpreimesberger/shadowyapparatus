package cmd

import (
	"fmt"
	"runtime"
	"time"

	"github.com/spf13/cobra"
)

// Version information - these will be set at build time via ldflags
var (
	Version = "dev" // Version string (e.g., "0.1")
	// Use BuildNumber from tracker_client.go if available
	GitCommit = "unknown"         // Git commit hash
	BuildTime = "unknown"         // Build timestamp
	GoVersion = runtime.Version() // Go version used to build
)

// For backward compatibility, alias BuildNum to BuildNumber
var BuildNum = BuildNumber

// VersionInfo contains detailed version information
type VersionInfo struct {
	Version      string `json:"version"`
	BuildNum     string `json:"build_number"`
	GitCommit    string `json:"git_commit"`
	BuildTime    string `json:"build_time"`
	GoVersion    string `json:"go_version"`
	Platform     string `json:"platform"`
	Architecture string `json:"architecture"`
}

// GetVersionInfo returns detailed version information
func GetVersionInfo() *VersionInfo {
	return &VersionInfo{
		Version:      Version,
		BuildNum:     BuildNumber,
		GitCommit:    GitCommit,
		BuildTime:    BuildTime,
		GoVersion:    GoVersion,
		Platform:     runtime.GOOS,
		Architecture: runtime.GOARCH,
	}
}

// GetVersionString returns a formatted version string
func GetVersionString() string {
	if GitCommit == "unknown" || BuildTime == "unknown" {
		return fmt.Sprintf("Shadowy Blockchain 1.0.%s+%s", Version, BuildNumber)
	}

	// Parse build time
	buildTime, err := time.Parse(time.RFC3339, BuildTime)
	if err != nil {
		buildTime = time.Now()
	}

	return fmt.Sprintf("Shadowy Blockchain %s.%s\nCommit: %s\nBuilt: %s\nGo: %s",
		Version,
		BuildNumber,
		GitCommit,
		buildTime.Format("2006-01-02 15:04:05 UTC"),
		GoVersion,
	)
}

// GetShortVersionString returns a compact version string for display
func GetShortVersionString() string {
	return fmt.Sprintf("v%s.%s", Version, BuildNumber)
}

// GetFullVersionString returns a detailed version string with build time
func GetFullVersionString() string {
	if BuildTime != "unknown" {
		if buildTime, err := time.Parse(time.RFC3339, BuildTime); err == nil {
			return fmt.Sprintf("v%s.%s (%s)", Version, BuildNumber, buildTime.Format("2006-01-02 15:04"))
		}
	}
	return fmt.Sprintf("v%s.%s", Version, BuildNumber)
}

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version information",
	Long: `Display detailed version information for the Shadowy blockchain.

This includes the version number, git commit, build time, and runtime information.`,
	Run: func(cmd *cobra.Command, args []string) {
		verbose, _ := cmd.Flags().GetBool("verbose")
		json, _ := cmd.Flags().GetBool("json")

		if json {
			// Output JSON format
			info := GetVersionInfo()
			fmt.Printf("{\n")
			fmt.Printf("  \"version\": \"%s\",\n", info.Version)
			fmt.Printf("  \"build_number\": \"%s\",\n", info.BuildNum)
			fmt.Printf("  \"git_commit\": \"%s\",\n", info.GitCommit)
			fmt.Printf("  \"build_time\": \"%s\",\n", info.BuildTime)
			fmt.Printf("  \"go_version\": \"%s\",\n", info.GoVersion)
			fmt.Printf("  \"platform\": \"%s\",\n", info.Platform)
			fmt.Printf("  \"architecture\": \"%s\"\n", info.Architecture)
			fmt.Printf("}\n")
		} else if verbose {
			// Verbose output
			info := GetVersionInfo()
			fmt.Printf("Shadowy Blockchain Version Information\n")
			fmt.Printf("=====================================\n")
			fmt.Printf("Version:      %s\n", info.Version)
			fmt.Printf("Build Number: %s\n", info.BuildNum)
			fmt.Printf("Git Commit:   %s\n", info.GitCommit)
			fmt.Printf("Build Time:   %s\n", info.BuildTime)
			fmt.Printf("Go Version:   %s\n", info.GoVersion)
			fmt.Printf("Platform:     %s\n", info.Platform)
			fmt.Printf("Architecture: %s\n", info.Architecture)
			fmt.Printf("\nFeatures:\n")
			fmt.Printf("• Proof-of-Storage Consensus\n")
			fmt.Printf("• Bitcoin-style Deflationary Tokenomics\n")
			fmt.Printf("• Multi-node P2P Network\n")
			fmt.Printf("• RESTful API\n")
			fmt.Printf("• HD Wallet Support\n")
			fmt.Printf("• Plot-based Mining\n")
		} else {
			// Simple output
			fmt.Println(GetVersionString())
		}
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)

	versionCmd.Flags().BoolP("verbose", "v", false, "Show detailed version information")
	versionCmd.Flags().BoolP("json", "j", false, "Output version information in JSON format")
}
