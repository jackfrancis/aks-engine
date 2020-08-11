// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package engine

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/Azure/aks-engine/pkg/swagger/models"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	DefaultAzureEnvironment   string = "AzurePublicCloud"
	DefaultLocation           string = "westus2"
	DefaultControlPlaneVMType string = "Standard_D2s_v3"
	DefaultNodeVMType         string = "Standard_D2s_v3"
	DefaultKubernetesVersion  string = "1.17.8"
	DefaultControlPlaneNodes  int    = 1
	DefaultNodes              int    = 1
	CalicoSpec                       = "https://raw.githubusercontent.com/kubernetes-sigs/cluster-api-provider-azure/master/templates/addons/calico.yaml"
)

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

type Cluster interface {
	Create() error
	GetKubeConfig(time.Duration, time.Duration) (string, error)
	IsReady(sleep, timeout time.Duration) error
	ApplyClusterAPIConfig(time.Duration, time.Duration) error
	IsProvisioning() *bool
	IsMgmtClusterReady(sleep, timeout time.Duration) error
	IsMgmtClusterAPIReady() *bool
	MgmtClusterNeedsClusterAPIInit() *bool
	NeedsPivot() *bool
	GetName() string
	GetMgmtClusterName() string
	GetMgmtClusterURL() string
	GetClusterConfig() []byte
	GetClient() *kubernetes.Clientset
}

type createStatus struct {
	mgmtClusterKubeConfigPath      string
	mgmtClusterNeedsClusterAPIInit *bool
	mgmtClusterClient              *kubernetes.Clientset
	mgmtClusterAPIReady            *bool
	clusterConfigYaml              []byte // TODO strongly type this
	kubeConfig                     []byte // TODO strongly type this
	clusterConfigApplied           *bool
	needsPivot                     *bool
	localClusterAPIReady           *bool
	pivotComplete                  *bool
	cleanupComplete                *bool
	kubeConfigPath                 string
}

type cluster struct {
	spec            models.CreateData
	createStatus    createStatus
	mgmtClusterName string
	mgmtClusterURL  string
	client          *kubernetes.Clientset
}

func NewCluster(spec models.CreateData) Cluster {
	return &cluster{
		spec: spec,
	}
}

