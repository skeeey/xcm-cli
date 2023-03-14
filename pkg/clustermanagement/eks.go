package clustermanagement

import (
	"context"
	"embed"
	"fmt"
	"os"
	"time"

	"github.com/skeeey/xcm-cli/pkg/configs"
	"github.com/skeeey/xcm-cli/pkg/constants"
	"github.com/skeeey/xcm-cli/pkg/genericflags"
	"github.com/skeeey/xcm-cli/pkg/helpers"
	"github.com/skeeey/xcm-cli/pkg/managedcluster"
	"github.com/skeeey/xcm-cli/pkg/resource"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	clusterclient "open-cluster-management.io/api/client/cluster/clientset/versioned"
	clusterv1alpha1 "open-cluster-management.io/api/cluster/v1alpha1"
)

const ocmconfigfile = "manifests/connector/ocmconfig.yaml"

var serviceFiles = []string{
	"manifests/connector/namespace.yaml",
	"manifests/connector/service.yaml",
}

var controlPlaneFiles = []string{
	"manifests/connector/clusterrolebinding.yaml",
	"manifests/connector/serviceaccount.yaml",
	"manifests/connector/controlplane-config-secret.yaml",
	"manifests/connector/deployment.yaml",
}

//go:embed manifests
var manifestFiles embed.FS

type ControlPlaneConfig struct {
	ControlPlaneKubeConfig []byte
	OCMConfig              []byte
	Namespace              string
	Hostname               string
	XCMServer              string
	ServiceType            corev1.ServiceType
}

type EKSDeployer struct {
	kubeClient     kubernetes.Interface
	clusterClient  clusterclient.Interface
	config         *ControlPlaneConfig
	controlPlaneID string
}

func BuildEKSDeployer(kubeconfigPath, namespace, xcmServer string) (*EKSDeployer, error) {
	kubeConfig, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return nil, err
	}

	kubeClient, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		return nil, err
	}

	clusterClient, err := clusterclient.NewForConfig(kubeConfig)
	if err != nil {
		return nil, err
	}

	return &EKSDeployer{
		kubeClient:    kubeClient,
		clusterClient: clusterClient,
		config: &ControlPlaneConfig{
			Namespace:   namespace,
			XCMServer:   xcmServer,
			ServiceType: corev1.ServiceTypeLoadBalancer,
		},
	}, nil
}

func (d *EKSDeployer) Connect(ctx context.Context) error {
	fmt.Fprintln(os.Stdout, "Deploy the xCM connector [connector] ...")
	if err := d.ensureControlPlane(ctx); err != nil {
		return fmt.Errorf("failed to deploy connector: %v", err)
	}

	if err := configs.SaveControlPlaneKubeConfig(d.config.ControlPlaneKubeConfig); err != nil {
		return fmt.Errorf("failed to save control plane kubeconfig: %v", err)
	}

	fmt.Fprintln(os.Stdout, "Connect to xCM ...")
	id, err := managedcluster.CreateClusterClaim(ctx, d.clusterClient, &clusterv1alpha1.ClusterClaim{
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

	d.controlPlaneID = id

	// TODO: below claims should be detected automatically
	if _, err := managedcluster.CreateClusterClaim(ctx, d.clusterClient, &clusterv1alpha1.ClusterClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: "product.open-cluster-management.io",
		},
		Spec: clusterv1alpha1.ClusterClaimSpec{
			Value: "EKS",
		},
	}); err != nil {
		return fmt.Errorf("failed to create cluster claim: %v", err)
	}

	if _, err := managedcluster.CreateClusterClaim(ctx, d.clusterClient, &clusterv1alpha1.ClusterClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: "platform.open-cluster-management.io",
		},
		Spec: clusterv1alpha1.ClusterClaimSpec{
			Value: "AWS",
		},
	}); err != nil {
		return fmt.Errorf("failed to create cluster claim: %v", err)
	}

	if _, err := managedcluster.CreateClusterClaim(ctx, d.clusterClient, &clusterv1alpha1.ClusterClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: "region.open-cluster-management.io",
		},
		Spec: clusterv1alpha1.ClusterClaimSpec{
			Value: "us-east-2",
		},
	}); err != nil {
		return fmt.Errorf("failed to create cluster claim: %v", err)
	}

	return nil
}

