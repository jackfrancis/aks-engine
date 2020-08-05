// Code generated by go-swagger; DO NOT EDIT.

package models

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"github.com/go-openapi/errors"
	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/swag"
	"github.com/go-openapi/validate"
)

// CreateData create data
//
// swagger:model createData
type CreateData struct {

	// azure environment
	// Min Length: 0
	AzureEnvironment *string `json:"azureEnvironment,omitempty"`

	// client ID
	// Min Length: 0
	ClientID *string `json:"clientID,omitempty"`

	// client secret
	// Min Length: 0
	ClientSecret *string `json:"clientSecret,omitempty"`

	// cluster name
	// Min Length: 0
	ClusterName *string `json:"clusterName,omitempty"`

	// control plane nodes
	// Min Length: 0
	ControlPlaneNodes int64 `json:"controlPlaneNodes,omitempty"`

	// control plane VM type
	// Min Length: 0
	ControlPlaneVMType *string `json:"controlPlaneVMType,omitempty"`

	// kubernetes version
	// Min Length: 0
	KubernetesVersion *string `json:"kubernetesVersion,omitempty"`

	// location
	// Min Length: 0
	Location *string `json:"location,omitempty"`

	// mgmt cluster kube config
	// Min Length: 0
	MgmtClusterKubeConfig *string `json:"mgmtClusterKubeConfig,omitempty"`

	// node VM type
	// Min Length: 0
	NodeVMType *string `json:"nodeVMType,omitempty"`

	// nodes
	// Min Length: 0
	Nodes int64 `json:"nodes,omitempty"`

	// resource group
	// Min Length: 0
	ResourceGroup *string `json:"resourceGroup,omitempty"`

	// ssh public key
	// Min Length: 0
	SSHPublicKey *string `json:"sshPublicKey,omitempty"`

	// subscription ID
	// Min Length: 0
	SubscriptionID *string `json:"subscriptionID,omitempty"`

	// tenant ID
	// Min Length: 0
	TenantID *string `json:"tenantID,omitempty"`

	// vnet name
	// Min Length: 0
	VnetName *string `json:"vnetName,omitempty"`
}

