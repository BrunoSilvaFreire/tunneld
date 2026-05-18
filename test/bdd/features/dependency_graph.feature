@docker @ssh @kubernetes @dependency @wip
Feature: Tunnel dependency graph

  Scenario: kubectl tunnel waits for SSH dependency
    Given the k3d cluster is running
    And a kubeconfig is available only through an SSH tunnel
    And tunnelD has an SSH tunnel named "kube-api-bastion"
    And tunnelD has a kubectl tunnel named "echo-port-forward"
    And "echo-port-forward" depends on "kube-api-bastion"
    When tunnelD starts the tunnel graph
    Then tunnelD should start "kube-api-bastion" before "echo-port-forward"
    And localhost port 18082 should return "hello-from-kubernetes"
