package main

import (
	"context"
	"flag"
	"fmt"
	"path/filepath"
	"time"

	"github.com/gofrs/flock"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/pkg/apis/clientauthentication"
	clientauthv1beta1 "k8s.io/client-go/pkg/apis/clientauthentication/v1beta1"
	"k8s.io/klog/v2"
)

const (
	modeTPM      = "tpm"
	modeAltToken = "alt-token"
	modeVMToken  = "vm-token"
	flockName    = "kubelet-client.lock"
)

var (
	mode = flag.String("mode", modeTPM, "Plugin mode, one of ['tpm', 'alt-token', 'vm-token'].")

	// TPM flags.
	cacheDir = flag.String("cache-dir", "/var/lib/kubelet/pki", "Path to directory to store key and certificate.")
	tpmPath  = flag.String("tpm-path", "/dev/tpm0", "path to a TPM character device or socket.")

	// alt-token flags
	altTokenURL  = flag.String("alt-token-url", "", "URL to token endpoint.")
	altTokenBody = flag.String("alt-token-body", "", "Body of token request.")

	scheme       = runtime.NewScheme()
	codecs       = serializer.NewCodecFactory(scheme)
	groupVersion = schema.GroupVersion{
		Group:   "client.authentication.k8s.io",
		Version: "v1beta1",
	}
)

func init() {
	metav1.AddToGroupVersion(scheme, schema.GroupVersion{Version: "v1"})
	clientauthv1beta1.AddToScheme(scheme)
	clientauthentication.AddToScheme(scheme)
}

func main() {
	klog.InitFlags(nil)
	defer klog.Flush()
	flag.Set("logtostderr", "true")
	flag.Parse()

	var key, cert []byte
	var token string
	var expirationTimestamp time.Time
	var err error

	switch *mode {
	case modeTPM:
		// Lock around certificate reading and CSRs. Prevents parallel
		// invocations creating duplicate CSRs if there is no cert yet.
		fileLock := flock.New(filepath.Join(*cacheDir, flockName))
		if err = fileLock.Lock(); err != nil {
			klog.Exit(err)
		}
		defer fileLock.Unlock()

		key, cert, err = getKeyCert(*cacheDir, requestCertificate)
		if err != nil {
			klog.Exit(err)
		}
		// Use a one hour expiration so we get reinvoked at least once an hour, we'll cache the cert for longer.
		expirationTimestamp = time.Now().Add(responseExpiry)

	case modeAltToken:
		if *altTokenURL == "" {
			klog.Exit("--alt-token-url must be set")
		}
		if *altTokenBody == "" {
			klog.Exit("--alt-token-body must be set")
		}
		tok, err := newAltTokenSource(*altTokenURL, *altTokenBody).Token()
		if err != nil {
			klog.Exit(err)
		}
		token = tok.AccessToken
		if tok.Expiry.IsZero() {
			// Use a one hour expiration if the token didn't have an expiration
			expirationTimestamp = time.Now().Add(responseExpiry)
		} else {
			// Use the token expiration with a little leeway to get called before the actual expiration
			expirationTimestamp = tok.Expiry.Add(-time.Minute)
		}

	case modeVMToken:
		tok, err := getVMToken(context.Background())
		if err != nil {
			klog.Exit(err)
		}
		token = tok.AccessToken
		if tok.Expiry.IsZero() {
			// Use a one hour expiration if the token didn't have an expiration
			expirationTimestamp = time.Now().Add(responseExpiry)
		} else {
			// Use the token expiration with a little leeway to get called before the actual expiration
			expirationTimestamp = tok.Expiry.Add(-time.Minute)
		}
	default:
		klog.Exitf("unrecognized --mode value %q, want one of [%q, %q]", *mode, modeAltToken, modeTPM)
	}

	if err := writeResponse(token, key, cert, expirationTimestamp); err != nil {
		klog.Exit(err)
	}
}

func writeResponse(token string, key, cert []byte, expirationTimestamp time.Time) error {
	resp := &clientauthentication.ExecCredential{
		Status: &clientauthentication.ExecCredentialStatus{
			ExpirationTimestamp:   &metav1.Time{Time: expirationTimestamp},
			Token:                 token,
			ClientCertificateData: string(cert),
			ClientKeyData:         string(key),
		},
	}
	data, err := runtime.Encode(codecs.LegacyCodec(groupVersion), resp)
	if err != nil {
		return err
	}
	fmt.Print(string(data))
	return nil
}
