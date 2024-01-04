package cmd

import (
	"fmt"
	cc "github.com/ivanpirog/coloredcobra"
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

const (
	envVarPrefix          = "KUBECOMPOSE_"
	fileFlagName          = "file"
	namespaceEnvVarName   = envVarPrefix + "NAMESPACE"
	namespaceFlagName     = "namespace"
	envIDEnvVarName       = envVarPrefix + "ENVID"
	envIDFlagName         = "env-id"
	envIdNoAppendFlagName = "env-id-no-append"
)

func Execute() error {
	log.SetOutput(os.Stdout)
	rootCmd := &cobra.Command{
		Use:               "kube-compose",
		Short:             "k8s",
		Long:              "Environments on k8s made easy",
		Version:           "0.6.3",
		PersistentPreRunE: setupLogging,
	}
	rootCmd.AddCommand(newDownCli(), newUpCli(), newGetCli())
	setRootCommandFlags(rootCmd)
	cc.Init(&cc.Config{
		RootCmd:  rootCmd,
		Headings: cc.HiCyan + cc.Bold + cc.Underline,
		Commands: cc.HiYellow + cc.Bold,
		Example:  cc.Italic,
		ExecName: cc.Bold,
		Flags:    cc.Bold,
	})
	return rootCmd.Execute()
}

func setRootCommandFlags(rootCmd *cobra.Command) {
	rootCmd.PersistentFlags().StringSliceP(fileFlagName, "f", []string{}, "Specify an alternate compose file")
	rootCmd.PersistentFlags().StringP(namespaceFlagName, "n", "", fmt.Sprintf("namespace for environment. "+
		"Defaults to the namespace of the current kube config context. (env %s)", namespaceEnvVarName))
	rootCmd.PersistentFlags().StringP(envIDFlagName, "e", "", "used to isolate environments deployed to a shared namespace, "+
		"by (1) using this value as a suffix of pod and service names and (2) using this value to isolate selectors. "+
		fmt.Sprintf("(env %s)", envIDEnvVarName))
	rootCmd.PersistentFlags().BoolP(envIdNoAppendFlagName, "E", false, "Do not append the '-{env-id}' to the k8s service/pod names (So DNS lookups can be done on the exact service names as listed in the docker-compose yaml)")
	rootCmd.PersistentFlags().StringP(logLevelFlagName, "l", "", fmt.Sprintf("Set to one of %s. "+
		"(env %s, default %s)", formattedLogLevelList, logLevelEnvVarName, logLevelDefault.String()))
}
