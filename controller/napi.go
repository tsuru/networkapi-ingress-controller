package controller

import (
	"context"
	"fmt"

	"github.com/tsuru/networkapi-ingress-controller/config"
	"github.com/tsuru/networkapi-ingress-controller/networkapi"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/types"
)

func (r *reconcileIngress) vipName(ing types.NamespacedName) string {
	return fmt.Sprintf("%s_%s_%s_%s", nameCommonPrefix, r.cfg.ClusterName, ing.Namespace, ing.Name)
}

func (r *reconcileIngress) poolName(ing types.NamespacedName) string {
	return r.vipName(ing)
}

func (r *reconcileIngress) targetName(tg target) string {
	return fmt.Sprintf("%s_%s_%s", nameCommonPrefix, r.cfg.ClusterName, tg.IP.String())
}

func newEquipment(name string, cfg config.Config) *networkapi.Equipment {
	return &networkapi.Equipment{
		Name:          name,
		EquipmentType: cfg.Equipment.Type,
		Model:         cfg.Equipment.Model,
		Environments: []networkapi.Environment{
			{Environment: cfg.Equipment.Environment},
		},
		Groups: []networkapi.IDOnly{
			{ID: cfg.Equipment.Group},
		},
	}
}

func newPool(name string, cfg config.Config) *networkapi.Pool {
	return &networkapi.Pool{
		Identifier:        name,
		DefaultPort:       80,
		Environment:       networkapi.IntOrID{ID: cfg.DefaultPoolEnvironmentID},
		ServiceDownAction: networkapi.ServiceDownAction{Name: "none"},
		LBMethod:          "round-robin",
		HealthCheck: networkapi.HealthCheck{
			Type:        "TCP",
			Destination: "*:*",
		},
		DefaultLimit: 0,
	}
}

func newVIP(name string, cfg config.Config, vip *networkapi.IP, pool *networkapi.Pool) *networkapi.VIP {
	return &networkapi.VIP{
		Name:           name,
		Service:        name,
		Business:       "tsuru gke",
		EnvironmentVIP: networkapi.IntOrID{ID: cfg.DefaultVIPEnvironmentID},
		IPv4:           &networkapi.IntOrID{ID: vip.ID},
		Ports: []networkapi.VIPPort{
			{
				Port: 80,
				Pools: []networkapi.VIPPool{
					{
						ServerPool: networkapi.IntOrID{ID: pool.ID},
						L7Rule:     networkapi.IntOrID{ID: cfg.DefaultVIPL7RuleID},
					},
				},
				Options: networkapi.VIPPortOptions{
					L4Protocol: networkapi.IntOrID{ID: cfg.DefaultVIPL4ProtocolID},
					L7Protocol: networkapi.IntOrID{ID: cfg.DefaultVIPL7ProtocolID},
				},
			},
		},
		Options: networkapi.VIPOptions{
			CacheGroup:    networkapi.IntOrID{ID: cfg.DefaultCacheGroupID},
			TrafficReturn: networkapi.IntOrID{ID: cfg.DefaultTrafficReturnID},
			Persistence:   networkapi.IntOrID{ID: cfg.DefaultPersistenceID},
			Timeout:       networkapi.IntOrID{ID: cfg.DefaultTimeoutID},
		},
	}
}

func (r *reconcileIngress) cleanupNetworkAPI(ctx context.Context, ingName types.NamespacedName) error {
	if r.cfg.DebugDisableCleanup {
		return nil
	}

	netapiCli := networkapi.Client(r.cfg.NetworkAPIURL, r.cfg.NetworkAPIUsername, r.cfg.NetworkAPIPassword)

	vipName := r.vipName(ingName)

	vip, err := netapiCli.GetVIP(ctx, vipName)
	if err != nil && !networkapi.IsNotFound(err) {
		return err
	}
	if !networkapi.IsNotFound(err) {
		if err := netapiCli.DeleteVIP(ctx, vip.ID); err != nil {
			return err
		}
	}

	vipIP, err := netapiCli.GetIPByName(ctx, vipName)
	if err != nil && !networkapi.IsNotFound(err) {
		return err
	}
	if !networkapi.IsNotFound(err) {
		if err := netapiCli.DeleteIP(ctx, vipIP.ID); err != nil {
			return err
		}
	}

	poolName := r.poolName(ingName)
	pool, err := netapiCli.GetIPByName(ctx, poolName)
	if err != nil && !networkapi.IsNotFound(err) {
		return err
	}
	if !networkapi.IsNotFound(err) {
		if err := netapiCli.DeletePool(ctx, pool.ID); err != nil {
			return err
		}
	}

	return nil
}

