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

	"github.com/briandowns/spinner"
	"github.com/fatih/color"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	createName             = "create"
	createShortDescription = "Create a new Kubernetes cluster"
	createLongDescription  = "Create a new Kubernetes cluster, enabled with cluster-api for cluster lifecycle management"
	calicoSpec             = "https://raw.githubusercontent.com/kubernetes-sigs/cluster-api-provider-azure/master/templates/addons/calico.yaml"
)

const (
	DefaultAzureEnvironment   string = "AzurePublicCloud"
	DefaultLocation           string = "westus2"
	DefaultControlPlaneVMType string = "Standard_D2s_v3"
	DefaultNodeVMType         string = "Standard_D2s_v3"
	DefaultKubernetesVersion  string = "1.17.8"
	DefaultControlPlaneNodes  int    = 1
	DefaultNodes              int    = 1
)

type CreateCmd struct {
	MgmtClusterKubeConfigPath  string
	MgmtClusterName            string
	MgmtClusterURL             string
	SubscriptionID             string
	TenantID                   string
	ClientID                   string
	ClientSecret               string
	AzureEnvironment           string
	ClusterName                string
	VnetName                   string
	ResourceGroup              string
	Location                   string
	ControlPlaneVMType         string
	NodeVMType                 string
	SSHPublicKey               string
	KubernetesVersion          string
	ControlPlaneNodes          int
	Nodes                      int
	NewClusterKubeConfigPath   string
	GetNewClusterConfigCmdArgs []string
	IsMgmtClusterReadyCmdArgs  []string
	IsClusterReadyCmdArgs      []string
	NeedsClusterAPIInit        bool
	NeedsPivot                 bool
}

type clusterCtlConfigMap map[string]string

type AzureJSON struct {
	Cloud                        string `json:"cloud,omitempty"`
	TenantID                     string `json:"tenantId,omitempty"`
	SubscriptionID               string `json:"subscriptionId,omitempty"`
	AADClientID                  string `json:"aadClientId,omitempty"`
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
	cc := CreateCmd{}

	createCmd := &cobra.Command{
		Use:   createName,
		Short: createShortDescription,
		Long:  createLongDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cc.Run()
		},
	}

	f := createCmd.Flags()
	f.StringVarP(&cc.MgmtClusterKubeConfigPath, "mgmt-cluster-kubeconfig", "m", "", "path to the kubeconfig of your cluster-api management cluster")
	f.StringVarP(&cc.SubscriptionID, "subscription-id", "s", "", "Azure Active Directory service principal password")
	f.StringVarP(&cc.TenantID, "tenant", "", "", "Azure Active Directory tenant, must provide when using service principals")
	f.StringVarP(&cc.ClientID, "client-id", "", "", "Azure Active Directory service principal ID")
	f.StringVarP(&cc.ClientSecret, "client-secret", "", "", "Azure Active Directory service principal password")
	f.StringVar(&cc.AzureEnvironment, "azure-env", DefaultAzureEnvironment, "the target Azure cloud")
	f.StringVarP(&cc.ClusterName, "cluster-name", "n", "", "name of cluster")
	f.StringVarP(&cc.VnetName, "vnet-name", "", "", "name of vnet the cluster will reside in")
	f.StringVarP(&cc.ResourceGroup, "resource-group", "g", "", "name of resource group the cluster IaaS will reside in")
	f.StringVarP(&cc.Location, "location", "l", DefaultLocation, "Azure region cluster IaaS will reside in")
	f.StringVarP(&cc.ControlPlaneVMType, "control-plan-vm-sku", "", DefaultControlPlaneVMType, "SKU for control plane VMs, default is Standard_D2s_v3")
	f.StringVarP(&cc.NodeVMType, "node-vm-sku", "", DefaultNodeVMType, "SKU for node VMs, default is Standard_D2s_v3")
	f.StringVarP(&cc.SSHPublicKey, "ssh-public-key", "", "", "SSH public key to install for remote access to VMs")
	f.StringVarP(&cc.KubernetesVersion, "kubernetes-version", "v", DefaultKubernetesVersion, "Kubernetes version to install, default is 1.17.8")
	f.IntVarP(&cc.ControlPlaneNodes, "control-plane-nodes", "", DefaultControlPlaneNodes, "number of control plane nodes, default is 1")
	f.IntVarP(&cc.Nodes, "nodes", "", DefaultNodes, "number of worker nodes, default is 1")

	return createCmd
}

