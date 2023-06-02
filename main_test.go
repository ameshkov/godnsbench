package main

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"testing"
	"time"

	"github.com/AdguardTeam/dnsproxy/proxy"
	"github.com/AdguardTeam/dnsproxy/upstream"
	"github.com/AdguardTeam/golibs/testutil"
	"github.com/miekg/dns"
	"github.com/stretchr/testify/require"
)

func Test_run(t *testing.T) {
	tlsConfig, _ := createServerTLSConfig(t, "example.org")
	p := createTestProxy(t, tlsConfig)

	p.RequestHandler = func(p *proxy.Proxy, d *proxy.DNSContext) (err error) {
		resp := &dns.Msg{}
		resp.SetReply(d.Req)
		d.Res = resp

		return nil
	}

	err := p.Start()
	require.NoError(t, err)
	testutil.CleanupAndRequireSuccess(t, p.Stop)

	addr := p.Addr(proxy.ProtoHTTPS)
	serverAddress := fmt.Sprintf("https://%s/dns-query", addr)

	o := &Options{
		Address:            serverAddress,
		Connections:        1,
		Query:              "example.org",
		Timeout:            10,
		Rate:               500,
		QueriesCount:       1000,
		InsecureSkipVerify: true,
	}

	state := run(o)

	require.Equal(t, o.QueriesCount, state.processed)
	require.Equal(t, 0, state.errors)
}

// createTestProxy creates a test DNS proxy that listens to all protocols.
func createTestProxy(t *testing.T, tlsConfig *tls.Config) (p *proxy.Proxy) {
	listenIP := "127.0.0.1"
	p = &proxy.Proxy{}

	if tlsConfig != nil {
		p.TLSListenAddr = []*net.TCPAddr{
			{Port: 0, IP: net.ParseIP(listenIP)},
		}
		p.HTTPSListenAddr = []*net.TCPAddr{
			{Port: 0, IP: net.ParseIP(listenIP)},
		}
		p.QUICListenAddr = []*net.UDPAddr{
			{Port: 0, IP: net.ParseIP(listenIP)},
		}
		p.TLSConfig = tlsConfig
	} else {
		p.UDPListenAddr = []*net.UDPAddr{
			{Port: 0, IP: net.ParseIP(listenIP)},
		}
		p.TCPListenAddr = []*net.TCPAddr{
			{Port: 0, IP: net.ParseIP(listenIP)},
		}
	}

	// Setting a local upstream, it won't be used anyway since RequestHandler
	// will handler all requests.
	p.UpstreamConfig = &proxy.UpstreamConfig{}
	dnsUpstream, err := upstream.AddressToUpstream(
		"127.0.0.1:12312",
		&upstream.Options{},
	)
	require.NoError(t, err)
	p.UpstreamConfig.Upstreams = append(p.UpstreamConfig.Upstreams, dnsUpstream)

	return p
}

// createServerTLSConfig creates a TLS configuration to be used by the server.
func createServerTLSConfig(
	t *testing.T,
	tlsServerName string,
) (tlsConfig *tls.Config, certPem []byte) {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	require.NoError(t, err)

	notBefore := time.Now()
	notAfter := notBefore.Add(5 * 365 * time.Hour * 24)

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"AdGuard Tests"},
		},
		NotBefore: notBefore,
		NotAfter:  notAfter,

		KeyUsage: x509.KeyUsageKeyEncipherment |
			x509.KeyUsageDigitalSignature |
			x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	template.DNSNames = append(template.DNSNames, tlsServerName)

	derBytes, err := x509.CreateCertificate(
		rand.Reader,
		&template,
		&template,
		publicKey(privateKey),
		privateKey,
	)
	require.NoError(t, err)

	certPem = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	keyPem := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)})

	cert, err := tls.X509KeyPair(certPem, keyPem)
	require.NoError(t, err)

	tlsConfig = &tls.Config{Certificates: []tls.Certificate{cert}, ServerName: tlsServerName}

	return tlsConfig, certPem
}

// publicKey returns a public key extracted from the specified private key.
func publicKey(priv any) (pk any) {
	switch k := priv.(type) {
	case *rsa.PrivateKey:
		return &k.PublicKey
	case *ecdsa.PrivateKey:
		return &k.PublicKey
	default:
		return nil
	}
}
