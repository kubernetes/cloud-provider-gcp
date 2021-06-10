package main

import (
	"encoding/json"
	"fmt"

	"k8s.io/cloud-provider-gcp/cmd/kubectl-gke-exec-auth-plugin/provider"
)

func main() {
	ec, err := provider.ExecCredential()
	if err != nil {
		msg := fmt.Errorf("unable to retrieve access token for GKE. Error : %v", err)
		panic(msg)
	}
	fmt.Printf("%s", formatToJSON(ec))
}

func formatToJSON(i interface{}) string {
	s, _ := json.MarshalIndent(i, "", "    ")
	return string(s)
}
