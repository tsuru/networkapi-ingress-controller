package networkapi

import "net"

type VIP struct {
	PoolIDs []string
}

type Pool struct {
	ID    string
	Reals []Equipment
}

type Equipment struct {
	ID  string
	IPs []EquipmentIP
}

type EquipmentIP struct {
	EquipmentID string
	Description string
	IP          net.IP
}

type NetworkAPI interface {
	GetVIP(id string) (*VIP, error)
	CreateVIP(vip *VIP) error
	GetPool(id string) (*Pool, error)
	CreatePool(pool *Pool) error
	CreateEquipment(equip *Equipment) error
	GetEquipment(id string) (*Equipment, error)
	CreateEquipmentIP(equipmentIP *EquipmentIP) error
	SetReals(poolID string, reals []Equipment) error
}

type networkAPI struct{}

var _ NetworkAPI = &networkAPI{}

func (networkAPI) GetVIP(id string) (*VIP, error) {
	return nil, nil
}

func (networkAPI) CreateVIP(vip *VIP) error {
	return nil
}

func (networkAPI) GetPool(id string) (*Pool, error) {
	return nil, nil
}

func (networkAPI) CreatePool(pool *Pool) error {
	return nil
}

func (networkAPI) CreateEquipment(equip *Equipment) error {
	return nil
}

func (networkAPI) GetEquipment(id string) (*Equipment, error) {
	return nil, nil
}

func (networkAPI) CreateEquipmentIP(equipmentIP *EquipmentIP) error {
	return nil
}

func (networkAPI) SetReals(poolID string, reals []Equipment) error {
	return nil
}

func Client() NetworkAPI {
	return &networkAPI{}
}