func (cc *CreateCmd) Run() error {
	s := spinner.New(spinner.CharSets[4], 100*time.Millisecond)
	green := color.New(color.FgGreen).SprintFunc()
	yellowbold := color.New(color.FgYellow, color.Bold).SprintFunc()
	magentabold := color.New(color.FgMagenta, color.Bold).SprintFunc()
	bold := color.New(color.FgWhite, color.Bold).SprintFunc()
	if cc.ClusterName == "" {
		cc.ClusterName = fmt.Sprintf("k8s-%s", strconv.Itoa(int(time.Now().Unix())))
	}
	if cc.VnetName == "" {
		cc.VnetName = cc.ClusterName
	}
	if cc.ResourceGroup == "" {
		cc.ResourceGroup = cc.ClusterName
	}

	h, err := os.UserHomeDir()
	if err != nil {
		log.Printf("Unable to get home dir: %s\n", err)
		return err
	}
	if cc.MgmtClusterKubeConfigPath == "" {
		cc.NeedsClusterAPIInit = true
		cc.MgmtClusterName = fmt.Sprintf("capi-mgmt-%s", strconv.Itoa(int(time.Now().Unix())))
		cc.NeedsPivot = true
		fmt.Printf("\nCreating an AKS management cluster %s for cluster-api management components:\n", magentabold(cc.MgmtClusterName))
		s.Color("yellow")
		s.Start()
		err = cc.createAKSMgmtClusterResourceGroupWithRetry(cc.MgmtClusterName, 30*time.Second, 3*time.Minute)
		if err != nil {
			log.Printf("Unable to create AKS management cluster resource group: %s\n", err)
			return err
		}
		err = cc.createAKSMgmtClusterWithRetry(cc.MgmtClusterName, 30*time.Second, 10*time.Minute)
		s.Stop()
		if err != nil {
			log.Printf("Unable to create AKS management cluster: %s\n", err)
			return err
		}
		fmt.Printf("\n%s\n", green("⎈⎈⎈"))
		cc.MgmtClusterKubeConfigPath = fmt.Sprintf("%s/.kube/%s.kubeconfig", h, cc.MgmtClusterName)
		fmt.Printf("\nGetting kubeconfig for AKS cluster-api management cluster %s:\n", magentabold(cc.MgmtClusterName))
		s.Start()
		err = cc.getAKSMgmtClusterCredsWithRetry(5*time.Second, 10*time.Minute)
		s.Stop()
		if err != nil {
			log.Printf("Unable to get AKS management cluster kubeconfig: %s\n", err)
			return err
		}
		fmt.Printf("\n%s\n", green("⎈⎈⎈"))
	}
	mgmtClusterKubeConfig, err := clientcmd.LoadFromFile(cc.MgmtClusterKubeConfigPath)
	if err != nil {
		log.Printf("Unable to load kubeconfig at %s: %s\n", cc.MgmtClusterKubeConfigPath, err)
		return err
	}
	if cc.MgmtClusterName != "" && cc.MgmtClusterName != mgmtClusterKubeConfig.CurrentContext {
		log.Printf("Got unexpected AKS management cluster kubeconfig")
		return err
	}
	cc.MgmtClusterName = mgmtClusterKubeConfig.CurrentContext
	for name, cluster := range mgmtClusterKubeConfig.Clusters {
		if name == cc.MgmtClusterName {
			cc.MgmtClusterURL = cluster.Server
		}
	}
	if cc.MgmtClusterURL == "" {
		log.Printf("Malformed kubeconfig at %s: %s\n", cc.MgmtClusterKubeConfigPath, err)
		return err
	}
	fmt.Printf("\nChecking if management cluster %s is ready at %s:\n", magentabold(cc.MgmtClusterName), bold(cc.MgmtClusterURL))
	cc.IsMgmtClusterReadyCmdArgs = append([]string{"kubectl"}, fmt.Sprintf("--kubeconfig=%s", cc.MgmtClusterKubeConfigPath), "cluster-info")
	fmt.Printf("%s\n", bold(fmt.Sprintf("$ %s", strings.Join(cc.IsMgmtClusterReadyCmdArgs, " "))))
	s.Color("yellow")
	s.Start()
	err = cc.isClusterReadyWithRetry(cc.IsMgmtClusterReadyCmdArgs, 30*time.Second, 20*time.Minute)
	s.Stop()
	if err != nil {
		log.Printf("management cluster %s not ready in 20 mins: %s\n", cc.MgmtClusterName, err)
		return err
	}
	fmt.Printf("\nWill use management cluster %s.", magentabold(cc.MgmtClusterName))
	fmt.Printf("\n\n%s\n", green("⎈⎈⎈"))
	cc.GetNewClusterConfigCmdArgs = append([]string{"kubectl"}, fmt.Sprintf("--kubeconfig=%s", cc.MgmtClusterKubeConfigPath), "get", fmt.Sprintf("secret/%s-kubeconfig", cc.ClusterName), "-o", "jsonpath={.data.value}")
	azureJSON := AzureJSON{
		Cloud:                        cc.AzureEnvironment,
		TenantID:                     cc.TenantID,
		SubscriptionID:               cc.SubscriptionID,
		AADClientID:                  cc.ClientID,
		AADClientSecret:              cc.ClientSecret,
		ResourceGroup:                cc.ResourceGroup,
		SecurityGroupName:            fmt.Sprintf("%s-node-nsg", cc.ClusterName),
		Location:                     cc.Location,
		VMType:                       "vmss",
		VNETName:                     cc.VnetName,
		VNETResourceGroup:            cc.ResourceGroup,
		SubnetName:                   fmt.Sprintf("%s-node-subnet", cc.ClusterName),
		RouteTableName:               fmt.Sprintf("%s-node-routetable", cc.ClusterName),
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
		"AZURE_SUBSCRIPTION_ID":            cc.SubscriptionID,
		"AZURE_SUBSCRIPTION_ID_B64":        base64.StdEncoding.EncodeToString([]byte(cc.SubscriptionID)),
		"AZURE_TENANT_ID":                  cc.TenantID,
		"AZURE_TENANT_ID_B64":              base64.StdEncoding.EncodeToString([]byte(cc.TenantID)),
		"AZURE_CLIENT_ID":                  cc.ClientID,
		"AZURE_CLIENT_ID_B64":              base64.StdEncoding.EncodeToString([]byte(cc.ClientID)),
		"AZURE_CLIENT_SECRET":              cc.ClientSecret,
		"AZURE_CLIENT_SECRET_B64":          base64.StdEncoding.EncodeToString([]byte(cc.ClientSecret)),
		"AZURE_ENVIRONMENT":                cc.AzureEnvironment,
		"KUBECONFIG":                       cc.MgmtClusterKubeConfigPath,
		"CLUSTER_NAME":                     cc.ClusterName,
		"AZURE_VNET_NAME":                  cc.VnetName,
		"AZURE_RESOURCE_GROUP":             cc.ResourceGroup,
		"AZURE_LOCATION":                   cc.Location,
		"AZURE_CONTROL_PLANE_MACHINE_TYPE": cc.ControlPlaneVMType,
		"AZURE_NODE_MACHINE_TYPE":          cc.NodeVMType,
		"AZURE_SSH_PUBLIC_KEY":             cc.SSHPublicKey,
		"AZURE_JSON_B64":                   base64.StdEncoding.EncodeToString(b),
	}
	for k, v := range clusterCtlConfig {
		err := os.Setenv(k, v)
		if err != nil {
			return errors.Wrap(err, fmt.Sprintf("setting %s env var", k))
		}
	}

	if !cc.NeedsClusterAPIInit {
		fmt.Printf("\nChecking if management cluster %s is cluster-api-ready:\n", magentabold(cc.MgmtClusterName))
		for _, namespace := range []string{
			"capi-system",
			"capi-webhook-system",
			"capi-kubeadm-control-plane-system",
			"capi-kubeadm-bootstrap-system",
			"capz-system",
		} {
			if err := cc.namespaceExistsOnMgmtClusterWithRetry(namespace, 1*time.Second, 5*time.Second); err != nil {
				cc.NeedsClusterAPIInit = true
				break
			}
		}
	}
	if cc.NeedsClusterAPIInit {
		fmt.Printf("\nInstalling cluster-api components on the management cluster %s:\n", magentabold(cc.MgmtClusterName))
		cmd := exec.Command("clusterctl", "init", "--kubeconfig", cc.MgmtClusterKubeConfigPath, "--infrastructure", "azure")
		fmt.Printf("%s\n", bold(fmt.Sprintf("$ %s", strings.Join(cmd.Args, " "))))
		s.Start()
		out, err := cmd.CombinedOutput()
		s.Stop()
		if err != nil && !strings.Contains(string(out), "there is already an instance of the \"infrastructure-azure\" provider installed in the \"capz-system\" namespace") {
			log.Printf("%\n", string(out))
			log.Printf("Unable to initialize management cluster %s for Azure: %s\n", cc.MgmtClusterName, err)
			return err
		}
	}
	fmt.Printf("\nmanagement cluster %s is cluster-api-ready.\n", magentabold(cc.MgmtClusterName))
	fmt.Printf("\n%s\n", green("⎈⎈⎈"))
	fmt.Printf("\nGenerating Azure cluster-api config for new cluster %s:\n", yellowbold(cc.ClusterName))
	cmd := exec.Command("clusterctl", "config", "cluster", "--infrastructure", "azure", cc.ClusterName, "--kubernetes-version", fmt.Sprintf("v%s", cc.KubernetesVersion), "--control-plane-machine-count", strconv.Itoa(cc.ControlPlaneNodes), "--worker-machine-count", strconv.Itoa(cc.Nodes))
	fmt.Printf("%s\n", bold(fmt.Sprintf("$ %s", strings.Join(cmd.Args, " "))))
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("%\n", string(out))
		log.Printf("Unable to generate cluster config: %s\n", err)
		return err
	}
	clusterConfigYaml := fmt.Sprintf("%s.yaml", cc.ClusterName)
	pwd, err := os.Getwd()
	if err != nil {
		log.Printf("Unable to get working directory: %s\n", err)
		return err
	}
	f, err := os.Create(clusterConfigYaml)
	if err != nil {
		log.Printf("Unable to create and open cluster config file for writing: %s\n", err)
		return err
	}
	fmt.Printf("\nWrote cluster config to %s/%s.\n\n", pwd, clusterConfigYaml)
	fmt.Printf("%s\n", green("⎈⎈⎈"))
	defer func() {
		if err := f.Close(); err != nil {
			panic(err)
		}
	}()
	if _, err := f.Write(out); err != nil {
		panic(err)
	}
	fmt.Printf("\nCreating cluster %s on cluster-api management cluster %s:\n", yellowbold(cc.ClusterName), magentabold(cc.MgmtClusterName))
	s.Start()
	err = cc.applyKubeSpecWithRetry(cc.MgmtClusterKubeConfigPath, clusterConfigYaml, 3*time.Second, 5*time.Minute)
	s.Stop()
	if err != nil {
		log.Printf("Unable to apply cluster config at path %s to cluster-api management cluster %s: %s\n", err, clusterConfigYaml, cc.MgmtClusterName)
		return err
	}
	fmt.Printf("\n%s\n", green("⎈⎈⎈"))
	fmt.Printf("\nFetching kubeconfig for cluster %s from cluster-api management cluster %s:\n", yellowbold(cc.ClusterName), magentabold(cc.MgmtClusterName))
	fmt.Printf("%s\n\n", bold(fmt.Sprintf("$ %s", strings.Join(cc.GetNewClusterConfigCmdArgs, " "))))
	s.Start()
	secret, err := cc.getClusterKubeConfigWithRetry(30*time.Second, 20*time.Minute)
	s.Stop()
	if err != nil {
		log.Printf("Unable to get cluster %s kubeconfig from cluster-api management cluster %s: %s\n", yellowbold(cc.ClusterName), yellowbold(cc.MgmtClusterName), err)
		return err
	}
	fmt.Printf("%s\n", green("⎈⎈⎈"))
	decodedBytes, err := base64.StdEncoding.DecodeString(secret)
	if err != nil {
		log.Printf("Unable to decode cluster %s kubeconfig: %s\n", cc.ClusterName, err)
	}
	cc.NewClusterKubeConfigPath = fmt.Sprintf("%s/.kube/%s.kubeconfig", h, cc.ClusterName)
	fmt.Printf("\nWriting cluster config to %s...\n", cc.NewClusterKubeConfigPath)
	f2, err := os.Create(cc.NewClusterKubeConfigPath)
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
	fmt.Printf("\nWaiting for cluster %s to become ready:\n", yellowbold(cc.ClusterName))
	cc.IsClusterReadyCmdArgs = append([]string{"kubectl"}, fmt.Sprintf("--kubeconfig=%s", cc.NewClusterKubeConfigPath), "cluster-info")
	cc.IsMgmtClusterReadyCmdArgs = append([]string{"kubectl"}, fmt.Sprintf("--kubeconfig=%s", cc.MgmtClusterKubeConfigPath), "cluster-info")
	fmt.Printf("%s\n\n", bold(fmt.Sprintf("$ %s", strings.Join(cc.IsClusterReadyCmdArgs, " "))))
	s.Start()
	err = cc.isClusterReadyWithRetry(cc.IsClusterReadyCmdArgs, 30*time.Second, 20*time.Minute)
	s.Stop()
	if err != nil {
		log.Printf("Cluster %s not ready in 20 mins: %s\n", cc.ClusterName, err)
		return err
	}
	fmt.Printf("%s\n", green("⎈⎈⎈"))
	fmt.Printf("\nApplying calico CNI spec to cluster %s...\n", yellowbold(cc.ClusterName))
	cmd = exec.Command("kubectl", "apply", "-f", calicoSpec, "--kubeconfig", cc.NewClusterKubeConfigPath)
	fmt.Printf("%s\n\n", bold(fmt.Sprintf("$ %s", strings.Join(cmd.Args, " "))))
	s.Start()
	out, err = cmd.CombinedOutput()
	s.Stop()
	if err != nil {
		log.Printf("%\n", string(out))
		log.Printf("Unable to apply cluster config to cluster-api management cluster: %s\n", err)
		return err
	}
	fmt.Printf("%s\n", green("⎈⎈⎈"))

	if cc.NeedsPivot {
		fmt.Printf("\nInstalling cluster-api components on cluster %s:\n", yellowbold(cc.ClusterName))
		cmd := exec.Command("clusterctl", "init", "--kubeconfig", cc.NewClusterKubeConfigPath, "--infrastructure", "azure")
		fmt.Printf("%s\n", bold(fmt.Sprintf("$ %s", strings.Join(cmd.Args, " "))))
		s.Start()
		out, err := cmd.CombinedOutput()
		s.Stop()
		if err != nil && !strings.Contains(string(out), "there is already an instance of the \"infrastructure-azure\" provider installed in the \"capz-system\" namespace") {
			log.Printf("%\n", string(out))
			log.Printf("Unable to install cluster-api components on cluster %s: %s\n", cc.ClusterName, err)
			return err
		}
		fmt.Printf("\nEnabling management of cluster %s via local cluster-api interfaces:\n", yellowbold(cc.ClusterName))
		cmd = exec.Command("clusterctl", "move", "--kubeconfig", cc.MgmtClusterKubeConfigPath, "--to-kubeconfig", cc.NewClusterKubeConfigPath)
		fmt.Printf("%s\n", bold(fmt.Sprintf("$ %s", strings.Join(cmd.Args, " "))))
		s.Start()
		out, err = cmd.CombinedOutput()
		s.Stop()
		if err != nil && !strings.Contains(string(out), "there is already an instance of the \"infrastructure-azure\" provider installed in the \"capz-system\" namespace") {
			log.Printf("%\n", string(out))
			log.Printf("Unable to move cluster-api objects from cluster %s to cluster %s: %s\n", cc.MgmtClusterName, cc.ClusterName, err)
			return err
		}
		fmt.Printf("\nCleaning up temporary AKS management cluster %s:\n", yellowbold(cc.MgmtClusterName))
		s.Start()
		err = cc.deleteAKSMgmtClusterWithRetry(cc.MgmtClusterName, 30*time.Second, 10*time.Minute)
		if err != nil {
			log.Printf("Unable to delete AKS management cluster: %s\n", err)
			return err
		}
		err = cc.deleteAKSMgmtClusterResourceGroupWithRetry(cc.MgmtClusterName, 30*time.Second, 3*time.Minute)
		s.Stop()
		if err != nil {
			log.Printf("Unable to delete AKS management cluster resource group: %s\n", err)
			return err
		}
		fmt.Printf("\n%s\n", green("⎈⎈⎈"))
	}

	fmt.Printf("\nYour new cluster %s is ready!\n", yellowbold(cc.ClusterName))
	fmt.Printf("\nE.g.:\n")
	cmd = exec.Command("kubectl", fmt.Sprintf("--kubeconfig=%s", cc.NewClusterKubeConfigPath), "get", "nodes", "-o", "wide")
	fmt.Printf("%s\n\n", bold(fmt.Sprintf("$ %s", strings.Join(cmd.Args, " "))))
	return nil
}

