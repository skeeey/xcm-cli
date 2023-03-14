package resource

import (
	"context"

	ocmoperatorclient "open-cluster-management.io/api/client/operator/clientset/versioned"
	ocmoperatorv1 "open-cluster-management.io/api/operator/v1"

	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
	"github.com/skeeey/xcm-cli/pkg/recorder"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	crdv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/kubernetes"
)

// ApplyResources apply resources, includes:
// - serviceaccount
// - secret
// - deployment
// - clusterrole
// - clusterrolebinding,
// - crdv1
// - klusterlet
func ApplyResources(ctx context.Context,
	kubeClient kubernetes.Interface,
	apiExtensionsClient apiextensionsclient.Interface,
	operatorClient ocmoperatorclient.Interface,
	objs ...runtime.Object) error {
	logger := &recorder.DumbRecorder{}
	errs := []error{}
	for _, obj := range objs {
		switch required := obj.(type) {
		case *corev1.Service:
			_, _, err := resourceapply.ApplyService(ctx, kubeClient.CoreV1(), logger, required)
			errs = append(errs, err)
		case *corev1.ServiceAccount:
			_, _, err := resourceapply.ApplyServiceAccount(ctx, kubeClient.CoreV1(), logger, required)
			errs = append(errs, err)
		case *corev1.Secret:
			_, _, err := resourceapply.ApplySecret(ctx, kubeClient.CoreV1(), logger, required)
			errs = append(errs, err)
		case *corev1.Namespace:
			_, _, err := resourceapply.ApplyNamespace(ctx, kubeClient.CoreV1(), logger, required)
			errs = append(errs, err)
		case *appsv1.Deployment:
			errs = append(errs, applyDeployment(ctx, kubeClient, logger, required))
		case *rbacv1.ClusterRole:
			_, _, err := resourceapply.ApplyClusterRole(ctx, kubeClient.RbacV1(), logger, required)
			errs = append(errs, err)
		case *rbacv1.ClusterRoleBinding:
			_, _, err := resourceapply.ApplyClusterRoleBinding(ctx, kubeClient.RbacV1(), logger, required)
			errs = append(errs, err)
		case *crdv1.CustomResourceDefinition:
			_, _, err := resourceapply.ApplyCustomResourceDefinitionV1(
				ctx,
				apiExtensionsClient.ApiextensionsV1(),
				logger,
				required,
			)
			errs = append(errs, err)
		case *ocmoperatorv1.Klusterlet:
			errs = append(errs, applyKlusterlet(ctx, operatorClient, logger, required))
		}
	}

	return utilerrors.NewAggregate(errs)
}

func applyDeployment(ctx context.Context, kubeClient kubernetes.Interface, recorder events.Recorder,
	required *appsv1.Deployment) error {
	existing, err := kubeClient.AppsV1().Deployments(required.Namespace).Get(ctx, required.Name, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		_, _, err := resourceapply.ApplyDeployment(ctx, kubeClient.AppsV1(), recorder, required, -1)
		return err
	}
	if err != nil {
		return err
	}

	_, _, err = resourceapply.ApplyDeployment(ctx, kubeClient.AppsV1(), recorder, required, existing.Generation)
	return err
}

func applyKlusterlet(ctx context.Context,
	client ocmoperatorclient.Interface, recorder events.Recorder, required *ocmoperatorv1.Klusterlet) error {
	existing, err := client.OperatorV1().Klusterlets().Get(ctx, required.Name, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		if _, err := client.OperatorV1().Klusterlets().Create(ctx, required, metav1.CreateOptions{}); err != nil {
			return err
		}

		return nil
	}
	if err != nil {
		return err
	}

	if equality.Semantic.DeepEqual(existing.Spec, required.Spec) {
		return nil
	}

	existing = existing.DeepCopy()
	existing.Spec = required.Spec
	if _, err := client.OperatorV1().Klusterlets().Update(ctx, existing, metav1.UpdateOptions{}); err != nil {
		return err
	}

	return nil
}
