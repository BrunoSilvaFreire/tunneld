package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	pb "github.com/BrunoSilvaFreire/tunneld/pkg/api/v1"
)

func slurpKeys(spec *pb.TunnelSpec) (map[string][]byte, error) {
	inlineKeys := make(map[string][]byte)

	switch s := spec.Type.(type) {
	case *pb.TunnelSpec_Ssh:
		if s.Ssh.IdentityKeyFile != "" {
			content, name, err := readLocalKey(s.Ssh.IdentityKeyFile)
			if err != nil {
				return nil, err
			}
			inlineKeys[name] = content
			s.Ssh.IdentityKeyFile = ""
			s.Ssh.IdentityKeyRef = name
		}
	case *pb.TunnelSpec_Kubectl:
		if s.Kubectl.KubeconfigFile != "" {
			content, name, err := readLocalKey(s.Kubectl.KubeconfigFile)
			if err != nil {
				return nil, err
			}
			inlineKeys[name] = content
			s.Kubectl.KubeconfigFile = ""
			s.Kubectl.KubeconfigRef = name
		}
	}

	return inlineKeys, nil
}

func readLocalKey(path string) ([]byte, string, error) {
	if strings.HasPrefix(path, "~") {
		home, _ := os.UserHomeDir()
		path = filepath.Join(home, path[1:])
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read local key %q: %v", path, err)
	}

	return content, filepath.Base(path), nil
}