// getClusterKubeConfigSecretResult is the result type for GetAllByPrefixAsync
type getClusterKubeConfigSecretResult struct {
	secret string
	err    error
}

func (cc *CreateCmd) getClusterKubeConfig() getClusterKubeConfigSecretResult {
	cmd := exec.Command(cc.GetNewClusterConfigCmdArgs[0], cc.GetNewClusterConfigCmdArgs[1:]...)
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
func (cc *CreateCmd) getClusterKubeConfigWithRetry(sleep, timeout time.Duration) (string, error) {
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

type isExecNonZeroExitResult struct {
	stdout []byte
	err    error
}

func (cc *CreateCmd) isClusterReady(cmdArgs []string) isExecNonZeroExitResult {
	cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
	out, err := cmd.CombinedOutput()
	return isExecNonZeroExitResult{
		stdout: out,
		err:    err,
	}
}

// isClusterReadyWithRetry will return if the new cluster is ready, retrying up to a timeout
func (cc *CreateCmd) isClusterReadyWithRetry(cmdArgs []string, sleep, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	ch := make(chan isExecNonZeroExitResult)
	var mostRecentIsClusterReadyWithRetryError error
	var stdout []byte
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				ch <- cc.isClusterReady(cmdArgs)
				time.Sleep(sleep)
			}
		}
	}()
	for {
		select {
		case result := <-ch:
			mostRecentIsClusterReadyWithRetryError = result.err
			stdout = result.stdout
			if mostRecentIsClusterReadyWithRetryError == nil {
				return nil
			}
		case <-ctx.Done():
			fmt.Printf("%s\n", string(stdout))
			return errors.Errorf("isClusterReadyWithRetry timed out: %s\n", mostRecentIsClusterReadyWithRetryError)
		}
	}
}

