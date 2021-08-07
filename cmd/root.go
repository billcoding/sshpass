package cmd

import (
	"fmt"
	"github.com/billcoding/sshpass/app"
	"github.com/spf13/cobra"
	"os"
	"strings"
	"time"
)

var password string
var useEnv bool
var timeout time.Duration
var disableConfirmHostAuthenticity bool

var rootCmd = &cobra.Command{
	Use:   "sshpass",
	Short: "Enter passwords to commands non-interactively",
	Long: `sshpass is essentially a go implementation of sshpass (https://linux.die.net/man/1/sshpass).
Though unlike sshpass it doesn't restrict itself to SSH logins.
It can supply a password to any process with an identifiable password prompt.`,
	Run: func(cmd *cobra.Command, args []string) {

		command := strings.Join(args, " ")

		if command == "" {
			fmt.Println("You must specify a command.")
			os.Exit(1)
		}

		if useEnv {
			password = os.Getenv("SSHPASS")
		}
		options := app.DefaultOptions
		options.AutoConfirmHostAuthenticity = !disableConfirmHostAuthenticity
		if err := app.Run(command, []string{password}, options); err != nil {
			fmt.Println("Error: " + err.Error())
			os.Exit(1)
		}
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&password, "password", "", "plaintext password (not recommended)")
	rootCmd.PersistentFlags().BoolVar(&useEnv, "env", false, "use value of $SSHPASS environment variable as password")
	rootCmd.PersistentFlags().DurationVar(&timeout, "timeout", time.Second*10, "timeout length to wait for prompt/confirmation")
	rootCmd.PersistentFlags().BoolVar(&disableConfirmHostAuthenticity, "disable-ssh-host-confirm", false, "sshpass will automatically confirm the authenticity of SSH hosts unless this option is specified")
}