func (c *cluster) Create() error {
	if to.String(c.spec.ClusterName) == "" {
		c.spec.ClusterName = to.StringPtr(fmt.Sprintf("k8s-%s", strconv.Itoa(int(time.Now().Unix()))))
	}
	if to.String(c.spec.VnetName) == "" {
		c.spec.VnetName = to.StringPtr(to.String(c.spec.ClusterName))
	}
	if to.String(c.spec.ResourceGroup) == "" {
		c.spec.ResourceGroup = to.StringPtr(to.String(c.spec.ClusterName))
	}

	h, err := os.UserHomeDir()
	if err != nil {
		return errors.Errorf("Unable to get home dir: %s\n", err)
	}
	if to.String(c.spec.MgmtClusterKubeConfig) == "" {
		c.createStatus.mgmtClusterNeedsClusterAPIInit = to.BoolPtr(true)
		c.mgmtClusterName = fmt.Sprintf("capi-mgmt-%s", strconv.Itoa(int(time.Now().Unix())))
		c.createStatus.needsPivot = to.BoolPtr(true)
		err = createAKSMgmtClusterResourceGroupWithRetry(c.mgmtClusterName, to.String(c.spec.Location), 30*time.Second, 3*time.Minute)
		if err != nil {
			return errors.Errorf("Unable to create AKS management cluster resource group: %s\n", err)
		}
		err = createAKSMgmtClusterWithRetry(c.mgmtClusterName, 30*time.Second, 10*time.Minute)
		if err != nil {
			return errors.Errorf("Unable to create AKS management cluster: %s\n", err)
		}
		c.createStatus.mgmtClusterKubeConfigPath = fmt.Sprintf("%s/.kube/%s.kubeconfig", h, c.mgmtClusterName)
		err = getAKSMgmtClusterCredsWithRetry(c.mgmtClusterName, c.createStatus.mgmtClusterKubeConfigPath, 5*time.Second, 10*time.Minute)
		if err != nil {
			log.Printf("Unable to get AKS management cluster kubeconfig: %s\n", err)
			return err
		}
		b, err := ioutil.ReadFile(c.createStatus.mgmtClusterKubeConfigPath)
		if err != nil {
			return errors.Errorf("Unable to open mgmt cluster kubeconfig file for reading: %s\n", err)
		}
		c.spec.MgmtClusterKubeConfig = to.StringPtr(string(b))
	} else {
		tmpfile, err := ioutil.TempFile("", "tmp.kubeconfig")
		if err != nil {
			return errors.Errorf("Unable to create temp kubeconfig: %s\n", err)
		}
		c.createStatus.mgmtClusterKubeConfigPath = tmpfile.Name()
		defer os.Remove(c.createStatus.mgmtClusterKubeConfigPath)
		if _, err := tmpfile.Write([]byte(to.String(c.spec.MgmtClusterKubeConfig))); err != nil {
			return errors.Errorf("Unable to write temp kubeconfig file: %s\n", err)
		}
	}
	clientConfig, err := clientcmd.NewClientConfigFromBytes([]byte(to.String(c.spec.MgmtClusterKubeConfig)))
	if err != nil {
		fmt.Printf("%#v\n", c.spec.MgmtClusterKubeConfig)
		return errors.Errorf("Unable to create k8s client from file: %s\n", err)
	}
	restConfig, err := clientConfig.ClientConfig()
	if err != nil {
		return errors.Errorf("Unable to create k8s rest client from file: %s\n", err)
	}
	config, err := clientConfig.RawConfig()
	if err != nil {
		return errors.Errorf("Unable to create raw kubeconfig from k8s client: %s\n", err)
	}
	c.mgmtClusterURL = restConfig.Host
	c.mgmtClusterName = config.CurrentContext
	c.createStatus.mgmtClusterClient, err = kubernetes.NewForConfig(restConfig)
	if err != nil {
		return errors.Errorf("Unable to create k8s client for mgmt cluster: %s\n", err)
	}
	err = c.IsMgmtClusterReady(5*time.Second, 20*time.Minute)
	if err != nil {
		return errors.Errorf("management cluster %s not ready in 20 mins: %s\n", err)
	}
	azureJSON := AzureJSON{
		Cloud:                        to.String(c.spec.AzureEnvironment),
		TenantID:                     to.String(c.spec.TenantID),
		SubscriptionID:               to.String(c.spec.SubscriptionID),
		AADClientID:                  to.String(c.spec.ClientID),
		AADClientSecret:              to.String(c.spec.ClientSecret),
		ResourceGroup:                to.String(c.spec.ResourceGroup),
		SecurityGroupName:            fmt.Sprintf("%s-node-nsg", to.String(c.spec.ClusterName)),
		Location:                     to.String(c.spec.Location),
		VMType:                       "vmss",
		VNETName:                     to.String(c.spec.VnetName),
		VNETResourceGroup:            to.String(c.spec.ResourceGroup),
		SubnetName:                   fmt.Sprintf("%s-node-subnet", to.String(c.spec.ClusterName)),
		RouteTableName:               fmt.Sprintf("%s-node-routetable", to.String(c.spec.ClusterName)),
		LoadBalancerSku:              "standard",
		MaximumLoadBalancerRuleCount: 250,
		UseManagedIdentityExtension:  false,
		UseInstanceMetadata:          true,
	}
	b, err := json.MarshalIndent(azureJSON, "", "    ")
	if err != nil {
		return errors.Errorf("Unable to generate azure.JSON config: %s\n", err)
	}

	clusterCtlConfig := clusterCtlConfigMap{
		"AZURE_SUBSCRIPTION_ID":            to.String(c.spec.SubscriptionID),
		"AZURE_SUBSCRIPTION_ID_B64":        base64.StdEncoding.EncodeToString([]byte(to.String(c.spec.SubscriptionID))),
		"AZURE_TENANT_ID":                  to.String(c.spec.TenantID),
		"AZURE_TENANT_ID_B64":              base64.StdEncoding.EncodeToString([]byte(to.String(c.spec.TenantID))),
		"AZURE_CLIENT_ID":                  to.String(c.spec.ClientID),
		"AZURE_CLIENT_ID_B64":              base64.StdEncoding.EncodeToString([]byte(to.String(c.spec.ClientID))),
		"AZURE_CLIENT_SECRET":              to.String(c.spec.ClientSecret),
		"AZURE_CLIENT_SECRET_B64":          base64.StdEncoding.EncodeToString([]byte(to.String(c.spec.ClientSecret))),
		"AZURE_ENVIRONMENT":                to.String(c.spec.AzureEnvironment),
		"KUBECONFIG":                       c.createStatus.mgmtClusterKubeConfigPath,
		"CLUSTER_NAME":                     to.String(c.spec.ClusterName),
		"AZURE_VNET_NAME":                  to.String(c.spec.VnetName),
		"AZURE_RESOURCE_GROUP":             to.String(c.spec.ResourceGroup),
		"AZURE_LOCATION":                   to.String(c.spec.Location),
		"AZURE_CONTROL_PLANE_MACHINE_TYPE": to.String(c.spec.ControlPlaneVMType),
		"AZURE_NODE_MACHINE_TYPE":          to.String(c.spec.NodeVMType),
		"AZURE_SSH_PUBLIC_KEY":             to.String(c.spec.SSHPublicKey),
		"AZURE_JSON_B64":                   base64.StdEncoding.EncodeToString(b),
	}
	for k, v := range clusterCtlConfig {
		err := os.Setenv(k, v)
		if err != nil {
			return errors.Errorf("Failed to set env var %s=%s: %s\n", k, v, err)
		}
	}

	if !to.Bool(c.createStatus.mgmtClusterNeedsClusterAPIInit) {
		for _, namespace := range []string{
			"capi-system",
			"capi-webhook-system",
			"capi-kubeadm-control-plane-system",
			"capi-kubeadm-bootstrap-system",
			"capz-system",
		} {
			if err := namespaceExistsWithRetry(c.createStatus.mgmtClusterKubeConfigPath, namespace, 1*time.Second, 5*time.Second); err != nil {
				c.createStatus.mgmtClusterNeedsClusterAPIInit = to.BoolPtr(true)
				break
			}
		}
	}
	if c.createStatus.mgmtClusterNeedsClusterAPIInit == nil {
		c.createStatus.mgmtClusterNeedsClusterAPIInit = to.BoolPtr(false)
	}
	if to.Bool(c.createStatus.mgmtClusterNeedsClusterAPIInit) {
		cmd := exec.Command("clusterctl", "init", "--kubeconfig", c.createStatus.mgmtClusterKubeConfigPath, "--infrastructure", "azure")
		fmt.Printf("%s\n", fmt.Sprintf("$ %s", strings.Join(cmd.Args, " ")))
		out, err := cmd.CombinedOutput()
		if err != nil && !strings.Contains(string(out), "there is already an instance of the \"infrastructure-azure\" provider installed in the \"capz-system\" namespace") {
			return errors.Errorf("Unable to initialize management cluster %s for Azure: %s\n", c.mgmtClusterName, err)
		}
	}
	c.createStatus.mgmtClusterAPIReady = to.BoolPtr(true)
	cmd := exec.Command("clusterctl", "config", "cluster", "--infrastructure", "azure", to.String(c.spec.ClusterName), "--kubernetes-version", fmt.Sprintf("v%s", to.String(c.spec.KubernetesVersion)), "--control-plane-machine-count", strconv.Itoa(int(c.spec.ControlPlaneNodes)), "--worker-machine-count", strconv.Itoa(int(c.spec.Nodes)))
	c.createStatus.clusterConfigYaml, err = cmd.CombinedOutput()
	if err != nil {
		log.Printf("%\n", string(c.createStatus.clusterConfigYaml))
		return errors.Errorf("Unable to generate cluster config: %s\n", err)
	}
	time.Sleep(2 * time.Minute)
	err = c.ApplyClusterAPIConfig(3*time.Second, 5*time.Minute)
	if err != nil {
		return errors.Errorf("Unable to apply cluster config to cluster-api management cluster %s: %s\n", c.mgmtClusterName, err)
	}
	secret, err := c.GetKubeConfig(30*time.Second, 20*time.Minute)
	if err != nil {
		return errors.Errorf("Unable to get cluster %s kubeconfig from cluster-api management cluster %s: %s\n", c.spec.ClusterName, c.mgmtClusterName, err)
	}
	c.createStatus.kubeConfig, err = base64.StdEncoding.DecodeString(secret)
	if err != nil {
		return errors.Errorf("Unable to decode cluster %s kubeconfig: %s\n", c.spec.ClusterName, err)
	}
	clientConfigNewCluster, err := clientcmd.NewClientConfigFromBytes(c.createStatus.kubeConfig)
	if err != nil {
		return errors.Errorf("Unable to client config: %s\n", err)
	}
	restConfigNewCluster, err := clientConfigNewCluster.ClientConfig()
	if err != nil {
		return errors.Errorf("Unable to create rest config from k8s client: %s\n", err)
	}
	c.client, err = kubernetes.NewForConfig(restConfigNewCluster)
	if err != nil {
		return errors.Errorf("Unable to create k8s client from rest config: %s\n", err)
	}
	c.createStatus.kubeConfigPath = fmt.Sprintf("%s/.kube/%s.kubeconfig", h, to.String(c.spec.ClusterName))
	f2, err := os.Create(c.createStatus.kubeConfigPath)
	if err != nil {
		return errors.Errorf("Unable to create and open kubeconfig file for writing: %s\n", err)
	}
	defer func() {
		if err := f2.Close(); err != nil {
			panic(err)
		}
	}()
	if _, err := f2.Write(c.createStatus.kubeConfig); err != nil {
		panic(err)
	}
	err = c.IsReady(30*time.Second, 20*time.Minute)
	if err != nil {
		return errors.Errorf("Cluster %s not ready in 20 mins: %s\n", c.spec.ClusterName, err)
	}
	cmd = exec.Command("kubectl", "apply", "-f", CalicoSpec, "--kubeconfig", c.createStatus.kubeConfigPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("%\n", string(out))
		return errors.Errorf("Unable to apply cluster config to cluster-api management cluster: %s\n", err)
	}

	if to.Bool(c.createStatus.needsPivot) {
		if err := waitForMachineDeploymentReplicas(1, c.createStatus.mgmtClusterKubeConfigPath, 10*time.Second, 20*time.Minute); err != nil {
			return err
		}
		cmd := exec.Command("clusterctl", "init", "--kubeconfig", c.createStatus.kubeConfigPath, "--infrastructure", "azure")
		fmt.Printf("%s\n", fmt.Sprintf("$ %s", strings.Join(cmd.Args, " ")))
		out, err := cmd.CombinedOutput()
		if err != nil && !strings.Contains(string(out), "there is already an instance of the \"infrastructure-azure\" provider installed in the \"capz-system\" namespace") {
			log.Printf("%\n", string(out))
			return errors.Errorf("Unable to install cluster-api components on cluster %s: %s\n", c.spec.ClusterName, err)
		}
		c.createStatus.localClusterAPIReady = to.BoolPtr(true)
		time.Sleep(1 * time.Minute)
		cmd = exec.Command("clusterctl", "move", "--kubeconfig", c.createStatus.mgmtClusterKubeConfigPath, "--to-kubeconfig", c.createStatus.kubeConfigPath)
		fmt.Printf("%s\n", fmt.Sprintf("$ %s", strings.Join(cmd.Args, " ")))
		out, err = cmd.CombinedOutput()
		if err != nil && !strings.Contains(string(out), "there is already an instance of the \"infrastructure-azure\" provider installed in the \"capz-system\" namespace") {
			log.Printf("%\n", string(out))
			return errors.Errorf("Unable to move cluster-api objects from cluster %s to cluster %s: %s\n", c.mgmtClusterName, c.spec.ClusterName, err)
		}
		c.createStatus.pivotComplete = to.BoolPtr(true)
		err = deleteAKSMgmtClusterWithRetry(c.mgmtClusterName, 30*time.Second, 10*time.Minute)
		if err != nil {
			return errors.Errorf("Unable to delete AKS management cluster: %s\n", err)
		}
		err = deleteAKSMgmtClusterResourceGroupWithRetry(c.mgmtClusterName, 30*time.Second, 3*time.Minute)
		if err != nil {
			return errors.Errorf("Unable to delete AKS management cluster resource group: %s\n", err)
		}
		c.createStatus.cleanupComplete = to.BoolPtr(true)
	}
	return nil
}

