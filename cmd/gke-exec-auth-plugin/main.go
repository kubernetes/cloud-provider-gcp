package main

import (
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
	clientauthv1alpha1 "k8s.io/client-go/pkg/apis/clientauthentication/v1alpha1"
	"k8s.io/klog"
)

const (
	modeTPM      = "tpm"
	modeAltToken = "alt-token"
	flockName    = "kubelet-client.lock"
)

var (
	mode = flag.String("mode", modeTPM, "Plugin mode, one of ['tpm', 'alt-token'].")
	// TPM flags.
	cacheDir = flag.String("cache-dir", "/var/lib/kubelet/pki", "Path to directory to store key and certificate.")
	tpmPath  = flag.String("tpm-path", "/dev/tpm0", "path to a TPM character device or socket.")

	altTokenURL  = flag.String("alt-token-url", "", "URL to token endpoint.")
	altTokenBody = flag.String("alt-token-body", "", "Body of token request.")

	scheme       = runtime.NewScheme()
	codecs       = serializer.NewCodecFactory(scheme)
	groupVersion = schema.GroupVersion{
		Group:   "client.authentication.k8s.io",
		Version: "v1alpha1",
	}
)

func init() {
	metav1.AddToGroupVersion(scheme, schema.GroupVersion{Version: "v1"})
	clientauthv1alpha1.AddToScheme(scheme)
	clientauthentication.AddToScheme(scheme)
}

func main() {
	klog.InitFlags(nil)
	defer klog.Flush()
	flag.Set("logtostderr", "true")
	flag.Parse()

	var key, cert []byte
	var token string
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
	default:
		klog.Exitf("unrecognized --mode value %q, want one of [%q, %q]", *mode, modeAltToken, modeTPM)
	}

	if err := writeResponse(token, key, cert); err != nil {
		klog.Exit(err)
	}
}

func writeResponse(token string, key, cert []byte) error {
	resp := &clientauthentication.ExecCredential{
		Status: &clientauthentication.ExecCredentialStatus{
			// Make Kubelet poke us every hour, we'll cache the cert for longer.
			ExpirationTimestamp:   &metav1.Time{time.Now().Add(responseExpiry)},
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
