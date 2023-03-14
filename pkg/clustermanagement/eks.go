package clustermanagement

import (
	"context"
	"embed"
	"fmt"
	"os"
	"time"

	"github.com/skeeey/xcm-cli/pkg/cert"
	"github.com/skeeey/xcm-cli/pkg/configs"
	"github.com/skeeey/xcm-cli/pkg/constants"
	"github.com/skeeey/xcm-cli/pkg/genericflags"
	"github.com/skeeey/xcm-cli/pkg/helpers"
	"github.com/skeeey/xcm-cli/pkg/resource"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

var serviceFiles = []string{
	"manifests/connector/namespace.yaml",
	"manifests/connector/controlplane_service.yaml",
}

var controlPlaneFiles = []string{
	"manifests/connector/controlplane_clusterrolebinding.yaml",
	"manifests/connector/controlplane_serviceaccount.yaml",
	"manifests/connector/controlplane_admin_secret.yaml",
	"manifests/connector/controlplane_cert_secret.yaml",
	"manifests/connector/controlplane_deployment.yaml",
}

//go:embed manifests
var manifestFiles embed.FS

type ControlPlaneConfig struct {
	ServiceAccountKey []byte
	ClientCA          []byte
	ClientCAKey       []byte
	ServingCert       []byte
	ServingCertKey    []byte
	AdminKubeConfig   []byte
	Namespace         string
	HostName          string
	XCMServer         string
	ServiceType       corev1.ServiceType
}

type EKSDeployer struct {
	kubeClient     kubernetes.Interface
	config         *ControlPlaneConfig
	spokeDeployer  *SpokeDeployer
	kubeconfigPath string
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

	return &EKSDeployer{
		kubeClient: kubeClient,
		config: &ControlPlaneConfig{
			Namespace:   namespace,
			XCMServer:   xcmServer,
			ServiceType: corev1.ServiceTypeLoadBalancer,
		},
		kubeconfigPath: kubeconfigPath,
	}, nil
}

func (d *EKSDeployer) Connect(ctx context.Context) error {
	fmt.Fprintln(os.Stdout, "Deploy the xCM connector [connector] ...")
	if err := d.ensureControlPlane(ctx); err != nil {
		return fmt.Errorf("failed to deploy connector: %v", err)
	}

	if err := configs.SaveControlPlaneKubeConfig(d.config.AdminKubeConfig); err != nil {
		return fmt.Errorf("failed to save control plane kubeconfig: %v", err)
	}

	// import controll plane as a mananged cluster
	spokeDeployer, err := BuildSpokeDeployer(d.kubeconfigPath, true)
	if err != nil {
		return fmt.Errorf("failed to build spoke deployer with %q: %v", d.kubeconfigPath, err)
	}

	if err := spokeDeployer.Relay(ctx); err != nil {
		return err
	}

	d.spokeDeployer = spokeDeployer

	return nil
}

func (d *EKSDeployer) ensureControlPlane(ctx context.Context) error {
	if err := d.ensureLoadBalancer(ctx); err != nil {
		return err
	}

	deploy, err := d.kubeClient.AppsV1().Deployments(d.config.Namespace).Get(
		ctx, constants.ControlPlaneName, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		return d.deployControlPlane(ctx)
	}
	if err != nil {
		return err
	}

	// TODO do more check for control plane is deployed but not running
	if helpers.NumOfUnavailablePod(deploy) > 0 {
		return fmt.Errorf("control plane is degraded")
	}

	adminSecret, err := d.kubeClient.CoreV1().Secrets(d.config.Namespace).Get(
		ctx, constants.ControlPlaneAdminSecretName, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		return d.deployControlPlane(ctx)
	}
	if err != nil {
		return err
	}

	kubeconfigData, ok := adminSecret.Data["kubeconfig"]
	if !ok {
		return d.deployControlPlane(ctx)
	}

	d.config.AdminKubeConfig = kubeconfigData

	return nil
}

func (d *EKSDeployer) GetControlPlaneID() string {
	return d.spokeDeployer.GetClusterID()
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

		d.config.HostName = ingress[0].Hostname
		return true, nil
	})
}

func (d *EKSDeployer) deployControlPlane(ctx context.Context) error {
	if err := d.ensureControlPlaneConfigs(); err != nil {
		return err
	}

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
		if err != nil {
			return false, err
		}

		if helpers.NumOfUnavailablePod(deploy) > 0 {
			return false, nil
		}

		return true, nil
	})
}

func (d *EKSDeployer) ensureControlPlaneConfigs() error {
	apiServerCerts, err := cert.GenerateAPIServerCerts(d.config.HostName)
	if err != nil {
		return err
	}

	kubeConfig := configs.BuildControlPlaneKubeConfig(d.config.HostName, apiServerCerts)
	kubeConfigData, err := clientcmd.Write(kubeConfig)
	if err != nil {
		return err
	}

	d.config.ServiceAccountKey = apiServerCerts.ServiceAccountKey
	d.config.ClientCA = apiServerCerts.ClientCA
	d.config.ClientCAKey = apiServerCerts.ClientCAKey
	d.config.ServingCert = apiServerCerts.ServingCert
	d.config.ServingCertKey = apiServerCerts.ServingCertKey
	d.config.AdminKubeConfig = kubeConfigData

	// TODO save xcm tokens to secret

	return nil
}
