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

type createCmd struct {
	authProvider
	mgmtClusterKubeConfigPath  string
	mgmtClusterName            string
	mgmtClusterURL             string
	subscriptionID             string
	tenantID                   string
	clientID                   string
	clientSecret               string
	azureEnvironment           string
	clusterName                string
	vnetName                   string
	resourceGroup              string
	location                   string
	controlPlaneVMType         string
	nodeVMType                 string
	sshPublicKey               string
	kubernetesVersion          string
	controlPlaneNodes          int
	nodes                      int
	newClusterKubeConfigPath   string
	getNewClusterConfigCmdArgs []string
	isMgmtClusterReadyCmdArgs  []string
	isClusterReadyCmdArgs      []string
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
	f.StringVarP(&cc.mgmtClusterKubeConfigPath, "mgmt-cluster-kubeconfig", "m", "~/.kube/config", "path to the kubeconfig of your cluster-api management cluster")
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
	f.IntVarP(&cc.controlPlaneNodes, "control-plane-nodes", "", 1, "number of control plane nodes, default is 3")
	f.IntVarP(&cc.nodes, "nodes", "", 1, "number of worker nodes, default is 3")

	return createCmd
}

func (cc *createCmd) run() error {
	s := spinner.New(spinner.CharSets[4], 100*time.Millisecond)
	green := color.New(color.FgGreen).SprintFunc()
	yellowbold := color.New(color.FgYellow, color.Bold).SprintFunc()
	magentabold := color.New(color.FgMagenta, color.Bold).SprintFunc()
	bold := color.New(color.FgWhite, color.Bold).SprintFunc()
	if cc.clusterName == "" {
		cc.clusterName = fmt.Sprintf("k8s-%s", strconv.Itoa(int(time.Now().Unix())))
	}
	if cc.vnetName == "" {
		cc.vnetName = cc.clusterName
	}
	if cc.resourceGroup == "" {
		cc.resourceGroup = cc.clusterName
	}

	mgmtClusterKubeConfig, err := clientcmd.LoadFromFile(cc.mgmtClusterKubeConfigPath)
	if err != nil {
		log.Printf("Unable to load kubeconfig at %s: %s\n", cc.mgmtClusterKubeConfigPath, err)
		return err
	}
	cc.mgmtClusterName = mgmtClusterKubeConfig.CurrentContext
	for name, cluster := range mgmtClusterKubeConfig.Clusters {
		if name == cc.mgmtClusterName {
			cc.mgmtClusterURL = cluster.Server
		}
	}
	if cc.mgmtClusterURL == "" {
		log.Printf("Malformed kubeconfig at %s: %s\n", cc.mgmtClusterKubeConfigPath, err)
		return err
	}
	fmt.Printf("\nChecking if mgmt cluster %s is ready at %s:\n", magentabold(cc.mgmtClusterName), bold(cc.mgmtClusterURL))
	cc.isMgmtClusterReadyCmdArgs = append([]string{"kubectl"}, fmt.Sprintf("--kubeconfig=%s", cc.mgmtClusterKubeConfigPath), "cluster-info")
	fmt.Printf("%s\n", bold(fmt.Sprintf("$ %s", strings.Join(cc.isMgmtClusterReadyCmdArgs, " "))))
	s.Start()
	err = cc.isClusterReadyWithRetry(cc.isMgmtClusterReadyCmdArgs, 30*time.Second, 20*time.Minute)
	s.Stop()
	if err != nil {
		log.Printf("mgmt cluster %s not ready in 20 mins: %s\n", cc.mgmtClusterName, err)
		return err
	}
	fmt.Printf("\nWill use mgmt cluster %s.", magentabold(cc.mgmtClusterName))
	fmt.Printf("\n\n%s\n", green("⎈⎈⎈"))
	cc.getNewClusterConfigCmdArgs = append([]string{"kubectl"}, fmt.Sprintf("--kubeconfig=%s", cc.mgmtClusterKubeConfigPath), "get", fmt.Sprintf("secret/%s-kubeconfig", cc.clusterName), "-o", "jsonpath={.data.value}")
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
	fmt.Printf("\nChecking if mgmt cluster %s is cluster-api-ready:\n", yellowbold(cc.mgmtClusterName))
	var needsInit bool
	for _, namespace := range []string{
		"capi-system",
		"capi-webhook-system",
		"capi-kubeadm-control-plane-system",
		"capi-kubeadm-bootstrap-system",
		"capz-system",
	} {
		if err := cc.namespaceExistsOnMgmtClusterWithRetry(namespace, 1*time.Second, 5*time.Second); err != nil {
			needsInit = true
			break
		}
	}
	if needsInit {
		fmt.Printf("\nInitializing cluster-api on the management cluster at %s...\n", cc.mgmtClusterKubeConfigPath)
		cmd := exec.Command("clusterctl", "init", "--infrastructure", "azure")
		out, err := cmd.CombinedOutput()
		if err != nil && !strings.Contains(string(out), "there is already an instance of the \"infrastructure-azure\" provider installed in the \"capz-system\" namespace") {
			log.Printf("%\n", string(out))
			log.Printf("Unable to initialize management cluster for Azure: %s\n", err)
			return err
		}
	} else {
		fmt.Printf("\nmgmt cluster %s is cluster-api-ready.\n", magentabold(cc.mgmtClusterName))
		fmt.Printf("\n%s\n", green("⎈⎈⎈"))
	}
	fmt.Printf("\nGenerating Azure cluster-api config for new cluster %s:\n", yellowbold(cc.clusterName))
	cmd := exec.Command("clusterctl", "config", "cluster", "--infrastructure", "azure", cc.clusterName, "--kubernetes-version", fmt.Sprintf("v%s", cc.kubernetesVersion), "--control-plane-machine-count", strconv.Itoa(cc.controlPlaneNodes), "--worker-machine-count", strconv.Itoa(cc.nodes))
	fmt.Printf("%s\n", bold(fmt.Sprintf("$ %s", strings.Join(cmd.Args, " "))))
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("%\n", string(out))
		log.Printf("Unable to generate cluster config: %s\n", err)
		return err
	}
	clusterConfigYaml := fmt.Sprintf("%s.yaml", cc.clusterName)
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
	fmt.Printf("\nCreating cluster %s on cluster-api management cluster %s:\n", yellowbold(cc.clusterName), magentabold(cc.mgmtClusterName))
	cmd = exec.Command("kubectl", fmt.Sprintf("--kubeconfig=%s", cc.mgmtClusterKubeConfigPath), "apply", "-f", fmt.Sprintf("./%s", clusterConfigYaml))
	fmt.Printf("%s\n\n", bold(fmt.Sprintf("$ %s", strings.Join(cmd.Args, " "))))
	s.Color("yellow")
	s.Start()
	out, err = cmd.CombinedOutput()
	s.Stop()
	if err != nil {
		log.Printf("%\n", string(out))
		log.Printf("Unable to apply cluster config to cluster-api management cluster %s: %s\n", err, cc.mgmtClusterName)
		return err
	}
	fmt.Printf("%s\n", green("⎈⎈⎈"))
	fmt.Printf("\nFetching kubeconfig for cluster %s from cluster-api management cluster %s:\n", yellowbold(cc.clusterName), magentabold(cc.mgmtClusterName))
	fmt.Printf("%s\n\n", bold(fmt.Sprintf("$ %s", strings.Join(cc.getNewClusterConfigCmdArgs, " "))))
	s.Start()
	secret, err := cc.getClusterKubeConfigWithRetry(30*time.Second, 20*time.Minute)
	s.Stop()
	if err != nil {
		log.Printf("Unable to get cluster %s kubeconfig from cluster-api management cluster %s: %s\n", yellowbold(cc.clusterName), yellowbold(cc.mgmtClusterName), err)
		return err
	}
	fmt.Printf("%s\n", green("⎈⎈⎈"))
	decodedBytes, err := base64.StdEncoding.DecodeString(secret)
	if err != nil {
		log.Printf("Unable to decode cluster %s kubeconfig: %s\n", cc.clusterName, err)
	}
	h, err := os.UserHomeDir()
	if err != nil {
		log.Printf("Unable to get home dir: %s\n", err)
		return err
	}
	cc.newClusterKubeConfigPath = fmt.Sprintf("%s/.kube/%s.kubeconfig", h, cc.clusterName)
	fmt.Printf("\nWriting cluster config to %s...\n", cc.newClusterKubeConfigPath)
	f2, err := os.Create(cc.newClusterKubeConfigPath)
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
	fmt.Printf("\nWaiting for cluster %s to become ready:\n", yellowbold(cc.clusterName))
	cc.isClusterReadyCmdArgs = append([]string{"kubectl"}, fmt.Sprintf("--kubeconfig=%s", cc.newClusterKubeConfigPath), "cluster-info")
	cc.isMgmtClusterReadyCmdArgs = append([]string{"kubectl"}, fmt.Sprintf("--kubeconfig=%s", cc.mgmtClusterKubeConfigPath), "cluster-info")
	fmt.Printf("%s\n\n", bold(fmt.Sprintf("$ %s", strings.Join(cc.isClusterReadyCmdArgs, " "))))
	s.Start()
	err = cc.isClusterReadyWithRetry(cc.isClusterReadyCmdArgs, 30*time.Second, 20*time.Minute)
	s.Stop()
	if err != nil {
		log.Printf("Cluster %s not ready in 20 mins: %s\n", cc.clusterName, err)
		return err
	}
	fmt.Printf("%s\n", green("⎈⎈⎈"))
	fmt.Printf("\nApplying calico CNI spec to cluster %s...\n", yellowbold(cc.clusterName))
	cmd = exec.Command("kubectl", "apply", "-f", calicoSpec, "--kubeconfig", cc.newClusterKubeConfigPath)
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

	fmt.Printf("\nYour new cluster %s is ready!\n", yellowbold(cc.clusterName))
	fmt.Printf("\nE.g.:\n")
	cmd = exec.Command("kubectl", fmt.Sprintf("--kubeconfig=%s", cc.newClusterKubeConfigPath), "get", "nodes", "-o", "wide")
	fmt.Printf("%s\n\n", bold(fmt.Sprintf("$ %s", strings.Join(cmd.Args, " "))))
	return nil
}

