package networkapi

import (
	"context"
	"net"

	"github.com/pkg/errors"
)

var _ NetworkAPI = &FakeNetworkAPI{}

type FakeNetworkAPI struct {
	Pools   map[string]Pool
	IPsByID map[int]IP
	VIPs    map[string]VIP

	VIPUpdates []VIP
	VIPDeploys []int
}

func (f *FakeNetworkAPI) GetVIP(ctx context.Context, name string) (*VIP, error) {
	if f.VIPs == nil {
		return nil, errNotFound
	}
	vip, ok := f.VIPs[name]
	if !ok {
		return nil, errNotFound
	}
	return &vip, nil
}

func (f *FakeNetworkAPI) CreateVIP(ctx context.Context, vip *VIP) (*VIP, error) {
	return nil, errors.New("CreateVIP is not implemented yet")
}

func (f *FakeNetworkAPI) UpdateVIP(ctx context.Context, vip *VIP) (*VIP, error) {
	f.VIPUpdates = append(f.VIPUpdates, *vip)
	return vip, nil
}

func (f *FakeNetworkAPI) DeployVIP(ctx context.Context, vipID int) error {
	f.VIPDeploys = append(f.VIPDeploys, vipID)
	return nil
}

func (f *FakeNetworkAPI) GetPool(ctx context.Context, name string) (*Pool, error) {
	if f.Pools == nil {
		return nil, errNotFound
	}
	pool, ok := f.Pools[name]
	if !ok {
		return nil, errNotFound
	}

	return &pool, nil
}

func (f *FakeNetworkAPI) CreatePool(ctx context.Context, pool *Pool) (*Pool, error) {
	if f.Pools == nil {
		f.Pools = make(map[string]Pool)
	}
	f.Pools[pool.Identifier] = *pool
	return pool, nil
}

func (f *FakeNetworkAPI) UpdatePool(ctx context.Context, pool *Pool) (*Pool, error) {
	return nil, errors.New("UpdatePool is not implemented yet")
}

func (f *FakeNetworkAPI) CreateVIPIPv4(ctx context.Context, name string, vipEnvironmentID int) (*IP, error) {
	return nil, errors.New("CreateVIPIPv4 is not implemented yet")
}

func (f *FakeNetworkAPI) CreateIP(ctx context.Context, ip *IP) (*IP, error) {
	return nil, errors.New("CreateIP is not implemented yet")
}

func (f *FakeNetworkAPI) GetIPByName(ctx context.Context, name string) (*IP, error) {
	return nil, errors.New("GetIPByID is not implemented yet")
}

func (f *FakeNetworkAPI) GetIPByNetIP(ctx context.Context, ip net.IP) (*IP, error) {
	return nil, errors.New("GetIPByNetIP is not implemented yet")
}

func (f *FakeNetworkAPI) GetIPByID(ctx context.Context, id int) (*IP, error) {
	if f.IPsByID == nil {
		return nil, errNotFound
	}

	ip, ok := f.IPsByID[id]
	if !ok {
		return nil, errNotFound
	}
	return &ip, nil
}

func (f *FakeNetworkAPI) CreateEquipment(ctx context.Context, equip *Equipment) (*Equipment, error) {
	return nil, errors.New("CreateEquipment is not implemented yet")
}

func (f *FakeNetworkAPI) GetEquipment(ctx context.Context, name string) (*Equipment, error) {
	return nil, errors.New("GetEquipment is not implemented yet")
}

func (f *FakeNetworkAPI) DeleteIP(ctx context.Context, id int) error {
	return errors.New("DeleteIP is not implemented yet")
}

func (f *FakeNetworkAPI) DeletePool(ctx context.Context, id int) error {
	return errors.New("DeletePool is not implemented yet")
}

func (f *FakeNetworkAPI) DeleteVIP(ctx context.Context, vip *VIP) error {
	return errors.New("DeleteVIP is not implemented yet")
}
