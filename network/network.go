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

// root@frank:/home/frank/projects/go/zdocker# ./zdocker network create --driver bridge --subnet 192.168.10.1/24 testbridge
// root@frank:/home/frank/projects/go/zdocker# ./zdocker run -t --net testbridge busybox sh
// {"level":"info","msg":"init come on","time":"2025-06-22T21:30:48+08:00"}
// {"level":"error","msg":"Error Connect Network \u0026{testbridge 192.168.10.1/24 bridge},invalid CIDR address: \u003cnil\u003e","time":"2025-06-22T21:30:48+08:00"}
// {"level":"info","msg":"current location is /root/mnt/2517245037","time":"2025-06-22T21:30:48+08:00"}
// root@frank:/home/frank/projects/go/zdocker# {"level":"error","msg":"Exec loop path error exec: "": executable file not found in $PATH","time":"2025-06-22T21:30:48+08:00"}
// Error: exec: "": executable file not found in $PATH
// Usage:
// zdocker init
//
// Flags:
// -h, --help help for init
//
// exec: "": executable file not found in $PATH

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

	// 分配容器IP地址
	ip, err := ipAllocator.Allocate(network.IpRange)
	if err != nil {
		return err
	}

	// 创建网络端点
	ep := &Endpoint{
		ID:          fmt.Sprintf("%s-%s", cinfo.ID, networkName),
		IPAddress:   ip,
		Network:     network,
		PortMapping: cinfo.PortMapping,
	}
	// 调用网络驱动挂载和配置网络端点
	if err = drivers[network.Driver].Connect(network, ep); err != nil {
		return err
	}
	// 到容器的namespace配置容器网络设备IP地址
	if err = configEndpointIpAddressAndRoute(ep, cinfo); err != nil {
		return err
	}

	return configPortMapping(ep, cinfo)
}

func configEndpointIpAddressAndRoute(ep *Endpoint, cinfo *container.ContainerInfo) error {
	peerLink, err := netlink.LinkByName(ep.Device.PeerName)
	if err != nil {
		return fmt.Errorf("fail config endpoint: %v", err)
	}

	defer enterContainerNetns(&peerLink, cinfo)()

	interfaceIP := *ep.Network.IpRange
	interfaceIP.IP = ep.IPAddress

	if err = setInterfaceIP(ep.Device.PeerName, interfaceIP.String()); err != nil {
		return fmt.Errorf("%v,%s", ep.Network, err)
	}

	if err = setInterfaceUp(ep.Device.PeerName); err != nil {
		return err
	}

	if err = setInterfaceUp("lo"); err != nil {
		return err
	}

	_, cidr, _ := net.ParseCIDR("0.0.0.0/0")

	defaultRoute := &netlink.Route{
		LinkIndex: peerLink.Attrs().Index,
		Gw:        ep.Network.IpRange.IP,
		Dst:       cidr,
	}

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

	// 修改veth peer 另外一端移到容器的namespace中
	if err = netlink.LinkSetNsFd(*enLink, int(nsFD)); err != nil {
		log.Errorf("error set link netns , %v", err)
	}

	// 获取当前的网络namespace
	origns, err := netns.Get()
	if err != nil {
		log.Errorf("error get current netns, %v", err)
	}

	// 设置当前进程到新的网络namespace，并在函数执行完成之后再恢复到之前的namespace
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

func configPortMapping(ep *Endpoint, cinfo *container.ContainerInfo) error {
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