// getClusterKubeConfigSecretResult is the result type for GetAllByPrefixAsync
type getClusterKubeConfigSecretResult struct {
	secret string
	err    error
}

func getClusterKubeConfig(cmdArgs []string) getClusterKubeConfigSecretResult {
	cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
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

// GetKubeConfig will return the cluster kubeconfig, retrying up to a timeout
func (c *cluster) GetKubeConfig(sleep, timeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	ch := make(chan getClusterKubeConfigSecretResult)
	var mostRecentGetClusterKubeConfigWithRetryError error
	var secret string
	cmdArgs := append([]string{"kubectl"}, fmt.Sprintf("--kubeconfig=%s", c.createStatus.mgmtClusterKubeConfigPath), "get", fmt.Sprintf("secret/%s-kubeconfig", to.String(c.spec.ClusterName)), "-o", "jsonpath={.data.value}")
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				ch <- getClusterKubeConfig(cmdArgs)
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
			return secret, errors.Errorf("GetKubeConfig timed out: %s\n", mostRecentGetClusterKubeConfigWithRetryError)
		}
	}
}

type isExecNonZeroExitResult struct {
	stdout []byte
	err    error
}

func isClusterReady(client *kubernetes.Clientset) error {
	_, err := client.CoreV1().Namespaces().Get("kube-system", metav1.GetOptions{})
	if err != nil {
		return err
	}
	return nil
}

