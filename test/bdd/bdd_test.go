package bdd

import (
	"os"
	"testing"

	"github.com/BrunoSilvaFreire/tunneld/test/bdd/steps"
	"github.com/cucumber/godog"
)

func TestFeatures(t *testing.T) {
	if os.Getenv("TUNNELD_IT") != "1" {
		t.Skip("set TUNNELD_IT=1 to run distributed integration tests")
	}

	format := "progress"
	if testing.Verbose() {
		format = "pretty"
	}

	tags := os.Getenv("GODOG_TAGS")
	if tags == "" {
		tags = "~@wip"
	}

	suite := godog.TestSuite{
		Name:                "tunneld-distributed-integration",
		ScenarioInitializer: steps.InitializeScenario,
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
