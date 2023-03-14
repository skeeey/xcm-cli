package rest

import (
	"testing"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
)

func TestXxx(t *testing.T) {

	if err := CreateCluster("http://3.137.154.243", &clusterv1.ManagedCluster{
		ObjectMeta: v1.ObjectMeta{
			Name: "spoke-test",
		},
	}); err != nil {
		t.Errorf("%v", err)
	}
}