// IsReady will return if the new cluster is ready, retrying up to a timeout
func (c *cluster) IsReady(sleep, timeout time.Duration) error {
	if c.GetClient() == nil {
		return errors.Errorf("no k8s client")
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	ch := make(chan error)
	var mostRecentIsClusterReadyWithRetryError error
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				ch <- isClusterReady(c.client)
				time.Sleep(sleep)
			}
		}
	}()
	for {
		select {
		case mostRecentIsClusterReadyWithRetryError := <-ch:
			if mostRecentIsClusterReadyWithRetryError == nil {
				return nil
			}
		case <-ctx.Done():
			return errors.Errorf("IsReady timed out: %s\n", mostRecentIsClusterReadyWithRetryError)
		}
	}
}

// IsMgmtClusterReady will return if the mgmt cluster is ready, retrying up to a timeout
func (c *cluster) IsMgmtClusterReady(sleep, timeout time.Duration) error {
	if c.createStatus.mgmtClusterClient == nil {
		return errors.Errorf("no k8s client")
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	ch := make(chan error)
	var mostRecentIsMgmtClusterReadyWithRetryError error
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				ch <- isClusterReady(c.createStatus.mgmtClusterClient)
				time.Sleep(sleep)
			}
		}
	}()
	for {
		select {
		case mostRecentIsMgmtClusterReadyWithRetryError := <-ch:
			if mostRecentIsMgmtClusterReadyWithRetryError == nil {
				return nil
			}
		case <-ctx.Done():
			return errors.Errorf("IsReady timed out: %s\n", mostRecentIsMgmtClusterReadyWithRetryError)
		}
	}
}