func (cc *CreateCmd) namespaceExistsOnMgmtCluster(namespace string) isExecNonZeroExitResult {
	cmd := exec.Command("kubectl", fmt.Sprintf("--kubeconfig=%s", cc.MgmtClusterKubeConfigPath), "get", "namespace", namespace)
	fmt.Printf("%s\n", fmt.Sprintf("$ %s", strings.Join(cmd.Args, " ")))
	out, err := cmd.CombinedOutput()
	return isExecNonZeroExitResult{
		stdout: out,
		err:    err,
	}
}

func (cc *CreateCmd) applyKubeSpec(kubeconfigPath, yamlSpecPath string) isExecNonZeroExitResult {
	cmd := exec.Command("kubectl", fmt.Sprintf("--kubeconfig=%s", kubeconfigPath), "apply", "-f", fmt.Sprintf("./%s", yamlSpecPath))
	fmt.Printf("%s\n", fmt.Sprintf("$ %s", strings.Join(cmd.Args, " ")))
	out, err := cmd.CombinedOutput()
	return isExecNonZeroExitResult{
		stdout: out,
		err:    err,
	}
}

// applyKubeSpecWithRetry will run kubectl apply -f against given kubeconfig, retrying up to a timeout
func (cc *CreateCmd) applyKubeSpecWithRetry(kubeconfigPath, yamlSpecPath string, sleep, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	ch := make(chan isExecNonZeroExitResult)
	var mostRecentApplyKubeSpecWithRetryError error
	var stdout []byte
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				ch <- cc.applyKubeSpec(kubeconfigPath, yamlSpecPath)
				time.Sleep(sleep)
			}
		}
	}()
	for {
		select {
		case result := <-ch:
			mostRecentApplyKubeSpecWithRetryError = result.err
			stdout = result.stdout
			if mostRecentApplyKubeSpecWithRetryError == nil {
				return nil
			}
		case <-ctx.Done():
			fmt.Printf("%s\n", string(stdout))
			return errors.Errorf("applyKubeSpec timed out: %s\n", mostRecentApplyKubeSpecWithRetryError)
		}
	}
}

