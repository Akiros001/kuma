package cmd

import (
	"io/ioutil"
	"os"
	"path/filepath"

	kumadp_config "github.com/kumahq/kuma/app/kuma-dp/pkg/config"
	"github.com/kumahq/kuma/app/kuma-dp/pkg/dataplane/dnsserver"
	"github.com/kumahq/kuma/app/kuma-dp/pkg/dataplane/metrics"
	"github.com/kumahq/kuma/pkg/core/resources/model/rest"
	"github.com/kumahq/kuma/pkg/core/runtime/component"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/kumahq/kuma/app/kuma-dp/pkg/dataplane/accesslogs"
	"github.com/kumahq/kuma/app/kuma-dp/pkg/dataplane/envoy"
	"github.com/kumahq/kuma/pkg/config"
	config_types "github.com/kumahq/kuma/pkg/config/types"
	"github.com/kumahq/kuma/pkg/core"
	util_net "github.com/kumahq/kuma/pkg/util/net"
	kuma_version "github.com/kumahq/kuma/pkg/version"
)

var runLog = dataplaneLog.WithName("run")

// PersistentPreRunE in root command sets the logger and initial config
// PreRunE loads the Kuma DP config
// PostRunE actually runs all the components with loaded config
// To extend Kuma DP, plug your code in RunE. Use RootContext.Config and add components to RootContext.ComponentManager
func newRunCmd(rootCtx *RootContext) *cobra.Command {
	cfg := rootCtx.Config
	var dp *rest.Resource
	var tmpDir string
	var adminPort uint32
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Launch Dataplane (Envoy)",
		Long:  `Launch Dataplane (Envoy).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
		PreRunE: func(cmd *cobra.Command, args []string) error {
			// only support configuration via environment variables and args
			if err := config.Load("", cfg); err != nil {
				runLog.Error(err, "unable to load configuration")
				return err
			}
			if conf, err := config.ToJson(cfg); err == nil {
				runLog.Info("effective configuration", "config", string(conf))
			} else {
				runLog.Error(err, "unable to format effective configuration", "config", cfg)
				return err
			}

			var err error
			dp, err = readDataplaneResource(cmd, cfg)
			if err != nil {
				runLog.Error(err, "unable to read provided dataplane")
				return err
			}
			if dp != nil {
				if cfg.Dataplane.Name != "" || cfg.Dataplane.Mesh != "" {
					return errors.New("--name and --mesh cannot be specified when dataplane definition is provided. Mesh and name will be read from the dataplane definition.")
				}
				cfg.Dataplane.Mesh = dp.Meta.GetMesh()
				cfg.Dataplane.Name = dp.Meta.GetName()
			}

			if !cfg.Dataplane.AdminPort.Empty() {
				// unless a user has explicitly opted out of Envoy Admin API, pick a free port from the range
				adminPort, err = util_net.PickTCPPort("127.0.0.1", cfg.Dataplane.AdminPort.Lowest(), cfg.Dataplane.AdminPort.Highest())
				if err != nil {
					return errors.Wrapf(err, "unable to find a free port in the range %q for Envoy Admin API to listen on", cfg.Dataplane.AdminPort)
				}
				cfg.Dataplane.AdminPort = config_types.MustExactPort(adminPort)
				runLog.Info("picked a free port for Envoy Admin API to listen on", "port", cfg.Dataplane.AdminPort)
			}

			if cfg.DataplaneRuntime.ConfigDir == "" || cfg.DNS.ConfigDir == "" {
				tmpDir, err = ioutil.TempDir("", "kuma-dp-")
				if err != nil {
					runLog.Error(err, "unable to create a temporary directory to store generated config sat")
					return err
				}

				if cfg.DataplaneRuntime.ConfigDir == "" {
					cfg.DataplaneRuntime.ConfigDir = tmpDir
				}

				if cfg.DNS.ConfigDir == "" {
					cfg.DNS.ConfigDir = tmpDir
				}

				runLog.Info("generated configurations will be stored in a temporary directory", "dir", tmpDir)
			}

			if cfg.DataplaneRuntime.Token != "" {
				path := filepath.Join(cfg.DataplaneRuntime.ConfigDir, cfg.Dataplane.Name)
				if err := writeFile(path, []byte(cfg.DataplaneRuntime.Token), 0600); err != nil {
					runLog.Error(err, "unable to create file with dataplane token")
					return err
				}
				cfg.DataplaneRuntime.TokenPath = path
			}

			if cfg.DataplaneRuntime.TokenPath != "" {
				if err := kumadp_config.ValidateTokenPath(cfg.DataplaneRuntime.TokenPath); err != nil {
					return err
				}
			}

			if cfg.ControlPlane.CaCert == "" && cfg.ControlPlane.CaCertFile != "" {
				cert, err := ioutil.ReadFile(cfg.ControlPlane.CaCertFile)
				if err != nil {
					return errors.Wrapf(err, "could not read certificate file %s", cfg.ControlPlane.CaCertFile)
				}
				cfg.ControlPlane.CaCert = string(cert)
			}
			return nil
		},
		PostRunE: func(cmd *cobra.Command, _ []string) error {
			if tmpDir != "" { // clean up temp dir if it was created
				defer func() {
					if err := os.RemoveAll(tmpDir); err != nil {
						runLog.Error(err, "unable to remove a temporary directory with a generated Envoy config")
					}
				}()
			}

			shouldQuit := setupQuitChannel()
			components := []component.Component{
				accesslogs.NewAccessLogServer(cfg.Dataplane),
			}

			opts := envoy.Opts{
				Config:          *cfg,
				Generator:       rootCtx.BootstrapGenerator,
				Dataplane:       dp,
				DynamicMetadata: rootCtx.BootstrapDynamicMetadata,
				Stdout:          cmd.OutOrStdout(),
				Stderr:          cmd.OutOrStderr(),
				Quit:            shouldQuit,
				LogLevel:        rootCtx.LogLevel,
			}

			if cfg.DNS.Enabled {
				opts.DNSPort = cfg.DNS.EnvoyDNSPort
				opts.EmptyDNSPort = cfg.DNS.CoreDNSEmptyPort

				dnsOpts := &dnsserver.Opts{
					Config: *cfg,
					Stdout: cmd.OutOrStdout(),
					Stderr: cmd.OutOrStderr(),
					Quit:   shouldQuit,
				}

				dnsServer, err := dnsserver.New(dnsOpts)
				if err != nil {
					return err
				}

				components = append(components, dnsServer)
			}

			dataplane, err := envoy.New(opts)
			if err != nil {
				return err
			}

			components = append(components, dataplane)

			metricsServer := metrics.New(cfg.Dataplane, adminPort)
			components = append(components, metricsServer)

			if err := rootCtx.ComponentManager.Add(components...); err != nil {
				return err
			}

			runLog.Info("starting Kuma DP", "version", kuma_version.Build.Version)
			if err := rootCtx.ComponentManager.Start(shouldQuit); err != nil {
				runLog.Error(err, "error while running Kuma DP")
				return err
			}
			runLog.Info("stopping Kuma DP")
			return nil
		},
	}

	cmd.PersistentFlags().StringVar(&cfg.Dataplane.Name, "name", cfg.Dataplane.Name, "Name of the Dataplane")
	cmd.PersistentFlags().Var(&cfg.Dataplane.AdminPort, "admin-port", `Port (or range of ports to choose from) for Envoy Admin API to listen on. Empty value indicates that Envoy Admin API should not be exposed over TCP. Format: "9901 | 9901-9999 | 9901- | -9901"`)
	cmd.PersistentFlags().StringVar(&cfg.Dataplane.Mesh, "mesh", cfg.Dataplane.Mesh, "Mesh that Dataplane belongs to")
	cmd.PersistentFlags().StringVar(&cfg.ControlPlane.URL, "cp-address", cfg.ControlPlane.URL, "URL of the Control Plane Dataplane Server. Example: https://localhost:5678")
	cmd.PersistentFlags().StringVar(&cfg.ControlPlane.CaCertFile, "ca-cert-file", cfg.ControlPlane.CaCertFile, "Path to CA cert by which connection to the Control Plane will be verified if HTTPS is used")
	cmd.PersistentFlags().StringVar(&cfg.DataplaneRuntime.BinaryPath, "binary-path", cfg.DataplaneRuntime.BinaryPath, "Binary path of Envoy executable")
	cmd.PersistentFlags().StringVar(&cfg.Dataplane.BootstrapVersion, "bootstrap-version", cfg.Dataplane.BootstrapVersion, "Bootstrap version (and API version) of xDS config. If empty, default version defined in Kuma CP will be used. (ex. '2', '3')")
	cmd.PersistentFlags().StringVar(&cfg.DataplaneRuntime.ConfigDir, "config-dir", cfg.DataplaneRuntime.ConfigDir, "Directory in which Envoy config will be generated")
	cmd.PersistentFlags().StringVar(&cfg.DataplaneRuntime.TokenPath, "dataplane-token-file", cfg.DataplaneRuntime.TokenPath, "Path to a file with dataplane token (use 'kumactl generate dataplane-token' to get one)")
	cmd.PersistentFlags().StringVar(&cfg.DataplaneRuntime.Token, "dataplane-token", cfg.DataplaneRuntime.Token, "Dataplane Token")
	cmd.PersistentFlags().StringVar(&cfg.DataplaneRuntime.Resource, "dataplane", "", "Dataplane template to apply (YAML or JSON)")
	cmd.PersistentFlags().StringVarP(&cfg.DataplaneRuntime.ResourcePath, "dataplane-file", "d", "", "Path to Dataplane template to apply (YAML or JSON)")
	cmd.PersistentFlags().StringToStringVarP(&cfg.DataplaneRuntime.ResourceVars, "dataplane-var", "v", map[string]string{}, "Variables to replace Dataplane template")
	cmd.PersistentFlags().BoolVar(&cfg.DNS.Enabled, "dns-enabled", cfg.DNS.Enabled, "If true then builtin DNS functionality is enabled and CoreDNS server is started")
	cmd.PersistentFlags().Uint32Var(&cfg.DNS.EnvoyDNSPort, "dns-envoy-port", cfg.DNS.EnvoyDNSPort, "A port that handles Virtual IP resolving by Envoy. CoreDNS should be configured that it first tries to use this DNS resolver and then the real one")
	cmd.PersistentFlags().Uint32Var(&cfg.DNS.CoreDNSPort, "dns-coredns-port", cfg.DNS.CoreDNSPort, "A port that handles DNS requests. When transparent proxy is enabled then iptables will redirect DNS traffic to this port.")
	cmd.PersistentFlags().Uint32Var(&cfg.DNS.CoreDNSEmptyPort, "dns-coredns-empty-port", cfg.DNS.CoreDNSEmptyPort, "A port that always responds with empty NXDOMAIN respond. It is required to implement a fallback to a real DNS.")
	cmd.PersistentFlags().StringVar(&cfg.DNS.CoreDNSBinaryPath, "dns-coredns-path", cfg.DNS.CoreDNSBinaryPath, "A path to CoreDNS binary.")
	cmd.PersistentFlags().StringVar(&cfg.DNS.CoreDNSConfigTemplatePath, "dns-coredns-config-template-path", cfg.DNS.CoreDNSConfigTemplatePath, "A path to a CoreDNS config template.")
	cmd.PersistentFlags().StringVar(&cfg.DNS.ConfigDir, "dns-server-config-dir", cfg.DNS.ConfigDir, "Directory in which DNS Server config will be generated")
	cmd.PersistentFlags().Uint32Var(&cfg.DNS.PrometheusPort, "dns-prometheus-port", cfg.DNS.PrometheusPort, "A port for exposing Prometheus stats")
	return cmd
}

func writeFile(filename string, data []byte, perm os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(filename), perm); err != nil {
		return err
	}
	return ioutil.WriteFile(filename, data, perm)
}

func setupQuitChannel() chan struct{} {
	quit := make(chan struct{})
	quitOnSignal := core.SetupSignalHandler()
	go func() {
		<-quitOnSignal
		runLog.Info("Kuma DP caught an exit signal")
		if quit != nil {
			close(quit)
		}
	}()

	return quit
}
