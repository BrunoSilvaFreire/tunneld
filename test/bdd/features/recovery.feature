@docker @ssh @recovery
Feature: Recovery from node failure

  Scenario: tunnelD detects bastion interruption and recovers
    Given an SSH tunnel through node-bastion is healthy
    When node-bastion is disconnected from the Docker network
    Then tunnelD should mark the SSH tunnel as unhealthy or degraded
    When node-bastion is reconnected to the Docker network
    Then tunnelD should eventually mark the SSH tunnel as healthy again
