package networkapi

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"reflect"

	"github.com/pkg/errors"
)

var errNotFound = errors.New("not found")

type NetworkAPI interface {
	GetVIP(ctx context.Context, name string) (*VIP, error)
	CreateVIP(ctx context.Context, vip *VIP) (*VIP, error)
	UpdateVIP(ctx context.Context, vip *VIP) (*VIP, error)
	DeployVIP(ctx context.Context, vipID int) error
	GetPool(ctx context.Context, name string) (*Pool, error)
	CreatePool(ctx context.Context, pool *Pool) (*Pool, error)
	UpdatePool(ctx context.Context, pool *Pool) (*Pool, error)
	CreateVIPIPv4(ctx context.Context, name string, vipEnvironmentID int) (*IP, error)
	CreateIP(ctx context.Context, ip *IP) (*IP, error)
	GetIPByID(ctx context.Context, id int) (*IP, error)
	GetIPByName(ctx context.Context, name string) (*IP, error)
	GetIPByNetIP(ctx context.Context, ip net.IP) (*IP, error)
	CreateEquipment(ctx context.Context, equip *Equipment) (*Equipment, error)
	GetEquipment(ctx context.Context, name string) (*Equipment, error)
	DeleteIP(ctx context.Context, id int) error
	DeletePool(ctx context.Context, id int) error
	DeleteVIP(ctx context.Context, vip *VIP) error
}

type VIP struct {
	ID             int        `json:"id,omitempty"`
	Name           string     `json:"name,omitempty"`
	Service        string     `json:"service,omitempty"`
	Business       string     `json:"business,omitempty"`
	EnvironmentVIP IntOrID    `json:"environmentvip,omitempty"`
	IPv4           *IntOrID   `json:"ipv4"`
	IPv6           *IntOrID   `json:"ipv6"`
	Ports          []VIPPort  `json:"ports,omitempty"`
	Options        VIPOptions `json:"options,omitempty"`
	Created        bool       `json:"created,omitempty"`
}

func (v1 VIP) DeepEqual(v2 VIP) bool {
	return reflect.DeepEqual(v1, v2)
}

type VIPPort struct {
	ID      int            `json:"id,omitempty"`
	Port    int            `json:"port,omitempty"`
	Pools   []VIPPool      `json:"pools,omitempty"`
	Options VIPPortOptions `json:"options,omitempty"`
}

type VIPOptions struct {
	CacheGroup    IntOrID `json:"cache_group"`
	TrafficReturn IntOrID `json:"traffic_return"`
	Persistence   IntOrID `json:"persistence"`
	Timeout       IntOrID `json:"timeout"`
}

type VIPPool struct {
	ID         int     `json:"id,omitempty"`
	ServerPool IntOrID `json:"server_pool,omitempty"`
	L7Rule     IntOrID `json:"l7_rule"`
}

type VIPPortOptions struct {
	L4Protocol IntOrID `json:"l4_protocol,omitempty"`
	L7Protocol IntOrID `json:"l7_protocol,omitempty"`
}

type Pool struct {
	ID                int               `json:"id,omitempty"`
	Identifier        string            `json:"identifier,omitempty"`
	DefaultPort       int               `json:"default_port,omitempty"`
	Environment       IntOrID           `json:"environment,omitempty"`
	ServiceDownAction ServiceDownAction `json:"servicedownaction,omitempty"`
	LBMethod          string            `json:"lb_method,omitempty"`
	HealthCheck       HealthCheck       `json:"healthcheck,omitempty"`
	DefaultLimit      int               `json:"default_limit"`
	Members           []PoolMember      `json:"server_pool_members,omitempty"`
	PoolCreated       bool              `json:"pool_created,omitempty"`
}

func (p1 Pool) DeepEqual(p2 Pool) bool {
	return reflect.DeepEqual(p1, p2)
}

type PoolMemberIP struct {
	ID         int    `json:"id,omitempty"`
	IPFormated string `json:"ip_formated,omitempty"`
}

type PoolMember struct {
	ID           int           `json:"id,omitempty"`
	IP           *PoolMemberIP `json:"ip"`
	IPv6         *PoolMemberIP `json:"ipv6"`
	Priority     int           `json:"priority"`
	Weight       int           `json:"weight"`
	Limit        int           `json:"limit"`
	PortReal     int           `json:"port_real"`
	MemberStatus int           `json:"member_status"`
}

type HealthCheck struct {
	Identifier  string `json:"identifier"`
	Type        string `json:"healthcheck_type"`
	Request     string `json:"healthcheck_request"`
	Expect      string `json:"healthcheck_expect"`
	Destination string `json:"destination"`
}

type ServiceDownAction struct {
	Name string `json:"name,omitempty"`
}

type IntOrID struct {
	ID int
}

func (v *IntOrID) UnmarshalJSON(data []byte) error {
	var id int
	err := json.Unmarshal(data, &id)
	if err == nil {
		v.ID = id
		return nil
	}
	var asStruct IDOnly
	err = json.Unmarshal(data, &asStruct)
	if err == nil {
		v.ID = asStruct.ID
	}
	return err
}

