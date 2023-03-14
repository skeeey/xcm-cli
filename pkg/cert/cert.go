package cert

import (
	"bytes"
	cryptorand "crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"time"

	"k8s.io/client-go/util/keyutil"
)

const duration365d = time.Hour * 24 * 365

const certificateBlockType = "CERTIFICATE"

type APIServerCerts struct {
	// service account key
	ServiceAccountKey []byte

	// client certificates
	ClientCA      []byte
	ClientCAKey   []byte
	ClientCert    []byte
	ClientCertKey []byte

	// serving certificates
	ServingCA      []byte
	ServingCAKey   []byte
	ServingCert    []byte
	ServingCertKey []byte
}

func GenerateAPIServerCerts(apiHostName string) (*APIServerCerts, error) {
	apiServerCerts := &APIServerCerts{}

	// service account key
	serviceAccountKey, err := rsa.GenerateKey(cryptorand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("service account key failed to generate: %v", err)
	}

	serviceAccountKeyBytes, err := generateKey(serviceAccountKey)
	if err != nil {
		return nil, fmt.Errorf("service account key failed to generate: %v", err)
	}

	apiServerCerts.ServiceAccountKey = serviceAccountKeyBytes

	// client certificates
	clientCAKey, err := rsa.GenerateKey(cryptorand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("client ca key failed to generate: %v", err)
	}

	clientCA, clientCAKeyBytes, clinetCACert, err := newSelfSignedCACert("xCMClientCA", clientCAKey)
	if err != nil {
		return nil, fmt.Errorf("self signed client ca failed to generate: %v", err)
	}

	apiServerCerts.ClientCA = clientCA
	apiServerCerts.ClientCAKey = clientCAKeyBytes

	clientCert, clinetKey, err := generateSelfSignedCertKey(
		clinetCACert, clientCAKey,
		"system:admin", []string{"system:masters"},
		[]string{},
		x509.ExtKeyUsageClientAuth,
	)
	if err != nil {
		return nil, fmt.Errorf("self signed client cert key failed to generate: %v", err)
	}

	apiServerCerts.ClientCert = clientCert
	apiServerCerts.ClientCertKey = clinetKey

	// serving certificates
	servingCAKey, err := rsa.GenerateKey(cryptorand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("serving ca key failed to generate: %v", err)
	}

	servingCA, servingCAKeyBytes, servingCACert, err := newSelfSignedCACert("xCMServingCA", servingCAKey)
	if err != nil {
		return nil, fmt.Errorf("self signed serving ca failed to generate: %v", err)
	}

	apiServerCerts.ServingCA = servingCA
	apiServerCerts.ServingCAKey = servingCAKeyBytes

	servingCert, servingKey, err := generateSelfSignedCertKey(
		servingCACert, servingCAKey,
		"kubernetes.default", []string{""},
		[]string{
			"kubernetes.default.svc",
			"localhost",
			apiHostName,
		},
		x509.ExtKeyUsageServerAuth,
	)
	if err != nil {
		return nil, fmt.Errorf("self signed serving cert key failed to generate: %v", err)
	}

	apiServerCerts.ServingCert = servingCert
	apiServerCerts.ServingCertKey = servingKey

	return apiServerCerts, nil
}

func newSelfSignedCACert(commonName string, key *rsa.PrivateKey) ([]byte, []byte, *x509.Certificate, error) {
	now := time.Now()
	tmpl := x509.Certificate{
		SerialNumber: new(big.Int).SetInt64(0),
		Subject: pkix.Name{
			CommonName: commonName,
		},
		NotBefore:             now.UTC(),
		NotAfter:              now.Add(duration365d * 10).UTC(),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	clientCADERBytes, err := x509.CreateCertificate(cryptorand.Reader, &tmpl, &tmpl, key.Public(), key)
	if err != nil {
		return nil, nil, nil, err
	}

	clinetCACert, err := x509.ParseCertificate(clientCADERBytes)
	if err != nil {
		return nil, nil, nil, err
	}

	clientCA, clientCAKey, err := generateCertAndKey(clientCADERBytes, key)
	if err != nil {
		return nil, nil, nil, err
	}

	return clientCA, clientCAKey, clinetCACert, nil
}

func generateSelfSignedCertKey(caCertificate *x509.Certificate,
	caKey *rsa.PrivateKey,
	commonName string,
	organizations, alternateDNS []string,
	extKeyUsages ...x509.ExtKeyUsage) ([]byte, []byte, error) {
	validFrom := time.Now().Add(-time.Hour) // valid an hour earlier to avoid flakes due to clock skew
	maxAge := time.Hour * 24 * 365          // one year self-signed certs

	priv, err := rsa.GenerateKey(cryptorand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject: pkix.Name{
			CommonName:   commonName,
			Organization: organizations,
		},
		NotBefore: validFrom,
		NotAfter:  validFrom.Add(maxAge),

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           extKeyUsages,
		BasicConstraintsValid: true,
	}

	template.DNSNames = append(template.DNSNames, alternateDNS...)

	derBytes, err := x509.CreateCertificate(cryptorand.Reader, &template, caCertificate, &priv.PublicKey, caKey)
	if err != nil {
		return nil, nil, err
	}

	return generateCertAndKey(derBytes, priv)
}

func generateCertAndKey(derBytes []byte, key *rsa.PrivateKey) ([]byte, []byte, error) {
	// generate cert
	certBuffer := bytes.Buffer{}
	if err := pem.Encode(&certBuffer, &pem.Block{Type: certificateBlockType, Bytes: derBytes}); err != nil {
		return nil, nil, err
	}

	keyBytes, err := generateKey(key)
	if err != nil {
		return nil, nil, err
	}

	return certBuffer.Bytes(), keyBytes, nil
}

func generateKey(key *rsa.PrivateKey) ([]byte, error) {
	// generate key
	keyBuffer := bytes.Buffer{}
	if err := pem.Encode(&keyBuffer, &pem.Block{
		Type: keyutil.RSAPrivateKeyBlockType, Bytes: x509.MarshalPKCS1PrivateKey(key)}); err != nil {
		return nil, err
	}
	return keyBuffer.Bytes(), nil
}
