package util

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"
)

const MaxExpandSize = 4096

func LastIP(n *net.IPNet) net.IP {
	ip := make(net.IP, len(n.IP))
	copy(ip, n.IP)
	for i := range ip {
		ip[i] |= ^n.Mask[i]
	}
	return ip
}
func NetworksOverlap(n1, n2 *net.IPNet) bool {
	if n1 == nil || n2 == nil {
		return false
	}
	n1_last := LastIP(n1)
	n2_last := LastIP(n2)

	return bytes.Compare(n1.IP, n2_last) <= 0 && bytes.Compare(n2.IP, n1_last) <= 0
}

func IsSubnet(n1, n2 *net.IPNet) bool {
	if n1 == nil || n2 == nil {
		return false
	}
	return n1.Contains(n2.IP) && n1.Contains(LastIP(n2))
}

func NextIP(ip net.IP) net.IP {
	nextIP := make(net.IP, len(ip))
	copy(nextIP, ip)
	for i := len(nextIP) - 1; i >= 0; i-- {
		nextIP[i]++
		if nextIP[i] > 0 {
			return nextIP
		}
	}
	return nil
}

func IPToUint32(ip net.IP) (uint32, error) {
	ipv4 := ip.To4()
	if ipv4 == nil {
		return 0, fmt.Errorf("'%s' is not a valid IPv4 address", ip)
	}
	return binary.BigEndian.Uint32(ipv4), nil
}

func CIDRToIPRange(cidr *net.IPNet) (net.IP, net.IP) {
	return cidr.IP, LastIP(cidr)
}

func SplitCIDR(n *net.IPNet) ([]*net.IPNet, error) {
	ones, bits := n.Mask.Size()
	if ones == bits {
		return nil, fmt.Errorf("cannot split a /%d network", ones)
	}

	newPrefixLen := ones + 1
	addrLen := len(n.IP)

	s1 := &net.IPNet{
		IP:   make(net.IP, addrLen),
		Mask: net.CIDRMask(newPrefixLen, bits),
	}
	copy(s1.IP, n.IP)

	s2 := &net.IPNet{
		IP:   make(net.IP, addrLen),
		Mask: net.CIDRMask(newPrefixLen, bits),
	}
	copy(s2.IP, n.IP)

	byteIndex := newPrefixLen / 8
	if newPrefixLen%8 == 0 {
		byteIndex--
	}
	bitIndex := 7 - (uint(newPrefixLen-1) % 8)
	s2.IP[byteIndex] |= 1 << bitIndex

	return []*net.IPNet{s1, s2}, nil
}

func Summarize(networks []*net.IPNet) []*net.IPNet {
	fmt.Println("Summarize is an advanced function. A robust implementation requires a dedicated library.")
	return networks
}

func ParsePortRange(portRange string) ([]int, error) {
	if portRange == "" {
		return []int{}, nil
	}

	portSet := make(map[int]struct{})
	parts := strings.Split(portRange, ",")

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		if strings.Contains(part, "-") {
			rangeParts := strings.Split(part, "-")
			if len(rangeParts) != 2 {
				return nil, fmt.Errorf("invalid port range format: '%s'", part)
			}
			start, err1 := strconv.Atoi(strings.TrimSpace(rangeParts[0]))
			end, err2 := strconv.Atoi(strings.TrimSpace(rangeParts[1]))
			if err1 != nil || err2 != nil {
				return nil, fmt.Errorf("invalid number in port range: '%s'", part)
			}
			if start < 1 || end > 65535 || start > end {
				return nil, fmt.Errorf("invalid port numbers in range '%s'; must be 1-65535 and start <= end", part)
			}
			for i := start; i <= end; i++ {
				portSet[i] = struct{}{}
			}
		} else {
			port, err := strconv.Atoi(part)
			if err != nil {
				return nil, fmt.Errorf("invalid port number: '%s'", part)
			}
			if port < 1 || port > 65535 {
				return nil, fmt.Errorf("invalid port number '%d'; must be 1-65535", port)
			}
			portSet[port] = struct{}{}
		}
	}

	ports := make([]int, 0, len(portSet))
	for p := range portSet {
		ports = append(ports, p)
	}
	sort.Ints(ports)

	return ports, nil
}

func ExpandCIDR(cidr *net.IPNet) ([]net.IP, error) {
	ones, bits := cidr.Mask.Size()
	numAddresses := 1 << (bits - ones)

	if numAddresses > MaxExpandSize {
		return nil, fmt.Errorf("CIDR size (%d) exceeds safety limit (%d)", numAddresses, MaxExpandSize)
	}

	ipList := make([]net.IP, 0, numAddresses)

	currentIP := make(net.IP, len(cidr.IP))
	copy(currentIP, cidr.IP)

	for i := 0; i < numAddresses; i++ {
		ipToAdd := make(net.IP, len(currentIP))
		copy(ipToAdd, currentIP)
		ipList = append(ipList, ipToAdd)

		currentIP = NextIP(currentIP)
		if currentIP == nil {
			break
		}
	}

	return ipList, nil
}

func ExpandIPRange(ipRange string) ([]net.IP, error) {
	parts := strings.Split(ipRange, "-")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid IP range format: expected 'startIP-endIP', got '%s'", ipRange)
	}

	startIP := net.ParseIP(strings.TrimSpace(parts[0]))
	if startIP == nil {
		return nil, fmt.Errorf("invalid start IP address: '%s'", parts[0])
	}
	endIP := net.ParseIP(strings.TrimSpace(parts[1]))
	if endIP == nil {
		return nil, fmt.Errorf("invalid end IP address: '%s'", parts[1])
	}

	if bytes.Compare(startIP, endIP) > 0 {
		return nil, fmt.Errorf("start IP (%s) cannot be after end IP (%s)", startIP, endIP)
	}

	ipList := make([]net.IP, 0)

	currentIP := make(net.IP, len(startIP))
	copy(currentIP, startIP)

	for {
		if len(ipList) >= MaxExpandSize {
			return nil, fmt.Errorf("IP range size exceeds safety limit (%d)", MaxExpandSize)
		}

		ipToAdd := make(net.IP, len(currentIP))
		copy(ipToAdd, currentIP)
		ipList = append(ipList, ipToAdd)

		if bytes.Equal(currentIP, endIP) {
			break
		}

		currentIP = NextIP(currentIP)
		if currentIP == nil {
			break
		}
	}

	return ipList, nil
}
