package kubeclient

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"k8s.io/client-go/tools/clientcmd"
)

const managedStoreDir = "/root/.kube/containerws/configs"

// ManagedStoreDir returns the directory used for pasted/managed kubeconfig files.
func ManagedStoreDir() string {
	return managedStoreDir
}

// ManagedFilePath returns the absolute path for a managed kubeconfig id.
func ManagedFilePath(id string) string {
	id = strings.TrimSpace(id)
	return filepath.Join(managedStoreDir, id+".yaml")
}

// EnsureManagedStore creates the managed kubeconfig directory with restrictive perms.
func EnsureManagedStore() error {
	return os.MkdirAll(managedStoreDir, 0o700)
}

// ValidateContent parses kubeconfig YAML and returns an error if invalid.
func ValidateContent(content []byte) error {
	if len(strings.TrimSpace(string(content))) == 0 {
		return fmt.Errorf("kubeconfig content is empty")
	}
	cfg, err := clientcmd.Load(content)
	if err != nil {
		return fmt.Errorf("invalid kubeconfig: %w", err)
	}
	if len(cfg.Contexts) == 0 {
		return fmt.Errorf("kubeconfig has no contexts")
	}
	return nil
}

// WriteFile writes kubeconfig content to path with mode 0600.
func WriteFile(path string, content []byte) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("path is required")
	}
	if !filepath.IsAbs(path) {
		return fmt.Errorf("path must be absolute")
	}
	if err := ValidateContent(content); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, content, 0o600)
}

// ReadFile reads kubeconfig bytes from path.
func ReadFile(path string) ([]byte, error) {
	path = ResolvePath(path)
	return os.ReadFile(path)
}

// IsManagedPath reports whether path is under the managed store directory.
func IsManagedPath(path string) bool {
	path = filepath.Clean(strings.TrimSpace(path))
	prefix := filepath.Clean(managedStoreDir) + string(os.PathSeparator)
	return strings.HasPrefix(path, prefix)
}