// GetName will return the cluster name
func (c *cluster) GetName() string {
	return to.String(c.spec.ClusterName)
}

// GetMgmtClusterName will return the name of the mgmt cluster
func (c *cluster) GetMgmtClusterName() string {
	return c.mgmtClusterName
}

// GetMgmtClusterURL will return the URL of the mgmt cluster
func (c *cluster) GetMgmtClusterURL() string {
	return c.mgmtClusterURL
}

// GetClusterConfig returns the raw cluster config
func (c *cluster) GetClusterConfig() []byte {
	return c.createStatus.clusterConfigYaml
}

// GetClient returns the k8s client instance
func (c *cluster) GetClient() *kubernetes.Clientset {
	return c.client
}

func (c *cluster) IsProvisioning() *bool {
	return c.createStatus.clusterConfigApplied
}

func (c *cluster) MgmtClusterNeedsClusterAPIInit() *bool {
	return c.createStatus.mgmtClusterNeedsClusterAPIInit
}

func (c *cluster) IsMgmtClusterAPIReady() *bool {
	return c.createStatus.mgmtClusterAPIReady
}

func (c *cluster) NeedsPivot() *bool {
	return c.createStatus.needsPivot
}

func namespaceExists(kubeconfigPath, namespace string) isExecNonZeroExitResult {
	cmd := exec.Command("kubectl", fmt.Sprintf("--kubeconfig=%s", kubeconfigPath), "get", "namespace", namespace)
	out, err := cmd.CombinedOutput()
	return isExecNonZeroExitResult{
		stdout: out,
		err:    err,
	}
}

