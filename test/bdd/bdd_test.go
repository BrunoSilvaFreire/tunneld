package bdd

import (
	"context"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/BrunoSilvaFreire/tunneld/test/bdd/steps"
	"github.com/cucumber/godog"
	"github.com/testcontainers/testcontainers-go/modules/compose"
)

func TestFeatures(t *testing.T) {
	if os.Getenv("TUNNELD_IT") != "1" && os.Getenv("CI") != "true" {
		t.Skip("set TUNNELD_IT=1 or CI=true to run distributed integration tests")
	}

	format := "progress"
	if testing.Verbose() {
		format = "pretty"
	}

	tags := os.Getenv("GODOG_TAGS")
	if tags == "" {
		tags = "~@wip"
	}

	var composeStack compose.ComposeStack

	suite := godog.TestSuite{
		Name:                "tunneld-distributed-integration",
		ScenarioInitializer: steps.InitializeScenario,
		TestSuiteInitializer: func(ctx *godog.TestSuiteContext) {
			ctx.BeforeSuite(func() {
				wd, err := os.Getwd()
				if err != nil {
					log.Fatalf("failed to get wd: %v", err)
				}
				
				runtimeDir := filepath.Join(wd, "..", ".runtime", "socket")
				if err := os.MkdirAll(runtimeDir, 0755); err != nil {
					log.Fatalf("failed to create runtime dir: %v", err)
				}
				artifactsDir := filepath.Join(wd, "..", "..", "artifacts")
				if err := os.MkdirAll(artifactsDir, 0755); err != nil {
					log.Fatalf("failed to create artifacts dir: %v", err)
				}

				sshKeyDir := filepath.Join(wd, "..", "fixtures", "ssh")
				if err := os.MkdirAll(sshKeyDir, 0755); err != nil {
					log.Fatalf("failed to create ssh dir: %v", err)
				}
				keyPath := filepath.Join(sshKeyDir, "id_ed25519")
				if _, err := os.Stat(keyPath); os.IsNotExist(err) {
					// generate key
					cmd := exec.Command("ssh-keygen", "-t", "ed25519", "-N", "", "-f", keyPath, "-C", "tunneld-integration-test")
					if err := cmd.Run(); err != nil {
						log.Fatalf("failed to generate ssh key: %v", err)
					}
					os.Chmod(keyPath, 0600)
					os.Chmod(keyPath+".pub", 0644)
				}

				composeFile := filepath.Join(wd, "..", "compose", "docker-compose.yml")

				// Load the docker-compose stack
				stack, err := compose.NewDockerCompose(composeFile)
				if err != nil {
					log.Fatalf("Failed to parse docker-compose.yml: %v", err)
				}
				
				composeStack = stack

				// Bring up the stack
				err = composeStack.
					WithEnv(map[string]string{}).
					Up(context.Background(), compose.Wait(true))

				if err != nil {
					log.Fatalf("Failed to bring up compose stack: %v", err)
				}
			})
			ctx.AfterSuite(func() {
				if composeStack != nil {
					_ = composeStack.Down(context.Background(), compose.RemoveOrphans(true), compose.RemoveVolumes(true))
				}
			})
		},
		Options: &godog.Options{
			Format:   format,
			Paths:    []string{"features"},
			Tags:     tags,
			Strict:   true,
			TestingT: t,
		},
	}

	if status := suite.Run(); status != 0 {
		t.Fatalf("godog suite failed with status %d", status)
	}
}
