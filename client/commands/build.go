package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	meshixv1 "gen/proto/meshix/v1"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// [{"drvPath":"/nix/store/pznj731mjim1xdd5mir97l20pk3gy5a8-hello-2.12.1.drv","outputs":{"out":"/nix/store/a7hnr9dcmx3qkkn8a20g7md1wya5zc9l-hello-2.12.1"}}]
type nixBuildOutput struct {
	DrvPaht string            `json:"drvPath"`
	Outputs map[string]string `json:"outputs"`
}

const mainOutput = "out"

type buildOverrides struct {
	Name    string `long:"o-name" description:"Name of the package"`
	Version string `long:"o-version" description:"Version of the package"`
	MainBin string `long:"o-main-bin" description:"Main binary of the package"`
}

type BuildCommand struct {
	HubUrl    string         `long:"hub-url" description:"Url of package hub"`
	Cache     string         `long:"cache" description:"Cache to push artifacts to, if not specified nothing is pushed"`
	Watch     bool           `long:"watch" description:"Watch the store and upload changes when building package"`
	All       bool           `long:"all" description:"Builds all packages in current flake"`
	Overrides buildOverrides `group:"overrides"`
}

func (x *BuildCommand) Execute(args []string) error {
	if len(args) != 1 && !x.All {
		return fmt.Errorf("Expected 1 argument or --all flag, got: %d", len(args))
	}
	ctx := context.Background()

	watchCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if x.Watch {
		go func() {
			err := WatchStore(watchCtx, "/nix/store/", x.Cache)
			if err != nil {
				// TODO better log
				fmt.Printf("Watching store failed: %v\n", err)
				os.Exit(1)
			}
		}()
	}
	expressions := []string{}
	if x.All {
		pkgs, err := getAllFlakePackages(ctx)
		if err != nil {
			return err
		}
		expressions = append(expressions, pkgs...)
	} else {
		expressions = append(expressions, args[0])
	}

	wg := sync.WaitGroup{}
	for _, expr := range expressions {
		wg.Add(1)
		go func() {
			defer wg.Done()

			err := x.buildExpr(ctx, expr)
			if err != nil {
				// TODO better log
				fmt.Printf("Expr build failed: %v\n", err)
				os.Exit(1)
			}
		}()
	}
	wg.Wait()

	return nil
}

