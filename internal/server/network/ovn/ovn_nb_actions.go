package ovn

import (
	"context"
	"fmt"
	"net"
	"slices"
	"strconv"
	"strings"
	"time"

	ovsClient "github.com/ovn-org/libovsdb/client"
	ovsModel "github.com/ovn-org/libovsdb/model"
	"github.com/ovn-org/libovsdb/ovsdb"

	"github.com/lxc/incus/v6/internal/iprange"
	ovnNB "github.com/lxc/incus/v6/internal/server/network/ovn/schema/ovn-nb"
	"github.com/lxc/incus/v6/shared/util"
)

// OVNRouter OVN router name.
type OVNRouter string

// OVNRouterPort OVN router port name.
type OVNRouterPort string

// OVNSwitch OVN switch name.
type OVNSwitch string

// OVNSwitchPort OVN switch port name.
type OVNSwitchPort string

// OVNSwitchPortUUID OVN switch port UUID.
type OVNSwitchPortUUID string

// OVNChassisGroup OVN HA chassis group name.
type OVNChassisGroup string

// OVNDNSUUID OVN DNS record UUID.
type OVNDNSUUID string

// OVNDHCPOptionsUUID DHCP Options set UUID.
type OVNDHCPOptionsUUID string

// OVNPortGroup OVN port group name.
type OVNPortGroup string

// OVNPortGroupUUID OVN port group UUID.
type OVNPortGroupUUID string

// OVNLoadBalancer OVN load balancer name.
type OVNLoadBalancer string

// OVNAddressSet OVN address set for ACLs.
type OVNAddressSet string

// OVNIPAllocationOpts defines IP allocation settings that can be applied to a logical switch.
type OVNIPAllocationOpts struct {
	PrefixIPv4  *net.IPNet
	PrefixIPv6  *net.IPNet
	ExcludeIPv4 []iprange.Range
}

// OVNIPv6AddressMode IPv6 router advertisement address mode.
type OVNIPv6AddressMode string

// OVNIPv6AddressModeSLAAC IPv6 SLAAC mode.
const OVNIPv6AddressModeSLAAC OVNIPv6AddressMode = "slaac"

// OVNIPv6AddressModeDHCPStateful IPv6 DHCPv6 stateful mode.
const OVNIPv6AddressModeDHCPStateful OVNIPv6AddressMode = "dhcpv6_stateful"

// OVNIPv6AddressModeDHCPStateless IPv6 DHCPv6 stateless mode.
const OVNIPv6AddressModeDHCPStateless OVNIPv6AddressMode = "dhcpv6_stateless"

// OVN External ID names used by Incus.
const ovnExtIDIncusSwitch = "incus_switch"
const ovnExtIDIncusSwitchPort = "incus_switch_port"
const ovnExtIDIncusProjectID = "incus_project_id"
const ovnExtIDIncusPortGroup = "incus_port_group"
const ovnExtIDIncusLocation = "incus_location"

// OVNIPv6RAOpts IPv6 router advertisements options that can be applied to a router.
type OVNIPv6RAOpts struct {
	SendPeriodic       bool
	AddressMode        OVNIPv6AddressMode
	MinInterval        time.Duration
	MaxInterval        time.Duration
	RecursiveDNSServer net.IP
	DNSSearchList      []string
	MTU                uint32
}

// OVNDHCPOptsSet is an existing DHCP options set in the northbound database.
type OVNDHCPOptsSet struct {
	UUID OVNDHCPOptionsUUID
	CIDR *net.IPNet
}

// OVNDHCPv4Opts IPv4 DHCP options that can be applied to a switch port.
type OVNDHCPv4Opts struct {
	ServerID           net.IP
	ServerMAC          net.HardwareAddr
	Router             net.IP
	RecursiveDNSServer []net.IP
	DomainName         string
	LeaseTime          time.Duration
	MTU                uint32
	Netmask            string
}

// OVNDHCPv6Opts IPv6 DHCP option set that can be created (and then applied to a switch port by resulting ID).
type OVNDHCPv6Opts struct {
	ServerID           net.HardwareAddr
	RecursiveDNSServer []net.IP
	DNSSearchList      []string
}

// OVNSwitchPortOpts options that can be applied to a swich port.
type OVNSwitchPortOpts struct {
	MAC          net.HardwareAddr   // Optional, if nil will be set to dynamic.
	IPs          []net.IP           // Optional, if empty IPs will be set to dynamic.
	DHCPv4OptsID OVNDHCPOptionsUUID // Optional, if empty, no DHCPv4 enabled on port.
	DHCPv6OptsID OVNDHCPOptionsUUID // Optional, if empty, no DHCPv6 enabled on port.
	Parent       OVNSwitchPort      // Optional, if set a nested port is created.
	VLAN         uint16             // Optional, use with Parent to request a specific VLAN for nested port.
	Location     string             // Optional, use to indicate the name of the server this port is bound to.
	RouterPort   OVNRouterPort      // Optional, the name of the associated logical router port.
}

// OVNACLRule represents an ACL rule that can be added to a logical switch or port group.
type OVNACLRule struct {
	Direction string // Either "from-lport" or "to-lport".
	Action    string // Either "allow-related", "allow", "drop", or "reject".
	Match     string // Match criteria. See OVN Southbound database's Logical_Flow table match column usage.
	Priority  int    // Priority (between 0 and 32767, inclusive). Higher values take precedence.
	Log       bool   // Whether or not to log matched packets.
	LogName   string // Log label name (requires Log be true).
}

// OVNLoadBalancerTarget represents an OVN load balancer Virtual IP target.
type OVNLoadBalancerTarget struct {
	Address net.IP
	Port    uint64
}

// OVNLoadBalancerVIP represents a OVN load balancer Virtual IP entry.
type OVNLoadBalancerVIP struct {
	Protocol      string // Either "tcp" or "udp". But only applies to port based VIPs.
	ListenAddress net.IP
	ListenPort    uint64
	Targets       []OVNLoadBalancerTarget
}

// OVNRouterRoute represents a static route added to a logical router.
type OVNRouterRoute struct {
	Prefix  net.IPNet
	NextHop net.IP
	Port    OVNRouterPort
	Discard bool
}

// OVNRouterPolicy represents a router policy.
type OVNRouterPolicy struct {
	Priority int
	Match    string
	Action   string
	NextHop  net.IP
}

// OVNRouterPeering represents a the configuration of a peering connection between two OVN logical routers.
type OVNRouterPeering struct {
	LocalRouter        OVNRouter
	LocalRouterPort    OVNRouterPort
	LocalRouterPortMAC net.HardwareAddr
	LocalRouterPortIPs []net.IPNet
	LocalRouterRoutes  []net.IPNet

	TargetRouter        OVNRouter
	TargetRouterPort    OVNRouterPort
	TargetRouterPortMAC net.HardwareAddr
	TargetRouterPortIPs []net.IPNet
	TargetRouterRoutes  []net.IPNet
}

// CreateLogicalRouter adds a named logical router.
// If mayExist is true, then an existing resource of the same name is not treated as an error.
func (o *NB) CreateLogicalRouter(ctx context.Context, routerName OVNRouter, mayExist bool) error {
	logicalRouter := ovnNB.LogicalRouter{
		Name: string(routerName),
	}

	// Check if already exists.
	err := o.get(ctx, &logicalRouter)
	if err != nil && err != ErrNotFound {
		return err
	}

	if logicalRouter.UUID != "" {
		if mayExist {
			return nil
		}

		return ErrExists
	}

	// Create the record.
	operations, err := o.client.Create(&logicalRouter)
	if err != nil {
		return err
	}

	// Apply the changes.
	resp, err := o.client.Transact(ctx, operations...)
	if err != nil {
		return err
	}

	_, err = ovsdb.CheckOperationResults(resp, operations)
	if err != nil {
		return err
	}

	return nil
}

// DeleteLogicalRouter deletes a named logical router.
func (o *NB) DeleteLogicalRouter(ctx context.Context, routerName OVNRouter) error {
	logicalRouter := ovnNB.LogicalRouter{
		Name: string(routerName),
	}

	err := o.get(ctx, &logicalRouter)
	if err != nil {
		// Logical router is already gone.
		if err == ErrNotFound {
			return nil
		}

		return err
	}

	operations, err := o.client.Where(&logicalRouter).Delete()
	if err != nil {
		return err
	}

	// Apply the changes.
	resp, err := o.client.Transact(ctx, operations...)
	if err != nil {
		return err
	}

	_, err = ovsdb.CheckOperationResults(resp, operations)
	if err != nil {
		return err
	}

	return nil
}

// GetLogicalRouter gets the OVN database record for the router.
func (o *NB) GetLogicalRouter(ctx context.Context, routerName OVNRouter) (*ovnNB.LogicalRouter, error) {
	logicalRouter := &ovnNB.LogicalRouter{
		Name: string(routerName),
	}

	err := o.get(ctx, logicalRouter)
	if err != nil {
		return nil, err
	}

	return logicalRouter, nil
}

// CreateLogicalRouterNAT adds an SNAT or DNAT rule to a logical router to translate packets from intNet to extIP.
func (o *NB) CreateLogicalRouterNAT(ctx context.Context, routerName OVNRouter, natType string, intNet *net.IPNet, extIP net.IP, intIP net.IP, stateless bool, mayExist bool) error {
	// Prepare the addresses.
	var logicalIP string
	var externalIP string

	if natType == "snat" {
		logicalIP = intNet.String()
		externalIP = extIP.String()
	} else if natType == "dnat_and_snat" {
		logicalIP = intIP.String()
		externalIP = extIP.String()
	} else {
		return fmt.Errorf("Invalid NAT rule type %q", natType)
	}

	// Get the logical router.
	logicalRouter, err := o.GetLogicalRouter(ctx, routerName)
	if err != nil {
		return err
	}

	// Check if the rule already exists.
	for _, natUUID := range logicalRouter.Nat {
		natRule := ovnNB.NAT{
			UUID: natUUID,
		}

		err = o.get(ctx, &natRule)
		if err != nil {
			return err
		}

		// Check if rule is of the requested type.
		if natRule.Type != natType {
			continue
		}

		// Check if matching our new rule.
		if natRule.LogicalIP == logicalIP && natRule.ExternalIP == externalIP {
			if mayExist {
				return nil
			}

			return ErrExists
		}
	}

	natRule := ovnNB.NAT{
		UUID:       "nat",
		Options:    map[string]string{"stateless": fmt.Sprintf("%v", stateless)},
		Type:       natType,
		LogicalIP:  logicalIP,
		ExternalIP: externalIP,
	}

	operations := []ovsdb.Operation{}

	createOps, err := o.client.Create(&natRule)
	if err != nil {
		return err
	}

	operations = append(operations, createOps...)

	// Add it to the router.
	updateOps, err := o.client.Where(logicalRouter).Mutate(logicalRouter, ovsModel.Mutation{
		Field:   &logicalRouter.Nat,
		Mutator: ovsdb.MutateOperationInsert,
		Value:   []string{natRule.UUID},
	})
	if err != nil {
		return err
	}

	operations = append(operations, updateOps...)

	// Apply the changes.
	resp, err := o.client.Transact(ctx, operations...)
	if err != nil {
		return err
	}

	_, err = ovsdb.CheckOperationResults(resp, operations)
	if err != nil {
		return err
	}

	return nil
}

