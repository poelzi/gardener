// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package app

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/gardener/gardener/cmd/utils"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	gardenmetrics "github.com/gardener/gardener/pkg/controllerutils/metrics"
	"github.com/gardener/gardener/pkg/features"
	"github.com/gardener/gardener/pkg/healthz"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/scheduler"
	"github.com/gardener/gardener/pkg/scheduler/apis/config"
	configloader "github.com/gardener/gardener/pkg/scheduler/apis/config/loader"
	"github.com/gardener/gardener/pkg/scheduler/apis/config/validation"
	shootcontroller "github.com/gardener/gardener/pkg/scheduler/controller/shoot"
	schedulerfeatures "github.com/gardener/gardener/pkg/scheduler/features"
	"github.com/gardener/gardener/pkg/server"
	"github.com/gardener/gardener/pkg/version/verflag"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	kubernetesclientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Options has all the context and parameters needed to run a GardenerScheduler.
type Options struct {
	// ConfigFile is the location of the GardenerScheduler's configuration file.
	ConfigFile string
	config     *config.SchedulerConfiguration
}

// AddFlags adds flags for a specific Scheduler to the specified FlagSet.
func (o *Options) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&o.ConfigFile, "config", o.ConfigFile, "The path to the configuration file.")
}

// Validate validates all the required options.
func (o *Options) validate(args []string) error {
	if len(o.ConfigFile) == 0 {
		return fmt.Errorf("missing GardenerScheduler config file")
	}
	if len(args) != 0 {
		return errors.New("arguments are not supported")
	}

	return nil
}

func (o *Options) run(ctx context.Context) error {
	if len(o.ConfigFile) > 0 {
		c, err := configloader.LoadFromFile(o.ConfigFile)
		if err != nil {
			return err
		}
		o.config = c
	}

	// Add feature flags
	if err := schedulerfeatures.FeatureGate.SetFromMap(o.config.FeatureGates); err != nil {
		return err
	}
	kubernetes.UseCachedRuntimeClients = schedulerfeatures.FeatureGate.Enabled(features.CachedRuntimeClients)

	gardener, err := NewGardenerScheduler(o.config)
	if err != nil {
		return err
	}

	return gardener.Run(ctx)
}

