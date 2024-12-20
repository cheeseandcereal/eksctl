package fargate

import (
	"context"
	"fmt"

	"github.com/weaveworks/eksctl/pkg/cfn/manager"

	api "github.com/weaveworks/eksctl/pkg/apis/eksctl.io/v1alpha5"
	"github.com/weaveworks/eksctl/pkg/cfn/outputs"
	"github.com/weaveworks/eksctl/pkg/eks"
	"github.com/weaveworks/eksctl/pkg/fargate"
)

func (m *Manager) Create(ctx context.Context) error {
	ctl := m.ctl
	cfg := m.cfg
	if ok, err := ctl.CanOperate(cfg); !ok {
		return fmt.Errorf("couldn't check cluster operable status: %w", err)
	}

	clusterStack, err := m.stackManager.DescribeClusterStackIfExists(ctx)
	if err != nil {
		return fmt.Errorf("couldn't check cluster stack: %w", err)
	}

	fargateRoleNeeded := false

	for _, profile := range cfg.FargateProfiles {
		if profile.PodExecutionRoleARN == "" {
			fargateRoleNeeded = true
			break
		}
	}

	if fargateRoleNeeded {
		if clusterStack != nil {
			if !m.fargateRoleExistsOnClusterStack(clusterStack) {
				err := ensureFargateRoleStackExists(ctx, cfg, ctl.AWSProvider, m.stackManager)
				if err != nil {
					return fmt.Errorf("couldn't ensure fargate role exists: %w", err)
				}
			}
			if err := ctl.LoadClusterIntoSpecFromStack(ctx, cfg, clusterStack); err != nil {
				return fmt.Errorf("couldn't load cluster into spec: %w", err)
			}
		} else {
			if err := ensureFargateRoleStackExists(ctx, cfg, ctl.AWSProvider, m.stackManager); err != nil {
				return fmt.Errorf("couldn't ensure unowned cluster is ready for fargate: %w", err)
			}
		}

		if !api.IsSetAndNonEmptyString(cfg.IAM.FargatePodExecutionRoleARN) {
			// Read back the default Fargate pod execution role ARN from CloudFormation:
			if err := m.stackManager.RefreshFargatePodExecutionRoleARN(ctx); err != nil {
				return fmt.Errorf("couldn't refresh role arn: %w", err)
			}
		}
	}

	fargateClient := fargate.NewFromProvider(cfg.Metadata.Name, ctl.AWSProvider, m.stackManager)
	if err := eks.DoCreateFargateProfiles(ctx, cfg, &fargateClient); err != nil {
		return fmt.Errorf("could not create fargate profiles: %w", err)
	}
	clientSet, err := m.newStdClientSet()
	if err != nil {
		return fmt.Errorf("couldn't create kubernetes client: %w", err)
	}
	return eks.ScheduleCoreDNSOnFargateIfRelevant(cfg, ctl, clientSet)
}

func (m *Manager) fargateRoleExistsOnClusterStack(clusterStack *manager.Stack) bool {
	for _, output := range clusterStack.Outputs {
		if *output.OutputKey == outputs.FargatePodExecutionRoleARN {
			return true
		}
	}
	return false
}
