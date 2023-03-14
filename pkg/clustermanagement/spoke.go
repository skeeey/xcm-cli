package clustermanagement

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	clusterclient "open-cluster-management.io/api/client/cluster/clientset/versioned"
	clusterv1alpha1 "open-cluster-management.io/api/cluster/v1alpha1"

	"github.com/skeeey/xcm-cli/pkg/configs"
	"github.com/skeeey/xcm-cli/pkg/constants"
	"github.com/skeeey/xcm-cli/pkg/genericflags"
	"github.com/skeeey/xcm-cli/pkg/managedcluster"
	"github.com/skeeey/xcm-cli/pkg/resource"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

var spokeDeployFiles = []string{
	"manifests/spoke/clusterrolebinding.yaml",
	"manifests/spoke/namespace.yaml",
	"manifests/spoke/serviceaccount.yaml",
	"manifests/spoke/bootstrap-secret.yaml",
	"manifests/spoke/deployment.yaml",
}

type BootstrapKubeConfigGetter func(kubeconfigPath string) []byte

type SpokeDeployer struct {
	kubeClient          kubernetes.Interface
	hubClusterClient    clusterclient.Interface
	spokeClusterClient  clusterclient.Interface
	bootstrapKubeconfig []byte
	clusterID           string
	clusterName         string
	host                string
}

func BuildSpokeDeployer(kubeconfigPath string) (*SpokeDeployer, error) {
	configDir, err := configs.ConfigDir()
	if err != nil {
		return nil, err
	}

	controlPlaneKubeConfigFileName := filepath.Join(configDir, constants.ControlPlaneKubeAdminFileName)
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

	hubClusterClient, err := clusterclient.NewForConfig(controlPlaneKubeconfigRest)
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

	spokeClusterClient, err := clusterclient.NewForConfig(kubeconfig)
	if err != nil {
		return nil, err
	}

	return &SpokeDeployer{
		kubeClient:          kubeClient,
		spokeClusterClient:  spokeClusterClient,
		hubClusterClient:    hubClusterClient,
		bootstrapKubeconfig: controlPlaneKubeconfigData,
		host:                kubeconfig.Host,
	}, nil
}

func (d *SpokeDeployer) Relay(ctx context.Context) error {
	fmt.Fprintln(os.Stdout, "Connect current cluster to xCM [managedcluster] ...")
	if err := d.ensureCluster(ctx); err != nil {
		return fmt.Errorf("faild to create cluster in the control plane, %v", err)
	}

	fmt.Fprintln(os.Stdout, "Connect current cluster to xCM [agent] ...")
	if err := d.importCluster(ctx); err != nil {
		return fmt.Errorf("faild to import current cluster to the control plane, %v", err)
	}

	// TODO: below claims should be detected automatically
	if _, err := managedcluster.CreateClusterClaim(ctx, d.spokeClusterClient, &clusterv1alpha1.ClusterClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: "product.open-cluster-management.io",
		},
		Spec: clusterv1alpha1.ClusterClaimSpec{
			Value: "EKS",
		},
	}); err != nil {
		return fmt.Errorf("failed to create cluster claim: %v", err)
	}

	if _, err := managedcluster.CreateClusterClaim(ctx, d.spokeClusterClient, &clusterv1alpha1.ClusterClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: "platform.open-cluster-management.io",
		},
		Spec: clusterv1alpha1.ClusterClaimSpec{
			Value: "AWS",
		},
	}); err != nil {
		return fmt.Errorf("failed to create cluster claim: %v", err)
	}

	if _, err := managedcluster.CreateClusterClaim(ctx, d.spokeClusterClient, &clusterv1alpha1.ClusterClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: "region.open-cluster-management.io",
		},
		Spec: clusterv1alpha1.ClusterClaimSpec{
			Value: "us-west-1",
		},
	}); err != nil {
		return fmt.Errorf("failed to create cluster claim: %v", err)
	}

	fmt.Fprintln(os.Stdout, "Connect current cluster to xCM ...")
	if err := managedcluster.WaitManagedClusterConnected(ctx, d.hubClusterClient, d.clusterName); err != nil {
		return fmt.Errorf("failed to connect current cluster to xCM: %v", err)
	}

	return nil
}

func (d *SpokeDeployer) GetClusterID() string {
	return d.clusterID
}

// create a cluster on the hub
func (d *SpokeDeployer) ensureCluster(ctx context.Context) error {
	clusterID, err := managedcluster.CreateClusterClaim(ctx, d.spokeClusterClient, &clusterv1alpha1.ClusterClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: "xcmid.open-cluster-management.io",
		},
		Spec: clusterv1alpha1.ClusterClaimSpec{
			Value: managedcluster.GetClusterID(),
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create cluster claim: %v", err)
	}
	clusterName := managedcluster.GetClusterName(clusterID)

	// TODO check if cluster exists (a same cluster connected then relay)
	if err := managedcluster.CreateManagedCluster(ctx, d.hubClusterClient, clusterName); err != nil {
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
		Namespace           string
	}{
		BootstrapKubeconfig: d.bootstrapKubeconfig,
		ClusterName:         d.clusterName,
		Namespace:           constants.DefaultControlPlaneAgentNamespace,
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
			nil,
			nil,
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
