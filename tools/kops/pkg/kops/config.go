package kops

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Config holds all parameters for kOps cluster lifecycle.
type Config struct {
	ClusterName              string
	GCPProject               string
	GCPLocation              string
	GCPZones                 string
	K8sVersion               string
	KopsBin                  string
	KopsBaseURL              string
	StateStore               string
	TemplateSrc              string
	TemplatePath             string
	AdminAccess              string
	ControlPlaneMachineType  string
	NodeMachineType          string
	NodeCount                int
	SSHPrivateKey            string
	SSHPublicKey             string // Derived internally
	KopsFeatureFlags         string
	GoogleAppCredentials     string
	NewCCMSpec               string
	ImageRepo                string
	ImageTag                 string
	ValidationWait           time.Duration
}

// NewConfigFromEnv initializes Config with values from environment variables,
// matching the logic in test/kops.sh.
func NewConfigFromEnv() (*Config, error) {
	repoRoot, err := getRepoRoot()
	if err != nil {
		return nil, err
	}

	clusterName := os.Getenv("CLUSTER_NAME")
	if clusterName == "" {
		clusterName = "kops.k8s.local"
	}

	workDir := os.Getenv("WORKDIR")
	if workDir == "" {
		// Matching workspace determination in kops.sh
		workspace := filepath.Dir(repoRoot)
		workDir = filepath.Join(workspace, "clusters", clusterName)
	}

	gcpLocation := getEnvDefault("GCP_LOCATION", "us-central1")
	gcpZones := getEnvDefault("GCP_ZONES", fmt.Sprintf("%s-b", gcpLocation))

	nodeCount, _ := strconv.Atoi(getEnvDefault("NODE_COUNT", "2"))

	sshPrivateKey := os.Getenv("SSH_PRIVATE_KEY")
	if sshPrivateKey == "" {
		sshPrivateKey = filepath.Join(workDir, "google_compute_engine")
	}

	k8sVersion := os.Getenv("K8S_VERSION")
	if k8sVersion == "" {
		k8sVersion, _ = getVersionFromFile(repoRoot)
	}
	if k8sVersion == "" {
		k8sVersion, err = getLatestK8sVersion()
		if err != nil {
			fmt.Printf("Warning: failed to fetch latest k8s version: %v. Falling back to empty.\n", err)
		}
	}

	kopsBaseURL := os.Getenv("KOPS_BASE_URL")
	if kopsBaseURL == "" {
		kopsBaseURL, err = getLatestKopsBaseURL()
		if err != nil {
			fmt.Printf("Warning: failed to fetch latest kOps base URL: %v\n", err)
		}
	}

	imageRepo := os.Getenv("IMAGE_REPO")
	imageTag := os.Getenv("IMAGE_TAG")

	validationWait, _ := time.ParseDuration(getEnvDefault("VALIDATION_WAIT", "20m"))

	return &Config{
		ClusterName:             clusterName,
		GCPProject:              os.Getenv("GCP_PROJECT"),
		GCPLocation:             gcpLocation,
		GCPZones:                gcpZones,
		K8sVersion:              k8sVersion,
		KopsBin:                 getEnvDefault("KOPS_BIN", filepath.Join(filepath.Dir(repoRoot), "bin", "kops")),
		KopsBaseURL:             kopsBaseURL,
		StateStore:              os.Getenv("KOPS_STATE_STORE"),
		TemplateSrc:             getEnvDefault("KOPS_TEMPLATE_SRC", filepath.Join(repoRoot, "test", "kops-cluster.yaml.template")),
		TemplatePath:            getEnvDefault("KOPS_TEMPLATE", filepath.Join(workDir, "kops-cluster.yaml")),
		AdminAccess:             getEnvDefault("ADMIN_ACCESS", "0.0.0.0/0"),
		ControlPlaneMachineType: getEnvDefault("CONTROL_PLANE_MACHINE_TYPE", "e2-standard-2"),
		NodeMachineType:         getEnvDefault("NODE_MACHINE_TYPE", "e2-standard-2"),
		NodeCount:               nodeCount,
		SSHPrivateKey:           sshPrivateKey,
		SSHPublicKey:            sshPrivateKey + ".pub",
		KopsFeatureFlags:        os.Getenv("KOPS_FEATURE_FLAGS"),
		GoogleAppCredentials:    os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"),
		NewCCMSpec:              os.Getenv("NEW_CCM_SPEC"),
		ImageRepo:               imageRepo,
		ImageTag:                imageTag,
		ValidationWait:          validationWait,
	}, nil
}

func getEnvDefault(key, defaultValue string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultValue
}

func getRepoRoot() (string, error) {
	pwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(pwd, ".git")); err == nil {
			return pwd, nil
		}
		parent := filepath.Dir(pwd)
		if parent == pwd {
			return "", fmt.Errorf("could not find repo root")
		}
		pwd = parent
	}
}

func getVersionFromFile(repoRoot string) (string, error) {
	path := filepath.Join(repoRoot, "ginko-test-package-version.env")
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func getLatestK8sVersion() (string, error) {
	resp, err := http.Get("https://dl.k8s.io/release/stable.txt")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(body)), nil
}

func getLatestKopsBaseURL() (string, error) {
	resp, err := http.Get("https://storage.googleapis.com/k8s-staging-kops/kops/releases/markers/master/latest-ci-updown-green.txt")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(body)), nil
}