func (v IntOrID) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.ID)
}

type IP struct {
	ID            int      `json:"id,omitempty" xml:"id"`
	Oct1          byte     `json:"oct1,omitempty"`
	Oct2          byte     `json:"oct2,omitempty"`
	Oct3          byte     `json:"oct3,omitempty"`
	Oct4          byte     `json:"oct4,omitempty"`
	NetworkIPv4ID int      `json:"networkipv4,omitempty"`
	Description   string   `json:"description,omitempty"`
	Equipments    []IDOnly `json:"equipments,omitempty"`
}

type IDOnly struct {
	ID int `json:"id,omitempty"`
}

type Environment struct {
	Environment  int  `json:"environment"`
	IsRouter     bool `json:"is_router"`
	IsController bool `json:"is_controller"`
}

type Equipment struct {
	ID            int           `json:"id,omitempty" xml:"id"`
	Name          string        `json:"name,omitempty"`
	EquipmentType int           `json:"equipment_type,omitempty"`
	Model         int           `json:"model,omitempty"`
	Environments  []Environment `json:"environments,omitempty"`
	Groups        []IDOnly      `json:"groups,omitempty"`
	Maintenance   bool          `json:"maintenance"`
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

type networkAPI struct {
	baseClient
}

var _ NetworkAPI = &networkAPI{}

func parseVIP(data []byte) (*VIP, error) {
	var result []VIP
	err := unmarshalField(data, "vips", &result)
	if err != nil {
		return nil, err
	}
	if len(result) == 0 {
		return nil, errNotFound
	}
	if len(result) > 1 {
		return nil, errors.Errorf("multiple vips found when one was expected: %#v", result)
	}
	return &result[0], nil
}

func (n *networkAPI) GetVIP(ctx context.Context, name string) (*VIP, error) {
	search, err := json.Marshal(map[string]interface{}{
		"extends_search": []interface{}{
			map[string]string{"name": name},
		},
	})
	if err != nil {
		return nil, err
	}
	data, err := n.doRequest(ctx, http.MethodGet, "/api/v3/vip-request/", url.Values{
		"search": []string{string(search)},
		"kind":   []string{"details"},
	}, nil)
	if err != nil {
		return nil, err
	}
	return parseVIP(data)
}

func (n *networkAPI) getVIPByID(ctx context.Context, id int) (*VIP, error) {
	u := fmt.Sprintf("/api/v3/vip-request/%d/", id)
	data, err := n.doRequest(ctx, http.MethodGet, u, url.Values{
		"kind": []string{"details"},
	}, nil)
	if err != nil {
		return nil, err
	}
	return parseVIP(data)
}

func (n *networkAPI) CreateVIP(ctx context.Context, vip *VIP) (*VIP, error) {
	id, err := n.doPost(ctx, "/api/v3/vip-request/", "vips", vip)
	if err != nil {
		return nil, err
	}
	return n.getVIPByID(ctx, id)
}

func (n *networkAPI) UpdateVIP(ctx context.Context, vip *VIP) (*VIP, error) {
	body, err := marshalField("vips", []interface{}{vip})
	if err != nil {
		return nil, err
	}
	if vip.Created {
		_, err = n.doRequest(ctx, http.MethodPut, fmt.Sprintf("/api/v3/vip-request/deploy/%d/", vip.ID), nil, body)
	} else {
		_, err = n.doRequest(ctx, http.MethodPut, fmt.Sprintf("/api/v3/vip-request/%d/", vip.ID), nil, body)
	}
	if err != nil {
		return nil, err
	}

	if err != nil {
		return nil, err
	}
	return n.getVIPByID(ctx, vip.ID)
}

func (n *networkAPI) DeployVIP(ctx context.Context, vipID int) error {
	u := fmt.Sprintf("/api/v3/vip-request/deploy/%d/", vipID)
	_, err := n.doRequest(ctx, http.MethodPost, u, nil, nil)
	return err
}

func parsePool(data []byte) (*Pool, error) {
	var result []Pool
	err := unmarshalField(data, "server_pools", &result)
	if err != nil {
		return nil, err
	}
	if len(result) == 0 {
		return nil, errNotFound
	}
	if len(result) > 1 {
		return nil, errors.Errorf("multiple pools found when one was expected: %#v", result)
	}
	return &result[0], nil
}

func (n *networkAPI) GetPool(ctx context.Context, name string) (*Pool, error) {
	search, err := json.Marshal(map[string]interface{}{
		"extends_search": []interface{}{
			map[string]string{"identifier": name},
		},
	})
	if err != nil {
		return nil, err
	}

	data, err := n.doRequest(ctx, http.MethodGet, "/api/v3/pool/", url.Values{
		"search": []string{string(search)},
		"kind":   []string{"details"},
	}, nil)
	if err != nil {
		return nil, err
	}
	return parsePool(data)
}

func (n *networkAPI) getPoolByID(ctx context.Context, id int) (*Pool, error) {
	u := fmt.Sprintf("/api/v3/pool/%d/", id)
	data, err := n.doRequest(ctx, http.MethodGet, u, url.Values{
		"kind": []string{"details"},
	}, nil)
	if err != nil {
		return nil, err
	}
	return parsePool(data)
}

func (n *networkAPI) CreatePool(ctx context.Context, pool *Pool) (*Pool, error) {
	id, err := n.doPost(ctx, "/api/v3/pool/", "server_pools", pool)
	if err != nil {
		return nil, err
	}
	return n.getPoolByID(ctx, id)
}

func (n *networkAPI) UpdatePool(ctx context.Context, pool *Pool) (*Pool, error) {
	body, err := marshalField("server_pools", []interface{}{pool})
	if err != nil {
		return nil, err
	}
	if pool.PoolCreated {
		_, err = n.doRequest(ctx, http.MethodPut, fmt.Sprintf("/api/v3/pool/deploy/%d/", pool.ID), nil, body)
	} else {
		_, err = n.doRequest(ctx, http.MethodPut, fmt.Sprintf("/api/v3/pool/%d/", pool.ID), nil, body)
	}
	if err != nil {
		return nil, err
	}

	if err != nil {
		return nil, err
	}
	return n.getPoolByID(ctx, pool.ID)
}

func (n *networkAPI) CreateVIPIPv4(ctx context.Context, name string, vipEnvironmentID int) (*IP, error) {
	const vipIPRequestTpl = `<?xml version="1.0" encoding="UTF-8"?><networkapi versao="1.0"><ip_map><id_evip>%d</id_evip><name>%s</name></ip_map></networkapi>`
	body := fmt.Sprintf(vipIPRequestTpl, vipEnvironmentID, name)
	u := fmt.Sprintf("/ip/availableip4/vip/%d/", vipEnvironmentID)
	data, err := n.doRequest(ctx, http.MethodPost, u, nil, []byte(body))
	if err != nil {
		return nil, err
	}
	var xmlData struct {
		XMLName xml.Name `xml:"networkapi"`
		IP      IP       `xml:"ip"`
	}
	err = xml.Unmarshal(data, &xmlData)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to unmarshal %q", string(data))
	}
	if xmlData.IP.ID != 0 {
		return n.GetIPByID(ctx, xmlData.IP.ID)
	}
	return nil, errors.Errorf("unable to parse ID from %q", string(data))
}