// DeleteLogicalRouterNAT deletes all NAT rules of a particular type from a logical router.
func (o *NB) DeleteLogicalRouterNAT(ctx context.Context, routerName OVNRouter, natType string, all bool, extIPs ...net.IP) error {
	// Quick checks.
	if all && len(extIPs) != 0 {
		return fmt.Errorf("Can't ask for all NAT rules to be deleted and specify specific addresses")
	}

	// Get the logical router.
	logicalRouter, err := o.GetLogicalRouter(ctx, routerName)
	if err != nil {
		return err
	}

	operations := []ovsdb.Operation{}

	// Go through all rules.
	for _, natUUID := range logicalRouter.Nat {
		natRule := ovnNB.NAT{
			UUID: natUUID,
		}

		err = o.get(ctx, &natRule)
		if err != nil {
			return err
		}

		// Check if rule is of the requested type.
		if natRule.Type != natType {
			continue
		}

		// Check if the address matches.
		if !all {
			found := false
			for _, extIP := range extIPs {
				if natRule.ExternalIP == extIP.String() {
					found = true
					break
				}
			}

			if !found {
				continue
			}
		}

		// Delete the rule.
		deleteOps, err := o.client.Where(&natRule).Delete()
		if err != nil {
			return err
		}

		operations = append(operations, deleteOps...)

		// Delete the entry from the logical router.
		deleteOps, err = o.client.Where(logicalRouter).Mutate(logicalRouter, ovsModel.Mutation{
			Field:   &logicalRouter.Nat,
			Mutator: ovsdb.MutateOperationDelete,
			Value:   []string{natRule.UUID},
		})

		if err != nil {
			return err
		}

		operations = append(operations, deleteOps...)
	}

	if len(operations) == 0 {
		return nil
	}

	// Apply the changes.
	resp, err := o.client.Transact(ctx, operations...)
	if err != nil {
		return err
	}

	_, err = ovsdb.CheckOperationResults(resp, operations)
	if err != nil {
		return err
	}

	return nil
}

// CreateLogicalRouterRoute adds a static route to the logical router.
func (o *NB) CreateLogicalRouterRoute(ctx context.Context, routerName OVNRouter, mayExist bool, routes ...OVNRouterRoute) error {
	// Get the logical router.
	logicalRouter, err := o.GetLogicalRouter(ctx, routerName)
	if err != nil {
		return err
	}

	// Get the existing routes.
	existingRoutes := make([]ovnNB.LogicalRouterStaticRoute, 0, len(logicalRouter.StaticRoutes))
	for _, uuid := range logicalRouter.StaticRoutes {
		route := ovnNB.LogicalRouterStaticRoute{
			UUID: uuid,
		}

		err = o.get(ctx, &route)
		if err != nil {
			return err
		}

		existingRoutes = append(existingRoutes, route)
	}

	// Add the new routes.
	operations := []ovsdb.Operation{}
	for i, route := range routes {
		// Check if already present.
		for _, existing := range existingRoutes {
			if existing.IPPrefix != route.Prefix.String() {
				continue
			}

			if existing.Nexthop == "discard" && !route.Discard {
				continue
			}

			if existing.Nexthop != route.NextHop.String() {
				continue
			}

			if existing.OutputPort == nil {
				if string(route.Port) != "" {
					continue
				}
			} else if *existing.OutputPort != string(route.Port) {
				continue
			}

			if mayExist {
				continue
			}

			return ErrExists
		}

		// Create the new record.
		staticRoute := ovnNB.LogicalRouterStaticRoute{
			UUID:     fmt.Sprintf("route_%d", i),
			IPPrefix: route.Prefix.String(),
		}

		if string(route.Port) != "" {
			value := string(route.Port)
			staticRoute.OutputPort = &value
		}

		if route.Discard {
			staticRoute.Nexthop = "discard"
		} else {
			staticRoute.Nexthop = route.NextHop.String()
		}

		createOps, err := o.client.Create(&staticRoute)
		if err != nil {
			return err
		}

		operations = append(operations, createOps...)

		// Add it to the router.
		updateOps, err := o.client.Where(logicalRouter).Mutate(logicalRouter, ovsModel.Mutation{
			Field:   &logicalRouter.StaticRoutes,
			Mutator: ovsdb.MutateOperationInsert,
			Value:   []string{staticRoute.UUID},
		})
		if err != nil {
			return err
		}

		operations = append(operations, updateOps...)
	}

	if len(operations) == 0 {
		return nil
	}

	// Apply the database changes.
	resp, err := o.client.Transact(ctx, operations...)
	if err != nil {
		return err
	}

	_, err = ovsdb.CheckOperationResults(resp, operations)
	if err != nil {
		return err
	}

	return nil
}

// DeleteLogicalRouterRoute deletes a static route from the logical router.
func (o *NB) DeleteLogicalRouterRoute(ctx context.Context, routerName OVNRouter, prefixes ...net.IPNet) error {
	// Get the logical router.
	logicalRouter, err := o.GetLogicalRouter(ctx, routerName)
	if err != nil {
		return err
	}

	// Get the existing routes.
	existingRoutes := make([]ovnNB.LogicalRouterStaticRoute, 0, len(logicalRouter.StaticRoutes))
	for _, uuid := range logicalRouter.StaticRoutes {
		route := ovnNB.LogicalRouterStaticRoute{
			UUID: uuid,
		}

		err = o.client.Get(ctx, &route)
		if err != nil {
			return err
		}

		existingRoutes = append(existingRoutes, route)
	}

	// Delete the requested routes.
	operations := []ovsdb.Operation{}
	for _, prefix := range prefixes {
		var route ovnNB.LogicalRouterStaticRoute

		// Look for a matching entry.
		for _, existing := range existingRoutes {
			// Normal CIDR entry.
			if existing.IPPrefix == prefix.String() {
				route = existing
				break
			}

			// IP-only entry.
			ones, bits := prefix.Mask.Size()
			if ones == bits && existing.IPPrefix == prefix.IP.String() {
				route = existing
				break
			}
		}

		if route.UUID == "" {
			continue
		}

		// Delete the entry.
		deleteOps, err := o.client.Where(&route).Delete()
		if err != nil {
			return err
		}

		operations = append(operations, deleteOps...)

		// Remove from the router.
		updateOps, err := o.client.Where(logicalRouter).Mutate(logicalRouter, ovsModel.Mutation{
			Field:   &logicalRouter.StaticRoutes,
			Mutator: ovsdb.MutateOperationDelete,
			Value:   []string{route.UUID},
		})
		if err != nil {
			return err
		}

		operations = append(operations, updateOps...)
	}

	if len(operations) == 0 {
		return nil
	}

	// Apply the database changes.
	resp, err := o.client.Transact(ctx, operations...)
	if err != nil {
		return err
	}

	_, err = ovsdb.CheckOperationResults(resp, operations)
	if err != nil {
		return err
	}

	return nil
}

// GetLogicalRouterPort gets the OVN database record for the logical router port.
func (o *NB) GetLogicalRouterPort(ctx context.Context, portName OVNRouterPort) (*ovnNB.LogicalRouterPort, error) {
	logicalRouterPort := &ovnNB.LogicalRouterPort{
		Name: string(portName),
	}

	err := o.get(ctx, logicalRouterPort)
	if err != nil {
		return nil, err
	}

	return logicalRouterPort, nil
}

// CreateLogicalRouterPort adds a named logical router port to a logical router.
func (o *NB) CreateLogicalRouterPort(ctx context.Context, routerName OVNRouter, portName OVNRouterPort, mac net.HardwareAddr, gatewayMTU uint32, ipAddr []*net.IPNet, haChassisGroupName OVNChassisGroup, mayExist bool) error {
	// Prepare the addresses.
	networks := make([]string, 0, len(ipAddr))

	for _, addr := range ipAddr {
		networks = append(networks, addr.String())
	}

	// Prepare the new router port entry.
	logicalRouterPort := ovnNB.LogicalRouterPort{
		Name: string(portName),
		UUID: "lrp",
	}

	// Check if the entry already exists.
	err := o.get(ctx, &logicalRouterPort)
	if err != nil && err != ErrNotFound {
		return err
	}

	if logicalRouterPort.UUID != "lrp" && !mayExist {
		return ErrExists
	}

	// Apply the configuration.
	logicalRouterPort.MAC = mac.String()
	logicalRouterPort.Networks = networks
	if haChassisGroupName != "" {
		haChassisGroup := ovnNB.HAChassisGroup{
			Name: string(haChassisGroupName),
		}

		err = o.get(ctx, &haChassisGroup)
		if err != nil {
			return err
		}

		logicalRouterPort.HaChassisGroup = &haChassisGroup.UUID
	}

	if gatewayMTU > 0 {
		if logicalRouterPort.Options == nil {
			logicalRouterPort.Options = map[string]string{}
		}

		logicalRouterPort.Options["gateway_mtu"] = fmt.Sprintf("%d", gatewayMTU)
	}

	operations := []ovsdb.Operation{}
	if logicalRouterPort.UUID != "lrp" {
		// If it already exists, update it.
		updateOps, err := o.client.Where(&logicalRouterPort).Update(&logicalRouterPort)
		if err != nil {
			return err
		}

		operations = append(operations, updateOps...)
	} else {
		// Else, create it.
		createOps, err := o.client.Create(&logicalRouterPort)
		if err != nil {
			return err
		}

		operations = append(operations, createOps...)

		// And connect it to the router.
		logicalRouter := ovnNB.LogicalRouter{
			Name: string(routerName),
		}

		updateOps, err := o.client.Where(&logicalRouter).Mutate(&logicalRouter, ovsModel.Mutation{
			Field:   &logicalRouter.Ports,
			Mutator: ovsdb.MutateOperationInsert,
			Value:   []string{logicalRouterPort.UUID},
		})
		if err != nil {
			return err
		}

		operations = append(operations, updateOps...)
	}

	// Apply the database changes.
	resp, err := o.client.Transact(ctx, operations...)
	if err != nil {
		return err
	}

	_, err = ovsdb.CheckOperationResults(resp, operations)
	if err != nil {
		return err
	}

	return nil
}

// DeleteLogicalRouterPort deletes a named logical router port from a logical router.
func (o *NB) DeleteLogicalRouterPort(ctx context.Context, routerName OVNRouter, portName OVNRouterPort) error {
	operations := []ovsdb.Operation{}

	// Get the logical router port.
	logicalRouterPort := ovnNB.LogicalRouterPort{
		Name: string(portName),
	}

	err := o.get(ctx, &logicalRouterPort)
	if err != nil {
		// Logical router port is already gone.
		if err == ErrNotFound {
			return nil
		}

		return err
	}

	// Remove the port from the router.
	logicalRouter := ovnNB.LogicalRouter{
		Name: string(routerName),
	}

	updateOps, err := o.client.Where(&logicalRouter).Mutate(&logicalRouter, ovsModel.Mutation{
		Field:   &logicalRouter.Ports,
		Mutator: ovsdb.MutateOperationDelete,
		Value:   []string{logicalRouterPort.UUID},
	})
	if err != nil {
		return err
	}

	operations = append(operations, updateOps...)

	// Delete the port itself.
	deleteOps, err := o.client.Where(&logicalRouterPort).Delete()
	if err != nil {
		return err
	}

	operations = append(operations, deleteOps...)

	// Apply the changes.
	resp, err := o.client.Transact(ctx, operations...)
	if err != nil {
		return err
	}

	_, err = ovsdb.CheckOperationResults(resp, operations)
	if err != nil {
		return err
	}

	return nil
}

// UpdateLogicalRouterPort updates properties of a logical router port.
func (o *NB) UpdateLogicalRouterPort(ctx context.Context, portName OVNRouterPort, ipv6ra *OVNIPv6RAOpts) error {
	lrp, err := o.GetLogicalRouterPort(ctx, portName)
	if err != nil {
		return err
	}

	if ipv6ra != nil {
		ipv6conf := map[string]string{}

		if ipv6ra.AddressMode != "" {
			ipv6conf["address_mode"] = string(ipv6ra.AddressMode)
		}

		if ipv6ra.MaxInterval > 0 {
			ipv6conf["max_interval"] = fmt.Sprintf("%d", ipv6ra.MaxInterval/time.Second)
		}

		if ipv6ra.MinInterval > 0 {
			ipv6conf["min_interval"] = fmt.Sprintf("%d", ipv6ra.MinInterval/time.Second)
		}

		if ipv6ra.MTU > 0 {
			ipv6conf["mtu"] = fmt.Sprintf("%d", ipv6ra.MTU)
		}

		if len(ipv6ra.DNSSearchList) > 0 {
			ipv6conf["dnssl"] = strings.Join(ipv6ra.DNSSearchList, ",")
		}

		if ipv6ra.RecursiveDNSServer != nil {
			ipv6conf["rdnss"] = ipv6ra.RecursiveDNSServer.String()
		}

		lrp.Ipv6RaConfigs = ipv6conf
	}

	// Update the record.
	operations, err := o.client.Where(lrp).Update(lrp)
	if err != nil {
		return err
	}

	// Apply the changes.
	resp, err := o.client.Transact(ctx, operations...)
	if err != nil {
		return err
	}

	_, err = ovsdb.CheckOperationResults(resp, operations)
	if err != nil {
		return err
	}

	return nil
}

