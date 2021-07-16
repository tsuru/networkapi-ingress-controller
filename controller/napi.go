package controller

import (
	"context"
	"fmt"
	"reflect"

	"github.com/tsuru/networkapi-ingress-controller/config"
	"github.com/tsuru/networkapi-ingress-controller/networkapi"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
)

func (r *reconcileIngress) vipName(ing *networkingv1.Ingress) string {
	return fmt.Sprintf("%s_%s_%s_%s", nameCommonPrefix, r.cfg.ClusterName, ing.Namespace, ing.Name)
}

func (r *reconcileIngress) poolName(ing *networkingv1.Ingress) string {
	return fmt.Sprintf("%s_%s_%s_%s", nameCommonPrefix, r.cfg.ClusterName, ing.Namespace, ing.Name)
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

func newPool(name string, cfg config.Config, ing *networkingv1.Ingress) *networkapi.Pool {
	return &networkapi.Pool{
		Identifier:        name,
		DefaultPort:       80,
		Environment:       networkapi.IntOrID{ID: cfg.Equipment.Environment},
		ServiceDownAction: networkapi.ServiceDownAction{Name: "none"},
		LBMethod:          "round-robin",
		HealthCheck: networkapi.HealthCheck{
			Type:        "TCP",
			Destination: "*:*",
		},
		DefaultLimit: 0,
	}
}

func (r *reconcileIngress) reconcileNetworkAPI(ctx context.Context, ing *networkingv1.Ingress, targets []target) error {
	netapiCli := networkapi.Client(r.cfg.NetworkAPIURL, r.cfg.NetworkAPIUsername, r.cfg.NetworkAPIPassword)

	wantedPool := newPool(r.poolName(ing), r.cfg, ing)

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

	vipPool, err := netapiCli.GetPool(ctx, r.poolName(ing))
	if err != nil && !networkapi.IsNotFound(err) {
		return err
	}

	if networkapi.IsNotFound(err) {
		vipPool, err = netapiCli.CreatePool(ctx, wantedPool)
	} else if !vipPool.DeepEqual(*wantedPool) {
		vipPool, err = netapiCli.UpdatePool(ctx, wantedPool)
	}
	if err != nil {
		return err
	}

	vipIP, err := netapiCli.GetIPByName(ctx, r.vipName(ing))
	if err != nil && !networkapi.IsNotFound(err) {
		return err
	}
	if networkapi.IsNotFound(err) {
		// TODO: get vip environment id from annotations
		vipIP, err = netapiCli.CreateVIPIPv4(ctx, r.vipName(ing), r.cfg.DefaultVIPEnvironmentID)
	}
	if err != nil {
		return err
	}

	existingVIP, err := netapiCli.GetVIP(ctx, r.vipName(ing))
	if err != nil && !networkapi.IsNotFound(err) {
		return err
	}

	wantedVIP := &networkapi.VIP{
		Name:    r.vipName(ing),
		IPv4ID:  vipIP.ID,
		PoolIDs: []int{vipPool.ID},
	}
	if networkapi.IsNotFound(err) {
		err = netapiCli.CreateVIP(ctx, wantedVIP)
	} else if !reflect.DeepEqual(existingVIP, wantedVIP) {
		err = netapiCli.UpdateVIP(ctx, wantedVIP)
	}
	if err != nil {
		return err
	}

	ing.Status.LoadBalancer.Ingress = []corev1.LoadBalancerIngress{
		{
			IP: vipIP.ToNetIP().String(),
		},
	}
	return r.client.Status().Update(ctx, ing)
}
