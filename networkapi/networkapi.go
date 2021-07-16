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

type VIP struct {
	Name          string
	EnvironmentID int
	IPv4ID        int
	PoolIDs       []int
}

type PoolMemberIP struct {
	ID         int    `json:"id,omitempty"`
	IPFormated string `json:"ip_formated,omitempty"`
}

type PoolMember struct {
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
}

func (p1 Pool) DeepEqual(p2 Pool) bool {
	p1.ID = 0
	p2.ID = 0
	return reflect.DeepEqual(p1, p2)
}

type IP struct {
	ID            int      `json:"id,omitempty"`
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
	CreateEquipment(ctx context.Context, equip *Equipment) (*Equipment, error)
	GetEquipment(ctx context.Context, name string) (*Equipment, error)
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

func (n *networkAPI) UpdateVIP(ctx context.Context, vip *VIP) error {
	return nil
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
	body, err := marshalField("server_pools", []interface{}{pool})
	if err != nil {
		return nil, err
	}
	data, err := n.doRequest(ctx, http.MethodPost, "/api/v3/pool/", nil, body)
	if err != nil {
		return nil, err
	}
	ids, err := unmarshalIDs(data)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, errors.Errorf("no pools created: %s", string(data))
	}
	if len(ids) > 1 {
		return nil, errors.Errorf("multiple pools created: %s", string(data))
	}
	return n.getPoolByID(ctx, ids[0])
}

func (n *networkAPI) UpdatePool(ctx context.Context, pool *Pool) (*Pool, error) {
	body, err := marshalField("server_pools", []interface{}{pool})
	if err != nil {
		return nil, err
	}
	u := fmt.Sprintf("/api/v4/pool/%d/", pool.ID)
	data, err := n.doRequest(ctx, http.MethodPut, u, nil, body)
	if err != nil {
		return nil, err
	}
	ids, err := unmarshalIDs(data)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, errors.Errorf("no pools created: %s", string(data))
	}
	if len(ids) > 1 {
		return nil, errors.Errorf("multiple pools created: %s", string(data))
	}
	return n.getPoolByID(ctx, ids[0])
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
		return n.getIPByID(ctx, xmlData.IP.ID)
	}
	return nil, errors.Errorf("unable to parse ID from %q", string(data))
}

func (n *networkAPI) CreateEquipment(ctx context.Context, equip *Equipment) (*Equipment, error) {
	body, err := marshalField("equipments", []interface{}{equip})
	if err != nil {
		return nil, err
	}
	data, err := n.doRequest(ctx, http.MethodPost, "/api/v4/equipment/", nil, body)
	if err != nil {
		return nil, err
	}
	ids, err := unmarshalIDs(data)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, errors.Errorf("no equipments created: %s", string(data))
	}
	if len(ids) > 1 {
		return nil, errors.Errorf("multiple equipments created: %s", string(data))
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
