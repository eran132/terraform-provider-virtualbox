package provider

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	multierror "github.com/hashicorp/go-multierror"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func netTfToVbox(ctx context.Context, d *schema.ResourceData) ([]NIC, error) {
	tfToVboxNetworkType := func(attr string) (NICNetwork, error) {
		switch attr {
		case "bridged":
			return NICNetBridged, nil
		case "nat":
			return NICNetNAT, nil
		case "hostonly":
			return NICNetHostonly, nil
		case "internal":
			return NICNetInternal, nil
		case "generic":
			return NICNetGeneric, nil
		default:
			return "", fmt.Errorf("invalid virtual network adapter type: %s", attr)
		}
	}

	tfToVboxNetDevice := func(attr string) (NICHardware, error) {
		switch attr {
		case "PCIII":
			return AMDPCNetPCIII, nil
		case "FASTIII":
			return AMDPCNetFASTIII, nil
		case "IntelPro1000MTDesktop":
			return IntelPro1000MTDesktop, nil
		case "IntelPro1000TServer":
			return IntelPro1000TServer, nil
		case "IntelPro1000MTServer":
			return IntelPro1000MTServer, nil
		case "VirtIO":
			return VirtIO, nil
		default:
			return "", fmt.Errorf("invalid virtual network device: %s", attr)
		}
	}

	var err error
	var errs []error
	nicCount := d.Get("network_adapter.#").(int)
	adapters := make([]NIC, 0, nicCount)

	for i := 0; i < nicCount; i++ {
		prefix := fmt.Sprintf("network_adapter.%d.", i)
		var adapter NIC

		if attr, ok := d.Get(prefix + "type").(string); ok && attr != "" {
			adapter.Network, err = tfToVboxNetworkType(attr)
		}
		if attr, ok := d.Get(prefix + "device").(string); ok && attr != "" {
			adapter.Hardware, err = tfToVboxNetDevice(attr)
		}
		/* 'Hostonly' and 'bridged' network need property 'host_interface' been set */
		if adapter.Network == NICNetHostonly || adapter.Network == NICNetBridged {
			var ok bool
			adapter.HostInterface, ok = d.Get(prefix + "host_interface").(string)
			if !ok || adapter.HostInterface == "" {
				err = fmt.Errorf("'host_interface' property not set for '#%d' network adapter", i)
			}
		}

		if err != nil {
			errs = append(errs, err)
			continue
		}

		tflog.Debug(ctx, "adding new converted network adapter", map[string]any{
			"adapter": fmt.Sprintf("%+v", adapter),
		})
		adapters = append(adapters, adapter)
	}

	if len(errs) > 0 {
		return nil, &multierror.Error{Errors: errs}
	}

	return adapters, nil
}

// countRuntimeNics will return the number of NICs found after VM successfully started.
func countRuntimeNICs(vm *Machine) (int, error) {
	count, err := getGuestProperty(vm.UUID, "/VirtualBox/GuestInfo/Net/Count")

	if err != nil {
		return 0, err
	}

	if count == "" {
		return 0, nil
	}

	return strconv.Atoi(count)
}

