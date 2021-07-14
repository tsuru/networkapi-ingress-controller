package networkapi

import (
	"context"
	"net"
)

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
	ID        int
	IP        net.IP
	NetworkID int
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
	GetIP(ctx context.Context, ip net.IP) (*IP, error)
	GetIPByName(ctx context.Context, name string) (*IP, error)
}

type networkAPI struct{}

var _ NetworkAPI = &networkAPI{}

func (networkAPI) GetVIP(ctx context.Context, name string) (*VIP, error) {
	return nil, nil
}

func (networkAPI) CreateVIP(ctx context.Context, vip *VIP) error {
	return nil
}

func (networkAPI) GetPool(ctx context.Context, name string) (*Pool, error) {
	return nil, nil
}

func (networkAPI) CreatePool(ctx context.Context, pool *Pool) (*Pool, error) {
	return nil, nil
}

func (networkAPI) CreateVIPIPv4(ctx context.Context, name string, vipEnvironmentID int) (*IP, error) {
	return nil, nil
}

func (networkAPI) UpdateVIP(ctx context.Context, vip *VIP) error {
	return nil
}

func (networkAPI) UpdatePool(ctx context.Context, pool *Pool) (*Pool, error) {
	return nil, nil
}

func (networkAPI) CreateIP(ctx context.Context, ip *IP) (*IP, error) {
	return nil, nil
}

func (networkAPI) GetIP(ctx context.Context, ip net.IP) (*IP, error) {
	return nil, nil
}

func (networkAPI) GetIPByName(ctx context.Context, name string) (*IP, error) {
	return nil, nil
}

func Client() NetworkAPI {
	return &networkAPI{}
}

func IsNotFound(err error) bool {
	return false
}
