package configs

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/skeeey/xcm-cli/pkg/cert"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

func BuildControlPlaneKubeConfig(host string, certs *cert.APIServerCerts) clientcmdapi.Config {
	config := clientcmdapi.Config{
		Clusters: map[string]*clientcmdapi.Cluster{"default-cluster": {
			Server:                   fmt.Sprintf("https://%s", host),
			CertificateAuthorityData: certs.ServingCA,
		}},
		AuthInfos: map[string]*clientcmdapi.AuthInfo{"default-auth": {
			ClientCertificateData: certs.ClientCert,
			ClientKeyData:         certs.ClientCertKey,
		}},
		Contexts: map[string]*clientcmdapi.Context{"default-context": {
			Cluster:   "default-cluster",
			AuthInfo:  "default-auth",
			Namespace: "configuration",
		}},
		CurrentContext: "default-context",
	}

	return config
}

func SaveControlPlaneKubeConfig(kubeconfig []byte) error {
	configDir, err := ConfigDir()
	if err != nil {
		return err
	}

	fileName := filepath.Join(configDir, "controlplane-admin.kubeconfig")
	// TODO read the file and compare content
	return os.WriteFile(fileName, kubeconfig, 0600)
}
