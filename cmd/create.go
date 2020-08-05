// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package cmd

import (
	"context"
	"fmt"
	"io/ioutil"
	"time"

	"github.com/Azure/aks-engine/pkg/swagger/models"
	"github.com/Azure/aks-engine/pkg/v2/engine"
	"github.com/Azure/go-autorest/autorest/to"
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

func (cc *CreateCmd) Run() error {
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
	})
	//green := color.New(color.FgGreen).SprintFunc()
	yellowbold := color.New(color.FgYellow, color.Bold).SprintFunc()
	magentabold := color.New(color.FgMagenta, color.Bold).SprintFunc()
	//bold := color.New(color.FgWhite, color.Bold).SprintFunc()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	clusterCreateDoneChannel := make(chan error)
	mgmtclusterReadyChannel := make(chan error)
	clusterReadyChannel := make(chan error)
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
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
		defer cancel()
		mgmtClusterNeedsClusterAPIInitChannel := make(chan *bool)
		for {
			select {
			case <-ctx.Done():
				return
			default:
				mgmtClusterNeedsClusterAPIInitChannel <- cluster.MgmtClusterNeedsClusterAPIInit()
				if needsInit := <-mgmtClusterNeedsClusterAPIInitChannel; needsInit != nil {
					if to.Bool(needsInit) {
						fmt.Printf("\nWill create cluster-api components on mgmt cluster.\n")
					}
					return
				}
				time.Sleep(1 * time.Second)
			}
		}
	}()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
		defer cancel()
		needsPivotChannel := make(chan *bool)
		for {
			select {
			case <-ctx.Done():
				return
			default:
				needsPivotChannel <- cluster.NeedsPivot()
				if needsPivot := <-needsPivotChannel; needsPivot != nil {
					if to.Bool(needsPivot) {
						fmt.Printf("\nWill install cluster-api components on target cluster.\n")
					}
					return
				}
				time.Sleep(1 * time.Second)
			}
		}
	}()
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				mgmtclusterReadyChannel <- cluster.IsMgmtClusterReady(1*time.Second, 20*time.Minute)
				if err := <-mgmtclusterReadyChannel; err == nil {
					return
				}
			}
		}
	}()
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				clusterReadyChannel <- cluster.IsReady(1*time.Second, 20*time.Minute)
				if ready := <-clusterReadyChannel; ready == nil {
					return
				}
			}
		}
	}()
	if cc.MgmtClusterKubeConfigPath == "" {
		fmt.Printf("\nWill create a temporary AKS management cluster to install cluster-api management components.\n")
	}
	for {
		select {
		case err := <-clusterCreateDoneChannel:
			if err != nil {
				return err
			}
			fmt.Printf("\nYour new cluster %s is ready!\n", yellowbold(cluster.GetName()))
			fmt.Printf("\nE.g.:\n")
			//cmd = exec.Command("kubectl", fmt.Sprintf("--kubeconfig=%s", c.createStatus.kubeConfigPath), "get", "nodes", "-o", "wide")
			//fmt.Printf("%s\n\n", bold(fmt.Sprintf("$ %s", strings.Join(cmd.Args, " "))))
			return nil
		case mgmtClusterIsReady := <-mgmtclusterReadyChannel:
			if mgmtClusterIsReady == nil {
				fmt.Printf("\nmanagement cluster %s is cluster-api-ready.\n", magentabold(cluster.GetMgmtClusterName()))
			}
		case <-ctx.Done():
			return errors.Errorf("create cluster timed out")
		}
	}
}
