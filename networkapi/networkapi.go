package networkapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"

	"github.com/pkg/errors"
)

var errNotFound = errors.New("not found")

type VIP struct {
	Name          string
	EnvironmentID int
	IPv4ID        int
	PoolIDs       []int
}

type PoolMember struct {
	Port int
	IPID int
}

type Pool struct {
	ID    int
	Name  string
	Reals []PoolMember
}

type IP struct {
	ID            int    `json:"id,omitempty"`
	Oct1          byte   `json:"oct1,omitempty"`
	Oct2          byte   `json:"oct2,omitempty"`
	Oct3          byte   `json:"oct3,omitempty"`
	Oct4          byte   `json:"oct4,omitempty"`
	NetworkIPv4ID int    `json:"networkipv4,omitempty"`
	Description   string `json:"description,omitempty"`
}

func IPFromNetIP(netIP net.IP) IP {
	ip := IP{}
	netIP = netIP.To4()
	ip.Oct1 = netIP[0]
	ip.Oct2 = netIP[1]
	ip.Oct3 = netIP[2]
	ip.Oct4 = netIP[3]
	return ip
}

func (ip *IP) ToNetIP() net.IP {
	return net.IPv4(ip.Oct1, ip.Oct2, ip.Oct3, ip.Oct4)
}

type NetworkAPI interface {
	GetVIP(ctx context.Context, name string) (*VIP, error)
	CreateVIP(ctx context.Context, vip *VIP) error
	UpdateVIP(ctx context.Context, vip *VIP) error
	GetPool(ctx context.Context, name string) (*Pool, error)
	CreatePool(ctx context.Context, pool *Pool) (*Pool, error)
	UpdatePool(ctx context.Context, pool *Pool) (*Pool, error)
	CreateVIPIPv4(ctx context.Context, name string, vipEnvironmentID int) (*IP, error)
	CreateIP(ctx context.Context, ip *IP) (*IP, error)
	GetIPByName(ctx context.Context, name string) (*IP, error)
	GetIPByNetIP(ctx context.Context, ip net.IP) (*IP, error)
}

type networkAPI struct {
	baseClient
}

var _ NetworkAPI = &networkAPI{}

func (n *networkAPI) GetVIP(ctx context.Context, name string) (*VIP, error) {
	return nil, nil
}

func (n *networkAPI) CreateVIP(ctx context.Context, vip *VIP) error {
	return nil
}

func (n *networkAPI) GetPool(ctx context.Context, name string) (*Pool, error) {
	return nil, nil
}

func (n *networkAPI) CreatePool(ctx context.Context, pool *Pool) (*Pool, error) {
	return nil, nil
}

func (n *networkAPI) CreateVIPIPv4(ctx context.Context, name string, vipEnvironmentID int) (*IP, error) {
	return nil, nil
}

func (n *networkAPI) UpdateVIP(ctx context.Context, vip *VIP) error {
	return nil
}

func (n *networkAPI) UpdatePool(ctx context.Context, pool *Pool) (*Pool, error) {
	return nil, nil
}

func (n *networkAPI) CreateIP(ctx context.Context, ip *IP) (*IP, error) {
	body, err := marshalField("ips", []interface{}{ip})
	if err != nil {
		return nil, err
	}
	data, err := n.doRequest(ctx, http.MethodPost, "/api/v3/ipv4/", nil, body)
	if err != nil {
		return nil, err
	}
	ids, err := unmarshalIDs(data)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, errors.Errorf("no IP created: %s", string(data))
	}
	if len(ids) > 1 {
		return nil, errors.Errorf("multiple IPs created: %s", string(data))
	}
	return n.getIPByID(ctx, ids[0])
}

func parseIP(data []byte) (*IP, error) {
	var result []IP
	err := unmarshalField(data, "ips", &result)
	if err != nil {
		return nil, err
	}
	if len(result) == 0 {
		return nil, errNotFound
	}
	if len(result) > 1 {
		return nil, errors.Errorf("multiple IPs found when one was expected: %#v", result)
	}
	return &result[0], nil
}

func (n *networkAPI) getIPByID(ctx context.Context, id int) (*IP, error) {
	url := fmt.Sprintf("/api/v3/ipv4/%d/", id)
	data, err := n.doRequest(ctx, http.MethodGet, url, nil, nil)
	if err != nil {
		return nil, err
	}
	return parseIP(data)
}

func (n *networkAPI) GetIPByNetIP(ctx context.Context, ip net.IP) (*IP, error) {
	ip = ip.To4()
	if ip == nil {
		return nil, errors.New("ipv4 required")
	}

	search, err := json.Marshal(map[string]interface{}{
		"extends_search": []interface{}{
			IPFromNetIP(ip),
		},
	})
	if err != nil {
		return nil, err
	}

	data, err := n.doRequest(ctx, http.MethodGet, "/api/v3/ipv4/", url.Values{
		"search": []string{string(search)},
	}, nil)
	if err != nil {
		return nil, err
	}
	return parseIP(data)
}

func (n *networkAPI) GetIPByName(ctx context.Context, name string) (*IP, error) {
	search, err := json.Marshal(map[string]interface{}{
		"extends_search": []interface{}{
			map[string]string{"descricao": name},
		},
	})
	if err != nil {
		return nil, err
	}

	data, err := n.doRequest(ctx, http.MethodGet, "/api/v3/ipv4/", url.Values{
		"search": []string{string(search)},
	}, nil)
	if err != nil {
		return nil, err
	}
	return parseIP(data)
}

func Client(baseURL, user, password string) NetworkAPI {
	return &networkAPI{
		baseClient: baseClient{
			baseURL:  baseURL,
			username: user,
			password: password,
		},
	}
}

func IsNotFound(err error) bool {
	return err != nil && err == errNotFound
}
