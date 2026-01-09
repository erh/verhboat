package verhboat

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"

	"strings"
)

const GarminPrefix = "172.16."

type Network struct {
	InterfaceName string
	Addr          net.Addr
	Iface         net.Interface
}

func (n Network) Good() bool {
	return n.InterfaceName != ""
}

func (n Network) IP() string {
	var s = n.Addr.String()
	s = strings.Split(s, "/")[0]
	s = strings.Split(s, ":")[0]
	return s
}

func FindGarminInterface() (Network, error) {
	return findInterface(GarminPrefix)
}

func findInterface(prefix string) (Network, error) {
	all, err := findAllGoodNetworks()
	if err != nil {
		return Network{}, err
	}

	for _, n := range all {
		if strings.HasPrefix(n.Addr.String(), prefix) {
			return n, nil
		}
	}

	return Network{}, nil
}

var goodRanges = []string{
	"192.",
	"10.",
	"172.16",
}

func findAllGoodNetworks() ([]Network, error) {
	all, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("error getting interfaces: %w", err)
	}

	good := []Network{}

	for _, i := range all {
		if (i.Flags & net.FlagUp) == 0 {
			continue
		}
		if (i.Flags & net.FlagMulticast) == 0 {
			continue
		}

		addrs, err := i.Addrs()
		if err != nil {
			continue
		}

		for _, a := range addrs {
			for _, g := range goodRanges {
				if strings.HasPrefix(a.String(), g) {
					good = append(good, Network{i.Name, a, i})
					break
				}
			}
		}
	}

	return good, nil
}

func DownloadBytes(url string) ([]byte, string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, "", fmt.Errorf("failed to fetch URL: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("failed with status code: %d", resp.StatusCode)
	}

	var buf bytes.Buffer
	_, err = io.Copy(&buf, resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read response body: %v", err)
	}

	contentType := resp.Header.Get("Content-Type")

	return buf.Bytes(), contentType, nil
}
