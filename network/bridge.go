package network

import (
	"fmt"
	"net"
	"os/exec"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"
)

type BridgeNetworkDriver struct{}

func (b *BridgeNetworkDriver) Create(subnet string, name string) (*Network, error) {
	ip, ipRange, _ := net.ParseCIDR(subnet)
	ipRange.IP = ip

	n := &Network{
		Name:    name,
		IpRange: ipRange,
		Driver:  b.Name(),
	}

	err := b.initBridge(n)
	if err != nil {
		log.Errorf("error init bridge: %v", err)
	}

	return n, nil
}

func (b *BridgeNetworkDriver) Delete(network Network) error {
	bridgeName := network.Name
	br, err := netlink.LinkByName(bridgeName)
	if err != nil {
		return err
	}

	return netlink.LinkDel(br)
}

func (b *BridgeNetworkDriver) Connect(network *Network, endpoint *Endpoint) error {
	bridgeName := network.Name
	br, err := netlink.LinkByName(bridgeName)
	if err != nil {
		return err
	}

	la := netlink.NewLinkAttrs()
	la.Name = endpoint.ID[:5]
	la.MasterIndex = br.Attrs().Index

	endpoint.Device = netlink.Veth{
		LinkAttrs: la,
		PeerName:  "cif-" + endpoint.ID[:5],
	}

	if err = netlink.LinkAdd(&endpoint.Device); err != nil {
		return fmt.Errorf("error Add Endpoint Device: %v", err)
	}

	if err = netlink.LinkSetUp(&endpoint.Device); err != nil {
		return fmt.Errorf("error Add Endpoint Device: %v", err)
	}
	return nil
}

func (b *BridgeNetworkDriver) Disconnect(network *Network, endpoint *Endpoint) error {
	return nil
}

func (b *BridgeNetworkDriver) Name() string {
	return "bridge"
}

func (b *BridgeNetworkDriver) initBridge(n *Network) error {
	bridgeName := n.Name

	// create bridge virtual device
	if err := createBridgeInterface(bridgeName); err != nil {
		log.Errorf("error add bridge: %s, error %v", bridgeName, err)
	}

	// set bridge IP and Route
	gatewayIP := *n.IpRange
	gatewayIP.IP = n.IpRange.IP
	if err := setInterfaceIP(bridgeName, gatewayIP.String()); err != nil {
		return fmt.Errorf("error assigning address: %s on bridge: %s with an error of: %v", gatewayIP, bridgeName, err)
	}
	// set up bridge device
	if err := setInterfaceUp(bridgeName); err != nil {
		return fmt.Errorf("error set bridge up: %s, Error: %v", bridgeName, err)
	}

	// set iptables' NAT rules
	if err := setupIPTables(bridgeName, n.IpRange); err != nil {
		return fmt.Errorf("error setting iptables for %s: %v", bridgeName, err)
	}

	return nil
}

func createBridgeInterface(bridgeName string) error {
	_, err := net.InterfaceByName(bridgeName)
	if err == nil || !strings.Contains(err.Error(), "no such network interface") {
		return err
	}

	// Initialize netlink's Link base object, the name of the Link is the name of the Bridge virtual device.
	la := netlink.NewLinkAttrs()
	la.Name = bridgeName

	br := &netlink.Bridge{LinkAttrs: la}
	// equal to `ip link add xxx`
	if err := netlink.LinkAdd(br); err != nil {
		return fmt.Errorf("bridge creation failed for bridge %s: %v", bridgeName, err)
	}

	return nil
}

// setInterfaceIP set the IP address of a network interface, e.g. setInterfaceIP(testbridge, "192 168.0.1124")
func setInterfaceIP(bridgeName string, rawIP string) error {
	iface, err := netlink.LinkByName(bridgeName)
	if err != nil {
		return fmt.Errorf("err get interface %v", err)
	}

	// Since netlink.ParseIPNet is a wrapper around net.ParseCIDR,
	// we can integrate IP and net in the return value of net.ParseCIDR.
	// The ipNet in the returned value contains both the segment information,
	// 192 168.0.0/24, and the original worker, 192.168.0.1.
	ipNet, err := netlink.ParseIPNet(rawIP)
	if err != nil {
		return err
	}
	// Configure the address of the network interface via netlink .AddrAdd,
	// which is equivalent to the addr add xxx command.
	//
	// Also, if the address is configured with the segment information, e.g. 192.168.0 0/24,
	// the routing table 192 168 0/24 is configured to forward the address to the bridge.
	//
	// The routing table 192 168 0/24 is also configured for forwarding to the bridge's network interface.
	addr := &netlink.Addr{IPNet: ipNet}

	return netlink.AddrAdd(iface, addr)
}

// set up network interface
func setInterfaceUp(bridgeName string) error {
	iface, err := netlink.LinkByName(bridgeName)
	if err != nil {
		return fmt.Errorf("error retrieving a link named [ %s ]: %v", iface.Attrs().Name, err)
	}
	// Set the state of the direct port to "UP" via the "LinkSetUp" method of "net link".
	// Equivalent to the ip link set xxx up command.
	if err := netlink.LinkSetUp(iface); err != nil {
		return fmt.Errorf("error enabling nterface for %s: %v", bridgeName, err)
	}

	return nil
}

func setupIPTables(bridgeName string, subnet *net.IPNet) error {
	// command to create iptables
	// iptables -t nat -A POSTROUTING -s <bridgeName> ! -o <bridgeName> -] MASQUERADE
	iptablesCmd := fmt.Sprintf("-t nat -A POSTROUTING -s %s ! -o %s -j MASQUERADE", subnet.String(), bridgeName)
	cmd := exec.Command("iptables", strings.Split(iptablesCmd, " ")...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Errorf("iptables Output, %v", output)
	}

	return nil
}