func (x *BuildCommand) buildExpr(ctx context.Context, expr string) error {
	meta, err := getPackageMeta(ctx, expr)
	if err != nil {
		return err
	}

	buildOutput, err := buildPackage(ctx, expr)
	if err != nil {
		return err
	}

	if x.Cache != "" {
		err = pushPackage(ctx, x.Cache, expr)
		if err != nil {
			return err
		}
	}

	version, err := getPackageVersion(ctx, expr, x, meta)
	if err != nil {
		return err
	}
	mainBin := getPackageMainBin(x, meta)

	if x.HubUrl != "" {
		cc, err := grpc.NewClient(x.HubUrl, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithConnectParams(grpc.ConnectParams{
			MinConnectTimeout: 2 * time.Second,
		}))
		if err != nil {
			return err
		}

		client := meshixv1.NewMeshixServiceClient(cc)
		_, err = client.PushPackage(ctx, &meshixv1.PushPackageRequest{
			Package: &meshixv1.Package{
				Name:    meta.Name,
				Version: version,
				NixMetadata: &meshixv1.NixMetadata{
					StorePath: buildOutput.Outputs[mainOutput],
					MainBin:   mainBin,
				},
			},
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func getAllFlakePackages(ctx context.Context) ([]string, error) {
	evalArgs := []string{
		"flake", "show", "--quiet", "--json",
	}
	output, err := runNixCmd(ctx, "nix", evalArgs...)
	if err != nil {
		return nil, fmt.Errorf("Failed to show flake data 'nix %s' : %w", strings.Join(evalArgs, " "), err)
	}
	var data map[string]any
	err = json.NewDecoder(output).Decode(&data)
	if err != nil {
		return nil, err
	}
	packages, ok := data["packages"].(map[string]any)
	if !ok {
		// TODO log no package to build
		return []string{}, nil
	}

	// TODO determine which system to build for
	linuxPackages, ok := packages["x86_64-linux"].(map[string]any)
	if !ok {
		// TODO log no package to build
		return []string{}, nil
	}
	result := []string{}
	for pkgs := range linuxPackages {
		result = append(result, fmt.Sprintf(".#packages.x86_64-linux.%s", pkgs))
	}

	return result, nil
}

func getPackageMainBin(cmd *BuildCommand, meta *nixMeta) string {
	if cmd.Overrides.MainBin != "" {
		return cmd.Overrides.MainBin
	}

	return meta.MainProgram
}

func getPackageVersion(ctx context.Context, expr string, cmd *BuildCommand, meta *nixMeta) (string, error) {
	if cmd.Overrides.Version != "" {
		return cmd.Overrides.Version, nil
	}

	evalExpr := expr
	if strings.HasSuffix(evalExpr, "#") {
		evalExpr += "default.version"
	} else {
		evalExpr += ".version"
	}
	evalArgs := []string{
		"eval", "--quiet", "--json", evalExpr,
	}
	output, err := runNixCmd(ctx, "nix", evalArgs...)
	if err != nil {
		return "", fmt.Errorf("Failed to eval package meta 'nix %s' : %w", strings.Join(evalArgs, " "), err)
	}

	var version string
	err = json.NewDecoder(output).Decode(&version)
	if err != nil {
		return "", err
	}

	return version, nil
}

func pushPackage(ctx context.Context, cacheUrl string, expr string) error {
	slog.Info("Pushing to binary cache", "expr", expr)

	_, err := runNixCmd(ctx, "nix", "copy", "--quiet", "--to", cacheUrl, expr)
	if err != nil {
		return fmt.Errorf("Failed to push to binary cache: %w", err)
	}
	return nil
}

func buildPackage(ctx context.Context, expr string) (*nixBuildOutput, error) {
	slog.Info(fmt.Sprintf("Building %s", expr))
	// TODO add substituters and trusted keys
	output, err := runNixCmd(ctx, "nix", "build", "--quiet", "--json", expr)
	if err != nil {
		return nil, err
	}

	var buildOutputs []nixBuildOutput
	err = json.NewDecoder(output).Decode(&buildOutputs)
	if err != nil {
		return nil, fmt.Errorf("Failed to build package : %w", err)
	}
	if len(buildOutputs) < 1 {
		return nil, fmt.Errorf("Expected at least one build output, got: %d", len(buildOutputs))
	}
	if len(buildOutputs) > 1 {
		return nil, fmt.Errorf("Builds with more then one outputs not supported yet, got: %d", len(buildOutputs))
	}
	buildOutput := buildOutputs[0]
	slog.Info("Build derivation", "drv", buildOutput.DrvPaht, "out", buildOutput.Outputs[mainOutput])

	return &buildOutput, nil
}

func getPackageMeta(ctx context.Context, expr string) (*nixMeta, error) {
	evalExpr := expr
	if strings.HasSuffix(evalExpr, "#") {
		evalExpr += "default.meta"
	} else {
		evalExpr += ".meta"
	}
	evalArgs := []string{
		"eval", "--json", "--quiet", evalExpr,
	}
	output, err := runNixCmd(ctx, "nix", evalArgs...)
	if err != nil {
		return nil, fmt.Errorf("Failed to eval package meta 'nix %s' : %w", strings.Join(evalArgs, " "), err)
	}
	var meta nixMeta
	err = json.NewDecoder(output).Decode(&meta)
	if err != nil {
		return nil, err
	}
	if meta.MainProgram != "" {
		slog.Info("Found main bin", "bin", meta.MainProgram)
	} else {
		return nil, fmt.Errorf("No main program found for %s. Package needs to have meta.mainProgram defined", expr)
	}

	return &meta, nil
}

// {"available":true,"broken":false,"insecure":false,"name":"meshix-server","outputsToInstall":["out"],"platforms":["x86_64-darwin","i686-darwin","aarch64-darwin","armv7a-darwin","aarch64-linux","armv5tel-linux","armv6l-linux","armv7a-linux","armv7l-linux","i686-linux","loongarch64-linux","m68k-linux","microblaze-linux","microblazeel-linux","mips-linux","mips64-linux","mips64el-linux","mipsel-linux","powerpc64-linux","powerpc64le-linux","riscv32-linux","riscv64-linux","s390-linux","s390x-linux","x86_64-linux","wasm64-wasi","wasm32-wasi","i686-freebsd","x86_64-freebsd","aarch64-freebsd"],"position":"/nix/store/w6hcacb97bi6fdr4w1l0d159738hbk39-source/nix/server.nix:60","unfree":false,"unsupported":false}
type nixMeta struct {
	Available        bool     `json:"available"`
	Broken           bool     `json:"broken"`
	Description      string   `json:"description"`
	Insecure         bool     `json:"insecure"`
	MainProgram      string   `json:"mainProgram"`
	Name             string   `json:"name"`
	OutputsToInstall []string `json:"outputsToInstall"`
	Platforms        []string `json:"platforms"`
	Position         string   `json:"position"`
	Unfree           bool     `json:"unfree"`
	Unsupported      bool     `json:"unsupported"`
}

func runNixCmd(ctx context.Context, command string, args ...string) (*bytes.Buffer, error) {
	cmd := exec.CommandContext(ctx, command, args...)
	output := bytes.NewBuffer([]byte{})
	cmd.Stdout = output
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		return nil, err
	}

	return output, nil
}
