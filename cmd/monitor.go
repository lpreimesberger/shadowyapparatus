package cmd

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
)

var (
	monitorPort    int
	monitorAPIURL  string
	monitorRefresh int
)

// monitorCmd represents the monitor command
var monitorCmd = &cobra.Command{
	Use:   "monitor",
	Short: "Start web monitoring dashboard for blockchain health",
	Long: `Start a web-based monitoring dashboard that provides real-time insights into:

• Node health and system metrics
• Blockchain status and recent blocks  
• Mining performance and statistics
• Consensus status and peer connections
• Mempool and transaction activity
• Real-time charts and alerts

The dashboard connects to the blockchain node's HTTP API to collect data
and presents it in an easy-to-use web interface perfect for monitoring
during development, testing, and burn-in scenarios.

Examples:
  # Start monitor on default port 9999
  shadowy monitor

  # Start monitor on custom port
  shadowy monitor --port 8888

  # Monitor remote node
  shadowy monitor --api-url http://remote-node:8080

  # Custom refresh rate (default 5 seconds)
  shadowy monitor --refresh 10`,
	Run: runMonitor,
}

func init() {
	rootCmd.AddCommand(monitorCmd)

	monitorCmd.Flags().IntVarP(&monitorPort, "port", "p", 9999, "Port for web monitoring dashboard")
	monitorCmd.Flags().StringVar(&monitorAPIURL, "api-url", "http://localhost:8080", "Base URL for blockchain node API")
	monitorCmd.Flags().IntVar(&monitorRefresh, "refresh", 5, "Dashboard refresh rate in seconds")
}

func runMonitor(cmd *cobra.Command, args []string) {
	log.Printf("🌑 Starting Shadowy Blockchain Web Monitor")
	log.Printf("Dashboard Port: %d", monitorPort)
	log.Printf("API URL: %s", monitorAPIURL)
	log.Printf("Refresh Rate: %d seconds", monitorRefresh)

	// Create web monitor instance
	webMonitor := NewWebMonitor(monitorPort, monitorAPIURL)
	webMonitor.refreshRate = monitorRefresh

	// Set up graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start monitor in goroutine
	go func() {
		if err := webMonitor.Start(); err != nil {
			log.Fatalf("Failed to start web monitor: %v", err)
		}
	}()

	log.Printf("✅ Web monitoring dashboard started successfully")
	log.Printf("🌐 Open your browser to: http://localhost:%d", monitorPort)
	log.Printf("Press Ctrl+C to stop the monitor")

	// Wait for shutdown signal
	<-sigChan
	log.Println("🛑 Shutting down web monitor...")

	if err := webMonitor.Stop(); err != nil {
		log.Printf("Error stopping web monitor: %v", err)
	}

	log.Println("✅ Web monitor stopped gracefully")
}