// getClusterKubeConfigSecretResult is the result type for GetAllByPrefixAsync
type getClusterKubeConfigSecretResult struct {
	secret string
	err    error
}

func (cc *createCmd) getClusterKubeConfig() getClusterKubeConfigSecretResult {
	cmd := exec.Command(cc.getNewClusterConfigCmdArgs[0], cc.getNewClusterConfigCmdArgs[1:]...)
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

type isExecNonZeroExitResult struct {
	stdout []byte
	err    error
}

func (cc *createCmd) isClusterReady(cmdArgs []string) isExecNonZeroExitResult {
	cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
	out, err := cmd.CombinedOutput()
	return isExecNonZeroExitResult{
		stdout: out,
		err:    err,
	}
}

// isClusterReadyWithRetry will return if the new cluster is ready, retrying up to a timeout
func (cc *createCmd) isClusterReadyWithRetry(cmdArgs []string, sleep, timeout time.Duration) error {
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

func (cc *createCmd) namespaceExistsOnMgmtCluster(namespace string) isExecNonZeroExitResult {
	cmd := exec.Command("kubectl", fmt.Sprintf("--kubeconfig=%s", cc.mgmtClusterKubeConfigPath), "get", "namespace", namespace)
	fmt.Printf("%s\n", fmt.Sprintf("$ %s", strings.Join(cmd.Args, " ")))
	out, err := cmd.CombinedOutput()
	return isExecNonZeroExitResult{
		stdout: out,
		err:    err,
	}
}

// namespaceExistsOnMgmtClusterWithRetry will return if the namespace exists on the cluster-api mgmt cluster, retrying up to a timeout
func (cc *createCmd) namespaceExistsOnMgmtClusterWithRetry(namespace string, sleep, timeout time.Duration) error {
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
