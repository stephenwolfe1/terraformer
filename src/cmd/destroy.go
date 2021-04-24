package terraformer

import (
	"os"
	"os/exec"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// destroyCmd represents the terraform destroy command
var destroyCmd = &cobra.Command{
	Use:   "destroy",
	Short: "destroys terraform code",
	Long: `Runs terraform destroy.`,
	Run: rundestroy,
}

func init() {
	rootCmd.AddCommand(destroyCmd)
}

func rundestroy(*cobra.Command, []string) {
	args := append([]string{"-chdir="+viper.GetString("directory"), "destroy", "-lock=true", "-lock-timeout=0s", "-refresh=true"}, defaultOptions...)
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
