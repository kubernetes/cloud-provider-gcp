package main

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/pflag"
	"k8s.io/cloud-provider-gcp/cmd/kubectl-gke-exec-auth-plugin/provider"
	"k8s.io/component-base/version/verflag"
)

func main() {
	pflag.Parse()
	verflag.PrintAndExitIfRequested()

	ec, err := provider.ExecCredential()
	if err != nil {
		msg := fmt.Errorf("unable to retrieve access token for GKE. Error : %v\n", err)
		panic(msg)
	}

	ecStr, err := formatToJSON(ec)
	if err != nil {
		msg := fmt.Errorf("unable to convert ExecCredential object to json format. Error :%v\n", err)
		panic(msg)
	}
	fmt.Printf("%s", ecStr)
}

func formatToJSON(i interface{}) (string, error) {
	s, err := json.MarshalIndent(i, "", "    ")
	if err != nil {
		return "", err
	}
	return string(s), nil
}
