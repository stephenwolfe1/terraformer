package terraformer

import (
	"os"
	"os/exec"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// initCmd represents the terraform init command
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Inits terraform code",
	Long: `Runs terraform init.`,
	Run: runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
}

func runInit(*cobra.Command, []string) {
	args := append([]string{"-chdir="+viper.GetString("directory"), "init", "-upgrade", "-get=true", "-plugin-dir=/providers/plugins", "-reconfigure"}, defaultOptions...)

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
