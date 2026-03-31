package provider

// MachineState represents the state of a VirtualBox VM.
type MachineState string

const (
	MachineStatePoweroff MachineState = "poweroff"
	MachineStateRunning  MachineState = "running"
	MachineStatePaused   MachineState = "paused"
	MachineStateSaved    MachineState = "saved"
	MachineStateAborted  MachineState = "aborted"
)

// Machine represents a VirtualBox VM parsed from showvminfo --machinereadable.
type Machine struct {
	UUID       string
	Name       string
	State      MachineState
	OSType     string
	CPUs       uint
	Memory     uint // MiB
	VRAM       uint // MiB
	Flag       uint
	NICs       []NIC
	BootOrder  []string
	BaseFolder string
}

// Hardware flags matching go-virtualbox's constants
const (
	FlagACPI         uint = 1 << 0
	FlagIOAPIC       uint = 1 << 1
	FlagRTCUSEUTC    uint = 1 << 2
	FlagPAE          uint = 1 << 3
	FlagHWVIRTEX     uint = 1 << 4
	FlagNESTEDPAGING uint = 1 << 5
	FlagLARGEPAGES   uint = 1 << 6
	FlagLONGMODE     uint = 1 << 7
	FlagVTXVPID      uint = 1 << 8
	FlagVTXUX        uint = 1 << 9
)

// NICNetwork represents a NIC network type.
type NICNetwork string

const (
	NICNetNAT      NICNetwork = "nat"
	NICNetBridged  NICNetwork = "bridged"
	NICNetHostonly  NICNetwork = "hostonly"
	NICNetInternal NICNetwork = "internal"
	NICNetGeneric  NICNetwork = "generic"
)

// NICHardware represents a NIC hardware type.
type NICHardware string

const (
	AMDPCNetPCIII         NICHardware = "Am79C970A"
	AMDPCNetFASTIII       NICHardware = "Am79C973"
	IntelPro1000MTDesktop NICHardware = "82540EM"
	IntelPro1000TServer   NICHardware = "82543GC"
	IntelPro1000MTServer  NICHardware = "82545EM"
	VirtIO                NICHardware = "virtio"
)

// NIC represents a network adapter.
type NIC struct {
	Network       NICNetwork
	Hardware      NICHardware
	HostInterface string
	MacAddr       string
}
