package control

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"os"
	"path/filepath"
	"time"
)

type Authority struct {
	Cert *x509.Certificate
	Key  *ecdsa.PrivateKey
	PEM  []byte
}

func LoadAuthority(dir string) (*Authority, error) {
	if dir == "" {
		return nil, errors.New("CA directory required")
	}
	if e := os.MkdirAll(dir, 0700); e != nil {
		return nil, e
	}
	certPath, keyPath := filepath.Join(dir, "node-ca.crt"), filepath.Join(dir, "node-ca.key")
	certBytes, e := os.ReadFile(certPath)
	if e == nil {
		keyBytes, e := os.ReadFile(keyPath)
		if e != nil {
			return nil, e
		}
		cb, _ := pem.Decode(certBytes)
		kb, _ := pem.Decode(keyBytes)
		if cb == nil || kb == nil {
			return nil, errors.New("invalid CA files")
		}
		cert, e := x509.ParseCertificate(cb.Bytes)
		if e != nil {
			return nil, e
		}
		key, e := x509.ParseECPrivateKey(kb.Bytes)
		if e != nil {
			return nil, e
		}
		return &Authority{cert, key, certBytes}, nil
	}
	if !os.IsNotExist(e) {
		return nil, e
	}
	if _, e := os.Stat(keyPath); e == nil {
		return nil, errors.New("CA key exists without certificate; restore the certificate instead of rotating silently")
	}
	key, e := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if e != nil {
		return nil, e
	}
	serial, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	template := &x509.Certificate{SerialNumber: serial, Subject: pkix.Name{CommonName: "FlowLight Node Authority"}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().AddDate(5, 0, 0), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageCRLSign}
	der, e := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if e != nil {
		return nil, e
	}
	keyDER, _ := x509.MarshalECPrivateKey(key)
	certBytes = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	if e = os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}), 0600); e != nil {
		return nil, e
	}
	if e = os.WriteFile(certPath, certBytes, 0644); e != nil {
		return nil, e
	}
	cert, _ := x509.ParseCertificate(der)
	return &Authority{cert, key, certBytes}, nil
}
func (a *Authority) Issue(csrPEM, nodeID string) (string, string, error) {
	p, _ := pem.Decode([]byte(csrPEM))
	if p == nil {
		return "", "", errors.New("CSR required")
	}
	csr, e := x509.ParseCertificateRequest(p.Bytes)
	if e != nil || csr.CheckSignature() != nil {
		return "", "", errors.New("invalid CSR")
	}
	key, ok := csr.PublicKey.(*ecdsa.PublicKey)
	if !ok || key.Curve != elliptic.P256() {
		return "", "", errors.New("P-256 node key required")
	}
	serial, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	template := &x509.Certificate{SerialNumber: serial, Subject: pkix.Name{CommonName: nodeID}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().AddDate(0, 3, 0), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}}
	der, e := x509.CreateCertificate(rand.Reader, template, a.Cert, csr.PublicKey, a.Key)
	if e != nil {
		return "", "", e
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})), hash(string(der)), nil
}
