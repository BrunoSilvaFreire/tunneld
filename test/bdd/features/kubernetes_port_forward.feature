@kubernetes @k3d
Feature: Kubernetes port-forward tunnel

  Scenario: tunnelD starts a kubectl port-forward tunnel to a service
    Given the k3d cluster is running
    And the Kubernetes echo service is ready
    When tunnelD starts a kubectl tunnel to service "echo" port 8080
    Then localhost port 18081 should return "hello-from-kubernetes"
    And tunnelD should report the Kubernetes tunnel as healthy
