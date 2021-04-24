package terraformer

import (
	"os"
	"os/exec"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// validateCmd represents the terraform validate command
var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validates terraform code",
	Long: `Runs terraform validate.`,
	Run: runValidate,
}

func init() {
	rootCmd.AddCommand(validateCmd)
}

func runValidate(*cobra.Command, []string) {
	args := []string{"-chdir="+viper.GetString("directory"), "validate"}
	if viper.GetString("global-options") != "" {
		args = append([]string{viper.GetString("global-options")}, args...)
	}
	if viper.GetString("options") != "" {
		args = append(args, viper.GetString("options"))
	}

	cmd := exec.Command("terraform", args...)
	cmd.Env = append(os.Environ(),
		"TF_DATA_DIR="+os.ExpandEnv("${PWD}")+"/"+viper.GetString("data-directory"))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	err := cmd.Run()
	if err != nil {
		log.Error(err)
	}
}