// NewCommandStartGardenerScheduler creates a *cobra.Command object with default parameters
func NewCommandStartGardenerScheduler() *cobra.Command {
	opts := &Options{
		config: new(config.SchedulerConfiguration),
	}

	cmd := &cobra.Command{
		Use:   "gardener-scheduler",
		Short: "Launch the Gardener scheduler",
		Long:  `The Gardener scheduler is a controller that tries to find the best matching seed cluster for a shoot. The scheduler takes the cloud provider and the distance between the seed (hosting the control plane) and the shoot cluster region into account.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			verflag.PrintAndExitIfRequested()

			if err := opts.validate(args); err != nil {
				return err
			}
			return opts.run(cmd.Context())
		},
	}

	flags := cmd.Flags()
	verflag.AddFlags(flags)
	opts.AddFlags(flags)
	return cmd
}

// GardenerScheduler represents all the parameters required to start the
// Gardener scheduler.
type GardenerScheduler struct {
	Config                 *config.SchedulerConfiguration
	Identity               *gardencorev1beta1.Gardener
	GardenerNamespace      string
	K8sGardenClient        kubernetes.Interface
	K8sGardenCoreInformers gardencoreinformers.SharedInformerFactory
	Logger                 *logrus.Logger
	Recorder               record.EventRecorder
	LeaderElection         *leaderelection.LeaderElectionConfig
}

// NewGardenerScheduler is the main entry point of instantiating a new Gardener Scheduler.
func NewGardenerScheduler(cfg *config.SchedulerConfiguration) (*GardenerScheduler, error) {
	// validate the configuration
	if err := validation.ValidateConfiguration(cfg); err != nil {
		return nil, err
	}

	// Initialize logger
	logger := logger.NewLogger(cfg.LogLevel)
	logger.Info("Starting Gardener scheduler ...")
	logger.Infof("Feature Gates: %s", schedulerfeatures.FeatureGate.String())

	// Prepare a Kubernetes client object for the Garden cluster which contains all the Clientsets
	// that can be used to access the Kubernetes API.
	if kubeconfig := os.Getenv("KUBECONFIG"); kubeconfig != "" {
		cfg.ClientConnection.Kubeconfig = kubeconfig
	}

	restCfg, err := kubernetes.RESTConfigFromClientConnectionConfiguration(&cfg.ClientConnection, nil)
	if err != nil {
		return nil, err
	}

	k8sGardenClient, err := kubernetes.NewWithConfig(
		kubernetes.WithRESTConfig(restCfg),
		kubernetes.WithClientOptions(
			client.Options{
				Scheme: kubernetes.GardenScheme,
			}),
	)
	if err != nil {
		return nil, err
	}

	// Set up leader election if enabled and prepare event recorder.
	var (
		leaderElectionConfig *leaderelection.LeaderElectionConfig
		recorder             = utils.CreateRecorder(k8sGardenClient.Kubernetes(), "gardener-scheduler")
	)

	if cfg.LeaderElection.LeaderElect {
		k8sGardenClientLeaderElection, err := kubernetesclientset.NewForConfig(restCfg)
		if err != nil {
			return nil, fmt.Errorf("failed to create garden client for leader election: %w", err)
		}

		leaderElectionConfig, err = utils.MakeLeaderElectionConfig(
			cfg.LeaderElection.LeaderElectionConfiguration,
			cfg.LeaderElection.LockObjectNamespace,
			cfg.LeaderElection.LockObjectName,
			k8sGardenClientLeaderElection,
			recorder,
		)
		if err != nil {
			return nil, err
		}
	}

	return &GardenerScheduler{
		Config:                 cfg,
		Logger:                 logger,
		Recorder:               recorder,
		K8sGardenClient:        k8sGardenClient,
		K8sGardenCoreInformers: gardencoreinformers.NewSharedInformerFactory(k8sGardenClient.GardenCore(), 0),
		LeaderElection:         leaderElectionConfig,
	}, nil
}

// Run runs the Gardener Scheduler. This should never exit.
func (g *GardenerScheduler) Run(ctx context.Context) error {
	controllerCtx, controllerCancel := context.WithCancel(ctx)
	defer controllerCancel()

	// Prepare a reusable run function.
	run := func(ctx context.Context) {
		g.startScheduler(ctx)
	}

	// Initialize /healthz manager.
	healthManager := healthz.NewDefaultHealthz()
	healthManager.Start()

	// Start HTTP server.
	go server.
		NewBuilder().
		WithBindAddress(g.Config.Server.HTTP.BindAddress).
		WithPort(g.Config.Server.HTTP.Port).
		WithHandler("/metrics", promhttp.Handler()).
		WithHandlerFunc("/healthz", healthz.HandlerFunc(healthManager)).
		Build().
		Start(ctx)

	leaderElectionCtx, leaderElectionCancel := context.WithCancel(context.Background())

	// If leader election is enabled, run via LeaderElector until done and exit.
	if g.LeaderElection != nil {
		g.LeaderElection.Callbacks = leaderelection.LeaderCallbacks{
			OnStartedLeading: func(_ context.Context) {
				g.Logger.Info("Acquired leadership, starting scheduler.")
				run(controllerCtx)
				leaderElectionCancel()
			},
			OnStoppedLeading: func() {
				g.Logger.Info("Lost leadership, terminating.")
				controllerCancel()
			},
		}

		leaderElector, err := leaderelection.NewLeaderElector(*g.LeaderElection)
		if err != nil {
			return fmt.Errorf("couldn't create leader elector: %v", err)
		}

		leaderElector.Run(leaderElectionCtx)
		return nil
	}

	// Leader election is disabled, thus run directly until done.
	leaderElectionCancel()
	run(controllerCtx)
	return nil
}

func (g *GardenerScheduler) startScheduler(ctx context.Context) {
	shootScheduler := shootcontroller.NewGardenerScheduler(g.K8sGardenClient, g.K8sGardenCoreInformers, g.Config, g.Recorder)
	//backupBucketScheduler := backupbucketcontroller.NewGardenerScheduler(ctx, g.K8sGardenClient, g.K8sGardenCoreInformers, g.Config, g.Recorder)

	// Initialize the Controller metrics collection.
	gardenmetrics.RegisterControllerMetrics(
		scheduler.ControllerWorkerSum,
		scheduler.ScrapeFailures,
		shootScheduler,
		// backupBucketScheduler,
	)

	go shootScheduler.Run(ctx)
	// TODO: Enable later
	// go backupBucketScheduler.Run(ctx, g.K8sGardenCoreInformers)

	// Shutdown handling
	<-ctx.Done()

	logger.Logger.Infof("I have received a stop signal and will no longer watch events of the Garden API group.")
	logger.Logger.Infof("Bye Bye!")
}
