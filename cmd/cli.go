package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/openshieldai/openshield/lib"
	"github.com/openshieldai/openshield/server"
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
	rootCmd.AddCommand(startServerCmd)
	rootCmd.AddCommand(stopServerCmd)
	dbCmd.AddCommand(createTablesCmd)
	dbCmd.AddCommand(createMockDataCmd)
	configCmd.AddCommand(editConfigCmd)
	configCmd.AddCommand(addRuleCmd)
	configCmd.AddCommand(removeRuleCmd)
	configCmd.AddCommand(configWizardCmd)
	uploadFileCmd.Flags().BoolP("detectsensitive", "s", false, "Enable sensitive data detection")
	rootCmd.AddCommand(uploadFileCmd)

}

var uploadFileCmd = &cobra.Command{
	Use:   "upload-file [filepath]",
	Short: "Upload a file to the Python endpoint",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		filePath := args[0]
		detectSensitive, _ := cmd.Flags().GetBool("detectsensitive")
		err := uploadFile(filePath, detectSensitive)
		if err != nil {
			fmt.Printf("Error uploading file: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("File uploaded successfully")
	},
}

var dbCmd = &cobra.Command{
	Use:   "db",
	Short: "Database related commands",
}

var createTablesCmd = &cobra.Command{
	Use:   "create-tables",
	Short: "Create database tables from models",
	Run: func(cmd *cobra.Command, args []string) {
		lib.DB()
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
func uploadFile(filePath string, detectSensitive bool) error {
	config := lib.GetConfig()

	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %v", err)
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return fmt.Errorf("failed to create form file: %v", err)
	}
	_, err = io.Copy(part, file)
	if err != nil {
		return fmt.Errorf("failed to copy file content: %v", err)
	}

	// Add API settings to the request
	apiSettings := map[string]interface{}{
		"database_url": config.Settings.RagServer.Database.URI,
	}

	if !detectSensitive {
		apiSettings["chunk_size"] = config.Settings.RagServer.Chunking.Size
		apiSettings["chunk_overlap"] = config.Settings.RagServer.Chunking.Overlap
	}

	apiSettingsJson, err := json.Marshal(apiSettings)
	if err != nil {
		return fmt.Errorf("failed to marshal API settings: %v", err)
	}

	err = writer.WriteField("api_settings", string(apiSettingsJson))
	if err != nil {
		return fmt.Errorf("failed to write API settings field: %v", err)
	}

	err = writer.Close()
	if err != nil {
		return fmt.Errorf("failed to close multipart writer: %v", err)
	}

	endpoint := "/upload"
	if detectSensitive {
		endpoint = "/upload_sensitive_detection"
	}

	req, err := http.NewRequest("POST", config.Settings.RagServer.Url+endpoint, body)
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d, response: %s", resp.StatusCode, string(respBody))
	}

	fmt.Printf("Response from server: %s\n", respBody)
	return nil
}