func applyKubeSpec(kubeconfigPath, yamlSpecPath string) isExecNonZeroExitResult {
	cmd := exec.Command("kubectl", fmt.Sprintf("--kubeconfig=%s", kubeconfigPath), "apply", "-f", fmt.Sprintf("./%s", yamlSpecPath))
	out, err := cmd.CombinedOutput()
	return isExecNonZeroExitResult{
		stdout: out,
		err:    err,
	}
}

// ApplyClusterAPIConfig will run kubectl apply -f against given kubeconfig, retrying up to a timeout
func (c *cluster) ApplyClusterAPIConfig(sleep, timeout time.Duration) error {
	clusterConfigYaml := fmt.Sprintf("%s.yaml", to.String(c.spec.ClusterName))
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
	if _, err := f.Write(c.createStatus.clusterConfigYaml); err != nil {
		panic(err)
	}
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
				ch <- applyKubeSpec(c.createStatus.mgmtClusterKubeConfigPath, clusterConfigYaml)
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
				c.createStatus.clusterConfigApplied = to.BoolPtr(true)
				return nil
			}
		case <-ctx.Done():
			fmt.Printf("%s\n", string(stdout))
			return errors.Errorf("applyKubeSpec timed out: %s\n", mostRecentApplyKubeSpecWithRetryError)
		}
	}
}

// namespaceExistsOnMgmtClusterWithRetry will return if the namespace exists on a cluster, retrying up to a timeout
func namespaceExistsWithRetry(kubeconfigPath, namespace string, sleep, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	ch := make(chan isExecNonZeroExitResult)
	var mostRecentNamespaceExistsWithRetryError error
	var stdout []byte
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				ch <- namespaceExists(kubeconfigPath, namespace)
				time.Sleep(sleep)
			}
		}
	}()
	for {
		select {
		case result := <-ch:
			mostRecentNamespaceExistsWithRetryError = result.err
			stdout = result.stdout
			if mostRecentNamespaceExistsWithRetryError == nil {
				return nil
			}
		case <-ctx.Done():
			fmt.Printf("%s\n", string(stdout))
			return errors.Errorf("namespaceExistsWithRetry timed out: %s\n", mostRecentNamespaceExistsWithRetryError)
		}
	}
}

func getAKSMgmtClusterCreds(name, kubeconfig string) isExecNonZeroExitResult {
	cmd := exec.Command("az", "aks", "get-credentials", "-g", name, "-n", name, "-f", kubeconfig)
	fmt.Printf("%s\n", fmt.Sprintf("$ %s", strings.Join(cmd.Args, " ")))
	out, err := cmd.CombinedOutput()
	return isExecNonZeroExitResult{
		stdout: out,
		err:    err,
	}
}

