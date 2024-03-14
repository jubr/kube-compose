package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/kube-compose/kube-compose/internal/app/up"
	"github.com/kube-compose/kube-compose/internal/pkg/progress/reporter"
	"github.com/kube-compose/kube-compose/internal/pkg/util"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

const (
	registryUserEnvVarName = envVarPrefix + "REGISTRY_USER"

	registryPassEnvVarName = envVarPrefix + "REGISTRY_PASS"
)

var registryUserFromEnv = util.Ternary(os.Getenv(registryUserEnvVarName), "unused")
var registryPassFromEnv = os.Getenv(registryPassEnvVarName)

func newUpCli() *cobra.Command {

	var upCmd = &cobra.Command{
		Use:   "up",
		Short: "Create and start containers running on K8s",
		Long:  "creates pods and services in an order that respects depends_on in the docker compose file",
		RunE:  upCommand,
	}
	upCmd.PersistentFlags().BoolP("detach", "d", false, "Run in "+util.AnsiColorWrap("d", "4", "0")+"etached mode: runs containers in the background")
	upCmd.PersistentFlags().BoolP("event-diffs", "v", false, "Show e"+util.AnsiColorWrap("v", "4", "0")+"ent diffs as they come in from k8s. Very useful for debugging k8s internals.")
	upCmd.PersistentFlags().StringP("registry-user", "", registryUserFromEnv,
		fmt.Sprintf("The docker registry user to authenticate as. The default is common for Openshift clusters. (env %s)", registryUserEnvVarName))
	upCmd.PersistentFlags().StringP("registry-pass", "", registryPassFromEnv,
		fmt.Sprintf("The docker registry password to authenticate with. When unset, will use the Bearer Token from Kube config as is common for Openshift clusters. (env %s)", registryPassEnvVarName))
	upCmd.PersistentFlags().BoolP("run-as-user", "", false, "When set, the runAsUser/runAsGroup will be set for each pod based on the "+
		"user of the pod's image and the \"user\" key of the pod's docker-compose service")
	upCmd.PersistentFlags().BoolP("skip-host-aliases", "a", false, "Skip adding all services ClusterIP in Pod host "+util.AnsiColorWrap("a", "4", "0")+"liases (useful when in-cluster name resolving is sufficient)")
	upCmd.PersistentFlags().BoolP("skip-push", "p", false, "Skip "+util.AnsiColorWrap("p", "4", "0")+"ushing images to registry: assumes they were previously pushed (helps get around connection problems to registry)")
	upCmd.PersistentFlags().Int64P("tail-lines", "t", 10, "Pod history log lines to show when starting to "+util.AnsiColorWrap("t", "4", "0")+"ail logs.")
	return upCmd
}

func upCommand(cmd *cobra.Command, args []string) error {
	cfg, err := getCommandConfig(cmd, args)
	if err != nil {
		return err
	}
	opts := &up.Options{}
	opts.Context = context.Background()
	opts.Detach, _ = cmd.Flags().GetBool("detach")
	opts.EventDiffs, _ = cmd.Flags().GetBool("event-diffs")
	opts.RunAsUser, _ = cmd.Flags().GetBool("run-as-user")
	opts.SkipPush, _ = cmd.Flags().GetBool("skip-push")
	opts.SkipHostAliases, _ = cmd.Flags().GetBool("skip-host-aliases")
	opts.TailLines, _ = cmd.Flags().GetInt64("tail-lines")

	opts.Reporter = reporter.New(os.Stdout)
	if opts.Reporter.IsTerminal() {
		log.StandardLogger().SetOutput(opts.Reporter.LogSink())
		go func() {
			for {
				opts.Reporter.Refresh()
				time.Sleep(reporter.RefreshInterval)
			}
		}()
	}

	opts.RegistryUser, _ = cmd.Flags().GetString("registry-user")
	opts.RegistryPass, _ = cmd.Flags().GetString("registry-pass")

	err = up.Run(cfg, opts)
	if err != nil {
		log.Error(err)
		opts.Reporter.Refresh()
		os.Exit(1)
	}
	opts.Reporter.Refresh()
	return nil
}
