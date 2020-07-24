// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package cmd

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

const (
	createName             = "create"
	createShortDescription = "Create a new Kubernetes cluster"
	createLongDescription  = "Create a new Kubernetes cluster, enabled with cluster-api for cluster lifecycle management"
	calicoSpec             = "https://raw.githubusercontent.com/kubernetes-sigs/cluster-api-provider-azure/master/templates/addons/calico.yaml"
)

type createCmd struct {
	authProvider
	mgmtClusterKubeConfigPath string
	subscriptionID            string
	tenantID                  string
	clientID                  string
	clientSecret              string
	azureEnvironment          string
	clusterName               string
	vnetName                  string
	resourceGroup             string
	location                  string
	controlPlaneVMType        string
	nodeVMType                string
	sshPublicKey              string
	kubernetesVersion         string
	controlPlaneNodes         int
	nodes                     int
}

type clusterCtlConfigMap map[string]string

type AzureJSON struct {
	Cloud                        string `json:"cloud,omitempty"`
	TenantId                     string `json:"tenantId,omitempty"`
	SubscriptionId               string `json:"subscriptionId,omitempty"`
	AADClientId                  string `json:"aadClientId,omitempty"`
	AADClientSecret              string `json:"aadClientSecret,omitempty"`
	ResourceGroup                string `json:"resourceGroup,omitempty"`
	SecurityGroupName            string `json:"securityGroupName,omitempty"`
	Location                     string `json:"location,omitempty"`
	VMType                       string `json:"vmType,omitempty"`
	VNETName                     string `json:"vnetName,omitempty"`
	VNETResourceGroup            string `json:"vnetResourceGroup,omitempty"`
	SubnetName                   string `json:"subnetName,omitempty"`
	RouteTableName               string `json:"routeTableName,omitempty"`
	LoadBalancerSku              string `json:"loadBalancerSku,omitempty"`
	MaximumLoadBalancerRuleCount int    `json:"maximumLoadBalancerRuleCount,omitempty"`
	UseManagedIdentityExtension  bool   `json:"useManagedIdentityExtension,omitempty"`
	UseInstanceMetadata          bool   `json:"useInstanceMetadata,omitempty"`
}

func newCreateCmd() *cobra.Command {
	cc := createCmd{}

	createCmd := &cobra.Command{
		Use:   createName,
		Short: createShortDescription,
		Long:  createLongDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cc.run()
		},
	}

	f := createCmd.Flags()
	f.StringVarP(&cc.mgmtClusterKubeConfigPath, "mgmt-cluster-kubeconfig", "m", "", "path to the kubeconfig of your cluster-api management cluster")
	f.StringVarP(&cc.subscriptionID, "subscription-id", "s", "", "Azure Active Directory service principal password")
	f.StringVarP(&cc.tenantID, "tenant", "", "", "Azure Active Directory tenant, must provide when using service principals")
	f.StringVarP(&cc.clientID, "client-id", "", "", "Azure Active Directory service principal ID")
	f.StringVarP(&cc.clientSecret, "client-secret", "", "", "Azure Active Directory service principal password")
	f.StringVar(&cc.azureEnvironment, "azure-env", "AzurePublicCloud", "the target Azure cloud")
	f.StringVarP(&cc.clusterName, "cluster-name", "n", "", "name of cluster")
	f.StringVarP(&cc.vnetName, "vnet-name", "", "", "name of vnet the cluster will reside in")
	f.StringVarP(&cc.resourceGroup, "resource-group", "g", "", "name of resource group the cluster IaaS will reside in")
	f.StringVarP(&cc.location, "location", "l", "", "Azure region cluster IaaS will reside in")
	f.StringVarP(&cc.controlPlaneVMType, "control-plan-vm-sku", "", "Standard_D2s_v3", "SKU for control plane VMs, default is Standard_D2s_v3")
	f.StringVarP(&cc.nodeVMType, "node-vm-sku", "", "Standard_D2s_v3", "SKU for node VMs, default is Standard_D2s_v3")
	f.StringVarP(&cc.sshPublicKey, "ssh-public-key", "", "", "SSH public key to install for remote access to VMs")
	f.StringVarP(&cc.kubernetesVersion, "kubernetes-version", "v", "1.17.8", "Kubernetes version to install, default is 1.17.8")
	f.IntVarP(&cc.controlPlaneNodes, "control-plane-nodes", "", 3, "number of control plane nodes, default is 3")
	f.IntVarP(&cc.nodes, "nodes", "", 3, "number of worker nodes, default is 3")

	return createCmd
}

