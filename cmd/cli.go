package cmd

import (
	"fmt"
	"github.com/openshieldai/openshield/lib"
	"github.com/openshieldai/openshield/models"
	"github.com/openshieldai/openshield/server"
	"github.com/spf13/cobra"
	"gorm.io/gorm"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var rootCmd = &cobra.Command{
	Use:   "openshield",
	Short: "OpenShield CLI",
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.AddCommand(dbCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(startServerCmd)
	rootCmd.AddCommand(stopServerCmd)
	dbCmd.AddCommand(createTablesCmd)
	dbCmd.AddCommand(createMockDataCmd)
	configCmd.AddCommand(editConfigCmd)
	configCmd.AddCommand(addRuleCmd)
	configCmd.AddCommand(removeRuleCmd)
	configCmd.AddCommand(configWizardCmd)
}

var dbCmd = &cobra.Command{
	Use:   "db",
	Short: "Database related commands",
}

var createTablesCmd = &cobra.Command{
	Use:   "create-tables",
	Short: "Create database tables from models",
	Run: func(cmd *cobra.Command, args []string) {
		createTables()
	},
}

var createMockDataCmd = &cobra.Command{
	Use:   "create-mock-data",
	Short: "Create mock data in the database",
	Run: func(cmd *cobra.Command, args []string) {
		createMockData()
	},
}

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Configuration related commands",
}

var editConfigCmd = &cobra.Command{
	Use:   "edit",
	Short: "Edit the config.yaml file",
	Run: func(cmd *cobra.Command, args []string) {
		editConfig()
	},
}

var addRuleCmd = &cobra.Command{
	Use:   "add-rule",
	Short: "Add a new rule to the configuration",
	Run: func(cmd *cobra.Command, args []string) {
		addRule()
	},
}

var removeRuleCmd = &cobra.Command{
	Use:   "remove-rule",
	Short: "Remove a rule from the configuration",
	Run: func(cmd *cobra.Command, args []string) {
		removeRule()
	},
}

var configWizardCmd = &cobra.Command{
	Use:   "wizard",
	Short: "Interactive wizard to create or update config.yaml",
	Run: func(cmd *cobra.Command, args []string) {
		runConfigWizard()
	},
}

var startServerCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the server",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Starting the server...")
		if err := server.StartServer(); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
	},
}

var stopServerCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the server",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Sending stop signal to the server...")
		if err := stopServer(); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Stop signal sent to the server")
	},
}

func stopServer() error {
	// Create a channel to receive OS signals
	sigs := make(chan os.Signal, 1)

	// Register for SIGINT
	signal.Notify(sigs, syscall.SIGINT)

	// Send SIGINT to the current process
	p, err := os.FindProcess(os.Getpid())
	if err != nil {
		return fmt.Errorf("failed to find current process: %v", err)
	}

	err = p.Signal(syscall.SIGINT)
	if err != nil {
		return fmt.Errorf("failed to send interrupt signal: %v", err)
	}

	// Wait for a short time to allow the signal to be processed
	select {
	case <-sigs:
		fmt.Println("Interrupt signal received")
	case <-time.After(2 * time.Second):
		fmt.Println("No confirmation received, but signal was sent")
	}

	return nil
}
func createTables(db ...*gorm.DB) {
	database := lib.DB()
	if len(db) > 0 && db[0] != nil {
		database = db[0]
		err := database.AutoMigrate(
			&models.Tags{},
			&models.AiModels{},
			&models.ApiKeys{},
			&models.AuditLogs{},
			&models.Products{},
			&models.Usage{},
			&models.Workspaces{},
		)
		if err != nil {
			_ = fmt.Errorf("failed to migrate models: %v", err)
		}
	}
}
