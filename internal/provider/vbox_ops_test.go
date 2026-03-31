package provider

import (
	"testing"
)

func TestParseMachineInfo_Basic(t *testing.T) {
	output := `name="test-vm"
UUID="abc-123-def"
ostype="Ubuntu_64"
cpus=2
memory=1024
vram=20
VMState="running"
nic1="nat"
macaddress1="080027AABBCC"
nic2="hostonly"
hostonlyadapter2="vboxnet0"
macaddress2="080027DDEEFF"
boot1="disk"
boot2="dvd"
boot3="none"
boot4="none"
CfgFile="C:\\VirtualBox VMs\\test-vm\\test-vm.vbox"
`
	m, err := parseMachineInfo(output)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if m.Name != "test-vm" {
		t.Errorf("expected name 'test-vm', got %q", m.Name)
	}
	if m.UUID != "abc-123-def" {
		t.Errorf("expected UUID 'abc-123-def', got %q", m.UUID)
	}
	if m.OSType != "Ubuntu_64" {
		t.Errorf("expected ostype 'Ubuntu_64', got %q", m.OSType)
	}
	if m.CPUs != 2 {
		t.Errorf("expected 2 cpus, got %d", m.CPUs)
	}
	if m.Memory != 1024 {
		t.Errorf("expected 1024 memory, got %d", m.Memory)
	}
	if m.VRAM != 20 {
		t.Errorf("expected 20 vram, got %d", m.VRAM)
	}
	if m.State != MachineStateRunning {
		t.Errorf("expected state running, got %q", m.State)
	}
	if len(m.NICs) != 2 {
		t.Fatalf("expected 2 NICs, got %d", len(m.NICs))
	}
	if m.NICs[0].Network != NICNetNAT {
		t.Errorf("expected NIC 0 nat, got %q", m.NICs[0].Network)
	}
	if m.NICs[0].MacAddr != "080027AABBCC" {
		t.Errorf("expected MAC 080027AABBCC, got %q", m.NICs[0].MacAddr)
	}
	if m.NICs[1].Network != NICNetHostonly {
		t.Errorf("expected NIC 1 hostonly, got %q", m.NICs[1].Network)
	}
	if m.NICs[1].HostInterface != "vboxnet0" {
		t.Errorf("expected host interface vboxnet0, got %q", m.NICs[1].HostInterface)
	}
	if m.BootOrder[0] != "disk" {
		t.Errorf("expected boot1 disk, got %q", m.BootOrder[0])
	}
}

func TestParseMachineInfo_AllStates(t *testing.T) {
	tests := []struct {
		state    string
		expected MachineState
	}{
		{"running", MachineStateRunning},
		{"poweroff", MachineStatePoweroff},
		{"powered off", MachineStatePoweroff},
		{"paused", MachineStatePaused},
		{"saved", MachineStateSaved},
		{"aborted", MachineStateAborted},
		{"unknown", MachineStatePoweroff}, // default
	}

	for _, tt := range tests {
		t.Run(tt.state, func(t *testing.T) {
			output := `VMState="` + tt.state + `"`
			m, err := parseMachineInfo(output)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if m.State != tt.expected {
				t.Errorf("state %q: expected %q, got %q", tt.state, tt.expected, m.State)
			}
		})
	}
}

func TestParseMachineInfo_AllNICTypes(t *testing.T) {
	tests := []struct {
		nicVal   string
		expected NICNetwork
	}{
		{"nat", NICNetNAT},
		{"bridged", NICNetBridged},
		{"hostonly", NICNetHostonly},
		{"intnet", NICNetInternal},
		{"generic", NICNetGeneric},
	}

	for _, tt := range tests {
		t.Run(tt.nicVal, func(t *testing.T) {
			output := `nic1="` + tt.nicVal + `"`
			m, err := parseMachineInfo(output)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(m.NICs) != 1 {
				t.Fatalf("expected 1 NIC, got %d", len(m.NICs))
			}
			if m.NICs[0].Network != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, m.NICs[0].Network)
			}
		})
	}
}

func TestParseMachineInfo_NoNICs(t *testing.T) {
	output := `name="bare-vm"
UUID="xxx"
VMState="poweroff"
nic1="none"
`
	m, err := parseMachineInfo(output)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(m.NICs) != 0 {
		t.Errorf("expected 0 NICs, got %d", len(m.NICs))
	}
}

func TestParseMachineInfo_Empty(t *testing.T) {
	m, err := parseMachineInfo("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Name != "" {
		t.Errorf("expected empty name, got %q", m.Name)
	}
}

func TestParseMachineInfo_BridgedAdapter(t *testing.T) {
	output := `nic1="bridged"
bridgeadapter1="Intel Wi-Fi"
macaddress1="AABBCCDDEEFF"
`
	m, err := parseMachineInfo(output)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.NICs[0].HostInterface != "Intel Wi-Fi" {
		t.Errorf("expected bridge adapter 'Intel Wi-Fi', got %q", m.NICs[0].HostInterface)
	}
}

func TestToTarPath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{`C:\Users\test\file.box`, "/c/Users/test/file.box"},
		{`D:\data\image.tar.gz`, "/d/data/image.tar.gz"},
		{"/usr/local/file.box", "/usr/local/file.box"},
		{"relative/path.box", "relative/path.box"},
		{`C:\`, "/c/"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := toTarPath(tt.input)
			if result != tt.expected {
				t.Errorf("toTarPath(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestDetectVBoxManage_CachesPath(t *testing.T) {
	// Reset cache
	oldPath := vboxManagePath
	vboxManagePath = ""
	defer func() { vboxManagePath = oldPath }()

	// This test just verifies the function doesn't panic
	// Actual detection depends on VBoxManage being installed
	_, _ = detectVBoxManage()
}