// GetLogicalSwitch gets the OVN database record for the switch.
func (o *NB) GetLogicalSwitch(ctx context.Context, switchName OVNSwitch) (*ovnNB.LogicalSwitch, error) {
	logicalSwitch := &ovnNB.LogicalSwitch{
		Name: string(switchName),
	}

	err := o.get(ctx, logicalSwitch)
	if err != nil {
		return nil, err
	}

	return logicalSwitch, nil
}

// CreateLogicalSwitch adds a named logical switch.
// If mayExist is true, then an existing resource of the same name is not treated as an error.
func (o *NB) CreateLogicalSwitch(ctx context.Context, switchName OVNSwitch, mayExist bool) error {
	logicalSwitch := ovnNB.LogicalSwitch{
		Name: string(switchName),
	}

	// Check if already exists.
	err := o.get(ctx, &logicalSwitch)
	if err != nil && err != ErrNotFound {
		return err
	}

	if logicalSwitch.UUID != "" {
		if mayExist {
			return nil
		}

		return ErrExists
	}

	// Create the record.
	operations, err := o.client.Create(&logicalSwitch)
	if err != nil {
		return err
	}

	// Apply the changes.
	resp, err := o.client.Transact(ctx, operations...)
	if err != nil {
		return err
	}

	_, err = ovsdb.CheckOperationResults(resp, operations)
	if err != nil {
		return err
	}

	return nil
}

// DeleteLogicalSwitch deletes a named logical switch.
func (o *NB) DeleteLogicalSwitch(ctx context.Context, switchName OVNSwitch) error {
	ls := ovnNB.LogicalSwitch{
		Name: string(switchName),
	}

	// Check if the switch exists.
	err := o.get(ctx, &ls)
	if err != nil {
		return err
	}

	// Delete the switch itself.
	operations := []ovsdb.Operation{}

	deleteOps, err := o.client.Where(&ls).Delete()
	if err != nil {
		return err
	}

	operations = append(operations, deleteOps...)

	// Delete all associated port groups.
	portGroups := []ovnNB.PortGroup{}
	err = o.client.WhereCache(func(pg *ovnNB.PortGroup) bool {
		return pg.ExternalIDs != nil && pg.ExternalIDs[ovnExtIDIncusSwitch] == string(switchName)
	}).List(ctx, &portGroups)
	if err != nil {
		return err
	}

	for _, pg := range portGroups {
		deleteOps, err := o.client.Where(&pg).Delete()
		if err != nil {
			return err
		}

		operations = append(operations, deleteOps...)
	}

	// Delete all associated DHCP options.
	dhcpOptions := []ovnNB.DHCPOptions{}
	err = o.client.WhereCache(func(do *ovnNB.DHCPOptions) bool {
		return do.ExternalIDs != nil && do.ExternalIDs[ovnExtIDIncusSwitch] == string(switchName)
	}).List(ctx, &dhcpOptions)
	if err != nil {
		return err
	}

	for _, do := range dhcpOptions {
		deleteOps, err := o.client.Where(&do).Delete()
		if err != nil {
			return err
		}

		operations = append(operations, deleteOps...)
	}

	// Delete all associated DNS records.
	dnsRecords := []ovnNB.DNS{}
	err = o.client.WhereCache(func(dr *ovnNB.DNS) bool {
		return dr.ExternalIDs != nil && dr.ExternalIDs[ovnExtIDIncusSwitch] == string(switchName)
	}).List(ctx, &dnsRecords)
	if err != nil {
		return err
	}

	for _, dr := range dnsRecords {
		deleteOps, err := o.client.Where(&dr).Delete()
		if err != nil {
			return err
		}

		operations = append(operations, deleteOps...)
	}

	// Apply the database changes.
	resp, err := o.client.Transact(ctx, operations...)
	if err != nil {
		return err
	}

	_, err = ovsdb.CheckOperationResults(resp, operations)
	if err != nil {
		return err
	}

	return nil
}

// logicalSwitchParseExcludeIPs parses the ips into OVN exclude_ips format.
func (o *NB) logicalSwitchParseExcludeIPs(ips []iprange.Range) ([]string, error) {
	excludeIPs := make([]string, 0, len(ips))
	for _, v := range ips {
		if v.Start == nil || v.Start.To4() == nil {
			return nil, fmt.Errorf("Invalid exclude IPv4 range start address")
		} else if v.End == nil {
			excludeIPs = append(excludeIPs, v.Start.String())
		} else {
			if v.End != nil && v.End.To4() == nil {
				return nil, fmt.Errorf("Invalid exclude IPv4 range end address")
			}

			excludeIPs = append(excludeIPs, fmt.Sprintf("%s..%s", v.Start.String(), v.End.String()))
		}
	}

	return excludeIPs, nil
}

// UpdateLogicalSwitchIPAllocation sets the IP allocation config on the logical switch.
func (o *NB) UpdateLogicalSwitchIPAllocation(ctx context.Context, switchName OVNSwitch, opts *OVNIPAllocationOpts) error {
	// Get the logical switch.
	logicalSwitch, err := o.GetLogicalSwitch(ctx, switchName)
	if err != nil {
		return err
	}

	// Update the configuration.
	if logicalSwitch.OtherConfig == nil {
		logicalSwitch.OtherConfig = map[string]string{}
	}

	if opts.PrefixIPv4 != nil {
		logicalSwitch.OtherConfig["subnet"] = opts.PrefixIPv4.String()
	} else {
		delete(logicalSwitch.OtherConfig, "subnet")
	}

	if opts.PrefixIPv6 != nil {
		logicalSwitch.OtherConfig["ipv6_prefix"] = opts.PrefixIPv6.String()
	} else {
		delete(logicalSwitch.OtherConfig, "ipv6_prefix")
	}

	if len(opts.ExcludeIPv4) > 0 {
		excludeIPs, err := o.logicalSwitchParseExcludeIPs(opts.ExcludeIPv4)
		if err != nil {
			return err
		}

		logicalSwitch.OtherConfig["exclude_ips"] = strings.Join(excludeIPs, " ")
	} else {
		delete(logicalSwitch.OtherConfig, "exclude_ips")
	}

	operations, err := o.client.Where(logicalSwitch).Update(logicalSwitch)
	if err != nil {
		return err
	}

	// Apply the database changes.
	resp, err := o.client.Transact(ctx, operations...)
	if err != nil {
		return err
	}

	_, err = ovsdb.CheckOperationResults(resp, operations)
	if err != nil {
		return err
	}

	return nil
}

// UpdateLogicalSwitchDHCPv4Revervations sets the DHCPv4 IP reservations.
func (o *NB) UpdateLogicalSwitchDHCPv4Revervations(ctx context.Context, switchName OVNSwitch, reservedIPs []iprange.Range) error {
	// Get the logical switch.
	logicalSwitch, err := o.GetLogicalSwitch(ctx, switchName)
	if err != nil {
		return err
	}

	// Update the configuration.
	if logicalSwitch.OtherConfig == nil {
		logicalSwitch.OtherConfig = map[string]string{}
	}

	if len(reservedIPs) > 0 {
		excludeIPs, err := o.logicalSwitchParseExcludeIPs(reservedIPs)
		if err != nil {
			return err
		}

		logicalSwitch.OtherConfig["exclude_ips"] = strings.Join(excludeIPs, " ")
	} else {
		delete(logicalSwitch.OtherConfig, "exclude_ips")
	}

	operations, err := o.client.Where(logicalSwitch).Update(logicalSwitch)
	if err != nil {
		return err
	}

	// Apply the database changes.
	resp, err := o.client.Transact(ctx, operations...)
	if err != nil {
		return err
	}

	_, err = ovsdb.CheckOperationResults(resp, operations)
	if err != nil {
		return err
	}

	return nil
}

// GetLogicalSwitchDHCPv4Revervations gets the DHCPv4 IP reservations.
func (o *NB) GetLogicalSwitchDHCPv4Revervations(ctx context.Context, switchName OVNSwitch) ([]iprange.Range, error) {
	// Get the logical switch.
	logicalSwitch, err := o.GetLogicalSwitch(ctx, switchName)
	if err != nil {
		return nil, err
	}

	// Get the list of excluded IPs.
	if logicalSwitch.OtherConfig == nil {
		return []iprange.Range{}, nil
	}

	// Check if no dynamic IPs set.
	excludeIPsRaw := strings.TrimSpace(logicalSwitch.OtherConfig["exclude_ips"])
	if excludeIPsRaw == "" || excludeIPsRaw == "[]" {
		return []iprange.Range{}, nil
	}

	excludeIPsRaw, err = unquote(excludeIPsRaw)
	if err != nil {
		return nil, fmt.Errorf("Failed unquoting exclude_ips: %w", err)
	}

	excludeIPsParts := util.SplitNTrimSpace(strings.TrimSpace(excludeIPsRaw), " ", -1, true)
	excludeIPs := make([]iprange.Range, 0, len(excludeIPsParts))

	for _, excludeIPsPart := range excludeIPsParts {
		ip := net.ParseIP(excludeIPsPart) // Check if single IP part.
		if ip == nil {
			// Check if IP range part.
			start, end, found := strings.Cut(excludeIPsPart, "..")
			if !found {
				return nil, fmt.Errorf("Unrecognised exclude_ips part: %q", excludeIPsPart)
			}

			startIP := net.ParseIP(start)
			endIP := net.ParseIP(end)

			if startIP == nil || endIP == nil {
				return nil, fmt.Errorf("Invalid exclude_ips range: %q", excludeIPsPart)
			}

			// Add range IP part to list.
			excludeIPs = append(excludeIPs, iprange.Range{Start: startIP, End: endIP})
		} else {
			// Add single IP part to list.
			excludeIPs = append(excludeIPs, iprange.Range{Start: ip})
		}
	}

	return excludeIPs, nil
}

// UpdateLogicalSwitchDHCPv4Options creates or updates a DHCPv4 option set associated with the specified switchName
// and subnet. If uuid is non-empty then the record that exists with that ID is updated, otherwise a new record
// is created.
func (o *NB) UpdateLogicalSwitchDHCPv4Options(ctx context.Context, switchName OVNSwitch, uuid OVNDHCPOptionsUUID, subnet *net.IPNet, opts *OVNDHCPv4Opts) error {
	dhcpOption := ovnNB.DHCPOptions{}
	if uuid != "" {
		// Load the existing record.
		dhcpOption.UUID = string(uuid)
		err := o.get(ctx, &dhcpOption)
		if err != nil {
			return err
		}
	}

	if dhcpOption.ExternalIDs == nil {
		dhcpOption.ExternalIDs = map[string]string{}
	}

	if dhcpOption.Options == nil {
		dhcpOption.Options = map[string]string{}
	}

	dhcpOption.ExternalIDs[ovnExtIDIncusSwitch] = string(switchName)
	dhcpOption.Cidr = subnet.String()

	dhcpOption.Options["server_id"] = opts.ServerID.String()
	dhcpOption.Options["server_mac"] = opts.ServerMAC.String()
	dhcpOption.Options["lease_time"] = fmt.Sprintf("%d", opts.LeaseTime/time.Second)

	if opts.Router != nil {
		dhcpOption.Options["router"] = opts.Router.String()
	}

	if opts.RecursiveDNSServer != nil {
		nsIPs := make([]string, 0, len(opts.RecursiveDNSServer))
		for _, nsIP := range opts.RecursiveDNSServer {
			if nsIP.To4() == nil {
				continue // Only include IPv4 addresses.
			}

			nsIPs = append(nsIPs, nsIP.String())
		}

		dhcpOption.Options["dns_server"] = fmt.Sprintf("{%s}", strings.Join(nsIPs, ","))
	}

	if opts.DomainName != "" {
		// Special quoting to allow domain names.
		dhcpOption.Options["domain_name"] = fmt.Sprintf(`"%s"`, opts.DomainName)
	}

	if opts.MTU > 0 {
		dhcpOption.Options["mtu"] = fmt.Sprintf("%d", opts.MTU)
	}

	if opts.Netmask != "" {
		dhcpOption.Options["netmask"] = opts.Netmask
	}

	// Prepare the changes.
	operations := []ovsdb.Operation{}
	if dhcpOption.UUID == "" {
		// Create a new record.
		createOps, err := o.client.Create(&dhcpOption)
		if err != nil {
			return err
		}

		operations = append(operations, createOps...)
	} else {
		// Update the record.
		updateOps, err := o.client.Where(&dhcpOption).Update(&dhcpOption)
		if err != nil {
			return err
		}

		operations = append(operations, updateOps...)
	}

	// Apply the database changes.
	resp, err := o.client.Transact(ctx, operations...)
	if err != nil {
		return err
	}

	_, err = ovsdb.CheckOperationResults(resp, operations)
	if err != nil {
		return err
	}

	return nil
}