// namespaceExistsOnMgmtClusterWithRetry will return if the namespace exists on the cluster-api management cluster, retrying up to a timeout
func (cc *CreateCmd) namespaceExistsOnMgmtClusterWithRetry(namespace string, sleep, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	ch := make(chan isExecNonZeroExitResult)
	var mostRecentNamespaceExistsOnMgmtClusterWithRetryError error
	var stdout []byte
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				ch <- cc.namespaceExistsOnMgmtCluster(namespace)
				time.Sleep(sleep)
			}
		}
	}()
	for {
		select {
		case result := <-ch:
			mostRecentNamespaceExistsOnMgmtClusterWithRetryError = result.err
			stdout = result.stdout
			if mostRecentNamespaceExistsOnMgmtClusterWithRetryError == nil {
				return nil
			}
		case <-ctx.Done():
			fmt.Printf("%s\n", string(stdout))
			return errors.Errorf("namespaceExistsOnMgmtClusterWithRetry timed out: %s\n", mostRecentNamespaceExistsOnMgmtClusterWithRetryError)
		}
	}
}

func (cc *CreateCmd) getAKSMgmtClusterCreds() isExecNonZeroExitResult {
	cmd := exec.Command("az", "aks", "get-credentials", "-g", cc.MgmtClusterName, "-n", cc.MgmtClusterName, "-f", cc.MgmtClusterKubeConfigPath)
	fmt.Printf("%s\n", fmt.Sprintf("$ %s", strings.Join(cmd.Args, " ")))
	out, err := cmd.CombinedOutput()
	return isExecNonZeroExitResult{
		stdout: out,
		err:    err,
	}
}

