package proxy

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"log/slog"
	"math/big"
	"os"
	"time"
)

func GenerateSslCerts(caOrganisation string) (tls.Certificate, error) {
	// Generate the root certificate and key
	rootCertTemplate, rootKey, err := generateSelfSignedRootCertificate(caOrganisation)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("error generating root certificate: %w", err)
	}
	slog.Info("Root certificate and private key generated successfully.")

	// Generate the server certificate signed by the root
	serverCertDER, serverKey, err := generateServerCertificate(rootCertTemplate, rootKey, caOrganisation)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("error generating server certificate: %w", err)
	}

	slog.Info("Server certificate and private key generated successfully.")

	return tls.Certificate{
		Certificate: [][]byte{serverCertDER},
		PrivateKey:  serverKey,
	}, nil
}

func generateSelfSignedRootCertificate(caOrganisation string) (*x509.Certificate, *rsa.PrivateKey, error) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}

	notBefore := time.Now()
	notAfter := notBefore.Add(365 * 24 * time.Hour) // Valid for one year

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, nil, err
	}

	template := x509.Certificate{
		SerialNumber:          serialNumber,
		Subject:               pkix.Name{Organization: []string{caOrganisation}},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	return &template, priv, nil
}

func generateServerCertificate(rootCert *x509.Certificate, rootKey *rsa.PrivateKey, caOrganisation string) ([]byte, *rsa.PrivateKey, error) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}

	notBefore := time.Now()
	notAfter := notBefore.Add(365 * 24 * time.Hour) // Valid for one year

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, nil, err
	}

	template := x509.Certificate{
		SerialNumber:          serialNumber,
		Subject:               pkix.Name{Organization: []string{caOrganisation}},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, rootCert, &priv.PublicKey, rootKey)
	if err != nil {
		return nil, nil, err
	}

	return certDER, priv, nil
}

func saveCertificateToFile(certBytes []byte, filename string) (filepath string, err error) {
	tempFile, err := os.CreateTemp("", filename)
	if err != nil {
		return "", err
	}
	defer tempFile.Close()

	err = pem.Encode(tempFile, &pem.Block{Type: "CERTIFICATE", Bytes: certBytes})
	if err != nil {
		return "", err
	}

	return tempFile.Name(), nil
}

func savePrivateKeyToFile(key *rsa.PrivateKey, filename string) (filepath string, err error) {
	tempFile, err := os.CreateTemp("", filename)
	if err != nil {
		return "", err
	}
	defer tempFile.Close()

	keyBytes := x509.MarshalPKCS1PrivateKey(key)
	err = pem.Encode(tempFile, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: keyBytes})
	if err != nil {
		return "", err
	}

	return tempFile.Name(), nil
}