// UpdateLogicalSwitchDHCPv6Options creates or updates a DHCPv6 option set associated with the specified switchName
// and subnet. If uuid is non-empty then the record that exists with that ID is updated, otherwise a new record
// is created.
func (o *NB) UpdateLogicalSwitchDHCPv6Options(ctx context.Context, switchName OVNSwitch, uuid OVNDHCPOptionsUUID, subnet *net.IPNet, opts *OVNDHCPv6Opts) error {
	dhcpOption := ovnNB.DHCPOptions{}
	if uuid != "" {
		// Load the existing record.
		dhcpOption.UUID = string(uuid)
		err := o.get(ctx, &dhcpOption)
		if err != nil {
			return err
		}
	}

	if dhcpOption.ExternalIDs == nil {
		dhcpOption.ExternalIDs = map[string]string{}
	}

	if dhcpOption.Options == nil {
		dhcpOption.Options = map[string]string{}
	}

	dhcpOption.ExternalIDs[ovnExtIDIncusSwitch] = string(switchName)
	dhcpOption.Cidr = subnet.String()
	dhcpOption.Options["server_id"] = opts.ServerID.String()

	if len(opts.DNSSearchList) > 0 {
		// Special quoting to allow domain names.
		dhcpOption.Options["domain_search"] = fmt.Sprintf(`"%s"`, strings.Join(opts.DNSSearchList, ","))
	}

	if opts.RecursiveDNSServer != nil {
		nsIPs := make([]string, 0, len(opts.RecursiveDNSServer))
		for _, nsIP := range opts.RecursiveDNSServer {
			if nsIP.To4() != nil {
				continue // Only include IPv6 addresses.
			}

			nsIPs = append(nsIPs, nsIP.String())
		}

		dhcpOption.Options["dns_server"] = fmt.Sprintf("{%s}", strings.Join(nsIPs, ","))
	}

	// Prepare the changes.
	operations := []ovsdb.Operation{}
	if dhcpOption.UUID == "" {
		// Create a new record.
		createOps, err := o.client.Create(&dhcpOption)
		if err != nil {
			return err
		}

		operations = append(operations, createOps...)
	} else {
		// Update the record.
		updateOps, err := o.client.Where(&dhcpOption).Update(&dhcpOption)
		if err != nil {
			return err
		}

		operations = append(operations, updateOps...)
	}

	// Apply the database changes.
	resp, err := o.client.Transact(ctx, operations...)
	if err != nil {
		return err
	}

	_, err = ovsdb.CheckOperationResults(resp, operations)
	if err != nil {
		return err
	}

	return nil
}

// GetLogicalSwitchDHCPOptions retrieves the existing DHCP options defined for a logical switch.
func (o *NB) GetLogicalSwitchDHCPOptions(ctx context.Context, switchName OVNSwitch) ([]OVNDHCPOptsSet, error) {
	// Get the matching DHCP options.
	dhcpOptions := []ovnNB.DHCPOptions{}
	err := o.client.WhereCache(func(do *ovnNB.DHCPOptions) bool {
		return do.ExternalIDs != nil && do.ExternalIDs[ovnExtIDIncusSwitch] == string(switchName)
	}).List(ctx, &dhcpOptions)
	if err != nil {
		return nil, err
	}

	dhcpOpts := []OVNDHCPOptsSet{}
	for _, dhcpOption := range dhcpOptions {
		_, cidr, err := net.ParseCIDR(dhcpOption.Cidr)
		if err != nil {
			return nil, err
		}

		dhcpOpts = append(dhcpOpts, OVNDHCPOptsSet{
			UUID: OVNDHCPOptionsUUID(dhcpOption.UUID),
			CIDR: cidr,
		})
	}

	return dhcpOpts, nil
}

// DeleteLogicalSwitchDHCPOption deletes the specified DHCP options defined for a switch.
func (o *NB) DeleteLogicalSwitchDHCPOption(ctx context.Context, switchName OVNSwitch, uuids ...OVNDHCPOptionsUUID) error {
	operations := []ovsdb.Operation{}

	// Prepare deletion requests.
	for _, uuid := range uuids {
		dhcpOption := ovnNB.DHCPOptions{
			UUID: string(uuid),
		}

		deleteOps, err := o.client.Where(&dhcpOption).Delete()
		if err != nil {
			return err
		}

		operations = append(operations, deleteOps...)
	}

	// Check if there's anything to do.
	if len(operations) == 0 {
		return nil
	}

	// Apply the database changes.
	resp, err := o.client.Transact(ctx, operations...)
	if err != nil {
		return err
	}

	_, err = ovsdb.CheckOperationResults(resp, operations)
	if err != nil {
		return err
	}

	return nil
}

// LogicalSwitchSetACLRules applies a set of rules to the specified logical switch. Any existing rules are removed.
func (o *NB) LogicalSwitchSetACLRules(switchName OVNSwitch, aclRules ...OVNACLRule) error {
	// Remove any existing rules assigned to the entity.
	args := []string{"clear", "logical_switch", string(switchName), "acls"}

	// Add new rules.
	externalIDs := map[string]string{
		ovnExtIDIncusSwitch: string(switchName),
	}

	args = o.aclRuleAddAppendArgs(args, "logical_switch", string(switchName), externalIDs, nil, aclRules...)

	_, err := o.nbctl(args...)
	if err != nil {
		return err
	}

	return nil
}

// logicalSwitchPortACLRules returns the ACL rule UUIDs belonging to a logical switch port.
func (o *NB) logicalSwitchPortACLRules(ctx context.Context, portName OVNSwitchPort) ([]string, error) {
	acls := []ovnNB.ACL{}

	err := o.client.WhereCache(func(acl *ovnNB.ACL) bool {
		return acl.ExternalIDs != nil && acl.ExternalIDs[ovnExtIDIncusSwitchPort] == string(portName)
	}).List(ctx, &acls)
	if err != nil {
		return nil, err
	}

	ruleUUIDs := []string{}
	for _, acl := range acls {
		ruleUUIDs = append(ruleUUIDs, acl.UUID)
	}

	return ruleUUIDs, nil
}

// GetLogicalSwitchPorts returns a map of logical switch ports (name and UUID) for a switch.
// Includes non-instance ports, such as the router port.
func (o *NB) GetLogicalSwitchPorts(ctx context.Context, switchName OVNSwitch) (map[OVNSwitchPort]OVNSwitchPortUUID, error) {
	// Get the logical switch.
	logicalSwitch, err := o.GetLogicalSwitch(ctx, switchName)
	if err != nil {
		return nil, err
	}

	ports := make(map[OVNSwitchPort]OVNSwitchPortUUID, len(logicalSwitch.Ports))
	for _, portUUID := range logicalSwitch.Ports {
		// Get the logical switch port.
		lsp := ovnNB.LogicalSwitchPort{
			UUID: portUUID,
		}

		err := o.get(ctx, &lsp)
		if err != nil {
			return nil, err
		}

		ports[OVNSwitchPort(lsp.Name)] = OVNSwitchPortUUID(lsp.UUID)
	}

	return ports, nil
}

// LogicalSwitchIPs returns a list of IPs associated to each port connected to switch.
func (o *NB) LogicalSwitchIPs(switchName OVNSwitch) (map[OVNSwitchPort][]net.IP, error) {
	output, err := o.nbctl("--format=csv", "--no-headings", "--data=bare", "--colum=name,addresses,dynamic_addresses", "find", "logical_switch_port",
		fmt.Sprintf("external_ids:%s=%s", ovnExtIDIncusSwitch, switchName),
	)
	if err != nil {
		return nil, err
	}

	lines := util.SplitNTrimSpace(strings.TrimSpace(output), "\n", -1, true)
	portIPs := make(map[OVNSwitchPort][]net.IP, len(lines))

	for _, line := range lines {
		fields := util.SplitNTrimSpace(line, ",", -1, true)
		portName := OVNSwitchPort(fields[0])
		var ips []net.IP

		// Parse all IPs mentioned in addresses and dynamic_addresses fields.
		for i := 1; i < len(fields); i++ {
			for _, address := range util.SplitNTrimSpace(fields[i], " ", -1, true) {
				ip := net.ParseIP(address)
				if ip != nil {
					ips = append(ips, ip)
				}
			}
		}

		portIPs[portName] = ips
	}

	return portIPs, nil
}

// GetLogicalSwitchPortUUID returns the logical switch port UUID.
func (o *NB) GetLogicalSwitchPortUUID(ctx context.Context, portName OVNSwitchPort) (OVNSwitchPortUUID, error) {
	// Get the logical switch port.
	lsp := ovnNB.LogicalSwitchPort{
		Name: string(portName),
	}

	err := o.get(ctx, &lsp)
	if err != nil {
		return "", err
	}

	return OVNSwitchPortUUID(lsp.UUID), nil
}

// CreateLogicalSwitchPort adds a named logical switch port to a logical switch, and sets options if provided.
// If mayExist is true, then an existing resource of the same name is not treated as an error.
func (o *NB) CreateLogicalSwitchPort(ctx context.Context, switchName OVNSwitch, portName OVNSwitchPort, opts *OVNSwitchPortOpts, mayExist bool) error {
	// Prepare the new switch port entry.
	logicalSwitchPort := ovnNB.LogicalSwitchPort{
		Name:        string(portName),
		UUID:        "lsp",
		ExternalIDs: map[string]string{},
	}

	// Check if the entry already exists.
	err := o.get(ctx, &logicalSwitchPort)
	if err != nil && err != ErrNotFound {
		return err
	}

	if logicalSwitchPort.UUID != "lsp" && !mayExist {
		return ErrExists
	}

	// Set switch port options if supplied.
	if opts != nil {
		// Created nested VLAN port if requested.
		if opts.Parent != "" {
			parentName := string(opts.Parent)
			tag := int(opts.VLAN)

			logicalSwitchPort.ParentName = &parentName
			logicalSwitchPort.Tag = &tag
		}

		if opts.RouterPort != "" {
			logicalSwitchPort.Type = "router"
			logicalSwitchPort.Addresses = []string{"router"}
			logicalSwitchPort.Options = map[string]string{"router-port": string(opts.RouterPort)}
		} else {
			ipStr := make([]string, 0, len(opts.IPs))
			for _, ip := range opts.IPs {
				ipStr = append(ipStr, ip.String())
			}

			var addresses string
			if opts.MAC != nil && len(ipStr) > 0 {
				addresses = fmt.Sprintf("%s %s", opts.MAC.String(), strings.Join(ipStr, " "))
			} else if opts.MAC != nil && len(ipStr) <= 0 {
				addresses = fmt.Sprintf("%s %s", opts.MAC.String(), "dynamic")
			} else {
				addresses = "dynamic"
			}

			logicalSwitchPort.Addresses = []string{addresses}
		}

		if opts.DHCPv4OptsID != "" {
			dhcp4opts := string(opts.DHCPv4OptsID)
			logicalSwitchPort.Dhcpv4Options = &dhcp4opts
		}

		if opts.DHCPv6OptsID != "" {
			dhcp6opts := string(opts.DHCPv6OptsID)
			logicalSwitchPort.Dhcpv6Options = &dhcp6opts
		}

		if opts.Location != "" {
			logicalSwitchPort.ExternalIDs[ovnExtIDIncusLocation] = opts.Location
		}
	}

	logicalSwitchPort.ExternalIDs[ovnExtIDIncusSwitch] = string(switchName)

	// Apply the changes.
	operations := []ovsdb.Operation{}
	if logicalSwitchPort.UUID != "lsp" {
		// If it already exists, update it.
		updateOps, err := o.client.Where(&logicalSwitchPort).Update(&logicalSwitchPort)
		if err != nil {
			return err
		}

		operations = append(operations, updateOps...)
	} else {
		// Else, create it.
		createOps, err := o.client.Create(&logicalSwitchPort)
		if err != nil {
			return err
		}

		operations = append(operations, createOps...)

		// And connect it to the switch.
		logicalSwitch := ovnNB.LogicalSwitch{
			Name: string(switchName),
		}

		updateOps, err := o.client.Where(&logicalSwitch).Mutate(&logicalSwitch, ovsModel.Mutation{
			Field:   &logicalSwitch.Ports,
			Mutator: ovsdb.MutateOperationInsert,
			Value:   []string{logicalSwitchPort.UUID},
		})
		if err != nil {
			return err
		}

		operations = append(operations, updateOps...)
	}

	// Apply the database changes.
	resp, err := o.client.Transact(ctx, operations...)
	if err != nil {
		return err
	}

	_, err = ovsdb.CheckOperationResults(resp, operations)
	if err != nil {
		return err
	}

	return nil
}