// getAKSMgmtClusterCredsWithRetry gets AKS cluster kubeconfig, retrying up to a timeout
func getAKSMgmtClusterCredsWithRetry(name, kubeconfig string, sleep, timeout time.Duration) error {
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
				ch <- getAKSMgmtClusterCreds(name, kubeconfig)
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

func getMachineDeploymentReplicas(kubeconfigPath string) isExecNonZeroExitResult {
	cmd := exec.Command("kubectl", "--kubeconfig", kubeconfigPath, "get", "machinedeployments", "-o", "custom-columns=REPLICAS:.status.readyReplicas", "--no-headers=true")
	out, err := cmd.CombinedOutput()
	return isExecNonZeroExitResult{
		stdout: out,
		err:    err,
	}
}

// waitForMachineDeploymentReplicas waits for a minimum number of machinedeployment replicas, retrying up to a timeout
func waitForMachineDeploymentReplicas(num int, kubeconfigPath string, sleep, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	ch := make(chan isExecNonZeroExitResult)
	var mostRecentwaitForMachineDeploymentReplicasWithRetryError error
	var stdout []byte
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				ch <- getMachineDeploymentReplicas(kubeconfigPath)
				time.Sleep(sleep)
			}
		}
	}()
	for {
		select {
		case result := <-ch:
			mostRecentwaitForMachineDeploymentReplicasWithRetryError = result.err
			stdout = result.stdout
			if mostRecentwaitForMachineDeploymentReplicasWithRetryError == nil {
				replicas, err := strconv.Atoi(strings.TrimSuffix(string(stdout), "\n"))
				if err == nil && replicas > 0 {
					return nil
				}
			}
		case <-ctx.Done():
			return errors.Errorf("waitForMachineDeploymentReplicas timed out: %s\n", mostRecentwaitForMachineDeploymentReplicasWithRetryError)
		}
	}
}

func createAKSMgmtClusterResourceGroup(name, location string) isExecNonZeroExitResult {
	cmd := exec.Command("az", "group", "create", "-g", name, "-l", location)
	fmt.Printf("%s\n", fmt.Sprintf("$ %s", strings.Join(cmd.Args, " ")))
	out, err := cmd.CombinedOutput()
	return isExecNonZeroExitResult{
		stdout: out,
		err:    err,
	}
}

func deleteAKSMgmtClusterResourceGroup(name string) isExecNonZeroExitResult {
	cmd := exec.Command("az", "group", "delete", "-g", name, "--no-wait", "-y")
	fmt.Printf("%s\n", fmt.Sprintf("$ %s", strings.Join(cmd.Args, " ")))
	out, err := cmd.CombinedOutput()
	return isExecNonZeroExitResult{
		stdout: out,
		err:    err,
	}
}

// createAKSMgmtClusterResourceGroupWithRetry will create the resource group for an AKS cluster, retrying up to a timeout
func createAKSMgmtClusterResourceGroupWithRetry(name, location string, sleep, timeout time.Duration) error {
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
				ch <- createAKSMgmtClusterResourceGroup(name, location)
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
func deleteAKSMgmtClusterResourceGroupWithRetry(name string, sleep, timeout time.Duration) error {
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
				ch <- deleteAKSMgmtClusterResourceGroup(name)
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

func createAKSMgmtCluster(name string) isExecNonZeroExitResult {
	cmd := exec.Command("az", "aks", "create", "-g", name, "-n", name, "--kubernetes-version", "1.17.7", "-c", "1", "-s", "Standard_B2s")
	fmt.Printf("%s\n", fmt.Sprintf("$ %s", strings.Join(cmd.Args, " ")))
	out, err := cmd.CombinedOutput()
	return isExecNonZeroExitResult{
		stdout: out,
		err:    err,
	}
}

func deleteAKSMgmtCluster(name string) isExecNonZeroExitResult {
	cmd := exec.Command("az", "aks", "delete", "-g", name, "-n", name, "--no-wait", "-y")
	fmt.Printf("%s\n", fmt.Sprintf("$ %s", strings.Join(cmd.Args, " ")))
	out, err := cmd.CombinedOutput()
	return isExecNonZeroExitResult{
		stdout: out,
		err:    err,
	}
}

// createAKSMgmtClusterWithRetry will create an AKS cluster, retrying up to a timeout
func createAKSMgmtClusterWithRetry(name string, sleep, timeout time.Duration) error {
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
				ch <- createAKSMgmtCluster(name)
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
func deleteAKSMgmtClusterWithRetry(name string, sleep, timeout time.Duration) error {
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
				ch <- deleteAKSMgmtCluster(name)
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
