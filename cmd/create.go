// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package cmd

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/Azure/aks-engine/pkg/swagger/models"
	"github.com/Azure/aks-engine/pkg/v2/engine"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/briandowns/spinner"
	"github.com/fatih/color"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

const (
	createName             = "create"
	createShortDescription = "Create a new Kubernetes cluster"
	createLongDescription  = "Create a new Kubernetes cluster, enabled with cluster-api for cluster lifecycle management"
	calicoSpec             = "https://raw.githubusercontent.com/kubernetes-sigs/cluster-api-provider-azure/master/templates/addons/calico.yaml"
)

const (
	reconcile    bool = true
	validateOnce bool = false
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
	f.StringVar(&cc.AzureEnvironment, "azure-env", engine.DefaultAzureEnvironment, "the target Azure cloud")
	f.StringVarP(&cc.ClusterName, "cluster-name", "n", "", "name of cluster")
	f.StringVarP(&cc.VnetName, "vnet-name", "", "", "name of vnet the cluster will reside in")
	f.StringVarP(&cc.ResourceGroup, "resource-group", "g", "", "name of resource group the cluster IaaS will reside in")
	f.StringVarP(&cc.Location, "location", "l", engine.DefaultLocation, "Azure region cluster IaaS will reside in")
	f.StringVarP(&cc.ControlPlaneVMType, "control-plan-vm-sku", "", engine.DefaultControlPlaneVMType, "SKU for control plane VMs, default is Standard_D2s_v3")
	f.StringVarP(&cc.NodeVMType, "node-vm-sku", "", engine.DefaultNodeVMType, "SKU for node VMs, default is Standard_D2s_v3")
	f.StringVarP(&cc.SSHPublicKey, "ssh-public-key", "", "", "SSH public key to install for remote access to VMs")
	f.StringVarP(&cc.KubernetesVersion, "kubernetes-version", "v", engine.DefaultKubernetesVersion, "Kubernetes version to install, default is 1.17.8")
	f.IntVarP(&cc.ControlPlaneNodes, "control-plane-nodes", "", engine.DefaultControlPlaneNodes, "number of control plane nodes, default is 1")
	f.IntVarP(&cc.Nodes, "nodes", "", engine.DefaultNodes, "number of worker nodes, default is 1")

	return createCmd
}

type phaseReady func(engine.Cluster) bool
type phaseMessage func(engine.Cluster)

func validateClusterConfig(c engine.Cluster) bool {
	return c.GetClusterConfig() != nil
}

func validateMgmtClusterConfig(c engine.Cluster) bool {
	return c.GetMgmtClusterName() != "" && c.GetMgmtClusterURL() != ""
}

func validateMgmtClusterReady(c engine.Cluster) bool {
	if err := c.IsMgmtClusterReady(1*time.Second, 20*time.Minute); err == nil {
		return true
	}
	return false
}

func validateMgmtClusterForClusterAPIReadiness(c engine.Cluster) bool {
	return c.MgmtClusterNeedsClusterAPIInit() != nil
}

func validateMgmtClusterNeedsClusterAPIComponents(c engine.Cluster) bool {
	return c.MgmtClusterNeedsClusterAPIInit() != nil && to.Bool(c.MgmtClusterNeedsClusterAPIInit())
}

func validateMgmtClusterReadyForClusterAPI(c engine.Cluster) bool {
	return c.IsMgmtClusterAPIReady() != nil && to.Bool(c.IsMgmtClusterAPIReady())
}

func validateMgmtClusterIsProvisioning(c engine.Cluster) bool {
	return c.GetMgmtClusterName() != "" && to.Bool(c.IsMgmtClusterProvisioning())
}

func validateIsProvisioning(c engine.Cluster) bool {
	return to.Bool(c.IsProvisioning())
}

func validateIsPivoting(c engine.Cluster) bool {
	if to.Bool(c.NeedsPivot()) && to.Bool(c.IsProvisioning()) {
		err := c.IsReady(3*time.Second, 1*time.Second)
		return err == nil
	}
	return false
}

func validatePivotComplete(c engine.Cluster) bool {
	return to.Bool(c.IsPivotComplete())
}

func validateCleanupComplete(c engine.Cluster) bool {
	return to.Bool(c.IsCleanupComplete())
}

func validateReady(ctx context.Context, c engine.Cluster, ready phaseReady) bool {
	for {
		select {
		case <-ctx.Done():
			return false
		default:
			if ready(c) {
				return true
			}
			time.Sleep(1 * time.Second)
		}
	}
}