func (r *reconcileIngress) reconcileNetworkAPI(ctx context.Context, ing *networkingv1.Ingress, targets []target) error {
	netapiCli := networkapi.Client(r.cfg.NetworkAPIURL, r.cfg.NetworkAPIUsername, r.cfg.NetworkAPIPassword)

	wantedPool := newPool(r.poolName(namespacedName(ing)), r.cfg)
	for _, tg := range targets {
		targetName := r.targetName(tg)

		equip, err := netapiCli.GetEquipment(ctx, targetName)
		if networkapi.IsNotFound(err) {
			newEquip := newEquipment(targetName, r.cfg)
			equip, err = netapiCli.CreateEquipment(ctx, newEquip)
		}
		if err != nil {
			return err
		}

		netIP, err := netapiCli.GetIPByNetIP(ctx, tg.IP)
		if networkapi.IsNotFound(err) {
			ip := networkapi.IPFromNetIP(tg.IP)
			ip.NetworkIPv4ID = tg.NetworkID
			ip.Description = targetName
			ip.Equipments = []networkapi.IDOnly{{ID: equip.ID}}
			netIP, err = netapiCli.CreateIP(ctx, &ip)
		}
		if err != nil {
			return err
		}
		wantedPool.Members = append(wantedPool.Members, networkapi.PoolMember{
			IP: &networkapi.PoolMemberIP{
				ID:         netIP.ID,
				IPFormated: tg.IP.String(),
			},
			PortReal:     tg.Port,
			Priority:     1,
			Weight:       1,
			MemberStatus: 1,
		})
	}

	vipPool, err := netapiCli.GetPool(ctx, wantedPool.Identifier)
	if err != nil && !networkapi.IsNotFound(err) {
		return err
	}

	needsDeploy := false

	if networkapi.IsNotFound(err) {
		needsDeploy = true
		vipPool, err = netapiCli.CreatePool(ctx, wantedPool)
	} else if !vipPool.DeepEqual(*wantedPool) {
		needsDeploy = true
		wantedPool.ID = vipPool.ID
		vipPool, err = netapiCli.UpdatePool(ctx, wantedPool)
	}
	if err != nil {
		return err
	}

	vipName := r.vipName(namespacedName(ing))

	vipIP, err := netapiCli.GetIPByName(ctx, vipName)
	if err != nil && !networkapi.IsNotFound(err) {
		return err
	}
	if networkapi.IsNotFound(err) {
		// TODO: get vip environment id from annotations
		vipIP, err = netapiCli.CreateVIPIPv4(ctx, vipName, r.cfg.DefaultVIPEnvironmentID)
	}
	if err != nil {
		return err
	}

	wantedVIP := newVIP(vipName, r.cfg, vipIP, vipPool)

	vip, err := netapiCli.GetVIP(ctx, wantedVIP.Name)
	if err != nil && !networkapi.IsNotFound(err) {
		return err
	}

	if networkapi.IsNotFound(err) {
		needsDeploy = true
		vip, err = netapiCli.CreateVIP(ctx, wantedVIP)
	} else if !vip.DeepEqual(*wantedVIP) {
		needsDeploy = true
		wantedVIP.ID = vip.ID
		vip, err = netapiCli.UpdateVIP(ctx, wantedVIP)
	}
	if err != nil {
		return err
	}

	if needsDeploy {
		err = netapiCli.DeployVIP(ctx, vip.ID)
		if err != nil {
			return err
		}
	}

	ing.Status.LoadBalancer.Ingress = []corev1.LoadBalancerIngress{
		{
			IP: vipIP.ToNetIP().String(),
		},
	}
	return r.client.Status().Update(ctx, ing)
}
