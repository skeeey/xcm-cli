package helpers

import (
	"fmt"
	"net/url"

	appsv1 "k8s.io/api/apps/v1"
)

func NumOfUnavailablePod(deployment *appsv1.Deployment) int32 {
	desiredReplicas := int32(1)
	if deployment.Spec.Replicas != nil {
		desiredReplicas = *(deployment.Spec.Replicas)
	}

	if desiredReplicas <= deployment.Status.AvailableReplicas {
		return 0
	}

	return desiredReplicas - deployment.Status.AvailableReplicas
}

func ValidateURL(rawURL string) error {
	requestURL, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return err
	}

	if requestURL.Host == "" {
		return fmt.Errorf("an invalid url, host is mandatory")
	}

	return nil
}
