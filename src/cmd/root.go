package terraformer

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var defaultOptions = []string{}
var insecure = false

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "terraformer",
	Short: "Terraform cli wrapper",
	Long: `This application provides functionality for running Terraform cli
consistently in a CI/CD environment`,
	PersistentPreRunE: preFlight,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func preFlight(cmd *cobra.Command, args []string) error {
	//setup logging
	if err := setUpLogs(viper.GetString("verbosity")); err != nil {
		log.Fatal(err)
	}

	//print debug env info
	log.Debug(fmt.Sprintf("Env Vars: \n\tglobal-options: %s\n\toptions: %s\n\tdirectory: %s\n\tworking-directory: %s\n\tverbosity: %s\n",
		viper.GetString("global-options"),
		viper.GetString("options"),
		viper.GetString("directory"),
		viper.GetString("working-directory"),
		viper.GetString("verbosity")))

	return nil
}

func init() {
	// automatic environment variables
	viper.AutomaticEnv()

	// cli flags
	rootCmd.PersistentFlags().StringP("global-options", "g", "", "Set terraform global options")
	rootCmd.PersistentFlags().StringP("options", "o", "", "Set terraform command options")
	rootCmd.PersistentFlags().StringP("directory", "d", "", "Set terraform command directory")
	rootCmd.PersistentFlags().StringP("data-directory", "w", ".terraform", "Set terraform command directory")
	rootCmd.PersistentFlags().StringP("vault-addr", "a", "", "Set the vault address")
	rootCmd.PersistentFlags().StringP("vault-token", "t", "", "Set the vault token")
	rootCmd.PersistentFlags().StringP("verbosity", "v", log.InfoLevel.String(), "Log level (debug, info, warn, error, fatal, panic")

	// bind flags
	viper.BindPFlag("global-options", rootCmd.PersistentFlags().Lookup("global-options"))
	viper.BindPFlag("options", rootCmd.PersistentFlags().Lookup("options"))
	viper.BindPFlag("directory", rootCmd.PersistentFlags().Lookup("directory"))
	viper.BindPFlag("data-directory", rootCmd.PersistentFlags().Lookup("data-directory"))
	viper.BindPFlag("vault-addr", rootCmd.PersistentFlags().Lookup("vault-addr"))
	viper.BindPFlag("vault-token", rootCmd.PersistentFlags().Lookup("vault-token"))
	viper.BindPFlag("verbosity", rootCmd.PersistentFlags().Lookup("verbosity"))

	// bind to missnamed environment variabels and alias
	viper.BindEnv("global-options", "GLOBAL_OPTIONS")
	viper.BindEnv("options", "OPTIONS")
	viper.BindEnv("directory", "DIRECTORY")
	viper.BindEnv("data-directory", "TF_DATA_DIR")
	viper.BindEnv("vault-addr", "VAULT_ADDR")
	viper.BindEnv("vault-token", "VAULT_TOKEN")

	//support .vault-token file
	if viper.GetString("vault-token") == "" {
		token, err := ioutil.ReadFile(os.ExpandEnv("${HOME}") + "/.vault-token")
		if err != nil && !os.IsNotExist(err) {
			log.Error(err)
		}
		viper.Set("vault-token", strings.TrimSpace(string(token)))
	}
}

func setUpLogs(level string) error {
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp:          true,
		DisableLevelTruncation: true,
	})
	log.SetOutput(os.Stdout)

	lvl, err := log.ParseLevel(level)
	if err != nil {
		return errors.Wrap(err, "parsing log level")
	}
	log.SetLevel(lvl)

	return nil
}
