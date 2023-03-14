package managedcluster

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/skeeey/xcm-cli/pkg/genericflags"
	clusterclient "open-cluster-management.io/api/client/cluster/clientset/versioned"
	clusterv1 "open-cluster-management.io/api/cluster/v1"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

const ManagedClusterConditionConnected string = "ManagedClusterConditionConnected"

func CreateManagedCluster(ctx context.Context, clusterClient clusterclient.Interface,
	controlPlane bool, controlPlaneID, clusterName string) error {
	return wait.Poll(10*time.Second, genericflags.TimeOut(), func() (bool, error) {
		_, err := clusterClient.ClusterV1().ManagedClusters().Get(ctx, clusterName, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			if _, err := clusterClient.ClusterV1().ManagedClusters().Create(
				ctx,
				&clusterv1.ManagedCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: clusterName,
						Labels: map[string]string{
							"control-plane":    fmt.Sprintf("%t", controlPlane),
							"control-plane-id": controlPlaneID,
						},
					},
					Spec: clusterv1.ManagedClusterSpec{
						HubAcceptsClient: true,
					},
				},
				metav1.CreateOptions{},
			); err != nil {
				return false, nil
			}

			return true, nil
		}

		if err != nil {
			return false, nil
		}

		return true, nil
	})
}

func WaitManagedClusterConnected(ctx context.Context, clusterClient clusterclient.Interface, clusterName string) error {
	return wait.Poll(1*time.Second, genericflags.TimeOut(), func() (bool, error) {
		cluster, err := clusterClient.ClusterV1().ManagedClusters().Get(ctx, clusterName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		if meta.IsStatusConditionTrue(cluster.Status.Conditions, clusterv1.ManagedClusterConditionAvailable) &&
			meta.IsStatusConditionTrue(cluster.Status.Conditions, ManagedClusterConditionConnected) {
			return true, nil
		}

		return false, nil
	})
}

func GetClusterName(id string) string {
	return fmt.Sprintf("cluster-%s", id)
}

func GetClusterID() string {
	return uuid.New().String()
}