// GetLogicalSwitchPortIPs returns a list of IPs for a switch port.
func (o *NB) GetLogicalSwitchPortIPs(ctx context.Context, portName OVNSwitchPort) ([]net.IP, error) {
	lsp := ovnNB.LogicalSwitchPort{
		Name: string(portName),
	}

	err := o.get(ctx, &lsp)
	if err != nil {
		return nil, err
	}

	addresses := []net.IP{}
	for _, address := range lsp.Addresses {
		for _, entry := range strings.Split(address, " ") {
			ip := net.ParseIP(entry)
			if ip != nil {
				addresses = append(addresses, ip)
			}
		}
	}

	if lsp.DynamicAddresses != nil {
		for _, entry := range strings.Split(*lsp.DynamicAddresses, " ") {
			ip := net.ParseIP(entry)
			if ip != nil {
				addresses = append(addresses, ip)
			}
		}
	}

	return addresses, nil
}

// GetLogicalSwitchPortDynamicIPs returns a list of dynamc IPs for a switch port.
func (o *NB) GetLogicalSwitchPortDynamicIPs(ctx context.Context, portName OVNSwitchPort) ([]net.IP, error) {
	lsp := &ovnNB.LogicalSwitchPort{
		Name: string(portName),
	}

	err := o.get(ctx, lsp)
	if err != nil {
		return []net.IP{}, err
	}

	// Check if no dynamic IPs set.
	if lsp.DynamicAddresses == nil {
		return []net.IP{}, nil
	}

	dynamicAddresses := strings.Split(*lsp.DynamicAddresses, " ")
	dynamicIPs := make([]net.IP, 0, len(dynamicAddresses))

	for _, dynamicAddress := range dynamicAddresses {
		ip := net.ParseIP(dynamicAddress)
		if ip != nil {
			dynamicIPs = append(dynamicIPs, ip)
		}
	}

	return dynamicIPs, nil
}

// GetLogicalSwitchPortLocation returns the last set location of a logical switch port.
func (o *NB) GetLogicalSwitchPortLocation(ctx context.Context, portName OVNSwitchPort) (string, error) {
	lsp := ovnNB.LogicalSwitchPort{
		Name: string(portName),
	}

	err := o.get(ctx, &lsp)
	if err != nil {
		return "", err
	}

	if lsp.ExternalIDs == nil {
		return "", ErrNotFound
	}

	val, ok := lsp.ExternalIDs[ovnExtIDIncusLocation]
	if !ok {
		return "", ErrNotFound
	}

	return val, nil
}

// UpdateLogicalSwitchPortOptions sets the options for a logical switch port.
func (o *NB) UpdateLogicalSwitchPortOptions(ctx context.Context, portName OVNSwitchPort, options map[string]string) error {
	// Get the logical switch port.
	lsp := ovnNB.LogicalSwitchPort{
		Name: string(portName),
	}

	err := o.get(ctx, &lsp)
	if err != nil {
		return err
	}

	// Apply the changes.
	if lsp.Options == nil {
		lsp.Options = map[string]string{}
	}

	for key, value := range options {
		lsp.Options[key] = value
	}

	// Update the record.
	operations, err := o.client.Where(&lsp).Update(&lsp)
	if err != nil {
		return err
	}

	// Apply the changes.
	resp, err := o.client.Transact(ctx, operations...)
	if err != nil {
		return err
	}

	_, err = ovsdb.CheckOperationResults(resp, operations)
	if err != nil {
		return err
	}

	return nil
}

// LogicalSwitchPortSetDNS sets up the switch port DNS records for the DNS name.
// Returns the DNS record UUID, IPv4 and IPv6 addresses used for DNS records.
func (o *NB) LogicalSwitchPortSetDNS(switchName OVNSwitch, portName OVNSwitchPort, dnsName string, dnsIPs []net.IP) (OVNDNSUUID, error) {
	// Check if existing DNS record exists for switch port.
	dnsUUID, err := o.nbctl("--format=csv", "--no-headings", "--data=bare", "--colum=_uuid", "find", "dns",
		fmt.Sprintf("external_ids:%s=%s", ovnExtIDIncusSwitchPort, portName),
	)
	if err != nil {
		return "", err
	}

	cmdArgs := []string{
		fmt.Sprintf("external_ids:%s=%s", ovnExtIDIncusSwitch, switchName),
		fmt.Sprintf("external_ids:%s=%s", ovnExtIDIncusSwitchPort, portName),
	}

	// Only include DNS name record if IPs supplied.
	if len(dnsIPs) > 0 {
		var dnsIPsStr strings.Builder
		for i, dnsIP := range dnsIPs {
			if i > 0 {
				dnsIPsStr.WriteString(" ")
			}

			dnsIPsStr.WriteString(dnsIP.String())
		}

		cmdArgs = append(cmdArgs, fmt.Sprintf(`records={"%s"="%s"}`, strings.ToLower(dnsName), dnsIPsStr.String()))
	}

	dnsUUID = strings.TrimSpace(dnsUUID)
	if dnsUUID != "" {
		// Clear any existing DNS name if no IPs supplied.
		if len(dnsIPs) < 1 {
			cmdArgs = append(cmdArgs, "--", "clear", "dns", string(dnsUUID), "records")
		}

		// Update existing record if exists.
		_, err = o.nbctl(append([]string{"set", "dns", dnsUUID}, cmdArgs...)...)
		if err != nil {
			return "", err
		}
	} else {
		// Create new record if needed.
		dnsUUID, err = o.nbctl(append([]string{"create", "dns"}, cmdArgs...)...)
		if err != nil {
			return "", err
		}

		dnsUUID = strings.TrimSpace(dnsUUID)
	}

	// Add DNS record to switch DNS records.
	_, err = o.nbctl("add", "logical_switch", string(switchName), "dns_records", dnsUUID)
	if err != nil {
		return "", err
	}

	return OVNDNSUUID(dnsUUID), nil
}

// LogicalSwitchPortGetDNS returns the logical switch port DNS info (UUID, name and IPs).
func (o *NB) LogicalSwitchPortGetDNS(portName OVNSwitchPort) (OVNDNSUUID, string, []net.IP, error) {
	// Get UUID and DNS IPs for a switch port in the format: "<DNS UUID>,<DNS NAME>=<IP> <IP>"
	output, err := o.nbctl("--format=csv", "--no-headings", "--data=bare", "--colum=_uuid,records", "find", "dns",
		fmt.Sprintf("external_ids:%s=%s", ovnExtIDIncusSwitchPort, portName),
	)
	if err != nil {
		return "", "", nil, err
	}

	parts := strings.Split(strings.TrimSpace(output), ",")
	dnsUUID := strings.TrimSpace(parts[0])

	var dnsName string
	var ips []net.IP

	// Try and parse the DNS name and IPs.
	if len(parts) > 1 {
		dnsParts := strings.SplitN(strings.TrimSpace(parts[1]), "=", 2)
		if len(dnsParts) == 2 {
			dnsName = strings.TrimSpace(dnsParts[0])
			ipParts := strings.Split(dnsParts[1], " ")
			for _, ipPart := range ipParts {
				ip := net.ParseIP(strings.TrimSpace(ipPart))
				if ip != nil {
					ips = append(ips, ip)
				}
			}
		}
	}

	return OVNDNSUUID(dnsUUID), dnsName, ips, nil
}

// logicalSwitchPortDeleteDNSAppendArgs adds the command arguments to remove DNS records from a switch port.
// If destroyEntry the DNS entry record itself is also removed, otherwise it is just cleared but left in place.
// Returns args with the commands added to it.
func (o *NB) logicalSwitchPortDeleteDNSAppendArgs(args []string, switchName OVNSwitch, dnsUUID OVNDNSUUID, destroyEntry bool) []string {
	if len(args) > 0 {
		args = append(args, "--")
	}

	args = append(args, "remove", "logical_switch", string(switchName), "dns_records", string(dnsUUID), "--")

	if destroyEntry {
		args = append(args, "destroy", "dns", string(dnsUUID))
	} else {
		args = append(args, "clear", "dns", string(dnsUUID), "records")
	}

	return args
}

// LogicalSwitchPortDeleteDNS removes DNS records from a switch port.
// If destroyEntry the DNS entry record itself is also removed, otherwise it is just cleared but left in place.
func (o *NB) LogicalSwitchPortDeleteDNS(switchName OVNSwitch, dnsUUID OVNDNSUUID, destroyEntry bool) error {
	// Remove DNS record association from switch, and remove DNS record entry itself.
	_, err := o.nbctl(o.logicalSwitchPortDeleteDNSAppendArgs(nil, switchName, dnsUUID, destroyEntry)...)
	if err != nil {
		return err
	}

	return nil
}

// logicalSwitchPortDeleteAppendArgs adds the commands to delete the specified logical switch port.
// Returns args with the commands added to it.
func (o *NB) logicalSwitchPortDeleteAppendArgs(args []string, portName OVNSwitchPort) []string {
	if len(args) > 0 {
		args = append(args, "--")
	}

	args = append(args, "--if-exists", "lsp-del", string(portName))

	return args
}

// DeleteLogicalSwitchPort deletes a named logical switch port.
func (o *NB) DeleteLogicalSwitchPort(ctx context.Context, switchName OVNSwitch, portName OVNSwitchPort) error {
	operations := []ovsdb.Operation{}

	// Get the logical switch port.
	logicalSwitchPort := ovnNB.LogicalSwitchPort{
		Name: string(portName),
	}

	err := o.get(ctx, &logicalSwitchPort)
	if err != nil {
		// Logical switch port is already gone.
		if err == ErrNotFound {
			return nil
		}

		return err
	}

	// Remove the port from the switch.
	logicalSwitch := ovnNB.LogicalSwitch{
		Name: string(switchName),
	}

	updateOps, err := o.client.Where(&logicalSwitch).Mutate(&logicalSwitch, ovsModel.Mutation{
		Field:   &logicalSwitch.Ports,
		Mutator: ovsdb.MutateOperationDelete,
		Value:   []string{logicalSwitchPort.UUID},
	})
	if err != nil {
		return err
	}

	operations = append(operations, updateOps...)

	// Delete the port itself.
	deleteOps, err := o.client.Where(&logicalSwitchPort).Delete()
	if err != nil {
		return err
	}

	operations = append(operations, deleteOps...)

	// Apply the changes.
	resp, err := o.client.Transact(ctx, operations...)
	if err != nil {
		return err
	}

	_, err = ovsdb.CheckOperationResults(resp, operations)
	if err != nil {
		return err
	}

	return nil
}

// LogicalSwitchPortCleanup deletes the named logical switch port and its associated config.
func (o *NB) LogicalSwitchPortCleanup(portName OVNSwitchPort, switchName OVNSwitch, switchPortGroupName OVNPortGroup, dnsUUID OVNDNSUUID) error {
	// Remove any existing rules assigned to the entity.
	removeACLRuleUUIDs, err := o.logicalSwitchPortACLRules(context.TODO(), portName)
	if err != nil {
		return err
	}

	args := o.aclRuleDeleteAppendArgs(nil, "port_group", string(switchPortGroupName), removeACLRuleUUIDs)

	// Remove logical switch port.
	args = o.logicalSwitchPortDeleteAppendArgs(args, portName)

	// Remove DNS records.
	if dnsUUID != "" {
		args = o.logicalSwitchPortDeleteDNSAppendArgs(args, switchName, dnsUUID, false)
	}

	_, err = o.nbctl(args...)
	if err != nil {
		return err
	}

	return nil
}

