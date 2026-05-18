@docker @ssh
Feature: SSH tunnel between distributed Docker nodes

  Scenario: tunnelD exposes a private HTTP service through an SSH bastion
    Given the distributed Docker lab is running
    And node-client cannot directly rely on localhost to reach node-target
    And node-client can SSH into node-bastion
    When tunnelD starts an SSH tunnel from node-client through node-bastion to node-target port 8080
    Then node-client should receive "hello-from-private-target" from local port 18080
    And tunnelD should report the SSH tunnel as healthy
