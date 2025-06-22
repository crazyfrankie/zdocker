package network

import (
	"net"
	"os"
	"path/filepath"

	"github.com/bytedance/sonic"
	log "github.com/sirupsen/logrus"
)

const (
	ipamDefaultAllocatorPath = "/var/run/zdocker/network/ipam/subnet.json"
)

var ipAllocator = &IPAM{
	SubnetAllocatorPath: ipamDefaultAllocatorPath,
}

// IPAM store ip address allocate information
type IPAM struct {
	SubnetAllocatorPath string             // Allocation of document storage locations
	Subnets             *map[string][]byte // key is the network segment, value is an array of allocated bitmaps
}

func (i *IPAM) Allocate(subnet *net.IPNet) (net.IP, error) {
	var ip net.IP

	i.Subnets = &map[string][]byte{}

	err := i.load()
	if err != nil {
		log.Errorf("err load allocation info, %v", err)
	}

	ones, size := subnet.Mask.Size()

	// If the segment has not been assigned before, initialize the segment assignment configuration.
	if _, exist := (*i.Subnets)[subnet.String()]; !exist {
		// Fill this segment's configuration with "0", 1 << uint8(size - one) indicates how many addresses are available in this segment
		// "size - one" is the number of network bits after the subnet mask, and 2 ^ (size - one) represents the available IPs in the segment.
		// and 2 ^ (size - one) is equivalent to 1 << uint8(size - one)
		(*i.Subnets)[subnet.String()] = make([]byte, 1<<uint8(size-ones))
	}

	for c, bit := range (*i.Subnets)[subnet.String()] {
		if bit == '0' {
			(*i.Subnets)[subnet.String()][c] = '1'
			ip = subnet.IP
			for t := uint(4); t > 0; t -= 1 {
				[]byte(ip)[4-t] += uint8(c >> ((t - 1) * 8))
			}
			ip[3] += 1
			break
		}
	}

	i.dump()

	return ip, nil
}

func (i *IPAM) Release(subnet *net.IPNet, ipAddr *net.IP) error {
	i.Subnets = &map[string][]byte{}

	_, subnet, _ = net.ParseCIDR(subnet.String())

	err := i.load()
	if err != nil {
		log.Errorf("error load allocation info, %v", err)
	}

	// calculate IP address in bitmap
	c := 0
	releaseIP := ipAddr.To4()
	releaseIP[3] -= 1
	for t := uint(4); t > 0; t -= 1 {
		c += int(releaseIP[t-1]-subnet.IP[t-1]) << ((4 - t) * 8)
	}

	(*i.Subnets)[subnet.String()][c] = '0'

	i.dump()

	return nil
}

func (i *IPAM) load() error {
	if _, err := os.Stat(i.SubnetAllocatorPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		} else {
			return err
		}
	}
	data, err := os.ReadFile(i.SubnetAllocatorPath)
	if err != nil {
		return err
	}

	if err = sonic.Unmarshal(data, i.Subnets); err != nil {
		log.Errorf("error dump allocation info, %v", err)
		return err
	}

	return nil
}

func (i *IPAM) dump() error {
	ipamConfigDir, _ := filepath.Split(i.SubnetAllocatorPath)
	if _, err := os.Stat(ipamConfigDir); err != nil {
		if os.IsNotExist(err) {
			// equal to mkdir -p <dir>
			os.MkdirAll(ipamConfigDir, 0644)
		} else {
			return err
		}
	}

	// Open the storage file os.O_TRUNC means clear if it exists, os.O_CREATE means create if it doesn't exist.
	subnetConfigFile, err := os.OpenFile(i.SubnetAllocatorPath, os.O_TRUNC|os.O_WRONLY|os.O_CREATE, 0644)
	defer subnetConfigFile.Close()
	if err != nil {
		return err
	}

	data, err := sonic.Marshal(i.Subnets)
	if err != nil {
		return err
	}

	_, err = subnetConfigFile.Write(data)
	if err != nil {
		return err
	}

	return nil
}