// getAKSMgmtClusterCredsWithRetry gets AKS cluster kubeconfig, retrying up to a timeout
func (cc *CreateCmd) getAKSMgmtClusterCredsWithRetry(sleep, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	ch := make(chan isExecNonZeroExitResult)
	var mostRecentGetAKSMgmtClusterCredsWithRetryError error
	var stdout []byte
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				ch <- cc.getAKSMgmtClusterCreds()
				time.Sleep(sleep)
			}
		}
	}()
	for {
		select {
		case result := <-ch:
			mostRecentGetAKSMgmtClusterCredsWithRetryError = result.err
			stdout = result.stdout
			if mostRecentGetAKSMgmtClusterCredsWithRetryError == nil {
				return nil
			}
		case <-ctx.Done():
			fmt.Printf("%s\n", string(stdout))
			return errors.Errorf("getAKSMgmtClusterCredsWithRetry timed out: %s\n", mostRecentGetAKSMgmtClusterCredsWithRetryError)
		}
	}
}

func (cc *CreateCmd) createAKSMgmtClusterResourceGroup(name string) isExecNonZeroExitResult {
	cmd := exec.Command("az", "group", "create", "-g", name, "-l", cc.Location)
	fmt.Printf("%s\n", fmt.Sprintf("$ %s", strings.Join(cmd.Args, " ")))
	out, err := cmd.CombinedOutput()
	return isExecNonZeroExitResult{
		stdout: out,
		err:    err,
	}
}