// LogicalSwitchPortLinkRouter links a logical switch port to a logical router port.
func (o *NB) LogicalSwitchPortLinkRouter(switchPortName OVNSwitchPort, routerPortName OVNRouterPort) error {
	// Connect logical router port to switch.
	_, err := o.nbctl(
		"lsp-set-type", string(switchPortName), "router", "--",
		"lsp-set-addresses", string(switchPortName), "router", "--",
		"lsp-set-options", string(switchPortName), fmt.Sprintf("nat-addresses=%s", "router"), fmt.Sprintf("router-port=%s", string(routerPortName)),
	)
	if err != nil {
		return err
	}

	return nil
}

// LogicalSwitchPortLinkProviderNetwork links a logical switch port to a provider network.
func (o *NB) LogicalSwitchPortLinkProviderNetwork(switchPortName OVNSwitchPort, extNetworkName string) error {
	// Forward any unknown MAC frames down this port.
	_, err := o.nbctl(
		"lsp-set-addresses", string(switchPortName), "unknown", "--",
		"lsp-set-type", string(switchPortName), "localnet", "--",
		"lsp-set-options", string(switchPortName), fmt.Sprintf("network_name=%s", extNetworkName),
	)
	if err != nil {
		return err
	}

	return nil
}

// CreateChassisGroup adds a new HA chassis group.
// If mayExist is true, then an existing resource of the same name is not treated as an error.
func (o *NB) CreateChassisGroup(ctx context.Context, haChassisGroupName OVNChassisGroup, mayExist bool) error {
	// Define the new group.
	haChassisGroup := ovnNB.HAChassisGroup{
		Name: string(haChassisGroupName),
	}

	// Check if already exists.
	err := o.get(ctx, &haChassisGroup)
	if err != nil && err != ErrNotFound {
		return err
	}

	if haChassisGroup.UUID != "" {
		if mayExist {
			return nil
		}

		return ErrExists
	}

	// Create the record.
	operations, err := o.client.Create(&haChassisGroup)
	if err != nil {
		return err
	}

	// Apply the changes.
	resp, err := o.client.Transact(ctx, operations...)
	if err != nil {
		return err
	}

	_, err = ovsdb.CheckOperationResults(resp, operations)
	if err != nil {
		return err
	}

	return nil
}

// DeleteChassisGroup deletes an HA chassis group.
func (o *NB) DeleteChassisGroup(ctx context.Context, haChassisGroupName OVNChassisGroup) error {
	// Get the current chassis group.
	haChassisGroup := ovnNB.HAChassisGroup{
		Name: string(haChassisGroupName),
	}

	err := o.get(ctx, &haChassisGroup)
	if err != nil {
		// Already gone.
		if err == ErrNotFound {
			return nil
		}

		return err
	}

	// Delete the chassis group.
	deleteOps, err := o.client.Where(&haChassisGroup).Delete()
	if err != nil {
		return err
	}

	resp, err := o.client.Transact(ctx, deleteOps...)
	if err != nil {
		return err
	}

	_, err = ovsdb.CheckOperationResults(resp, deleteOps)
	if err != nil {
		return err
	}

	return nil
}

// SetChassisGroupPriority sets a given priority for the chassis ID in the chassis group..
func (o *NB) SetChassisGroupPriority(ctx context.Context, haChassisGroupName OVNChassisGroup, chassisID string, priority int) error {
	operations := []ovsdb.Operation{}

	// Get the chassis group.
	haGroup := ovnNB.HAChassisGroup{
		Name: string(haChassisGroupName),
	}

	err := o.get(ctx, &haGroup)
	if err != nil {
		return err
	}

	// Look for the chassis in the group.
	var haChassis ovnNB.HAChassis

	for _, entry := range haGroup.HaChassis {
		chassis := ovnNB.HAChassis{UUID: entry}
		err = o.get(ctx, &chassis)
		if err != nil {
			return err
		}

		if chassis.ChassisName == chassisID {
			haChassis = chassis
			break
		}
	}

	if haChassis.UUID == "" {
		// If asked to remove, then we're done.
		if priority < 0 {
			return nil
		}

		// No entry found, add a new one.
		haChassis = ovnNB.HAChassis{
			UUID:        "chassis",
			ChassisName: chassisID,
			Priority:    int(priority),
		}

		createOps, err := o.client.Create(&haChassis)
		if err != nil {
			return err
		}

		operations = append(operations, createOps...)

		// Add the HA Chassis to the group.
		updateOps, err := o.client.Where(&haGroup).Mutate(&haGroup, ovsModel.Mutation{
			Field:   &haGroup.HaChassis,
			Mutator: ovsdb.MutateOperationInsert,
			Value:   []string{haChassis.UUID},
		})
		if err != nil {
			return err
		}

		operations = append(operations, updateOps...)
	} else if priority < 0 {
		// Proceed with removing the entry.
		deleteOps, err := o.client.Where(&haChassis).Delete()
		if err != nil {
			return err
		}

		operations = append(operations, deleteOps...)

		// And removing it from the group.
		updateOps, err := o.client.Where(&haGroup).Mutate(&haGroup, ovsModel.Mutation{
			Field:   &haGroup.HaChassis,
			Mutator: ovsdb.MutateOperationDelete,
			Value:   []string{haChassis.UUID},
		})
		if err != nil {
			return err
		}

		operations = append(operations, updateOps...)
	} else if haChassis.Priority != priority {
		// Found but wrong priority, correct it.
		haChassis.Priority = int(priority)
		updateOps, err := o.client.Where(&haChassis).Update(&haChassis)
		if err != nil {
			return err
		}

		operations = append(operations, updateOps...)
	}

	// Apply the changes.
	resp, err := o.client.Transact(ctx, operations...)
	if err != nil {
		return err
	}

	_, err = ovsdb.CheckOperationResults(resp, operations)
	if err != nil {
		return err
	}

	return nil
}

// GetPortGroupInfo returns the port group UUID or empty string if port doesn't exist, and whether the port group has
// any ACL rules defined on it.
func (o *NB) GetPortGroupInfo(ctx context.Context, portGroupName OVNPortGroup) (OVNPortGroupUUID, bool, error) {
	pg := &ovnNB.PortGroup{
		Name: string(portGroupName),
	}

	err := o.get(ctx, pg)
	if err != nil {
		if err == ovsClient.ErrNotFound {
			return "", false, nil
		}

		return "", false, err
	}

	return OVNPortGroupUUID(pg.UUID), len(pg.ACLs) > 0, nil
}

// CreatePortGroup creates a new port group and optionally adds logical switch ports to the group.
func (o *NB) CreatePortGroup(ctx context.Context, projectID int64, portGroupName OVNPortGroup, associatedPortGroup OVNPortGroup, associatedSwitch OVNSwitch, initialPortMembers ...OVNSwitchPort) error {
	// Resolve the initial members.
	members := []string{}
	for _, portName := range initialPortMembers {
		lsp := ovnNB.LogicalSwitchPort{
			Name: string(portName),
		}

		err := o.get(ctx, &lsp)
		if err != nil {
			return err
		}

		members = append(members, lsp.UUID)
	}

	// Create the port group.
	pg := ovnNB.PortGroup{
		Name:  string(portGroupName),
		Ports: members,
		ExternalIDs: map[string]string{
			ovnExtIDIncusProjectID: fmt.Sprintf("%d", projectID),
		},
	}

	if associatedPortGroup != "" || associatedSwitch != "" {
		if associatedPortGroup != "" {
			pg.ExternalIDs[ovnExtIDIncusPortGroup] = string(associatedPortGroup)
		}

		if associatedSwitch != "" {
			pg.ExternalIDs[ovnExtIDIncusSwitch] = string(associatedSwitch)
		}
	}

	// Create the record.
	operations, err := o.client.Create(&pg)
	if err != nil {
		return err
	}

	// Apply the changes.
	resp, err := o.client.Transact(ctx, operations...)
	if err != nil {
		return err
	}

	_, err = ovsdb.CheckOperationResults(resp, operations)
	if err != nil {
		return err
	}

	return nil
}

// DeletePortGroup deletes port groups along with their ACL rules.
func (o *NB) DeletePortGroup(ctx context.Context, portGroupNames ...OVNPortGroup) error {
	operations := []ovsdb.Operation{}

	for _, portGroupName := range portGroupNames {
		pg := ovnNB.PortGroup{
			Name: string(portGroupName),
		}

		err := o.get(ctx, &pg)
		if err != nil {
			if err == ErrNotFound {
				// Already gone.
				continue
			}
		}

		deleteOps, err := o.client.Where(&pg).Delete()
		if err != nil {
			return err
		}

		operations = append(operations, deleteOps...)
	}

	// Check if we have anything to do.
	if len(operations) == 0 {
		return nil
	}

	// Apply the changes.
	resp, err := o.client.Transact(ctx, operations...)
	if err != nil {
		return err
	}

	_, err = ovsdb.CheckOperationResults(resp, operations)
	if err != nil {
		return err
	}

	return nil
}

// GetPortGroupsByProject finds the port groups that are associated to the project ID.
func (o *NB) GetPortGroupsByProject(ctx context.Context, projectID int64) ([]OVNPortGroup, error) {
	portGroups := []ovnNB.PortGroup{}

	err := o.client.WhereCache(func(pg *ovnNB.PortGroup) bool {
		return pg.ExternalIDs != nil && pg.ExternalIDs[ovnExtIDIncusProjectID] == fmt.Sprintf("%d", projectID)
	}).List(ctx, &portGroups)
	if err != nil {
		return nil, err
	}

	pgNames := make([]OVNPortGroup, 0, len(portGroups))
	for _, portGroup := range portGroups {
		pgNames = append(pgNames, OVNPortGroup(portGroup.Name))
	}

	return pgNames, nil
}

// PortGroupMemberChange adds/removes logical switch ports (by UUID) to/from existing port groups.
func (o *NB) PortGroupMemberChange(addMembers map[OVNPortGroup][]OVNSwitchPortUUID, removeMembers map[OVNPortGroup][]OVNSwitchPortUUID) error {
	args := []string{}

	for portGroupName, portMemberUUIDs := range addMembers {
		for _, portMemberUUID := range portMemberUUIDs {
			if len(args) > 0 {
				args = append(args, "--")
			}

			args = append(args, "add", "port_group", string(portGroupName), "ports", string(portMemberUUID))
		}
	}

	for portGroupName, portMemberUUIDs := range removeMembers {
		for _, portMemberUUID := range portMemberUUIDs {
			if len(args) > 0 {
				args = append(args, "--")
			}

			args = append(args, "--if-exists", "remove", "port_group", string(portGroupName), "ports", string(portMemberUUID))
		}
	}

	_, err := o.nbctl(args...)
	if err != nil {
		return err
	}

	return nil
}

// PortGroupSetACLRules applies a set of rules to the specified port group. Any existing rules are removed.
func (o *NB) PortGroupSetACLRules(portGroupName OVNPortGroup, matchReplace map[string]string, aclRules ...OVNACLRule) error {
	// Remove any existing rules assigned to the entity.
	args := []string{"clear", "port_group", string(portGroupName), "acls"}

	// Add new rules.
	externalIDs := map[string]string{
		ovnExtIDIncusPortGroup: string(portGroupName),
	}

	args = o.aclRuleAddAppendArgs(args, "port_group", string(portGroupName), externalIDs, matchReplace, aclRules...)

	_, err := o.nbctl(args...)
	if err != nil {
		return err
	}

	return nil
}