// applyNICSettings applies per-NIC settings that go-virtualbox doesn't expose
// (port forwarding, promiscuous mode, cable connected, NAT DNS).
// Must be called after vm.Modify() and before vm.Start().
func applyNICSettings(ctx context.Context, vmUUID string, d *schema.ResourceData) error {
	nicCount := d.Get("network_adapter.#").(int)

	for i := 0; i < nicCount; i++ {
		nicIdx := i + 1 // VirtualBox NICs are 1-indexed
		prefix := fmt.Sprintf("network_adapter.%d.", i)

		// Promiscuous mode
		if promisc, ok := d.GetOk(prefix + "promiscuous_mode"); ok {
			mode := promisc.(string)
			if mode != "deny" {
				if _, _, err := vboxRun(ctx, "modifyvm", vmUUID,
					fmt.Sprintf("--nicpromisc%d", nicIdx), mode); err != nil {
					return fmt.Errorf("failed to set promiscuous mode on NIC %d: %w", i, err)
				}
			}
		}

		// Cable connected
		if cable, ok := d.GetOk(prefix + "cable_connected"); ok {
			val := "on"
			if !cable.(bool) {
				val = "off"
			}
			if _, _, err := vboxRun(ctx, "modifyvm", vmUUID,
				fmt.Sprintf("--cableconnected%d", nicIdx), val); err != nil {
				return fmt.Errorf("failed to set cable connected on NIC %d: %w", i, err)
			}
		}

		// MAC address (if user specified)
		if mac, ok := d.GetOk(prefix + "mac_address"); ok && mac.(string) != "" {
			if _, _, err := vboxRun(ctx, "modifyvm", vmUUID,
				fmt.Sprintf("--macaddress%d", nicIdx), mac.(string)); err != nil {
				return fmt.Errorf("failed to set MAC address on NIC %d: %w", i, err)
			}
		}

		// NAT-specific settings (only for NAT adapters)
		nicType := d.Get(prefix + "type").(string)
		if nicType == "nat" {
			// NAT DNS host resolver
			if dnsResolver, ok := d.GetOk(prefix + "nat_dns_host_resolver"); ok && dnsResolver.(bool) {
				if _, _, err := vboxRun(ctx, "modifyvm", vmUUID,
					fmt.Sprintf("--natdnshostresolver%d", nicIdx), "on"); err != nil {
					return fmt.Errorf("failed to set NAT DNS host resolver on NIC %d: %w", i, err)
				}
			}

			// NAT DNS proxy
			if dnsProxy, ok := d.GetOk(prefix + "nat_dns_proxy"); ok && dnsProxy.(bool) {
				if _, _, err := vboxRun(ctx, "modifyvm", vmUUID,
					fmt.Sprintf("--natdnsproxy%d", nicIdx), "on"); err != nil {
					return fmt.Errorf("failed to set NAT DNS proxy on NIC %d: %w", i, err)
				}
			}

			// Port forwarding rules - first delete all existing, then add new
			// Delete existing rules (ignore errors as there may be none)
			vboxRun(ctx, "modifyvm", vmUUID, fmt.Sprintf("--natpf%d", nicIdx), "delete", "all") //nolint:errcheck

			pfCount := d.Get(fmt.Sprintf("network_adapter.%d.port_forwarding.#", i)).(int)
			for j := 0; j < pfCount; j++ {
				pfPrefix := fmt.Sprintf("network_adapter.%d.port_forwarding.%d.", i, j)
				ruleName := d.Get(pfPrefix + "name").(string)
				protocol := d.Get(pfPrefix + "protocol").(string)
				hostIP := d.Get(pfPrefix + "host_ip").(string)
				hostPort := d.Get(pfPrefix + "host_port").(int)
				guestIP := d.Get(pfPrefix + "guest_ip").(string)
				guestPort := d.Get(pfPrefix + "guest_port").(int)

				rule := fmt.Sprintf("%s,%s,%s,%d,%s,%d",
					ruleName, protocol, hostIP, hostPort, guestIP, guestPort)

				if _, _, err := vboxRun(ctx, "modifyvm", vmUUID,
					fmt.Sprintf("--natpf%d", nicIdx), rule); err != nil {
					return fmt.Errorf("failed to add port forwarding rule %q on NIC %d: %w", ruleName, i, err)
				}
			}
		}
	}

	return nil
}