func (d *EKSDeployer) ensureControlPlane(ctx context.Context) error {
	if err := d.ensureLoadBalancer(ctx); err != nil {
		return err
	}

	_, err := d.kubeClient.AppsV1().Deployments(d.config.Namespace).Get(ctx, constants.ControlPlaneName, metav1.GetOptions{})
	switch {
	case errors.IsNotFound(err):
		if err := d.deployControlPlane(ctx); err != nil {
			return err
		}
	case err != nil:
		return err
	}

	if err := wait.PollImmediate(1*time.Second, genericflags.TimeOut(), func() (bool, error) {
		adminSecret, err := d.kubeClient.CoreV1().Secrets(d.config.Namespace).Get(
			ctx, constants.ControlPlaneKubeconfigSecretName, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			return false, nil
		}
		if err != nil {
			return false, err
		}

		kubeconfigData, ok := adminSecret.Data["kubeconfig"]
		if !ok {
			return false, fmt.Errorf("the kubeconfig is not from the secret %s/%s",
				d.config.Namespace, constants.ControlPlaneKubeconfigSecretName)
		}

		d.config.ControlPlaneKubeConfig = kubeconfigData
		return true, nil
	}); err != nil {
		return fmt.Errorf("control plane is degraded,")
	}

	return nil
}

func (d *EKSDeployer) GetControlPlaneID() string {
	return d.controlPlaneID
}

// TODO put this in the relay command
// a2dfebcf572a44db3b12fa8480570b09-17958408.us-east-2.elb.amazonaws.com
func (d *EKSDeployer) ensureLoadBalancer(ctx context.Context) error {
	objects := []runtime.Object{}
	for _, file := range serviceFiles {
		template, err := manifestFiles.ReadFile(file)
		if err != nil {
			// this should not happen, if happened, panic here
			panic(err)
		}

		objects = append(objects, resource.MustCreateObjectFromTemplate(file, template, d.config))
	}

	return wait.PollImmediate(1*time.Second, genericflags.TimeOut(), func() (bool, error) {
		err := resource.ApplyResources(ctx, d.kubeClient, nil, nil, objects...)
		if errors.IsNotFound(err) {
			return false, nil
		}

		if err != nil {
			return false, err
		}

		svc, err := d.kubeClient.CoreV1().Services(d.config.Namespace).Get(
			ctx, constants.ControlPlaneName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		ingress := svc.Status.LoadBalancer.Ingress
		if len(ingress) == 0 {
			return false, nil
		}

		d.config.Hostname = ingress[0].Hostname
		return true, nil
	})
}

func (d *EKSDeployer) deployControlPlane(ctx context.Context) error {
	template, err := manifestFiles.ReadFile(ocmconfigfile)
	if err != nil {
		// this should not happen, if happened, panic here
		panic(err)
	}

	ocmconfig := resource.MustRenderFromTemplate(ocmconfigfile, template, d.config)
	d.config.OCMConfig = ocmconfig

	objects := []runtime.Object{}
	for _, file := range controlPlaneFiles {
		template, err := manifestFiles.ReadFile(file)
		if err != nil {
			// this should not happen, if happened, panic here
			panic(err)
		}

		objects = append(objects, resource.MustCreateObjectFromTemplate(file, template, d.config))
	}

	if err := wait.PollImmediate(1*time.Second, genericflags.TimeOut(), func() (done bool, err error) {
		applyErr := resource.ApplyResources(ctx, d.kubeClient, nil, nil, objects...)
		if errors.IsNotFound(applyErr) {
			return false, nil
		}

		if applyErr != nil {
			return false, applyErr
		}

		return true, nil
	}); err != nil {
		return err
	}

	// check deployment status
	return wait.Poll(1*time.Second, genericflags.TimeOut(), func() (done bool, err error) {
		deploy, err := d.kubeClient.AppsV1().Deployments(d.config.Namespace).Get(
			ctx, constants.ControlPlaneName, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			return false, nil
		}

		if err != nil {
			return false, err
		}

		if helpers.NumOfUnavailablePod(deploy) > 0 {
			return false, nil
		}

		return true, nil
	})
}
