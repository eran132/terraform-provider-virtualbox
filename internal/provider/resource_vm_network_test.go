package provider

import (
	"testing"
)

func TestNICNetworkConstants(t *testing.T) {
	// Verify constants have expected string values
	tests := []struct {
		network NICNetwork
		str     string
	}{
		{NICNetNAT, "nat"},
		{NICNetBridged, "bridged"},
		{NICNetHostonly, "hostonly"},
		{NICNetInternal, "internal"},
		{NICNetGeneric, "generic"},
	}

	for _, tt := range tests {
		if string(tt.network) != tt.str {
			t.Errorf("NICNetwork %q != %q", tt.network, tt.str)
		}
	}
}

func TestNICHardwareConstants(t *testing.T) {
	tests := []struct {
		hw  NICHardware
		str string
	}{
		{AMDPCNetPCIII, "Am79C970A"},
		{AMDPCNetFASTIII, "Am79C973"},
		{IntelPro1000MTDesktop, "82540EM"},
		{IntelPro1000TServer, "82543GC"},
		{IntelPro1000MTServer, "82545EM"},
		{VirtIO, "virtio"},
	}

	for _, tt := range tests {
		if string(tt.hw) != tt.str {
			t.Errorf("NICHardware %q != %q", tt.hw, tt.str)
		}
	}
}

func TestMachineStateConstants(t *testing.T) {
	tests := []struct {
		state MachineState
		str   string
	}{
		{MachineStatePoweroff, "poweroff"},
		{MachineStateRunning, "running"},
		{MachineStatePaused, "paused"},
		{MachineStateSaved, "saved"},
		{MachineStateAborted, "aborted"},
	}

	for _, tt := range tests {
		if string(tt.state) != tt.str {
			t.Errorf("MachineState %q != %q", tt.state, tt.str)
		}
	}
}

func TestHardwareFlags(t *testing.T) {
	// Verify flags are unique powers of 2
	flags := []uint{
		FlagACPI, FlagIOAPIC, FlagRTCUSEUTC, FlagPAE,
		FlagHWVIRTEX, FlagNESTEDPAGING, FlagLARGEPAGES,
		FlagLONGMODE, FlagVTXVPID, FlagVTXUX,
	}

	seen := make(map[uint]bool)
	for _, f := range flags {
		if seen[f] {
			t.Errorf("duplicate flag value: %d", f)
		}
		seen[f] = true
		// Check it's a power of 2
		if f == 0 || (f&(f-1)) != 0 {
			t.Errorf("flag %d is not a power of 2", f)
		}
	}
}

func TestFlagCombination(t *testing.T) {
	// Test combining flags like tfToVbox does
	flags := FlagACPI | FlagRTCUSEUTC | FlagHWVIRTEX

	if flags&FlagACPI == 0 {
		t.Error("ACPI should be set")
	}
	if flags&FlagRTCUSEUTC == 0 {
		t.Error("RTCUSEUTC should be set")
	}
	if flags&FlagIOAPIC != 0 {
		t.Error("IOAPIC should NOT be set")
	}
}