func (cc *CreateCmd) deleteAKSMgmtClusterResourceGroup(name string) isExecNonZeroExitResult {
	cmd := exec.Command("az", "group", "delete", "-g", name, "--no-wait", "-y")
	fmt.Printf("%s\n", fmt.Sprintf("$ %s", strings.Join(cmd.Args, " ")))
	out, err := cmd.CombinedOutput()
	return isExecNonZeroExitResult{
		stdout: out,
		err:    err,
	}
}

// createAKSMgmtClusterResourceGroupWithRetry will create the resource group for an AKS cluster, retrying up to a timeout
func (cc *CreateCmd) createAKSMgmtClusterResourceGroupWithRetry(name string, sleep, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	ch := make(chan isExecNonZeroExitResult)
	var mostRecentCreateAKSMgmtClusterResourceGroupWithRetryError error
	var stdout []byte
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				ch <- cc.createAKSMgmtClusterResourceGroup(name)
				time.Sleep(sleep)
			}
		}
	}()
	for {
		select {
		case result := <-ch:
			mostRecentCreateAKSMgmtClusterResourceGroupWithRetryError = result.err
			stdout = result.stdout
			if mostRecentCreateAKSMgmtClusterResourceGroupWithRetryError == nil {
				return nil
			}
		case <-ctx.Done():
			fmt.Printf("%s\n", string(stdout))
			return errors.Errorf("createAKSMgmtClusterResourceGroupWithRetry timed out: %s\n", mostRecentCreateAKSMgmtClusterResourceGroupWithRetryError)
		}
	}
}

