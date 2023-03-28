/*
SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package ssh

import (
	"bytes"
	"fmt"

	operationsv1alpha1 "github.com/gardener/gardener/pkg/apis/operations/v1alpha1"
	"github.com/spf13/pflag"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metav1beta1 "k8s.io/apimachinery/pkg/apis/meta/v1beta1"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/klog/v2"
)

type PublicKeyFile string

var (
	_ pflag.Value  = (*PublicKeyFile)(nil)
	_ fmt.Stringer = (*PublicKeyFile)(nil)
)

func (s *PublicKeyFile) Set(val string) error {
	*s = PublicKeyFile(val)

	return nil
}

func (s *PublicKeyFile) Type() string {
	return "string"
}

func (s *PublicKeyFile) String() string {
	return string(*s)
}

type PrivateKeyFile string

var _ fmt.Stringer = (*PrivateKeyFile)(nil)

func (s *PrivateKeyFile) String() string {
	return string(*s)
}

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
	// Address holds information about the IP address and hostname of the bastion host.
	Address
	// SSHPublicKeyFile is the full path to the file containing the public SSH key.
	SSHPublicKeyFile PublicKeyFile `json:"publicKeyFile"`
	// SSHPrivateKeyFile is the full path to the file containing the private SSH key.
	SSHPrivateKeyFile PrivateKeyFile `json:"privateKeyFile"`
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

func NewConnectInformation(bastion *operationsv1alpha1.Bastion, nodeHostname string, sshPublicKeyFile PublicKeyFile, sshPrivateKeyFile PrivateKeyFile, nodePrivateKeyFiles []PrivateKeyFile, nodes []corev1.Node) (*ConnectInformation, error) {
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
			PreferredAddress: preferredBastionAddress(bastion),
			Address: Address{
				IP:       bastion.Status.Ingress.IP,
				Hostname: bastion.Status.Ingress.Hostname,
			},
			SSHPublicKeyFile:  sshPublicKeyFile,
			SSHPrivateKeyFile: sshPrivateKeyFile,
		},
		NodeHostname:        nodeHostname,
		NodePrivateKeyFiles: nodePrivateKeyFiles,
		Nodes:               nodeList,
	}, nil
}

func (p *ConnectInformation) String() string {
	buf := bytes.Buffer{}

	nodeHostname := p.NodeHostname
	if nodeHostname == "" {
		nodeHostname = "IP_OR_HOSTNAME"

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

		fmt.Fprintln(&buf, "The shoot cluster has the following nodes:")
		fmt.Fprintln(&buf, "")

		printer := printers.NewTablePrinter(printers.PrintOptions{})
		if err := printer.PrintObj(table, &buf); err != nil {
			klog.Background().Error(err, "failed to output node table: %w")
			return ""
		}

		fmt.Fprintln(&buf, "")
	}

	connectCmd := sshCommandLine(p.Bastion.SSHPrivateKeyFile, p.Bastion.PreferredAddress, p.NodePrivateKeyFiles, nodeHostname)

	fmt.Fprintln(&buf, "Connect to shoot nodes by using the bastion as a proxy/jump host, for example:")
	fmt.Fprintln(&buf, "")
	fmt.Fprintln(&buf, connectCmd)
	fmt.Fprintln(&buf, "")

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

func toAdress(ingress *corev1.LoadBalancerIngress) *Address {
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
