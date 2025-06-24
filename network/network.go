package network

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"text/tabwriter"

	"github.com/bytedance/sonic"
	log "github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"

	"github.com/crazyfrank/zdocker/container"
)

var (
	defaultNetworkPath = "/var/run/zdocker/network/network/"
	drivers            = map[string]NetworkDriver{}
	networks           = map[string]*Network{}
)

type Network struct {
	Name    string     `json:"name"`    // network name
	IpRange *net.IPNet `json:"ipRange"` // network segment
	Driver  string     `json:"driver"`  // network driver name
}

type Endpoint struct {
	ID          string           `json:"id"`
	Device      netlink.Veth     `json:"device"`
	IPAddress   net.IP           `json:"ip"`
	MacAddress  net.HardwareAddr `json:"mac"`
	PortMapping []string         `json:"portMapping"`
	Network     *Network
}

type NetworkDriver interface {
	Name() string
	Create(subnet string, name string) (*Network, error)
	Delete(network Network) error
	Connect(network *Network, endpoint *Endpoint) error
	Disconnect(network *Network, endpoint *Endpoint) error
}

func InitNetwork() error {
	var bridgeDriver = BridgeNetworkDriver{}
	drivers[bridgeDriver.Name()] = &bridgeDriver

	if _, err := os.Stat(defaultNetworkPath); err != nil {
		if os.IsNotExist(err) {
			os.MkdirAll(defaultNetworkPath, 0644)
		} else {
			return err
		}
	}

	filepath.Walk(defaultNetworkPath, func(nwPath string, info os.FileInfo, err error) error {
		if strings.HasSuffix(nwPath, "/") {
			return nil
		}
		_, nwName := path.Split(nwPath)
		nw := &Network{
			Name: nwName,
		}

		if err := nw.load(nwPath); err != nil {
			log.Errorf("error load network: %s", err)
		}

		networks[nwName] = nw
		return nil
	})

	return nil
}

func CreateNetwork(driver string, subnet string, name string) error {
	//convert subnet into net.IPNet
	_, cidr, _ := net.ParseCIDR(subnet)

	// Assign the gateway IP via IPAM and get the first IP in the segment as the gateway IP.
	gatewayIP, err := ipAllocator.Allocate(cidr)
	if err != nil {
		return err
	}
	cidr.IP = gatewayIP

	// call corresponding driver to create network
	nw, err := drivers[driver].Create(subnet, name)
	if err != nil {
		return err
	}

	return nw.dump(defaultNetworkPath)
}

func ListNetwork() {
	w := tabwriter.NewWriter(os.Stdout, 12, 1, 3, ' ', 0)
	fmt.Fprint(w, "NAME\tIpRange\tDriver\n")
	for _, nw := range networks {
		fmt.Fprintf(w, "%s\t%s\t%s\n",
			nw.Name,
			nw.IpRange.String(),
			nw.Driver,
		)
	}
	if err := w.Flush(); err != nil {
		log.Errorf("Flush error %v", err)
		return
	}
}

func RemoveNetwork(netName string) error {
	nw, ok := networks[netName]
	if !ok {
		return fmt.Errorf("no Such Network: %s", netName)
	}

	if err := ipAllocator.Release(nw.IpRange, &nw.IpRange.IP); err != nil {
		return fmt.Errorf("error Remove Network gateway ip: %s", err)
	}

	if err := drivers[nw.Driver].Delete(*nw); err != nil {
		return fmt.Errorf("error Remove Network DriverError: %s", err)
	}

	return nw.remove(defaultNetworkPath)
}

func Connect(networkName string, cinfo *container.ContainerInfo) error {
	network, ok := networks[networkName]
	if !ok {
		return fmt.Errorf("no Such Network: %s", networkName)
	}

	// Assign container IP address
	ip, err := ipAllocator.Allocate(network.IpRange)
	if err != nil {
		return err
	}

	// Create network endpoints
	ep := &Endpoint{
		ID:          fmt.Sprintf("%s-%s", cinfo.ID, networkName),
		IPAddress:   ip,
		Network:     network,
		PortMapping: cinfo.PortMapping,
	}
	// Call network driver to mount and configure network endpoints
	if err = drivers[network.Driver].Connect(network, ep); err != nil {
		return err
	}

	// Configure the container network device IP address in the container's namespace.
	if err = configEndpointIpAddressAndRoute(ep, cinfo); err != nil {
		return err
	}

	return configPortMapping(ep)
}

