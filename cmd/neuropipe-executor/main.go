// Command neuropipe-executor is the standalone Neuropipe pipeline executor.
// It stores deployed pipeline bundles, executes them with the full Blueprint
// engine, hosts trusted cron schedules autonomously, and serves a
// token-authenticated gRPC endpoint for the desktop application.
package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"

	"github.com/FlameInTheDark/neuropipe/internal/catalog"
	"github.com/FlameInTheDark/neuropipe/internal/executord"
	executorv1 "github.com/FlameInTheDark/neuropipe/internal/proto/executor/v1"
	"github.com/FlameInTheDark/neuropipe/internal/remoteexec"
	"github.com/FlameInTheDark/neuropipe/internal/security"
	"github.com/urfave/cli/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

const tokenOnceBanner = `
========================================================================
A new shared token was generated and saved to:
  %s

%s

Copy this token into Neuropipe (Settings -> Remote Executors).
It is shown only once and will not be printed again.
========================================================================`

func main() {
	command := newApp()
	if err := command.Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}
}

// newApp assembles the executor CLI. Shared flags live on the root so both
// bare invocation (`neuropipe-executor`) and every subcommand accept them.
func newApp() *cli.Command {
	return &cli.Command{
		Name:    "neuropipe-executor",
		Usage:   "Run Neuropipe pipelines on this machine",
		Version: executord.ExecutorVersion,
		Description: "The executor stores pipeline bundles deployed by the Neuropipe desktop app, " +
			"executes them with the full Blueprint engine, and keeps trusted schedules running " +
			"while Neuropipe is closed. Starting without any configured token generates one " +
			"automatically; copy the printed value into Neuropipe once.",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "config",
				Value: "executor.json",
				Usage: "path to the boot configuration file",
			},
			&cli.StringFlag{
				Name:  "data-dir",
				Usage: "override the data directory from the config",
			},
			&cli.StringFlag{
				Name:  "listen",
				Usage: "override the listen address from the config",
			},
			&cli.StringFlag{
				Name:  "token",
				Usage: "shared auth token; overrides " + executord.TokenEnvVar + ", the config token file, and any saved token",
			},
		},
		// Bare invocation starts the daemon so service definitions stay simple.
		Action: serveAction,
		Commands: []*cli.Command{
			{
				Name:   "serve",
				Usage:  "Start the executor daemon",
				Action: serveAction,
			},
			{
				Name:  "token",
				Usage: "Manage the shared auth token",
				Commands: []*cli.Command{
					{
						Name:  "generate",
						Usage: "Generate a fresh token, save it, and print it exactly once",
						Description: "Any previously generated or configured token stops working. " +
							"Update the registration in Neuropipe afterwards.",
						Action: tokenGenerateAction,
					},
				},
			},
			{
				Name:  "status",
				Usage: "Show local configuration and deployment summary without starting the daemon",
				Description: "Prints where the effective token would come from (never its value), " +
					"the listen address, TLS mode, and how many pipelines and runs are stored locally.",
				Action: statusAction,
			},
		},
	}
}

// prepareBoot loads the boot file and applies command-line overrides.
func prepareBoot(cmd *cli.Command) (executord.BootConfig, error) {
	boot, err := executord.LoadBootConfig(cmd.String("config"))
	if err != nil {
		return executord.BootConfig{}, err
	}
	if dir := strings.TrimSpace(cmd.String("data-dir")); dir != "" {
		boot.DataDir = dir
	}
	// Every store, the vault, and the default token location share one
	// normalized root; an unset config value means ./data.
	if strings.TrimSpace(boot.DataDir) == "" {
		boot.DataDir = "data"
	}
	if address := strings.TrimSpace(cmd.String("listen")); address != "" {
		boot.ListenAddress = address
	}
	return boot, nil
}

func serveAction(ctx context.Context, cmd *cli.Command) error {
	if cmd.Args().Len() > 0 {
		return fmt.Errorf("unexpected argument %q; run 'neuropipe-executor --help' for usage", cmd.Args().First())
	}
	boot, err := prepareBoot(cmd)
	if err != nil {
		return err
	}
	resolved, err := executord.EnsureToken(boot, cmd.String("token"))
	if err != nil {
		return err
	}
	if resolved.Generated {
		fmt.Printf(tokenOnceBanner+"\n", resolved.Path, resolved.Value)
	}
	return runExecutor(ctx, boot, resolved.Value)
}

