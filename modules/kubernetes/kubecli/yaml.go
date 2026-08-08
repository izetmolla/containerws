package kubecli

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

// ToYAML marshals obj to YAML with managedFields stripped for readability.
func ToYAML(obj any) (string, error) {
	if obj == nil {
		return "", fmt.Errorf("object is nil")
	}
	if mo, ok := obj.(metav1.Object); ok {
		mo.SetManagedFields(nil)
	}
	b, err := yaml.Marshal(obj)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
