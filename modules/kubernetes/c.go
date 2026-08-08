package kubernetes

import (
	"log"

	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/containerws/config"
	"github.com/izetmolla/containerws/modules/kubernetes/applications"
	"github.com/izetmolla/containerws/modules/kubernetes/cluster"
	"github.com/izetmolla/containerws/modules/kubernetes/configapi"
	"github.com/izetmolla/containerws/modules/kubernetes/configs"
	"github.com/izetmolla/containerws/modules/kubernetes/ingresses"
	"github.com/izetmolla/containerws/modules/kubernetes/namespaces"
	"github.com/izetmolla/containerws/modules/kubernetes/networkpolicies"
	"github.com/izetmolla/containerws/modules/kubernetes/nodes"
	"github.com/izetmolla/containerws/modules/kubernetes/services"
	"github.com/izetmolla/containerws/modules/kubernetes/storage"
	"github.com/izetmolla/containerws/modules/kubernetes/workloads"
)

// SetupRoutesAPI mounts /api/kubernetes — Portainer-style cluster management.
// Kubeconfig secrets are stored in SQLite (k8s_keys); cluster resources are
// always read from the Kubernetes API.
func SetupRoutesAPI(router fiber.Router, appClients *config.AppClients) {
	if err := configapi.SeedMissingFromUserProfile(appClients.DB()); err != nil {
		log.Printf("k8s_keys seed from user kubeconfig failed: %v", err)
	}

	api := router.Group("/kubernetes")
	configapi.SetupRoutesAPI(api.Group("/config"), appClients)
	cluster.SetupRoutesAPI(api.Group("/cluster"), appClients)
	nodes.SetupRoutesAPI(api.Group("/nodes"), appClients)
	namespaces.SetupRoutesAPI(api.Group("/namespaces"), appClients)
	workloads.SetupRoutesAPI(api.Group("/workloads"), appClients)
	services.SetupRoutesAPI(api.Group("/services"), appClients)
	ingresses.SetupRoutesAPI(api.Group("/ingresses"), appClients)
	networkpolicies.SetupRoutesAPI(api.Group("/network-policies"), appClients)
	configs.SetupRoutesAPI(api.Group("/configs"), appClients)
	storage.SetupRoutesAPI(api.Group("/storage"), appClients)
	applications.SetupRoutesAPI(api.Group("/applications"), appClients)
}
