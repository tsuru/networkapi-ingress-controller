package controller

import (
	"context"
	"fmt"

	"github.com/kr/pretty"
	"github.com/pkg/errors"
	"github.com/tsuru/networkapi-ingress-controller/config"
	"github.com/tsuru/networkapi-ingress-controller/networkapi"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *reconcileIngress) vipName(ing types.NamespacedName) string {
	return fmt.Sprintf("%s_%s_%s_%s", config.IngressControllerName, r.cfg.ClusterName, ing.Namespace, ing.Name)
}

func (r *reconcileIngress) httpPoolName(ing types.NamespacedName) string {
	return fmt.Sprintf("%s_http", r.vipName(ing))
}

func (r *reconcileIngress) httpsPoolName(ing types.NamespacedName) string {
	return fmt.Sprintf("%s_https", r.vipName(ing))
}

func (r *reconcileIngress) targetName(tg target) string {
	return fmt.Sprintf("%s_%s_%s", config.IngressControllerName, r.cfg.ClusterName, tg.IP.String())
}

func newEquipment(name string, cfg config.InstanceConfig) *networkapi.Equipment {
	return &networkapi.Equipment{
		Name:          name,
		EquipmentType: cfg.BaseConfig.Equipment.Type,
		Model:         cfg.BaseConfig.Equipment.Model,
		Environments: []networkapi.Environment{
			{Environment: cfg.BaseConfig.Equipment.Environment},
		},
		Groups: []networkapi.IDOnly{
			{ID: cfg.BaseConfig.Equipment.Group},
		},
	}
}

func newPool(name string, port int, cfg config.InstanceConfig) *networkapi.Pool {
	return &networkapi.Pool{
		Identifier:        name,
		DefaultPort:       port,
		Environment:       networkapi.IntOrID{ID: cfg.PoolEnvironmentID},
		ServiceDownAction: networkapi.ServiceDownAction{Name: "none"},
		LBMethod:          "round-robin",
		HealthCheck: networkapi.HealthCheck{
			Type:        "TCP",
			Destination: "*:*",
		},
		DefaultLimit: 0,
	}
}

func newVIP(name string, cfg config.InstanceConfig, vipIP *networkapi.IP, http, https *networkapi.Pool) *networkapi.VIP {
	return &networkapi.VIP{
		Name:           name,
		Service:        name,
		Business:       "tsuru gke",
		EnvironmentVIP: networkapi.IntOrID{ID: cfg.VIPEnvironmentID},
		IPv4:           &networkapi.IntOrID{ID: vipIP.ID},
		Ports: []networkapi.VIPPort{
			{
				Port: 80,
				Pools: []networkapi.VIPPool{
					{
						ServerPool: networkapi.IntOrID{ID: http.ID},
						L7Rule:     networkapi.IntOrID{ID: cfg.VIPL7RuleID},
					},
				},
				Options: networkapi.VIPPortOptions{
					L4Protocol: networkapi.IntOrID{ID: cfg.VIPL4ProtocolID},
					L7Protocol: networkapi.IntOrID{ID: cfg.VIPL7ProtocolID},
				},
			},
			{
				Port: 443,
				Pools: []networkapi.VIPPool{
					{
						ServerPool: networkapi.IntOrID{ID: https.ID},
						L7Rule:     networkapi.IntOrID{ID: cfg.VIPL7RuleID},
					},
				},
				Options: networkapi.VIPPortOptions{
					L4Protocol: networkapi.IntOrID{ID: cfg.VIPL4ProtocolID},
					L7Protocol: networkapi.IntOrID{ID: cfg.VIPL7ProtocolID},
				},
			},
		},
		Options: networkapi.VIPOptions{
			CacheGroup:    networkapi.IntOrID{ID: cfg.CacheGroupID},
			TrafficReturn: networkapi.IntOrID{ID: cfg.TrafficReturnID},
			Persistence:   networkapi.IntOrID{ID: cfg.PersistenceID},
			Timeout:       networkapi.IntOrID{ID: cfg.TimeoutID},
		},
	}
}

func newPoolMember(tg target, netIP *networkapi.IP) networkapi.PoolMember {
	return networkapi.PoolMember{
		IP: &networkapi.PoolMemberIP{
			ID:         netIP.ID,
			IPFormated: tg.IP.String(),
		},
		PortReal:     tg.Port,
		Priority:     1,
		Weight:       1,
		MemberStatus: 0b011,
	}
}