func tokenGenerateAction(ctx context.Context, cmd *cli.Command) error {
	boot, err := prepareBoot(cmd)
	if err != nil {
		return err
	}
	resolved, err := executord.GenerateAndSaveToken(boot)
	if err != nil {
		return err
	}
	fmt.Printf(tokenOnceBanner+"\n", resolved.Path, resolved.Value)
	fmt.Println("The previous token is no longer valid; update Neuropipe if it was already connected.")
	return nil
}

func statusAction(ctx context.Context, cmd *cli.Command) error {
	boot, err := prepareBoot(cmd)
	if err != nil {
		return err
	}
	resolved, resolveErr := executord.ResolveToken(boot, cmd.String("token"))

	listenAddress := boot.ListenAddress
	if strings.TrimSpace(listenAddress) == "" {
		listenAddress = executord.DefaultListenAddress
	}
	dataDir := boot.DataDir
	if strings.TrimSpace(dataDir) == "" {
		dataDir = "data"
	}

	fmt.Println("Neuropipe executor")
	fmt.Printf("  version:      %s\n", executord.ExecutorVersion)
	fmt.Printf("  platform:     %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Printf("  data dir:     %s\n", dataDir)
	fmt.Printf("  listen:       %s\n", listenAddress)
	fmt.Printf("  TLS:          %v\n", boot.HasTLS())
	if resolveErr != nil {
		fmt.Printf("  token source: misconfigured (%v)\n", resolveErr)
	} else if resolved.Value == "" {
		fmt.Print("  token source: none yet (one is generated on first start)\n")
	} else {
		fmt.Printf("  token source: %s\n", string(resolved.Source))
	}

	bundles := executord.NewStore(dataDir)
	fmt.Printf("  deployed:     %d pipeline(s)\n", len(bundles.ListBundles()))
	fmt.Printf("  stored runs:  %d\n", len(bundles.RecentRuns(0)))
	return nil
}

// runExecutor composes and serves the gRPC daemon until interrupted.
func runExecutor(ctx context.Context, boot executord.BootConfig, token string) error {
	if err := executord.PrepareDataDir(boot.DataDir); err != nil {
		return err
	}

	vault, err := security.NewVault(boot.DataDir)
	if err != nil {
		return fmt.Errorf("open executor vault: %w", err)
	}
	runtimeConfig, err := executord.NewRuntimeStore(boot.DataDir)
	if err != nil {
		return err
	}
	globals, err := executord.NewExecutorGlobals(boot.DataDir)
	if err != nil {
		return err
	}
	bundles := executord.NewStore(boot.DataDir)
	tunnel := executord.NewTunnelCaller()
	notifier := executord.NewNotifier()

	runner := executord.NewRunner(bundles, catalog.New(), globals, tunnel, notifier, runtimeConfig, vault)
	runner.Start()
	defer runner.Stop()

	schedules := executord.NewCronScheduler(bundles, runner)
	if err := schedules.Start(); err != nil {
		return fmt.Errorf("start schedule worker: %w", err)
	}
	defer schedules.Stop()

	service := executord.NewService(executord.ExecutorVersion, bundles, runtimeConfig, vault, runner, tunnel, schedules)

	listener, err := net.Listen("tcp", boot.ListenAddress)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", boot.ListenAddress, err)
	}
	options := []grpc.ServerOption{
		grpc.ChainUnaryInterceptor(remoteexec.UnaryAuthInterceptor(token)),
		grpc.ChainStreamInterceptor(remoteexec.StreamAuthInterceptor(token)),
	}
	if boot.HasTLS() {
		transport, err := credentials.NewServerTLSFromFile(boot.TLSCert, boot.TLSKey)
		if err != nil {
			return fmt.Errorf("load TLS material: %w", err)
		}
		options = append(options, grpc.Creds(transport))
	}
	server := grpc.NewServer(options...)
	executorv1.RegisterExecutorServer(server, service)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-stop
		log.Println("shutting down")
		server.GracefulStop()
	}()

	log.Printf("neuropipe-executor %s listening on %s (data dir %s)", executord.ExecutorVersion, boot.ListenAddress, boot.DataDir)
	if err := server.Serve(listener); err != nil {
		return fmt.Errorf("serve gRPC: %w", err)
	}
	return nil
}
