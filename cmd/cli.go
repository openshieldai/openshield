package cmd

import (
	"github.com/spf13/cobra"
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
	dbCmd.AddCommand(createTablesCmd)
	dbCmd.AddCommand(createMockDataCmd)
	configCmd.AddCommand(editConfigCmd)
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

var configWizardCmd = &cobra.Command{
	Use:   "wizard",
	Short: "Interactive wizard to create or update config.yaml",
	Run: func(cmd *cobra.Command, args []string) {
		runConfigWizard()
	},
}