func validatePhaseProgress(ctx context.Context, c engine.Cluster, input, output chan struct{}, ready phaseReady, validateContinually bool, enterMessage, inputMessage, outputMessage phaseMessage) {
	go func() {
		green := color.New(color.FgGreen).SprintFunc()
		if enterMessage != nil {
			fmt.Printf("\n")
			enterMessage(c)
			fmt.Printf("\n")
		}
		for {
			select {
			case <-ctx.Done():
				return
			case <-input:
				if inputMessage != nil {
					inputMessage(c)
				}
				if validateContinually {
					if ready == nil || validateReady(ctx, c, ready) {
						if inputMessage != nil {
							fmt.Printf(" %s\n", green("✓"))
						}
						if outputMessage != nil {
							fmt.Printf("\n")
							outputMessage(c)
							fmt.Printf("\n")
						}
						output <- struct{}{}
						return
					}
				} else {
					if ready == nil || ready(c) {
						if outputMessage != nil {
							fmt.Printf("\n")
							outputMessage(c)
							fmt.Printf("\n")
						}
						output <- struct{}{}
					}
					return
				}
			}
		}
	}()
}

func waitForCondition(ctx context.Context, c engine.Cluster, output chan struct{}, ready phaseReady, enterMessage, outputMessage phaseMessage) {
	go func() {
		if enterMessage != nil {
			enterMessage(c)
		}
		for {
			select {
			case <-ctx.Done():
				return
			default:
				if ready == nil || ready(c) {
					if outputMessage != nil {
						fmt.Printf("\n")
						outputMessage(c)
						fmt.Printf("\n")
					}
					if output != nil {
						output <- struct{}{}
					}
					return
				}
			}
			time.Sleep(100 * time.Millisecond)
		}
	}()
}

