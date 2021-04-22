package install

import (
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	kumactl_data "github.com/kumahq/kuma/app/kumactl/data"
	"github.com/kumahq/kuma/app/kumactl/pkg/install/data"
	"github.com/kumahq/kuma/app/kumactl/pkg/install/k8s"
)

type loggingTemplateArgs struct {
	Namespace string
}

func newInstallLogging() *cobra.Command {
	args := struct {
		Namespace string
	}{
		Namespace: "kuma-logging",
	}
	cmd := &cobra.Command{
		Use:   "logging",
		Short: "Install Logging backend in Kubernetes cluster (Loki)",
		Long:  `Install Logging backend in Kubernetes cluster (Loki) in a 'kuma-logging' namespace`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			templateArgs := loggingTemplateArgs{
				Namespace: args.Namespace,
			}

			templateFiles, err := data.ReadFiles(kumactl_data.InstallLoggingFS())
			if err != nil {
				return errors.Wrap(err, "Failed to read template files")
			}

			renderedFiles, err := renderFiles(templateFiles, templateArgs, simpleTemplateRenderer)
			if err != nil {
				return errors.Wrap(err, "Failed to render template files")
			}

			sortedResources, err := k8s.SortResourcesByKind(renderedFiles)
			if err != nil {
				return errors.Wrap(err, "Failed to sort resources by kind")
			}

			singleFile := data.JoinYAML(sortedResources)

			if _, err := cmd.OutOrStdout().Write(singleFile.Data); err != nil {
				return errors.Wrap(err, "Failed to output rendered resources")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&args.Namespace, "namespace", args.Namespace, "namespace to install logging to")
	return cmd
}