func (cc *createCmd) run() error {
	azureJSON := AzureJSON{
		Cloud:                        cc.azureEnvironment,
		TenantId:                     cc.tenantID,
		SubscriptionId:               cc.subscriptionID,
		AADClientId:                  cc.clientID,
		AADClientSecret:              cc.clientSecret,
		ResourceGroup:                cc.resourceGroup,
		SecurityGroupName:            fmt.Sprintf("%s-node-nsg", cc.clusterName),
		Location:                     cc.location,
		VMType:                       "vmss",
		VNETName:                     cc.vnetName,
		VNETResourceGroup:            cc.resourceGroup,
		SubnetName:                   fmt.Sprintf("%s-node-subnet", cc.clusterName),
		RouteTableName:               fmt.Sprintf("%s-node-routetable", cc.clusterName),
		LoadBalancerSku:              "standard",
		MaximumLoadBalancerRuleCount: 250,
		UseManagedIdentityExtension:  false,
		UseInstanceMetadata:          true,
	}
	b, err := json.MarshalIndent(azureJSON, "", "    ")
	if err != nil {
		log.Printf("Unable to generate azure.JSON config: %s\n", err)
	}

	clusterCtlConfig := clusterCtlConfigMap{
		"AZURE_SUBSCRIPTION_ID":            cc.subscriptionID,
		"AZURE_SUBSCRIPTION_ID_B64":        base64.StdEncoding.EncodeToString([]byte(cc.subscriptionID)),
		"AZURE_TENANT_ID":                  cc.tenantID,
		"AZURE_TENANT_ID_B64":              base64.StdEncoding.EncodeToString([]byte(cc.tenantID)),
		"AZURE_CLIENT_ID":                  cc.clientID,
		"AZURE_CLIENT_ID_B64":              base64.StdEncoding.EncodeToString([]byte(cc.clientID)),
		"AZURE_CLIENT_SECRET":              cc.clientSecret,
		"AZURE_CLIENT_SECRET_B64":          base64.StdEncoding.EncodeToString([]byte(cc.clientSecret)),
		"AZURE_ENVIRONMENT":                cc.azureEnvironment,
		"KUBECONFIG":                       cc.mgmtClusterKubeConfigPath,
		"CLUSTER_NAME":                     cc.clusterName,
		"AZURE_VNET_NAME":                  cc.vnetName,
		"AZURE_RESOURCE_GROUP":             cc.resourceGroup,
		"AZURE_LOCATION":                   cc.location,
		"AZURE_CONTROL_PLANE_MACHINE_TYPE": cc.controlPlaneVMType,
		"AZURE_NODE_MACHINE_TYPE":          cc.nodeVMType,
		"AZURE_SSH_PUBLIC_KEY":             cc.sshPublicKey,
		"AZURE_JSON_B64":                   base64.StdEncoding.EncodeToString(b),
	}
	for k, v := range clusterCtlConfig {
		err := os.Setenv(k, v)
		if err != nil {
			return errors.Wrap(err, fmt.Sprintf("setting %s env var", k))
		}
	}
	cmd := exec.Command("clusterctl", "init", "--infrastructure", "azure")
	out, err := cmd.CombinedOutput()
	if err != nil && !strings.Contains(string(out), "there is already an instance of the \"infrastructure-azure\" provider installed in the \"capz-system\" namespace") {
		log.Printf("%\n", string(out))
		log.Printf("Unable to initialize management cluster for Azure: %s\n", err)
		return err
	}
	cmd = exec.Command("clusterctl", "config", "cluster", "--infrastructure", "azure", cc.clusterName, "--kubernetes-version", fmt.Sprintf("v%s", cc.kubernetesVersion), "--control-plane-machine-count", strconv.Itoa(cc.controlPlaneNodes), "--worker-machine-count", strconv.Itoa(cc.nodes))
	out, err = cmd.CombinedOutput()
	if err != nil {
		log.Printf("%\n", string(out))
		log.Printf("Unable to generate cluster config: %s\n", err)
		return err
	}
	clusterConfigYaml := fmt.Sprintf("%s.yaml", cc.clusterName)
	f, err := os.Create(clusterConfigYaml)
	if err != nil {
		log.Printf("Unable to create and open cluster config file for writing: %s\n", err)
		return err
	}
	defer func() {
		if err := f.Close(); err != nil {
			panic(err)
		}
	}()
	if _, err := f.Write(out); err != nil {
		panic(err)
	}
	cmd = exec.Command("kubectl", "apply", "-f", fmt.Sprintf("./%s", clusterConfigYaml))
	out, err = cmd.CombinedOutput()
	if err != nil {
		log.Printf("%\n", string(out))
		log.Printf("Unable to apply cluster config to cluster-api management cluster: %s\n", err)
		return err
	}
	secret, err := cc.getClusterKubeConfigWithRetry(30*time.Second, 20*time.Minute)
	if err != nil {
		log.Printf("Unable to get cluster %s kubeconfig from cluster-api management cluster: %s\n", cc.clusterName, err)
		return err
	}
	decodedBytes, err := base64.StdEncoding.DecodeString(secret)
	if err != nil {
		log.Printf("Unable to decode cluster %s kubeconfig: %s\n", cc.clusterName, err)
	}
	h, err := os.UserHomeDir()
	if err != nil {
		log.Printf("Unable to get home dir: %s\n", err)
		return err
	}
	newClusterKubeConfig := fmt.Sprintf("%s/.kube/%s.kubeconfig", h, cc.clusterName)
	f2, err := os.Create(newClusterKubeConfig)
	if err != nil {
		log.Printf("Unable to create and open kubeconfig file for writing: %s\n", err)
		return err
	}
	defer func() {
		if err := f2.Close(); err != nil {
			panic(err)
		}
	}()
	if _, err := f2.Write(decodedBytes); err != nil {
		panic(err)
	}
	// TODO insert appropriate wait conditions before applying CNI spec
	cmd = exec.Command("kubectl", "apply", "-f", calicoSpec, "--kubeconfig", newClusterKubeConfig)
	out, err = cmd.CombinedOutput()
	if err != nil {
		log.Printf("%\n", string(out))
		log.Printf("Unable to apply cluster config to cluster-api management cluster: %s\n", err)
		return err
	}

	return nil
}

