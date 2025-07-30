package cmd

import (
	"log/slog"
	"os"

	"github.com/nsxbet/sql-reviewer-cli/pkg/logger"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string

// initializeLogger sets up the global slog logger based on viper configuration
func initializeLogger() {
	logLevel := slog.LevelInfo
	if viper.GetBool("debug") {
		logLevel = slog.LevelDebug
	} else if viper.GetBool("verbose") {
		logLevel = slog.LevelInfo
	}
	
	customLogger := logger.NewWithLevel(logLevel)
	slog.SetDefault(customLogger.GetSlogLogger())
}

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "sql-reviewer",
	Short: "A SQL review tool for database migrations",
	Long: `SQL Reviewer is a command-line tool that analyzes SQL statements 
against configurable rules to ensure code quality and consistency.

It supports multiple database engines including MySQL, PostgreSQL, 
Oracle, SQL Server, and Snowflake.`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.sql-reviewer.yaml)")
	rootCmd.PersistentFlags().Bool("verbose", false, "enable verbose output")
	rootCmd.PersistentFlags().Bool("debug", false, "enable debug output")

	// Bind flags to viper
	_ = viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose"))
	_ = viper.BindPFlag("debug", rootCmd.PersistentFlags().Lookup("debug"))
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	// Initialize logger early based on flags
	initializeLogger()
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		// Search config in home directory with name ".sql-reviewer" (without extension).
		viper.AddConfigPath(home)
		viper.AddConfigPath(".")
		viper.SetConfigType("yaml")
		viper.SetConfigName(".sql-reviewer")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err != nil {
		// Don't fail if config file is not found or has issues
		// Only show debug info if debug flag is enabled
		if viper.GetBool("debug") {
			slog.Debug("Config file error (ignoring)", "error", err)
			slog.Debug("Config file used", "file", viper.ConfigFileUsed())
		}
	} else {
		if viper.GetBool("debug") {
			slog.Debug("Using config file", "file", viper.ConfigFileUsed())
		}
	}
}
