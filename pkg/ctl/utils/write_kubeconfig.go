package utils

import (
	"context"
	"fmt"

	"github.com/kris-nova/logger"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	api "github.com/weaveworks/eksctl/pkg/apis/eksctl.io/v1alpha5"
	"github.com/weaveworks/eksctl/pkg/ctl/cmdutils"
	"github.com/weaveworks/eksctl/pkg/eks"
	"github.com/weaveworks/eksctl/pkg/utils/kubeconfig"
)

func writeKubeconfigCmd(cmd *cmdutils.Cmd) {
	cfg := api.NewClusterConfig()
	cmd.ClusterConfig = cfg

	var (
		outputPath           string
		authenticatorRoleARN string
		setContext, autoPath bool
	)

	cmd.SetDescription("write-kubeconfig", "Write kubeconfig file for a given cluster", "")

	cmd.CobraCommand.RunE = func(_ *cobra.Command, args []string) error {
		cmd.NameArg = cmdutils.GetNameArg(args)
		return doWriteKubeconfigCmd(cmd, outputPath, authenticatorRoleARN, setContext, autoPath)
	}

	cmd.FlagSetGroup.InFlagSet("General", func(fs *pflag.FlagSet) {
		cmdutils.AddClusterFlagWithDeprecated(fs, cfg.Metadata)
		cmdutils.AddRegionFlag(fs, &cmd.ProviderConfig)
		cmdutils.AddTimeoutFlag(fs, &cmd.ProviderConfig.WaitTimeout)
		cmdutils.AddConfigFileFlag(fs, &cmd.ClusterConfigFile)
	})

	cmd.FlagSetGroup.InFlagSet("Output kubeconfig", func(fs *pflag.FlagSet) {
		cmdutils.AddCommonFlagsForKubeconfig(fs, &outputPath, &authenticatorRoleARN, &setContext, &autoPath, "<name>")
	})

	cmdutils.AddCommonFlagsForAWS(cmd, &cmd.ProviderConfig, false)
}

func doWriteKubeconfigCmd(cmd *cmdutils.Cmd, outputPath, roleARN string, setContext, autoPath bool) error {
	if err := cmdutils.NewMetadataLoader(cmd).Load(); err != nil {
		return err
	}

	cfg := cmd.ClusterConfig

	// TODO: move this into a loader when --config-file gets added to this command
	if cfg.Metadata.Name != "" && cmd.NameArg != "" {
		return cmdutils.ErrClusterFlagAndArg(cmd, cfg.Metadata.Name, cmd.NameArg)
	}

	if cmd.NameArg != "" {
		cfg.Metadata.Name = cmd.NameArg
	}

	if cfg.Metadata.Name == "" {
		return cmdutils.ErrMustBeSet(cmdutils.ClusterNameFlag(cmd))
	}

	if autoPath {
		if outputPath != kubeconfig.DefaultPath() {
			return fmt.Errorf("--kubeconfig and --auto-kubeconfig %s", cmdutils.IncompatibleFlags)
		}
		outputPath = kubeconfig.AutoPath(cfg.Metadata.Name)
	}

	ctl, err := cmd.NewProviderForExistingCluster(context.Background())
	if err != nil {
		return err
	}

	if ok, err := ctl.CanOperate(cfg); !ok {
		return err
	}

	kubectlConfig := kubeconfig.NewForKubectl(cfg, eks.GetUsername(ctl.Status.IAMRoleARN), roleARN, ctl.AWSProvider.Profile().Name)
	filename, err := kubeconfig.Write(outputPath, *kubectlConfig, setContext)
	if err != nil {
		return fmt.Errorf("writing kubeconfig: %w", err)
	}

	logger.Success("saved kubeconfig as %q", filename)

	return nil
}