// getClusterKubeConfigSecretResult is the result type for GetAllByPrefixAsync
type getClusterKubeConfigSecretResult struct {
	secret string
	err    error
}

func (cc *createCmd) getClusterKubeConfig() getClusterKubeConfigSecretResult {
	cmd := exec.Command("kubectl", "get", fmt.Sprintf("secret/%s-kubeconfig", cc.clusterName), "-o", "jsonpath={.data.value}")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return getClusterKubeConfigSecretResult{
			secret: "",
			err:    err,
		}
	}
	return getClusterKubeConfigSecretResult{
		secret: string(out),
		err:    nil,
	}
}

// getClusterKubeConfigWithRetry will return the cluster kubeconfig, retrying up to a timeout
func (cc *createCmd) getClusterKubeConfigWithRetry(sleep, timeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	ch := make(chan getClusterKubeConfigSecretResult)
	var mostRecentGetClusterKubeConfigWithRetryError error
	var secret string
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				ch <- cc.getClusterKubeConfig()
				time.Sleep(sleep)
			}
		}
	}()
	for {
		select {
		case result := <-ch:
			mostRecentGetClusterKubeConfigWithRetryError = result.err
			secret = result.secret
			if mostRecentGetClusterKubeConfigWithRetryError == nil {
				return secret, nil
			}
		case <-ctx.Done():
			return secret, errors.Errorf("getClusterKubeConfigWithRetry timed out: %s\n", mostRecentGetClusterKubeConfigWithRetryError)
		}
	}
}