// aclRuleAddAppendArgs adds the commands to args that add the provided ACL rules to the specified OVN entity.
// Returns args with the ACL rule add commands added to it.
func (o *NB) aclRuleAddAppendArgs(args []string, entityTable string, entityName string, externalIDs map[string]string, matchReplace map[string]string, aclRules ...OVNACLRule) []string {
	for i, rule := range aclRules {
		if len(args) > 0 {
			args = append(args, "--")
		}

		// Perform any replacements requested on the Match string.
		for find, replace := range matchReplace {
			rule.Match = strings.ReplaceAll(rule.Match, find, replace)
		}

		// Add command to create ACL rule.
		args = append(args, fmt.Sprintf("--id=@id%d", i), "create", "acl",
			fmt.Sprintf("action=%s", rule.Action),
			fmt.Sprintf("direction=%s", rule.Direction),
			fmt.Sprintf("priority=%d", rule.Priority),
			fmt.Sprintf("match=%s", strconv.Quote(rule.Match)),
		)

		if rule.Log {
			args = append(args, "log=true")

			if rule.LogName != "" {
				args = append(args, fmt.Sprintf("name=%s", rule.LogName))
			}
		}

		for k, v := range externalIDs {
			args = append(args, fmt.Sprintf("external_ids:%s=%s", k, v))
		}

		// Add command to assign ACL rule to entity.
		args = append(args, "--", "add", entityTable, entityName, "acl", fmt.Sprintf("@id%d", i))
	}

	return args
}

// aclRuleDeleteAppendArgs adds the commands to args that delete the provided ACL rules from the specified OVN entity.
// Returns args with the ACL rule delete commands added to it.
func (o *NB) aclRuleDeleteAppendArgs(args []string, entityTable string, entityName string, aclRuleUUIDs []string) []string {
	for _, aclRuleUUID := range aclRuleUUIDs {
		if len(args) > 0 {
			args = append(args, "--")
		}

		args = append(args, "remove", entityTable, string(entityName), "acl", aclRuleUUID)
	}

	return args
}

// PortGroupPortSetACLRules applies a set of rules for the logical switch port in the specified port group.
// Any existing rules for that logical switch port in the port group are removed.
func (o *NB) PortGroupPortSetACLRules(portGroupName OVNPortGroup, portName OVNSwitchPort, aclRules ...OVNACLRule) error {
	// Remove any existing rules assigned to the entity.
	removeACLRuleUUIDs, err := o.logicalSwitchPortACLRules(context.TODO(), portName)
	if err != nil {
		return err
	}

	args := o.aclRuleDeleteAppendArgs(nil, "port_group", string(portGroupName), removeACLRuleUUIDs)

	// Add new rules.
	externalIDs := map[string]string{
		ovnExtIDIncusPortGroup:  string(portGroupName),
		ovnExtIDIncusSwitchPort: string(portName),
	}

	args = o.aclRuleAddAppendArgs(args, "port_group", string(portGroupName), externalIDs, nil, aclRules...)

	_, err = o.nbctl(args...)
	if err != nil {
		return err
	}

	return nil
}

// PortGroupPortClearACLRules clears any rules assigned to the logical switch port in the specified port group.
func (o *NB) PortGroupPortClearACLRules(portGroupName OVNPortGroup, portName OVNSwitchPort) error {
	// Remove any existing rules assigned to the entity.
	removeACLRuleUUIDs, err := o.logicalSwitchPortACLRules(context.TODO(), portName)
	if err != nil {
		return err
	}

	args := o.aclRuleDeleteAppendArgs(nil, "port_group", string(portGroupName), removeACLRuleUUIDs)

	if len(args) > 0 {
		_, err = o.nbctl(args...)
		if err != nil {
			return err
		}
	}

	return nil
}

// LoadBalancerApply creates a new load balancer (if doesn't exist) on the specified routers and switches.
// Providing an empty set of vips will delete the load balancer.
func (o *NB) LoadBalancerApply(loadBalancerName OVNLoadBalancer, routers []OVNRouter, switches []OVNSwitch, vips ...OVNLoadBalancerVIP) error {
	lbTCPName := fmt.Sprintf("%s-tcp", loadBalancerName)
	lbUDPName := fmt.Sprintf("%s-udp", loadBalancerName)

	// Remove existing load balancers if they exist.
	args := []string{"--if-exists", "lb-del", lbTCPName, "--", "lb-del", lbUDPName}

	// ipToString wraps IPv6 addresses in square brackets.
	ipToString := func(ip net.IP) string {
		if ip.To4() == nil {
			return fmt.Sprintf("[%s]", ip.String())
		}

		return ip.String()
	}

	// We have to use a separate load balancer for UDP rules so use this to keep track of whether we need it.
	lbNames := make(map[string]struct{})

	// Build up the commands to add VIPs to the load balancer.
	for _, r := range vips {
		if r.ListenAddress == nil {
			return fmt.Errorf("Missing VIP listen address")
		}

		if len(r.Targets) == 0 {
			return fmt.Errorf("Missing VIP target(s)")
		}

		if r.Protocol == "udp" {
			args = append(args, "--", "lb-add", lbUDPName)
			lbNames[lbUDPName] = struct{}{} // Record that UDP load balancer is created.
		} else {
			args = append(args, "--", "lb-add", lbTCPName)
			lbNames[lbTCPName] = struct{}{} // Record that TCP load balancer is created.
		}

		targetArgs := make([]string, 0, len(r.Targets))

		for _, target := range r.Targets {
			if (r.ListenPort > 0 && target.Port <= 0) || (target.Port > 0 && r.ListenPort <= 0) {
				return fmt.Errorf("The listen and target ports must be specified together")
			}

			if r.ListenPort > 0 {
				targetArgs = append(targetArgs, fmt.Sprintf("%s:%d", ipToString(target.Address), target.Port))
			} else {
				targetArgs = append(targetArgs, ipToString(target.Address))
			}
		}

		if r.ListenPort > 0 {
			args = append(args,
				fmt.Sprintf("%s:%d", ipToString(r.ListenAddress), r.ListenPort),
				strings.Join(targetArgs, ","),
				r.Protocol,
			)
		} else {
			args = append(args,
				ipToString(r.ListenAddress),
				strings.Join(targetArgs, ","),
			)
		}
	}

	// If there are some VIP rules then associate the load balancer to the requested routers and switches.
	if len(vips) > 0 {
		for _, r := range routers {
			_, found := lbNames[lbTCPName]
			if found {
				args = append(args, "--", "lr-lb-add", string(r), lbTCPName)
			}

			_, found = lbNames[lbUDPName]
			if found {
				args = append(args, "--", "lr-lb-add", string(r), lbUDPName)
			}
		}

		for _, s := range switches {
			_, found := lbNames[lbTCPName]
			if found {
				args = append(args, "--", "ls-lb-add", string(s), lbTCPName)
			}

			_, found = lbNames[lbUDPName]
			if found {
				args = append(args, "--", "ls-lb-add", string(s), lbUDPName)
			}
		}
	}

	_, err := o.nbctl(args...)
	if err != nil {
		return err
	}

	return nil
}

// DeleteLoadBalancer deletes the specified load balancer(s).
func (o *NB) DeleteLoadBalancer(ctx context.Context, loadBalancerNames ...OVNLoadBalancer) error {
	operations := []ovsdb.Operation{}
	for _, loadBalancerName := range loadBalancerNames {
		// Check for a TCP load-balancer.
		lb := ovnNB.LoadBalancer{
			Name: fmt.Sprintf("%s-tcp", loadBalancerName),
		}

		err := o.get(ctx, &lb)
		if err == nil {
			// Delete the load balancer.
			deleteOps, err := o.client.Where(&lb).Delete()
			if err != nil {
				return err
			}

			operations = append(operations, deleteOps...)
		} else if err != ErrNotFound {
			return err
		}

		// Check for a UDP load-balancer.
		lb = ovnNB.LoadBalancer{
			Name: fmt.Sprintf("%s-udp", loadBalancerName),
		}

		err = o.get(ctx, &lb)
		if err == nil {
			// Delete the load balancer.
			deleteOps, err := o.client.Where(&lb).Delete()
			if err != nil {
				return err
			}

			operations = append(operations, deleteOps...)
		} else if err != ErrNotFound {
			return err
		}
	}

	// Check if anything to delete.
	if len(operations) == 0 {
		return nil
	}

	// Apply the changes.
	resp, err := o.client.Transact(ctx, operations...)
	if err != nil {
		return err
	}

	_, err = ovsdb.CheckOperationResults(resp, operations)
	if err != nil {
		return err
	}

	return nil
}

// CreateAddressSet creates address sets for IP versions 4 and 6 in the format "<addressSetPrefix>_ip<IP version>".
// Populates them with the relevant addresses supplied.
func (o *NB) CreateAddressSet(ctx context.Context, addressSetPrefix OVNAddressSet, addresses ...net.IPNet) error {
	// Define the new address sets.
	ipv4Set := ovnNB.AddressSet{
		Name:      fmt.Sprintf("%s_ip4", addressSetPrefix),
		Addresses: []string{},
	}

	ipv6Set := ovnNB.AddressSet{
		Name:      fmt.Sprintf("%s_ip6", addressSetPrefix),
		Addresses: []string{},
	}

	// Add addresses.
	for _, address := range addresses {
		if address.IP.To4() == nil {
			ipv6Set.Addresses = append(ipv6Set.Addresses, address.String())
		} else {
			ipv4Set.Addresses = append(ipv4Set.Addresses, address.String())
		}
	}

	// Create the records.
	operations := []ovsdb.Operation{}

	createOps, err := o.client.Create(&ipv4Set)
	if err != nil {
		return err
	}

	operations = append(operations, createOps...)

	createOps, err = o.client.Create(&ipv6Set)
	if err != nil {
		return err
	}

	operations = append(operations, createOps...)

	// Apply the changes.
	resp, err := o.client.Transact(ctx, operations...)
	if err != nil {
		return err
	}

	_, err = ovsdb.CheckOperationResults(resp, operations)
	if err != nil {
		return err
	}

	return nil
}

// UpdateAddressSetAdd adds the supplied addresses to the address sets.
// If the set is missing, it will get automatically created.
// The address set name used is "<addressSetPrefix>_ip<IP version>", e.g. "foo_ip4".
func (o *NB) UpdateAddressSetAdd(ctx context.Context, addressSetPrefix OVNAddressSet, addresses ...net.IPNet) error {
	// Get the address sets.
	ipv4Set := ovnNB.AddressSet{
		Name: fmt.Sprintf("%s_ip4", addressSetPrefix),
	}

	err := o.get(ctx, &ipv4Set)
	if err != nil && err != ErrNotFound {
		return err
	}

	if ipv4Set.Addresses == nil {
		ipv4Set.Addresses = []string{}
	}

	ipv6Set := ovnNB.AddressSet{
		Name: fmt.Sprintf("%s_ip6", addressSetPrefix),
	}

	err = o.get(ctx, &ipv6Set)
	if err != nil && err != ErrNotFound {
		return err
	}

	if ipv6Set.Addresses == nil {
		ipv6Set.Addresses = []string{}
	}

	// Add the addresses.
	for _, address := range addresses {
		if address.IP.To4() == nil {
			if !slices.Contains(ipv6Set.Addresses, address.String()) {
				ipv6Set.Addresses = append(ipv6Set.Addresses, address.String())
			}
		} else {
			if !slices.Contains(ipv4Set.Addresses, address.String()) {
				ipv4Set.Addresses = append(ipv4Set.Addresses, address.String())
			}
		}
	}

	// Prepare the records.
	operations := []ovsdb.Operation{}

	if ipv4Set.UUID == "" {
		createOps, err := o.client.Create(&ipv4Set)
		if err != nil {
			return err
		}

		operations = append(operations, createOps...)
	} else {
		updateOps, err := o.client.Where(&ipv4Set).Update(&ipv4Set)
		if err != nil {
			return err
		}

		operations = append(operations, updateOps...)
	}

	if ipv6Set.UUID == "" {
		createOps, err := o.client.Create(&ipv6Set)
		if err != nil {
			return err
		}

		operations = append(operations, createOps...)
	} else {
		updateOps, err := o.client.Where(&ipv6Set).Update(&ipv6Set)
		if err != nil {
			return err
		}

		operations = append(operations, updateOps...)
	}

	// Apply the changes.
	resp, err := o.client.Transact(ctx, operations...)
	if err != nil {
		return err
	}

	_, err = ovsdb.CheckOperationResults(resp, operations)
	if err != nil {
		return err
	}

	return nil
}