func configEndpointIpAddressAndRoute(ep *Endpoint, cinfo *container.ContainerInfo) error {
	peerLink, err := netlink.LinkByName(ep.Device.PeerName)
	if err != nil {
		return fmt.Errorf("fail config endpoint: %v", err)
	}

	// Add the container's network endpoints to the container's network space,
	// and makes the following operations of this function take place in that webspace
	defer enterContainerNetns(&peerLink, cinfo)()

	interfaceIP := *ep.Network.IpRange
	interfaceIP.IP = ep.IPAddress
	// Call the setInterfaceIP function to set the IP of the Veth endpoint within the container.
	if err = setInterfaceIP(ep.Device.PeerName, interfaceIP.String()); err != nil {
		return fmt.Errorf("%v,%s", ep.Network, err)
	}
	// set up the veth within container
	if err = setInterfaceUp(ep.Device.PeerName); err != nil {
		return err
	}
	// The default "lo" NIC in the Net Namespace at local address 127.0.0.1 is turned off.
	// Enable it to ensure that the container accesses its own requests.
	if err = setInterfaceUp("lo"); err != nil {
		return err
	}

	// Set all external requests within the container to be accessed through the Veth endpoint within the container.
	// 0.0.0.0 for all IP address segments.
	_, cidr, _ := net.ParseCIDR("0.0.0.0/0")
	// Construct the routing data to be added, including the network device, gateway IP, and destination network segment
	// Equivalent to route add -net 0.0.0.0/0 gw {Bridge address} dev ｛Veth endpoint device in the container}.
	defaultRoute := &netlink.Route{
		LinkIndex: peerLink.Attrs().Index,
		Gw:        ep.Network.IpRange.IP,
		Dst:       cidr,
	}

	// Call netlink RouteAdd to add a route to the container's network space.
	// The RouteAdd function is equivalent to the route add command.
	if err = netlink.RouteAdd(defaultRoute); err != nil {
		return err
	}

	return nil
}

func enterContainerNetns(enLink *netlink.Link, cinfo *container.ContainerInfo) func() {
	f, err := os.OpenFile(fmt.Sprintf("/proc/%s/ns/net", cinfo.PID), os.O_RDONLY, 0)
	if err != nil {
		log.Errorf("error get container net namespace, %v", err)
	}

	nsFD := f.Fd()
	runtime.LockOSThread()

	// Modify that the other end of the veth peer is moved to the namespace of the container.
	if err = netlink.LinkSetNsFd(*enLink, int(nsFD)); err != nil {
		log.Errorf("error set link netns , %v", err)
	}

	// Get the current network namespace
	origns, err := netns.Get()
	if err != nil {
		log.Errorf("error get current netns, %v", err)
	}

	// Set the current process to the new network namespace and revert to the previous namespace after the function has finished executing.
	if err = netns.Set(netns.NsHandle(nsFD)); err != nil {
		log.Errorf("error set netns, %v", err)
	}
	return func() {
		netns.Set(origns)
		origns.Close()
		runtime.UnlockOSThread()
		f.Close()
	}
}

func configPortMapping(ep *Endpoint) error {
	for _, pm := range ep.PortMapping {
		portMapping := strings.Split(pm, ":")
		if len(portMapping) != 2 {
			log.Errorf("port mapping format error, %v", pm)
			continue
		}
		iptablesCmd := fmt.Sprintf("-t nat -A PREROUTING -p tcp -m tcp --dport %s -j DNAT --to-destination %s:%s",
			portMapping[0], ep.IPAddress.String(), portMapping[1])
		cmd := exec.Command("iptables", strings.Split(iptablesCmd, " ")...)
		output, err := cmd.Output()
		if err != nil {
			log.Errorf("iptables Output, %v", output)
			continue
		}
	}
	return nil
}

func (nw *Network) dump(dumpPath string) error {
	if _, err := os.Stat(dumpPath); err != nil {
		if os.IsNotExist(err) {
			os.MkdirAll(dumpPath, 0644)
		} else {
			return err
		}
	}
	// file name is network's name
	nwPath := filepath.Join(dumpPath, nw.Name)
	nwFile, err := os.OpenFile(nwPath, os.O_TRUNC|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		log.Errorf("error %v", err)
		return err
	}
	defer nwFile.Close()

	data, err := sonic.Marshal(nw)
	if err != nil {
		log.Errorf("error %v", err)
		return err
	}

	_, err = nwFile.Write(data)
	if err != nil {
		log.Errorf("error %v", err)
		return err
	}

	return nil
}

func (nw *Network) load(dumpPath string) error {
	data, err := os.ReadFile(dumpPath)
	if err != nil {
		return err
	}

	err = sonic.Unmarshal(data, nw)
	if err != nil {
		log.Errorf("Error load nw info %v", err)
		return err
	}

	return nil
}

func (nw *Network) remove(dumpPath string) error {
	if _, err := os.Stat(filepath.Join(dumpPath, nw.Name)); err != nil {
		if os.IsNotExist(err) {
			return nil
		} else {
			return err
		}
	} else {
		return os.Remove(filepath.Join(dumpPath, nw.Name))
	}
}

// NetworkJSON is used for JSON serialization of Network
type NetworkJSON struct {
	Name    string `json:"name"`
	IpRange string `json:"ipRange"` // CIDR string format
	Driver  string `json:"driver"`
}

// MarshalJSON implements json.Marshaler interface
func (nw *Network) MarshalJSON() ([]byte, error) {
	networkJSON := NetworkJSON{
		Name:   nw.Name,
		Driver: nw.Driver,
	}
	if nw.IpRange != nil {
		networkJSON.IpRange = nw.IpRange.String()
	}
	return sonic.Marshal(networkJSON)
}

// UnmarshalJSON implements json.Unmarshaler interface
func (nw *Network) UnmarshalJSON(data []byte) error {
	var networkJSON NetworkJSON
	if err := sonic.Unmarshal(data, &networkJSON); err != nil {
		return err
	}

	nw.Name = networkJSON.Name
	nw.Driver = networkJSON.Driver

	if networkJSON.IpRange != "" {
		_, ipNet, err := net.ParseCIDR(networkJSON.IpRange)
		if err != nil {
			return fmt.Errorf("parse CIDR %s error: %v", networkJSON.IpRange, err)
		}
		nw.IpRange = ipNet
	}

	return nil
}