// deleteAKSMgmtClusterResourceGroupWithRetry will create the resource group for an AKS cluster, retrying up to a timeout
func (cc *CreateCmd) deleteAKSMgmtClusterResourceGroupWithRetry(name string, sleep, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	ch := make(chan isExecNonZeroExitResult)
	var mostRecentDeleteAKSMgmtClusterResourceGroupWithRetryError error
	var stdout []byte
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				ch <- cc.deleteAKSMgmtClusterResourceGroup(name)
				time.Sleep(sleep)
			}
		}
	}()
	for {
		select {
		case result := <-ch:
			mostRecentDeleteAKSMgmtClusterResourceGroupWithRetryError = result.err
			stdout = result.stdout
			if mostRecentDeleteAKSMgmtClusterResourceGroupWithRetryError == nil {
				return nil
			}
		case <-ctx.Done():
			fmt.Printf("%s\n", string(stdout))
			return errors.Errorf("deleteAKSMgmtClusterResourceGroupWithRetry timed out: %s\n", mostRecentDeleteAKSMgmtClusterResourceGroupWithRetryError)
		}
	}
}

func (cc *CreateCmd) createAKSMgmtCluster(name string) isExecNonZeroExitResult {
	cmd := exec.Command("az", "aks", "create", "-g", name, "-n", name, "--kubernetes-version", "1.17.7", "-c", "1", "-s", "Standard_B2s")
	fmt.Printf("%s\n", fmt.Sprintf("$ %s", strings.Join(cmd.Args, " ")))
	out, err := cmd.CombinedOutput()
	return isExecNonZeroExitResult{
		stdout: out,
		err:    err,
	}
}

func (cc *CreateCmd) deleteAKSMgmtCluster(name string) isExecNonZeroExitResult {
	cmd := exec.Command("az", "aks", "delete", "-g", name, "-n", name, "--no-wait", "-y")
	fmt.Printf("%s\n", fmt.Sprintf("$ %s", strings.Join(cmd.Args, " ")))
	out, err := cmd.CombinedOutput()
	return isExecNonZeroExitResult{
		stdout: out,
		err:    err,
	}
}

// createAKSMgmtClusterWithRetry will create an AKS cluster, retrying up to a timeout
func (cc *CreateCmd) createAKSMgmtClusterWithRetry(name string, sleep, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	ch := make(chan isExecNonZeroExitResult)
	var mostRecentCreateAKSMgmtClusterWithRetryError error
	var stdout []byte
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				ch <- cc.createAKSMgmtCluster(name)
				time.Sleep(sleep)
			}
		}
	}()
	for {
		select {
		case result := <-ch:
			mostRecentCreateAKSMgmtClusterWithRetryError = result.err
			stdout = result.stdout
			if mostRecentCreateAKSMgmtClusterWithRetryError == nil {
				return nil
			}
		case <-ctx.Done():
			fmt.Printf("%s\n", string(stdout))
			return errors.Errorf("createAKSMgmtClusterWithRetry timed out: %s\n", mostRecentCreateAKSMgmtClusterWithRetryError)
		}
	}
}

// deleteAKSMgmtClusterWithRetry will create an AKS cluster, retrying up to a timeout
func (cc *CreateCmd) deleteAKSMgmtClusterWithRetry(name string, sleep, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	ch := make(chan isExecNonZeroExitResult)
	var mostRecentDeleteAKSMgmtClusterWithRetryError error
	var stdout []byte
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				ch <- cc.deleteAKSMgmtCluster(name)
				time.Sleep(sleep)
			}
		}
	}()
	for {
		select {
		case result := <-ch:
			mostRecentDeleteAKSMgmtClusterWithRetryError = result.err
			stdout = result.stdout
			if mostRecentDeleteAKSMgmtClusterWithRetryError == nil {
				return nil
			}
		case <-ctx.Done():
			fmt.Printf("%s\n", string(stdout))
			return errors.Errorf("deleteAKSMgmtClusterWithRetry timed out: %s\n", mostRecentDeleteAKSMgmtClusterWithRetryError)
		}
	}
}