func (cc *CreateCmd) Run() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return errors.Errorf("Unable to get home dir: %s\n", err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		return errors.Errorf("Unable to get current working dir: %s\n", err)
	}
	var mgmtClusterKubeConfig string
	if cc.MgmtClusterKubeConfigPath != "" {
		b, err := ioutil.ReadFile(cc.MgmtClusterKubeConfigPath)
		if err != nil {
			return errors.Errorf("Unable to open mgmt cluster kubeconfig file for reading: %s\n", err)
		}
		mgmtClusterKubeConfig = string(b)
	}
	cluster := engine.NewCluster(models.CreateData{
		MgmtClusterKubeConfig: to.StringPtr(mgmtClusterKubeConfig),
		SubscriptionID:        to.StringPtr(cc.SubscriptionID),
		TenantID:              to.StringPtr(cc.TenantID),
		ClientID:              to.StringPtr(cc.ClientID),
		ClientSecret:          to.StringPtr(cc.ClientSecret),
		AzureEnvironment:      to.StringPtr(cc.AzureEnvironment),
		ClusterName:           to.StringPtr(cc.ClusterName),
		VnetName:              to.StringPtr(cc.VnetName),
		ResourceGroup:         to.StringPtr(cc.ResourceGroup),
		Location:              to.StringPtr(cc.Location),
		ControlPlaneVMType:    to.StringPtr(cc.ControlPlaneVMType),
		NodeVMType:            to.StringPtr(cc.NodeVMType),
		SSHPublicKey:          to.StringPtr(cc.SSHPublicKey),
		KubernetesVersion:     to.StringPtr(cc.KubernetesVersion),
		ControlPlaneNodes:     int64(cc.ControlPlaneNodes),
		Nodes:                 int64(cc.Nodes),
	}, "aks-engine.log")
	yellowbold := color.New(color.FgYellow, color.Bold).SprintFunc()
	magentabold := color.New(color.FgMagenta, color.Bold).SprintFunc()
	bold := color.New(color.FgWhite, color.Bold).SprintFunc()
	s := spinner.New(spinner.CharSets[4], 100*time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	clusterCreateDoneChannel := make(chan error)
	mgmtClusterProvisioning := make(chan struct{})
	mgmtClusterConfigValidated := make(chan struct{})
	mgmtClusterReady := make(chan struct{})
	mgmtClusterEvaluatedForClusterAPIReadiness := make(chan struct{})
	mgmtClusterNeedsClusterAPI := make(chan struct{})
	mgmtClusterAPIInstalled := make(chan struct{})
	clusterConfigGenerated := make(chan struct{})
	clusterProvisioning := make(chan struct{})
	pivotComplete := make(chan struct{})
	cleanupComplete := make(chan struct{})
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				clusterCreateDoneChannel <- cluster.Create()
				if err := clusterCreateDoneChannel; err != nil {
					return
				}
			}
		}
	}()
	var mgmtClusterProvisionInfoMessage phaseMessage = func(c engine.Cluster) {
		fmt.Printf("No management cluster provided: a temporary AKS Kubernetes cluster %s will be created to enable a cluster-api-powered installation %s....\n", magentabold(cluster.GetMgmtClusterName()), bold("☕"))
		if !s.Active() {
			s.Start()
		}
	}
	var waitForMgmtClusterReadyMessage phaseMessage = func(c engine.Cluster) {
		if s.Active() {
			s.Stop()
		}
		fmt.Printf("Checking if management cluster %s is ready at %s....", magentabold(cluster.GetMgmtClusterName()), bold(cluster.GetMgmtClusterURL()))
	}
	var waitForMgmtClusterAPIReadyMessage phaseMessage = func(c engine.Cluster) {
		fmt.Printf("Checking if management cluster %s is cluster-api-ready....", magentabold(cluster.GetMgmtClusterName()))
	}
	var installingClusterAPIOnMgmtClusterMessage phaseMessage = func(c engine.Cluster) {
		fmt.Printf("Installing cluster-api components on management cluster %s.... ", magentabold(cluster.GetMgmtClusterName()))
	}
	var clusterConfigGeneratedInfoMessage phaseMessage = func(c engine.Cluster) {
		fmt.Printf("Cluster config generated for new cluster %s and saved to %s.", yellowbold(cluster.GetName()), bold(fmt.Sprintf("%s/%s.yaml", cwd, cluster.GetName())))
	}
	var clusterProvisioningMessage phaseMessage = func(c engine.Cluster) {
		fmt.Printf("Creating cluster %s on cluster-api management cluster %s....", yellowbold(cluster.GetName()), magentabold(cluster.GetMgmtClusterName()))
	}
	var clusterProvisioningInfoMessage phaseMessage = func(c engine.Cluster) {
		fmt.Printf("Management cluster %s is provisioning your new cluster %s! %s....\n", magentabold(cluster.GetMgmtClusterName()), yellowbold(cluster.GetName()), bold("☕"))
		if !s.Active() {
			s.Start()
		}
	}
	var clusterPivotingInfoMessage phaseMessage = func(c engine.Cluster) {
		if s.Active() {
			s.Stop()
		}
		fmt.Printf("Installing cluster-api interfaces on new cluster %s %s....", yellowbold(cluster.GetName()), bold("☕"))
		if !s.Active() {
			s.Start()
		}
	}
	var cleaningUpInfoMessage phaseMessage = func(c engine.Cluster) {
		fmt.Printf("Cleaning up temporary AKS Kubernetes cluster...")
	}
	if cc.MgmtClusterKubeConfigPath == "" {
		waitForCondition(ctx, cluster, mgmtClusterProvisioning, validateMgmtClusterIsProvisioning, nil, mgmtClusterProvisionInfoMessage)
	}
	waitForCondition(ctx, cluster, mgmtClusterConfigValidated, validateMgmtClusterConfig, nil, nil)
	validatePhaseProgress(ctx, cluster, mgmtClusterConfigValidated, mgmtClusterReady, validateMgmtClusterReady, reconcile, nil, waitForMgmtClusterReadyMessage, nil)
	validatePhaseProgress(ctx, cluster, mgmtClusterReady, mgmtClusterEvaluatedForClusterAPIReadiness, validateMgmtClusterForClusterAPIReadiness, reconcile, nil, waitForMgmtClusterAPIReadyMessage, nil)
	validatePhaseProgress(ctx, cluster, mgmtClusterEvaluatedForClusterAPIReadiness, mgmtClusterNeedsClusterAPI, validateMgmtClusterNeedsClusterAPIComponents, validateOnce, nil, nil, nil)
	validatePhaseProgress(ctx, cluster, mgmtClusterNeedsClusterAPI, mgmtClusterAPIInstalled, validateMgmtClusterReadyForClusterAPI, reconcile, nil, installingClusterAPIOnMgmtClusterMessage, nil)
	waitForCondition(ctx, cluster, clusterConfigGenerated, validateClusterConfig, nil, clusterConfigGeneratedInfoMessage)
	validatePhaseProgress(ctx, cluster, clusterConfigGenerated, nil, nil, reconcile, nil, clusterProvisioningMessage, nil)
	waitForCondition(ctx, cluster, clusterProvisioning, validateIsProvisioning, nil, clusterProvisioningInfoMessage)
	waitForCondition(ctx, cluster, nil, validateIsPivoting, nil, clusterPivotingInfoMessage)
	waitForCondition(ctx, cluster, pivotComplete, validatePivotComplete, nil, nil)
	validatePhaseProgress(ctx, cluster, pivotComplete, cleanupComplete, validateCleanupComplete, reconcile, nil, cleaningUpInfoMessage, nil)
	for {
		select {
		case err := <-clusterCreateDoneChannel:
			if err != nil {
				return err
			}
			s.Stop()
			fmt.Printf("\nYour new cluster %s is ready!\n", yellowbold(cluster.GetName()))
			fmt.Printf("\nE.g.:\n")
			cmd := exec.Command("kubectl", fmt.Sprintf("--kubeconfig=%s", fmt.Sprintf("%s/.kube/%s.kubeconfig", homeDir, cluster.GetName())), "get", "nodes", "-o", "wide")
			fmt.Printf("%s\n\n", bold(fmt.Sprintf("$ %s", strings.Join(cmd.Args, " "))))
			return nil
		case <-ctx.Done():
			return errors.Errorf("create cluster timed out")
		}
	}
}
