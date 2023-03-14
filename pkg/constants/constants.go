package constants

// OfflineTokenPage is the URL of the page used to generate offline access tokens.
const OfflineTokenPage = "https://console.redhat.com/openshift/token" // #nosec G101

const ManagedClusterCRDName = "managedclusters.cluster.open-cluster-management.io"

const (
	ControlPlaneName            = "multicluster-controlplane"
	ControlPlaneAdminSecretName = "controlplane-admin"
	ControlPlaneAdminFileName   = "controlplane-admin.kubeconfig"
)

const (
	DefaultControlPlaneNamespace = "multicluster-controlplane"
	DefaultKlusterletNamespace   = "open-cluster-management-agent"
)