func netVboxToTf(vm *Machine, d *schema.ResourceData) error {
	vboxToTfNetworkType := func(netType NICNetwork) string {
		switch netType {
		case NICNetBridged:
			return "bridged"
		case NICNetNAT:
			return "nat"
		case NICNetHostonly:
			return "hostonly"
		case NICNetInternal:
			return "internal"
		case NICNetGeneric:
			return "generic"
		default:
			return ""
		}
	}

	vboxToTfVdevice := func(vdevice NICHardware) string {
		switch vdevice {
		case AMDPCNetPCIII:
			return "PCIII"
		case AMDPCNetFASTIII:
			return "FASTIII"
		case IntelPro1000MTDesktop:
			return "IntelPro1000MTDesktop"
		case IntelPro1000TServer:
			return "IntelPro1000TServer"
		case IntelPro1000MTServer:
			return "IntelPro1000MTServer"
		case VirtIO:
			return "VirtIO"
		default:
			return ""
		}
	}

	/* Collect NIC data from guest OS, available only when VM is running */
	if vm.State == MachineStateRunning {
		nicCount, err := countRuntimeNICs(vm)
		if err != nil {
			// Guest Additions not installed or not ready — skip guest property collection
			// and fall through to the non-running path which sets defaults
			nicCount = 0
		}

		if nicCount < len(vm.NICs) {
			// Not enough guest info available — use config-only data
			nics := make([]map[string]any, 0, len(vm.NICs))
			for _, nic := range vm.NICs {
				out := make(map[string]any)
				out["type"] = vboxToTfNetworkType(nic.Network)
				out["device"] = vboxToTfVdevice(nic.Hardware)
				out["host_interface"] = nic.HostInterface
				out["mac_address"] = nic.MacAddr
				out["cable_connected"] = true
				out["promiscuous_mode"] = "deny"
				out["nat_dns_host_resolver"] = false
				out["nat_dns_proxy"] = false
				out["status"] = "unknown"
				out["ipv4_address"] = ""
				out["ipv4_address_available"] = "no"
				nics = append(nics, out)
			}
			if err := d.Set("network_adapter", nics); err != nil {
				return fmt.Errorf("can't set network_adapter: %w", err)
			}
			return nil
		}

		/* NICs in guest OS (eth0, eth1, etc) does not neccessarily have save
		order as in VirtualBox (nic1, nic2, etc), so we use MAC address to setup a mapping */
		type OsNicData struct {
			ipv4Addr string
			status   string
		}
		osNicMap := make(map[string]OsNicData) // map from MAC address to data

		var errs []error
		for i := 0; i < nicCount; i++ {
			var osNic OsNicData

			/* NIC MAC address */
			macAddr, err := getGuestProperty(vm.UUID, fmt.Sprintf("/VirtualBox/GuestInfo/Net/%d/MAC", i))
			if err != nil {
				errs = append(errs, err)
				continue
			}
			if macAddr == "" {
				return nil
			}

			/* NIC status */
			status, err := getGuestProperty(vm.UUID, fmt.Sprintf("/VirtualBox/GuestInfo/Net/%d/Status", i))
			if err != nil {
				errs = append(errs, err)
				continue
			}
			if status == "" {
				return nil
			}
			osNic.status = strings.ToLower(status)

			/* NIC ipv4 address */
			ipv4Addr, err := getGuestProperty(vm.UUID, fmt.Sprintf("/VirtualBox/GuestInfo/Net/%d/V4/IP", i))
			if err != nil {
				errs = append(errs, err)
				continue
			}
			if ipv4Addr == "" {
				return nil
			}
			osNic.ipv4Addr = ipv4Addr

			osNicMap[macAddr] = osNic
		}

		if len(errs) > 0 {
			return &multierror.Error{Errors: errs}
		}

		// Assign NIC property to vbox structure and Terraform
		nics := make([]map[string]any, 0, 1)

		for _, nic := range vm.NICs {
			out := make(map[string]any)

			out["type"] = vboxToTfNetworkType(nic.Network)
			out["device"] = vboxToTfVdevice(nic.Hardware)
			out["host_interface"] = nic.HostInterface
			out["mac_address"] = nic.MacAddr
			out["cable_connected"] = true
			out["promiscuous_mode"] = "deny"
			out["nat_dns_host_resolver"] = false
			out["nat_dns_proxy"] = false

			osNic, ok := osNicMap[nic.MacAddr]
			if !ok {
				return nil
			}
			out["status"] = osNic.status
			out["ipv4_address"] = osNic.ipv4Addr
			if osNic.ipv4Addr == "" {
				out["ipv4_address_available"] = "no"
			} else {
				out["ipv4_address_available"] = "yes"
			}

			nics = append(nics, out)
		}

		err = d.Set("network_adapter", nics)
		if err != nil {
			return fmt.Errorf("can't set network_adapter: %w", err)
		}

	} else {
		// Assign NIC property to vbox structure and Terraform
		nics := make([]map[string]any, 0, 1)

		for _, nic := range vm.NICs {
			out := make(map[string]any)

			out["type"] = vboxToTfNetworkType(nic.Network)
			out["device"] = vboxToTfVdevice(nic.Hardware)
			out["host_interface"] = nic.HostInterface
			out["mac_address"] = nic.MacAddr
			out["cable_connected"] = true
			out["promiscuous_mode"] = "deny"
			out["nat_dns_host_resolver"] = false
			out["nat_dns_proxy"] = false

			out["status"] = "down"
			out["ipv4_address"] = ""
			out["ipv4_address_available"] = "no"

			nics = append(nics, out)
		}

		if err := d.Set("network_adapter", nics); err != nil {
			return fmt.Errorf("can't set network_adapter: %w", err)
		}

	}

	return nil
}
