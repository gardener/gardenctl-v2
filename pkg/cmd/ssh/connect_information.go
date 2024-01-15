/*
SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package ssh

import (
	"bytes"
	"fmt"

	operationsv1alpha1 "github.com/gardener/gardener/pkg/apis/operations/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metav1beta1 "k8s.io/apimachinery/pkg/apis/meta/v1beta1"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/klog/v2"
)

// ConnectInformation holds connect information required to establish an SSH connection
// to Shoot worker nodes.
type ConnectInformation struct {
	// Bastion holds information about the bastion host used to connect to the worker nodes.
	Bastion Bastion `json:"bastion"`

	// NodeHostname is the name of the Shoot cluster node that the user wants to connect to.
	NodeHostname string `json:"nodeHostname,omitempty"`

	// NodePrivateKeyFiles is a list of file paths containing the private SSH keys for the worker nodes.
	NodePrivateKeyFiles []PrivateKeyFile `json:"nodePrivateKeyFiles"`

	// Nodes is a list of Node objects containing information about the worker nodes.
	Nodes []Node `json:"nodes"`

	// User is the name of the Shoot cluster node ssh login username
	User string

	// Machines is a list of machines name
	Machines []string
}

var _ fmt.Stringer = &ConnectInformation{}

// Bastion holds information about the bastion host used to connect to the worker nodes.
type Bastion struct {
	// Name is the name of the Bastion resource.
	Name string `json:"name"`
	// Namespace is the namespace of the Bastion resource.
	Namespace string `json:"namespace"`
	// PreferredAddress is the preferred IP address or hostname to use when connecting to the bastion host.
	PreferredAddress string `json:"preferredAddress"`
	// Port is the port to use when connecting to the bastion host.
	Port string `json:"port"`
	// Address holds information about the IP address and hostname of the bastion host.
	Address
	// SSHPublicKeyFile is the full path to the file containing the public SSH key.
	SSHPublicKeyFile PublicKeyFile `json:"publicKeyFile"`
	// SSHPrivateKeyFile is the full path to the file containing the private SSH key.
	SSHPrivateKeyFile PrivateKeyFile `json:"privateKeyFile"`
	// UserKnownHostsFiles is a list of custom known hosts files for the SSH connection to the bastion.
	UserKnownHostsFiles []string `json:"userKnownHostsFiles"`
}

// Node holds information about a worker node.
type Node struct {
	// Name is the name of the worker node.
	Name string `json:"name"`
	// Status is the current status of the worker node.
	Status string `json:"status"`
	// Address holds information about the IP address and hostname of the worker node.
	Address
}

// Address holds information about an IP address and hostname.
type Address struct {
	Hostname string `json:"hostname"`
	IP       string `json:"ip"`
}

var _ fmt.Stringer = &Address{}

func NewConnectInformation(
	bastion *operationsv1alpha1.Bastion,
	bastionPreferredAddress string,
	bastionPort string,
	bastionUserKnownHostsFiles []string,
	nodeHostname string,
	sshPublicKeyFile PublicKeyFile,
	sshPrivateKeyFile PrivateKeyFile,
	nodePrivateKeyFiles []PrivateKeyFile,
	nodes []corev1.Node,
	machines []string,
	user string,
) (*ConnectInformation, error) {
	var nodeList []Node

	for _, node := range nodes {
		n := Node{}
		n.Name = node.Name
		n.Status = "Ready"

		if !isNodeReady(node) {
			n.Status = "Not Ready"
		}

		for _, addr := range node.Status.Addresses {
			switch addr.Type {
			case corev1.NodeInternalIP:
				n.IP = addr.Address

			case corev1.NodeInternalDNS:
				n.Hostname = addr.Address

			// internal names have priority, as we jump via a bastion host,
			// but in case the cloud provider does not offer internal IPs,
			// we fallback to external values

			case corev1.NodeExternalIP:
				if n.IP == "" {
					n.IP = addr.Address
				}

			case corev1.NodeExternalDNS:
				if n.Hostname == "" {
					n.Hostname = addr.Address
				}
			}
		}

		nodeList = append(nodeList, n)
	}

	return &ConnectInformation{
		Bastion: Bastion{
			Name:             bastion.Name,
			Namespace:        bastion.Namespace,
			PreferredAddress: bastionPreferredAddress,
			Port:             bastionPort,
			Address: Address{
				IP:       bastion.Status.Ingress.IP,
				Hostname: bastion.Status.Ingress.Hostname,
			},
			SSHPublicKeyFile:    sshPublicKeyFile,
			SSHPrivateKeyFile:   sshPrivateKeyFile,
			UserKnownHostsFiles: bastionUserKnownHostsFiles,
		},
		NodeHostname:        nodeHostname,
		NodePrivateKeyFiles: nodePrivateKeyFiles,
		Nodes:               nodeList,
		Machines:            machines,
		User:                user,
	}, nil
}

func (p *ConnectInformation) String() string {
	buf := bytes.Buffer{}

	nodeHostname := p.NodeHostname
	placeholderHostname := false

	if nodeHostname == "" {
		nodeHostname = "IP_OR_HOSTNAME"
		placeholderHostname = true

		table := &metav1beta1.Table{
			ColumnDefinitions: []metav1.TableColumnDefinition{
				{
					Name:   "Node Name",
					Type:   "string",
					Format: "name",
				},
				{
					Name: "Status",
					Type: "string",
				},
				{
					Name: "IP",
					Type: "string",
				},
				{
					Name: "Hostname",
					Type: "string",
				},
			},
			Rows: []metav1.TableRow{},
		}

		for _, node := range p.Nodes {
			table.Rows = append(table.Rows, metav1.TableRow{
				Cells: []interface{}{node.Name, node.Status, node.IP, node.Hostname},
			})
		}

		fmt.Fprintf(&buf, "> The shoot cluster has the following nodes:\n\n")

		printer := printers.NewTablePrinter(printers.PrintOptions{})
		if err := printer.PrintObj(table, &buf); err != nil {
			klog.Background().Error(err, "failed to output node table: %w")
			return ""
		}

		if len(p.Machines) != 0 && len(p.Machines) != len(p.Nodes) {
			type empty struct{}

			nodeSets := make(map[string]empty, len(p.Nodes))

			var missingNodes []string

			for _, node := range p.Nodes {
				nodeSets[node.Name] = empty{}
			}

			for _, machine := range p.Machines {
				if _, ok := nodeSets[machine]; !ok {
					missingNodes = append(missingNodes, machine)
				}
			}

			for _, node := range missingNodes {
				table.Rows = append(table.Rows, metav1.TableRow{
					Cells: []interface{}{node, "unknown", "unknown"},
				})
			}
		}

		fmt.Fprintln(&buf, "")
	}

	connectArgs := sshCommandArguments(
		p.Bastion.PreferredAddress,
		p.Bastion.Port,
		p.Bastion.SSHPrivateKeyFile,
		p.Bastion.UserKnownHostsFiles,
		nodeHostname,
		p.NodePrivateKeyFiles,
		p.User,
	)

	fmt.Fprintf(&buf, "> Connect to shoot nodes by using the bastion as a proxy/jump host.\n")
	fmt.Fprintf(&buf, "> Run the following command in a separate terminal:\n")

	if placeholderHostname {
		fmt.Fprintf(&buf, "> Note: The command includes a placeholder (IP_OR_HOSTNAME). Replace it with the actual IP or hostname before running the command.\n\n")
	}

	fmt.Fprintf(&buf, "ssh %s\n\n", connectArgs.String())

	return buf.String()
}

func isNodeReady(node corev1.Node) bool {
	for _, cond := range node.Status.Conditions {
		if cond.Type == corev1.NodeReady {
			return cond.Status == corev1.ConditionTrue
		}
	}

	return false
}

func toAddress(ingress *corev1.LoadBalancerIngress) *Address {
	if ingress == nil {
		return nil
	}

	return &Address{
		Hostname: ingress.Hostname,
		IP:       ingress.IP,
	}
}

func (a *Address) String() string {
	switch {
	case a.Hostname != "" && a.IP != "":
		return fmt.Sprintf("%s (%s)", a.IP, a.Hostname)
	case a.Hostname != "":
		return a.Hostname
	default:
		return a.IP
	}
}