// Validate validates this create data
func (m *CreateData) Validate(formats strfmt.Registry) error {
	var res []error

	if err := m.validateAzureEnvironment(formats); err != nil {
		res = append(res, err)
	}

	if err := m.validateClientID(formats); err != nil {
		res = append(res, err)
	}

	if err := m.validateClientSecret(formats); err != nil {
		res = append(res, err)
	}

	if err := m.validateClusterName(formats); err != nil {
		res = append(res, err)
	}

	if err := m.validateControlPlaneNodes(formats); err != nil {
		res = append(res, err)
	}

	if err := m.validateControlPlaneVMType(formats); err != nil {
		res = append(res, err)
	}

	if err := m.validateKubernetesVersion(formats); err != nil {
		res = append(res, err)
	}

	if err := m.validateLocation(formats); err != nil {
		res = append(res, err)
	}

	if err := m.validateMgmtClusterKubeConfig(formats); err != nil {
		res = append(res, err)
	}

	if err := m.validateNodeVMType(formats); err != nil {
		res = append(res, err)
	}

	if err := m.validateNodes(formats); err != nil {
		res = append(res, err)
	}

	if err := m.validateResourceGroup(formats); err != nil {
		res = append(res, err)
	}

	if err := m.validateSSHPublicKey(formats); err != nil {
		res = append(res, err)
	}

	if err := m.validateSubscriptionID(formats); err != nil {
		res = append(res, err)
	}

	if err := m.validateTenantID(formats); err != nil {
		res = append(res, err)
	}

	if err := m.validateVnetName(formats); err != nil {
		res = append(res, err)
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}

func (m *CreateData) validateAzureEnvironment(formats strfmt.Registry) error {

	if swag.IsZero(m.AzureEnvironment) { // not required
		return nil
	}

	if err := validate.MinLength("azureEnvironment", "body", string(*m.AzureEnvironment), 0); err != nil {
		return err
	}

	return nil
}

func (m *CreateData) validateClientID(formats strfmt.Registry) error {

	if swag.IsZero(m.ClientID) { // not required
		return nil
	}

	if err := validate.MinLength("clientID", "body", string(*m.ClientID), 0); err != nil {
		return err
	}

	return nil
}

func (m *CreateData) validateClientSecret(formats strfmt.Registry) error {

	if swag.IsZero(m.ClientSecret) { // not required
		return nil
	}

	if err := validate.MinLength("clientSecret", "body", string(*m.ClientSecret), 0); err != nil {
		return err
	}

	return nil
}

func (m *CreateData) validateClusterName(formats strfmt.Registry) error {

	if swag.IsZero(m.ClusterName) { // not required
		return nil
	}

	if err := validate.MinLength("clusterName", "body", string(*m.ClusterName), 0); err != nil {
		return err
	}

	return nil
}

func (m *CreateData) validateControlPlaneNodes(formats strfmt.Registry) error {

	if swag.IsZero(m.ControlPlaneNodes) { // not required
		return nil
	}

	if err := validate.MinLength("controlPlaneNodes", "body", string(m.ControlPlaneNodes), 0); err != nil {
		return err
	}

	return nil
}

func (m *CreateData) validateControlPlaneVMType(formats strfmt.Registry) error {

	if swag.IsZero(m.ControlPlaneVMType) { // not required
		return nil
	}

	if err := validate.MinLength("controlPlaneVMType", "body", string(*m.ControlPlaneVMType), 0); err != nil {
		return err
	}

	return nil
}

func (m *CreateData) validateKubernetesVersion(formats strfmt.Registry) error {

	if swag.IsZero(m.KubernetesVersion) { // not required
		return nil
	}

	if err := validate.MinLength("kubernetesVersion", "body", string(*m.KubernetesVersion), 0); err != nil {
		return err
	}

	return nil
}

func (m *CreateData) validateLocation(formats strfmt.Registry) error {

	if swag.IsZero(m.Location) { // not required
		return nil
	}

	if err := validate.MinLength("location", "body", string(*m.Location), 0); err != nil {
		return err
	}

	return nil
}

func (m *CreateData) validateMgmtClusterKubeConfig(formats strfmt.Registry) error {

	if swag.IsZero(m.MgmtClusterKubeConfig) { // not required
		return nil
	}

	if err := validate.MinLength("mgmtClusterKubeConfig", "body", string(*m.MgmtClusterKubeConfig), 0); err != nil {
		return err
	}

	return nil
}

func (m *CreateData) validateNodeVMType(formats strfmt.Registry) error {

	if swag.IsZero(m.NodeVMType) { // not required
		return nil
	}

	if err := validate.MinLength("nodeVMType", "body", string(*m.NodeVMType), 0); err != nil {
		return err
	}

	return nil
}

func (m *CreateData) validateNodes(formats strfmt.Registry) error {

	if swag.IsZero(m.Nodes) { // not required
		return nil
	}

	if err := validate.MinLength("nodes", "body", string(m.Nodes), 0); err != nil {
		return err
	}

	return nil
}

func (m *CreateData) validateResourceGroup(formats strfmt.Registry) error {

	if swag.IsZero(m.ResourceGroup) { // not required
		return nil
	}

	if err := validate.MinLength("resourceGroup", "body", string(*m.ResourceGroup), 0); err != nil {
		return err
	}

	return nil
}

func (m *CreateData) validateSSHPublicKey(formats strfmt.Registry) error {

	if swag.IsZero(m.SSHPublicKey) { // not required
		return nil
	}

	if err := validate.MinLength("sshPublicKey", "body", string(*m.SSHPublicKey), 0); err != nil {
		return err
	}

	return nil
}

func (m *CreateData) validateSubscriptionID(formats strfmt.Registry) error {

	if swag.IsZero(m.SubscriptionID) { // not required
		return nil
	}

	if err := validate.MinLength("subscriptionID", "body", string(*m.SubscriptionID), 0); err != nil {
		return err
	}

	return nil
}

func (m *CreateData) validateTenantID(formats strfmt.Registry) error {

	if swag.IsZero(m.TenantID) { // not required
		return nil
	}

	if err := validate.MinLength("tenantID", "body", string(*m.TenantID), 0); err != nil {
		return err
	}

	return nil
}

func (m *CreateData) validateVnetName(formats strfmt.Registry) error {

	if swag.IsZero(m.VnetName) { // not required
		return nil
	}

	if err := validate.MinLength("vnetName", "body", string(*m.VnetName), 0); err != nil {
		return err
	}

	return nil
}

// MarshalBinary interface implementation
func (m *CreateData) MarshalBinary() ([]byte, error) {
	if m == nil {
		return nil, nil
	}
	return swag.WriteJSON(m)
}

// UnmarshalBinary interface implementation
func (m *CreateData) UnmarshalBinary(b []byte) error {
	var res CreateData
	if err := swag.ReadJSON(b, &res); err != nil {
		return err
	}
	*m = res
	return nil
}