package clustermanagement

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	clusterclient "open-cluster-management.io/api/client/cluster/clientset/versioned"
	ocmoperatorclient "open-cluster-management.io/api/client/operator/clientset/versioned"

	"github.com/skeeey/xcm-cli/pkg/configs"
	"github.com/skeeey/xcm-cli/pkg/constants"
	"github.com/skeeey/xcm-cli/pkg/genericflags"
	"github.com/skeeey/xcm-cli/pkg/managedcluster"
	"github.com/skeeey/xcm-cli/pkg/resource"

	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

var spokeDeployFiles = []string{
	"manifests/spoke/klusterlet.crd.v1.yaml",
	"manifests/spoke/agent_namespace.yaml",
	"manifests/spoke/operator_namespace.yaml",
	"manifests/spoke/service_account.yaml",
	"manifests/spoke/cluster_role.yaml",
	"manifests/spoke/clusterrole_aggregate.yaml",
	"manifests/spoke/cluster_role_binding.yaml",
	"manifests/spoke/operator.yaml",
	"manifests/spoke/bootstrap_secret.yaml",
	"manifests/spoke/klusterlet.yaml",
}

type BootstrapKubeConfigGetter func(kubeconfigPath string) []byte

type SpokeDeployer struct {
	kubeClient          kubernetes.Interface
	apiExtensionsClient apiextensionsclient.Interface
	operatorClient      ocmoperatorclient.Interface
	clusterClient       clusterclient.Interface
	bootstrapKubeconfig []byte
	clusterID           string
	clusterName         string
	host                string
	controlPlaneSelf    bool
}

func BuildSpokeDeployer(kubeconfigPath string, controlPlaneSelf bool) (*SpokeDeployer, error) {
	configDir, err := configs.ConfigDir()
	if err != nil {
		return nil, err
	}

	controlPlaneKubeConfigFileName := filepath.Join(configDir, constants.ControlPlaneAdminFileName)
	controlPlaneKubeconfig, err := clientcmd.LoadFromFile(controlPlaneKubeConfigFileName)
	if err != nil {
		return nil, fmt.Errorf("failed to load control plane kube admin config, %v", err)
	}

	controlPlaneKubeconfigRest, err := clientcmd.BuildConfigFromFlags("", controlPlaneKubeConfigFileName)
	if err != nil {
		return nil, err
	}

	controlPlaneKubeconfigData, err := clientcmd.Write(*controlPlaneKubeconfig)
	if err != nil {
		return nil, err
	}

	clusterClient, err := clusterclient.NewForConfig(controlPlaneKubeconfigRest)
	if err != nil {
		return nil, err
	}

	kubeconfig, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return nil, err
	}

	kubeClient, err := kubernetes.NewForConfig(kubeconfig)
	if err != nil {
		return nil, err
	}

	apiExtensionsClient, err := apiextensionsclient.NewForConfig(kubeconfig)
	if err != nil {
		return nil, err
	}

	operatorClient, err := ocmoperatorclient.NewForConfig(kubeconfig)
	if err != nil {
		return nil, err
	}

	return &SpokeDeployer{
		kubeClient:          kubeClient,
		apiExtensionsClient: apiExtensionsClient,
		operatorClient:      operatorClient,
		clusterClient:       clusterClient,
		bootstrapKubeconfig: controlPlaneKubeconfigData,
		host:                kubeconfig.Host,
		controlPlaneSelf:    controlPlaneSelf,
	}, nil
}

func (d *SpokeDeployer) Relay(ctx context.Context) error {
	fmt.Fprintln(os.Stdout, "Connect current cluster to xCM [managedcluster] ...")
	if err := d.ensureCluster(ctx); err != nil {
		return fmt.Errorf("faild to create cluster in the control plane, %v", err)
	}

	fmt.Fprintln(os.Stdout, "Connect current cluster to xCM [klusterlet] ...")
	if err := d.importCluster(ctx); err != nil {
		return fmt.Errorf("faild to import current cluster to the control plane, %v", err)
	}

	fmt.Fprintln(os.Stdout, "Connect current cluster to xCM ...")
	if err := managedcluster.WaitManagedClusterConnected(ctx, d.clusterClient, d.clusterName); err != nil {
		return fmt.Errorf("failed to connect current cluster to xCM: %v", err)
	}

	return nil
}

func (d *SpokeDeployer) GetClusterID() string {
	return d.clusterID
}

// create a cluster on the hub
func (d *SpokeDeployer) ensureCluster(ctx context.Context) error {
	// TODO get control plane id from control plane - get control plane cluster itself
	controlPlaneID := ""

	clusterID := managedcluster.GetClusterID()
	if d.controlPlaneSelf {
		controlPlaneID = clusterID
	}
	clusterName := managedcluster.GetClusterName(clusterID)

	// TODO check if cluster exists (a same cluster connected then relay)
	if err := managedcluster.CreateManagedCluster(
		ctx, d.clusterClient, d.controlPlaneSelf, controlPlaneID, clusterName); err != nil {
		return err
	}

	// TODO create cluster claim

	d.clusterID = clusterID
	d.clusterName = clusterName

	return nil
}

func (d *SpokeDeployer) importCluster(ctx context.Context) error {
	config := struct {
		BootstrapKubeconfig []byte
		ClusterName         string
	}{
		BootstrapKubeconfig: d.bootstrapKubeconfig,
		ClusterName:         d.clusterName,
	}

	objects := []runtime.Object{}
	for _, file := range spokeDeployFiles {
		template, err := manifestFiles.ReadFile(file)
		if err != nil {
			// this should not happen, if happened, panic here
			panic(err)
		}

		objects = append(objects, resource.MustCreateObjectFromTemplate(file, template, config))
	}

	return wait.PollImmediate(1*time.Second, genericflags.TimeOut(), func() (bool, error) {
		err := resource.ApplyResources(
			ctx,
			d.kubeClient,
			d.apiExtensionsClient,
			d.operatorClient,
			objects...,
		)
		if errors.IsNotFound(err) {
			return false, nil
		}
		if err != nil {
			return false, err
		}

		return true, nil
	})
}