func (n *networkAPI) CreateEquipment(ctx context.Context, equip *Equipment) (*Equipment, error) {
	_, err := n.doPost(ctx, "/api/v3/equipment/", "equipments", equip)
	if err != nil {
		return nil, err
	}
	return n.GetEquipment(ctx, equip.Name)
}

func (n *networkAPI) GetEquipment(ctx context.Context, name string) (*Equipment, error) {
	data, err := n.doRequest(ctx, http.MethodGet, "/api/v4/equipment/", url.Values{
		"name": []string{name},
	}, nil)
	if err != nil {
		return nil, err
	}
	var result []Equipment
	err = unmarshalField(data, "equipments", &result)
	if err != nil {
		return nil, err
	}
	if len(result) == 0 {
		return nil, errNotFound
	}
	if len(result) > 1 {
		return nil, errors.Errorf("multiple equipments found when one was expected: %#v", result)
	}
	return &result[0], nil
}

func (n *networkAPI) CreateIP(ctx context.Context, ip *IP) (*IP, error) {
	id, err := n.doPost(ctx, "/api/v3/ipv4/", "ips", ip)
	if err != nil {
		return nil, err
	}
	return n.GetIPByID(ctx, id)
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

func (n *networkAPI) GetIPByID(ctx context.Context, id int) (*IP, error) {
	u := fmt.Sprintf("/api/v3/ipv4/%d/", id)
	data, err := n.doRequest(ctx, http.MethodGet, u, nil, nil)
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

func (n *networkAPI) delete(ctx context.Context, urlName string, id int) error {
	u := fmt.Sprintf("/api/v3/%s/%d/", urlName, id)
	_, err := n.doRequest(ctx, http.MethodDelete, u, nil, nil)
	return err
}

func (n *networkAPI) DeleteIP(ctx context.Context, id int) error {
	return n.delete(ctx, "ipv4", id)
}

func (n *networkAPI) DeletePool(ctx context.Context, id int) error {
	return n.delete(ctx, "pool", id)
}

func (n *networkAPI) DeleteVIP(ctx context.Context, vip *VIP) error {
	if vip.Created {
		_, err := n.doRequest(ctx, http.MethodDelete, fmt.Sprintf("/api/v3/vip-request/deploy/%d/", vip.ID), nil, nil)
		if err != nil {
			return err
		}
	}

	return n.delete(ctx, "vip-request", vip.ID)
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