// UpdateAddressSetRemove removes the supplied addresses from the address set.
// The address set name used is "<addressSetPrefix>_ip<IP version>", e.g. "foo_ip4".
func (o *NB) UpdateAddressSetRemove(ctx context.Context, addressSetPrefix OVNAddressSet, addresses ...net.IPNet) error {
	// Get the address sets.
	ipv4Set := ovnNB.AddressSet{
		Name: fmt.Sprintf("%s_ip4", addressSetPrefix),
	}

	err := o.get(ctx, &ipv4Set)
	if err != nil {
		return err
	}

	ipv6Set := ovnNB.AddressSet{
		Name: fmt.Sprintf("%s_ip6", addressSetPrefix),
	}

	err = o.get(ctx, &ipv6Set)
	if err != nil {
		return err
	}

	// Filter entries.
	ipv4Addresses := []string{}
	for _, entry := range ipv4Set.Addresses {
		found := false
		for _, address := range addresses {
			if entry == address.String() {
				found = true
				break
			}
		}

		if !found {
			ipv4Addresses = append(ipv4Addresses, entry)
		}
	}

	ipv4Set.Addresses = ipv4Addresses

	ipv6Addresses := []string{}
	for _, entry := range ipv6Set.Addresses {
		found := false
		for _, address := range addresses {
			if entry == address.String() {
				found = true
				break
			}
		}

		if !found {
			ipv6Addresses = append(ipv6Addresses, entry)
		}
	}

	ipv6Set.Addresses = ipv6Addresses

	// Prepare the records.
	operations := []ovsdb.Operation{}

	updateOps, err := o.client.Where(&ipv4Set).Update(&ipv4Set)
	if err != nil {
		return err
	}

	operations = append(operations, updateOps...)

	updateOps, err = o.client.Where(&ipv6Set).Update(&ipv6Set)
	if err != nil {
		return err
	}

	operations = append(operations, updateOps...)

	// Apply the changes.
	resp, err := o.client.Transact(ctx, operations...)
	if err != nil {
		return err
	}

	_, err = ovsdb.CheckOperationResults(resp, operations)
	if err != nil {
		return err
	}

	return nil
}

// DeleteAddressSet deletes address sets for IP versions 4 and 6 in the format "<addressSetPrefix>_ip<IP version>".
func (o *NB) DeleteAddressSet(ctx context.Context, addressSetPrefix OVNAddressSet) error {
	// Get the address sets.
	ipv4Set := ovnNB.AddressSet{
		Name: fmt.Sprintf("%s_ip4", addressSetPrefix),
	}

	err := o.get(ctx, &ipv4Set)
	if err != nil && err != ErrNotFound {
		return err
	}

	ipv6Set := ovnNB.AddressSet{
		Name: fmt.Sprintf("%s_ip6", addressSetPrefix),
	}

	err = o.get(ctx, &ipv6Set)
	if err != nil && err != ErrNotFound {
		return err
	}

	// Delete the records.
	operations := []ovsdb.Operation{}

	if ipv4Set.UUID != "" {
		deleteOps, err := o.client.Where(&ipv4Set).Delete()
		if err != nil {
			return err
		}

		operations = append(operations, deleteOps...)
	}

	if ipv6Set.UUID != "" {
		deleteOps, err := o.client.Where(&ipv6Set).Delete()
		if err != nil {
			return err
		}

		operations = append(operations, deleteOps...)
	}

	// Apply the changes.
	resp, err := o.client.Transact(ctx, operations...)
	if err != nil {
		return err
	}

	_, err = ovsdb.CheckOperationResults(resp, operations)
	if err != nil {
		return err
	}

	return nil
}

// LogicalRouterPolicyApply removes any existing policies and applies the new policies to the specified router.
func (o *NB) LogicalRouterPolicyApply(routerName OVNRouter, policies ...OVNRouterPolicy) error {
	args := []string{"lr-policy-del", string(routerName)}

	for _, policy := range policies {
		args = append(args, "--", "lr-policy-add", string(routerName), fmt.Sprintf("%d", policy.Priority), policy.Match, policy.Action)
	}

	_, err := o.nbctl(args...)
	if err != nil {
		return err
	}

	return nil
}

// LogicalRouterRoutes returns a list of static routes in the main route table of the logical router.
func (o *NB) LogicalRouterRoutes(routerName OVNRouter) ([]OVNRouterRoute, error) {
	output, err := o.nbctl("lr-route-list", string(routerName))
	if err != nil {
		return nil, err
	}

	lines := util.SplitNTrimSpace(strings.TrimSpace(output), "\n", -1, true)
	routes := make([]OVNRouterRoute, 0)

	mainTable := true // Assume output starts with main table (supports ovn versions without multiple tables).
	for i, line := range lines {
		if line == "IPv4 Routes" || line == "IPv6 Routes" {
			continue // Ignore heading category lines.
		}

		// Keep track of which route table we are looking at.
		if strings.HasPrefix(line, "Route Table") {
			if line == "Route Table <main>:" {
				mainTable = true
			} else {
				mainTable = false
			}

			continue
		}

		if !mainTable {
			continue // We don't currently consider routes in other route tables.
		}

		// E.g. "10.97.31.0/24 10.97.31.1 dst-ip [optional-some-router-port-name]"
		fields := strings.Fields(line)
		fieldsLen := len(fields)

		if fieldsLen <= 0 {
			continue // Ignore empty lines.
		} else if fieldsLen < 3 || fieldsLen > 4 {
			return nil, fmt.Errorf("Unrecognised static route item output on line %d: %q", i, line)
		}

		var route OVNRouterRoute

		// ovn-nbctl doesn't output single-host route prefixes in CIDR format, so do the conversion here.
		ip := net.ParseIP(fields[0])
		if ip != nil {
			subnetSize := 32
			if ip.To4() == nil {
				subnetSize = 128
			}

			fields[0] = fmt.Sprintf("%s/%d", ip.String(), subnetSize)
		}

		_, prefix, err := net.ParseCIDR(fields[0])
		if err != nil {
			return nil, fmt.Errorf("Invalid static route prefix on line %d: %q", i, fields[0])
		}

		route.Prefix = *prefix
		route.NextHop = net.ParseIP(fields[1])

		if fieldsLen > 3 {
			route.Port = OVNRouterPort(fields[3])
		}

		routes = append(routes, route)
	}

	return routes, nil
}

// LogicalRouterPeeringApply applies a peering relationship between two logical routers.
func (o *NB) LogicalRouterPeeringApply(opts OVNRouterPeering) error {
	if len(opts.LocalRouterPortIPs) <= 0 || len(opts.TargetRouterPortIPs) <= 0 {
		return fmt.Errorf("IPs not populated for both router ports")
	}

	// Remove peering router ports and static routes using ports from both routers.
	// Run the delete step as a separate command to workaround a bug in OVN.
	err := o.LogicalRouterPeeringDelete(opts)
	if err != nil {
		return err
	}

	// Start fresh command set.
	var args []string

	// Will use the first IP from each family of the router port interfaces.
	localRouterGatewayIPs := make(map[uint]net.IP, 0)
	targetRouterGatewayIPs := make(map[uint]net.IP, 0)

	// Setup local router port peered with target router port.
	args = append(args, "--", "lrp-add", string(opts.LocalRouter), string(opts.LocalRouterPort), opts.LocalRouterPortMAC.String())
	for _, ipNet := range opts.LocalRouterPortIPs {
		ipVersion := uint(4)
		if ipNet.IP.To4() == nil {
			ipVersion = 6
		}

		if localRouterGatewayIPs[ipVersion] == nil {
			localRouterGatewayIPs[ipVersion] = ipNet.IP
		}

		args = append(args, ipNet.String())
	}

	args = append(args, fmt.Sprintf("peer=%s", opts.TargetRouterPort))

	// Setup target router port peered with local router port.
	args = append(args, "--", "lrp-add", string(opts.TargetRouter), string(opts.TargetRouterPort), opts.TargetRouterPortMAC.String())
	for _, ipNet := range opts.TargetRouterPortIPs {
		ipVersion := uint(4)
		if ipNet.IP.To4() == nil {
			ipVersion = 6
		}

		if targetRouterGatewayIPs[ipVersion] == nil {
			targetRouterGatewayIPs[ipVersion] = ipNet.IP
		}

		args = append(args, ipNet.String())
	}

	args = append(args, fmt.Sprintf("peer=%s", opts.LocalRouterPort))

	// Add routes using the first router gateway IP for each family for next hop address.
	for _, route := range opts.LocalRouterRoutes {
		ipVersion := uint(4)
		if route.IP.To4() == nil {
			ipVersion = 6
		}

		nextHopIP := targetRouterGatewayIPs[ipVersion]

		if nextHopIP == nil {
			return fmt.Errorf("Missing target router port IPv%d address for local route %q nexthop address", ipVersion, route.String())
		}

		args = append(args, "--", "--may-exist", "lr-route-add", string(opts.LocalRouter), route.String(), nextHopIP.String(), string(opts.LocalRouterPort))
	}

	for _, route := range opts.TargetRouterRoutes {
		ipVersion := uint(4)
		if route.IP.To4() == nil {
			ipVersion = 6
		}

		nextHopIP := localRouterGatewayIPs[ipVersion]

		if nextHopIP == nil {
			return fmt.Errorf("Missing local router port IPv%d address for target route %q nexthop address", ipVersion, route.String())
		}

		args = append(args, "--", "--may-exist", "lr-route-add", string(opts.TargetRouter), route.String(), nextHopIP.String(), string(opts.TargetRouterPort))
	}

	if len(args) > 0 {
		_, err := o.nbctl(args...)
		if err != nil {
			return err
		}
	}

	return nil
}

// LogicalRouterPeeringDelete deletes a peering relationship between two logical routers.
// Requires LocalRouter, LocalRouterPort, TargetRouter and TargetRouterPort opts fields to be populated.
func (o *NB) LogicalRouterPeeringDelete(opts OVNRouterPeering) error {
	// Remove peering router ports and static routes using ports from both routers.
	if opts.LocalRouter == "" || opts.TargetRouter == "" {
		return fmt.Errorf("Router names not populated for both routers")
	}

	args := []string{
		"--if-exists", "lrp-del", string(opts.LocalRouterPort), "--",
		"--if-exists", "lrp-del", string(opts.TargetRouterPort),
	}

	// Remove static routes from both routers that use the respective peering router ports.
	staticRoutes, err := o.LogicalRouterRoutes(opts.LocalRouter)
	if err != nil {
		return fmt.Errorf("Failed getting static routes for local peer router %q: %w", opts.LocalRouter, err)
	}

	for _, staticRoute := range staticRoutes {
		if staticRoute.Port == opts.LocalRouterPort {
			args = append(args, "--", "lr-route-del", string(opts.LocalRouter), staticRoute.Prefix.String(), staticRoute.NextHop.String(), string(opts.LocalRouterPort))
		}
	}

	staticRoutes, err = o.LogicalRouterRoutes(opts.TargetRouter)
	if err != nil {
		return fmt.Errorf("Failed getting static routes for target peer router %q: %w", opts.TargetRouter, err)
	}

	for _, staticRoute := range staticRoutes {
		if staticRoute.Port == opts.TargetRouterPort {
			args = append(args, "--", "lr-route-del", string(opts.TargetRouter), staticRoute.Prefix.String(), staticRoute.NextHop.String(), string(opts.TargetRouterPort))
		}
	}

	if len(args) > 0 {
		_, err := o.nbctl(args...)
		if err != nil {
			return err
		}
	}

	return nil
}

// GetLogicalRouterPortHardwareAddress gets the hardware address of the logical router port.
func (o *NB) GetLogicalRouterPortHardwareAddress(ctx context.Context, ovnRouterPort OVNRouterPort) (string, error) {
	lrp, err := o.GetLogicalRouterPort(ctx, ovnRouterPort)
	if err != nil {
		return "", err
	}

	return lrp.MAC, nil
}

// GetName returns the OVN AZ name.
func (o *NB) GetName(ctx context.Context) (string, error) {
	// Get the global configuration.
	nbGlobal := []ovnNB.NBGlobal{}
	err := o.client.List(ctx, &nbGlobal)
	if err != nil {
		return "", err
	}

	// Check that we got a result.
	if len(nbGlobal) != 1 {
		return "", ovsClient.ErrNotFound
	}

	return nbGlobal[0].Name, nil
}