func fillPoolUpdate(existingPool, wantedPool *networkapi.Pool) {
	wantedPool.ID = existingPool.ID
	wantedPool.PoolCreated = existingPool.PoolCreated
	existingMemberIPMap := map[int]networkapi.PoolMember{}
	for _, existingMember := range existingPool.Members {
		existingMemberIPMap[existingMember.IP.ID] = existingMember
	}

	for i, wantedMember := range wantedPool.Members {
		if existingMember, ok := existingMemberIPMap[wantedMember.IP.ID]; ok {
			wantedMember.ID = existingMember.ID
			wantedPool.Members[i] = wantedMember
		}
	}
}

func fillVIPUpdate(existingVIP, wantedVIP *networkapi.VIP) {
	wantedVIP.ID = existingVIP.ID
	wantedVIP.Created = existingVIP.Created

	existingPortMap := map[int]networkapi.VIPPort{}
	for _, existingPort := range existingVIP.Ports {
		existingPortMap[existingPort.Port] = existingPort
	}

	for i, wantedPort := range wantedVIP.Ports {
		existingPort, ok := existingPortMap[wantedPort.Port]
		if !ok {
			continue
		}
		wantedPort.ID = existingPort.ID

		existingPoolMap := map[int]networkapi.VIPPool{}
		for _, existingPool := range existingPort.Pools {
			existingPoolMap[existingPool.ServerPool.ID] = existingPool
		}
		for j, wantedPool := range wantedPort.Pools {
			if existingPool, ok := existingPoolMap[wantedPool.ServerPool.ID]; ok {
				wantedPool.ID = existingPool.ID
				wantedPort.Pools[j] = wantedPool
			}
		}

		wantedVIP.Ports[i] = wantedPort
	}
}

func (r *reconcileIngress) getNetworkAPI() networkapi.NetworkAPI {
	if r.networkAPIClient != nil {
		return r.networkAPIClient
	}

	return networkapi.Client(r.cfg.NetworkAPIURL, r.cfg.NetworkAPIUsername, r.cfg.NetworkAPIPassword)
}

func (r *reconcileIngress) cleanupNetworkAPI(ctx context.Context, ingName types.NamespacedName) error {
	lg := log.FromContext(ctx)
	if r.cfg.DebugDisableCleanup {
		lg.Info("Would cleanup ingress from network api")
		return nil
	}

	netapiCli := r.getNetworkAPI()

	vipName := r.vipName(ingName)

	vip, err := netapiCli.GetVIP(ctx, vipName)
	if err != nil && !networkapi.IsNotFound(err) {
		return err
	}
	if !networkapi.IsNotFound(err) {
		if err = netapiCli.DeleteVIP(ctx, vip); err != nil {
			return err
		}
	}

	vipIP, err := netapiCli.GetIPByName(ctx, vipName)
	if err != nil && !networkapi.IsNotFound(err) {
		return err
	}
	if !networkapi.IsNotFound(err) {
		if err = netapiCli.DeleteIP(ctx, vipIP.ID); err != nil {
			return err
		}
	}

	httpPool, err := netapiCli.GetPool(ctx, r.httpPoolName(ingName))
	if err != nil && !networkapi.IsNotFound(err) {
		return err
	}

	if !networkapi.IsNotFound(err) {
		if err = netapiCli.DeletePool(ctx, httpPool.ID); err != nil {
			return err
		}
	}

	return nil
}

