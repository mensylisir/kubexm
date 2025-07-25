package helpers

import (
	"fmt"
	"math/big"
	"net"
)

func GetFirstIP(n *net.IPNet) (net.IP, error) {
	if n == nil {
		return nil, fmt.Errorf("input network cannot be nil")
	}

	ip := make(net.IP, len(n.IP))
	copy(ip, n.IP)

	if ip.To4() != nil {
		ip = ip.To4()
	} else if ip.To16() != nil {
		ip = ip.To16()
	} else {
		return nil, fmt.Errorf("invalid IP address format: %s", n.IP.String())
	}

	for i := len(ip) - 1; i >= 0; i-- {
		ip[i]++
		if ip[i] > 0 {
			break
		}
	}

	if !n.Contains(ip) {
		return nil, fmt.Errorf("calculated IP %s is not in the network %s", ip, n.String())
	}

	return ip, nil
}

func UniqueIPs(input []net.IP) []net.IP {
	seen := make(map[string]struct{})
	result := []net.IP{}

	for _, ip := range input {
		ipStr := ip.String()

		if _, ok := seen[ipStr]; !ok {
			seen[ipStr] = struct{}{}
			result = append(result, ip)
		}
	}
	return result
}

func GetIPAtIndex(n *net.IPNet, index int64) (net.IP, error) {
	if n == nil {
		return nil, fmt.Errorf("input network cannot be nil")
	}
	if index <= 0 {
		return nil, fmt.Errorf("index must be a positive integer")
	}
	ipInt := big.NewInt(0)
	ipInt.SetBytes(n.IP.To16())
	indexInt := big.NewInt(index)
	ipInt.Add(ipInt, indexInt)
	resultIP := net.IP(ipInt.Bytes())

	if !n.Contains(resultIP) {
		return nil, fmt.Errorf("calculated IP %s for index %d is not in the network %s", resultIP, index, n.String())
	}

	return resultIP, nil
}