func (r *reconcileIngress) reconcileNetworkAPI(ctx context.Context, ing *networkingv1.Ingress, targets []target) error {
	lg := log.FromContext(ctx)

	netapiCli := r.getNetworkAPI()

	instCfg := config.FromInstance(ing, r.cfg)

	wantedHTTPPool := newPool(r.httpPoolName(namespacedName(ing)), 80, instCfg)
	wantedHTTPSPool := newPool(r.httpsPoolName(namespacedName(ing)), 443, instCfg)

	for _, tg := range targets {
		targetName := r.targetName(tg)

		equip, err := netapiCli.GetEquipment(ctx, targetName)
		if networkapi.IsNotFound(err) {
			newEquip := newEquipment(targetName, instCfg)
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

		member := newPoolMember(tg, netIP)

		if tg.TLS {
			wantedHTTPSPool.Members = append(wantedHTTPSPool.Members, member)
		} else {
			wantedHTTPPool.Members = append(wantedHTTPPool.Members, member)
		}
	}

	httpPool, err := netapiCli.GetPool(ctx, wantedHTTPPool.Identifier)
	if err != nil && !networkapi.IsNotFound(err) {
		return err
	}

	if networkapi.IsNotFound(err) {
		httpPool, err = netapiCli.CreatePool(ctx, wantedHTTPPool)
	} else {
		fillPoolUpdate(httpPool, wantedHTTPPool)

		if !httpPool.DeepEqual(*wantedHTTPPool) {
			lg.Info("Updating pool with differences", "diff", pretty.Diff(*httpPool, *wantedHTTPPool))
			httpPool, err = netapiCli.UpdatePool(ctx, wantedHTTPPool)
		}
	}
	if err != nil {
		return err
	}

	httpsPool, err := netapiCli.GetPool(ctx, wantedHTTPSPool.Identifier)
	if err != nil && !networkapi.IsNotFound(err) {
		return err
	}

	if networkapi.IsNotFound(err) {
		httpsPool, err = netapiCli.CreatePool(ctx, wantedHTTPSPool)
	} else {
		fillPoolUpdate(httpsPool, wantedHTTPSPool)

		if !httpsPool.DeepEqual(*wantedHTTPSPool) {
			lg.Info("Updating pool with differences", "diff", pretty.Diff(*httpsPool, *wantedHTTPSPool))
			httpsPool, err = netapiCli.UpdatePool(ctx, wantedHTTPSPool)
		}
	}
	if err != nil {
		return err
	}

	if takeOverVIPName := ing.Annotations[config.TakeOverAnnotation]; takeOverVIPName != "" {
		return r.reconcileNetworkAPITakeOver(ctx, takeOverVIPName, ing, httpPool, httpsPool)
	}

	vipName := r.vipName(namespacedName(ing))

	vipIP, err := netapiCli.GetIPByName(ctx, vipName)
	if err != nil && !networkapi.IsNotFound(err) {
		return err
	}
	if networkapi.IsNotFound(err) {
		vipIP, err = netapiCli.CreateVIPIPv4(ctx, vipName, instCfg.VIPEnvironmentID)
	}
	if err != nil {
		return err
	}

	wantedVIP := newVIP(vipName, instCfg, vipIP, httpPool, httpsPool)

	vip, err := netapiCli.GetVIP(ctx, wantedVIP.Name)
	if err != nil && !networkapi.IsNotFound(err) {
		return err
	}

	if networkapi.IsNotFound(err) {
		vip, err = netapiCli.CreateVIP(ctx, wantedVIP)
	} else {
		fillVIPUpdate(vip, wantedVIP)
		if !vip.DeepEqual(*wantedVIP) {
			lg.Info("Updating vip with differences", "diff", pretty.Diff(*vip, *wantedVIP))
			vip, err = netapiCli.UpdateVIP(ctx, wantedVIP)
		}
	}
	if err != nil {
		return err
	}

	return r.deployAndUpdateStatus(ctx, ing, vip, vipIP)
}

func (r *reconcileIngress) deployAndUpdateStatus(ctx context.Context, ing *networkingv1.Ingress, vip *networkapi.VIP, vipIP *networkapi.IP) error {
	netapiCli := r.getNetworkAPI()

	if !vip.Created {
		err := netapiCli.DeployVIP(ctx, vip.ID)
		if err != nil {
			return err
		}
	}

	vipIPStr := vipIP.ToNetIP().String()

	if len(ing.Status.LoadBalancer.Ingress) != 1 || ing.Status.LoadBalancer.Ingress[0].IP != vipIPStr {
		ing.Status.LoadBalancer.Ingress = []corev1.LoadBalancerIngress{{IP: vipIPStr}}
		err := r.client.Status().Update(ctx, ing)
		if err != nil {
			return err
		}
	}

	return r.client.Status().Update(ctx, ing)
}

func (r *reconcileIngress) reconcileNetworkAPITakeOver(ctx context.Context, takeOverVIPName string, ing *networkingv1.Ingress, httpPool, httpsPool *networkapi.Pool) error {
	lg := log.FromContext(ctx)

	netapiCli := r.getNetworkAPI()

	vip, err := netapiCli.GetVIP(ctx, takeOverVIPName)
	if err != nil {
		return errors.Wrap(err, "could not get VIP")
	}

	if vip.IPv4 == nil {
		return errors.New("no ipv4 found")
	}

	vipIP, err := netapiCli.GetIPByID(ctx, vip.IPv4.ID)
	if err != nil {
		return errors.Wrap(err, "could not get VIP IP")
	}

	instCfg := config.FromInstance(ing, r.cfg)

	wantedVIP := newVIP(vip.Name, instCfg, vipIP, httpPool, httpsPool)

	fillVIPUpdate(vip, wantedVIP)

	if !vip.DeepEqual(*wantedVIP) {
		lg.Info("Updating vip with differences", "diff", pretty.Diff(*vip, *wantedVIP))
		vip, err = netapiCli.UpdateVIP(ctx, wantedVIP)
		if err != nil {
			return err
		}
	}
	return r.deployAndUpdateStatus(ctx, ing, vip, vipIP)
}
