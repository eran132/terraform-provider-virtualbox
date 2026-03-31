package provider

import (
	"fmt"
	"sync"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	vbox "github.com/terra-farm/go-virtualbox"
)

var (
	defaultBootOrder = []string{"disk", "none", "none", "none"}
)

func init() {
	vbox.Verbose = true
}

var imageOpMutex sync.Mutex

func resourceVM() *schema.Resource {
	return &schema.Resource{
		Exists:        resourceVMExists,
		CreateContext: resourceVMCreate,
		ReadContext:   resourceVMRead,
		UpdateContext: resourceVMUpdate,
		Delete:        resourceVMDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{

			"name": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},

			"image": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},

			"url": {
				Type:       schema.TypeString,
				Optional:   true,
				ForceNew:   true,
				Deprecated: "Use the \"image\" option with a URL",
			},

			"optical_disks": {
				Type:        schema.TypeList,
				Optional:    true,
				Description: "List of Optical Disks to attach",
				Elem:        &schema.Schema{Type: schema.TypeString},
			},

			"cpus": {
				Type:     schema.TypeInt,
				Optional: true,
				Default:  "2",
			},

			"memory": {
				Type:     schema.TypeString,
				Optional: true,
				Default:  "512mib",
			},

			"status": {
				Type:     schema.TypeString,
				Optional: true,
				Default:  "running",
			},

			"user_data": {
				Deprecated: "user_data is not working and is temporarily deprecated while we figure out how to make it work",
				Type:       schema.TypeString,
				Optional:   true,
				Default:    "",
			},

			"checksum": {
				Type:     schema.TypeString,
				Optional: true,
				Default:  "",
			},

			"checksum_type": {
				Type:     schema.TypeString,
				Optional: true,
				Default:  "",
			},

			"network_adapter": {
				Type:     schema.TypeList,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{

						"type": {
							Type:     schema.TypeString,
							Required: true,
						},

						"device": {
							Type:     schema.TypeString,
							Optional: true,
							Default:  "IntelPro1000MTServer",
						},

						"host_interface": {
							Type:     schema.TypeString,
							Optional: true,
						},

						"mac_address": {
							Type:     schema.TypeString,
							Optional: true,
							Computed: true,
						},

						"promiscuous_mode": {
							Type:        schema.TypeString,
							Optional:    true,
							Default:     "deny",
							Description: "Promiscuous mode: deny (default), allow-vms, or allow-all",
						},

						"cable_connected": {
							Type:     schema.TypeBool,
							Optional: true,
							Default:  true,
						},

						"nat_dns_host_resolver": {
							Type:        schema.TypeBool,
							Optional:    true,
							Default:     false,
							Description: "Use the host's DNS resolver for NAT networking",
						},

						"nat_dns_proxy": {
							Type:        schema.TypeBool,
							Optional:    true,
							Default:     false,
							Description: "Enable DNS proxy for NAT networking",
						},

						"port_forwarding": {
							Type:     schema.TypeList,
							Optional: true,
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"name": {
										Type:     schema.TypeString,
										Required: true,
									},
									"protocol": {
										Type:     schema.TypeString,
										Optional: true,
										Default:  "tcp",
									},
									"host_ip": {
										Type:     schema.TypeString,
										Optional: true,
										Default:  "",
									},
									"host_port": {
										Type:     schema.TypeInt,
										Required: true,
									},
									"guest_ip": {
										Type:     schema.TypeString,
										Optional: true,
										Default:  "",
									},
									"guest_port": {
										Type:     schema.TypeInt,
										Required: true,
									},
								},
							},
						},

						"status": {
							Type:     schema.TypeString,
							Computed: true,
						},

						"ipv4_address": {
							Type:     schema.TypeString,
							Computed: true,
						},

						"ipv4_address_available": {
							Type:     schema.TypeString,
							Computed: true,
						},
					},
				},
			},

			"boot_order": {
				Type:        schema.TypeList,
				Optional:    true,
				Description: "Boot order, max 4 slots, each in [none, floppy, dvd, disk, net]",
				Elem:        &schema.Schema{Type: schema.TypeString},
				MaxItems:    4,
			},

			"os_type": {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "Linux_64",
				Description: "The guest OS type identifier (e.g. Linux_64, Windows_64, Ubuntu_64). Run 'VBoxManage list ostypes' for valid values.",
			},

			"vram": {
				Type:        schema.TypeInt,
				Optional:    true,
				Default:     20,
				Description: "Video memory in MiB (default: 20)",
			},

			"firmware": {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "bios",
				Description: "Firmware type: bios (default), efi, efi32, or efi64",
				ValidateFunc: func(val any, key string) (warns []string, errs []error) {
					v := val.(string)
					valid := map[string]bool{"bios": true, "efi": true, "efi32": true, "efi64": true}
					if !valid[v] {
						errs = append(errs, fmt.Errorf("%q must be one of bios, efi, efi32, efi64, got: %s", key, v))
					}
					return
				},
			},

			"graphics_controller": {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "vmsvga",
				Description: "Graphics controller type: none, vboxvga, vmsvga (default), or vboxsvga",
				ValidateFunc: func(val any, key string) (warns []string, errs []error) {
					v := val.(string)
					valid := map[string]bool{"none": true, "vboxvga": true, "vmsvga": true, "vboxsvga": true}
					if !valid[v] {
						errs = append(errs, fmt.Errorf("%q must be one of none, vboxvga, vmsvga, vboxsvga, got: %s", key, v))
					}
					return
				},
			},

			"gui": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Start VM with GUI window (true) or headless (false, default)",
			},

			"customize": {
				Type:        schema.TypeList,
				Optional:    true,
				Description: "List of VBoxManage commands to run after VM configuration. Each element is a list of arguments. Use ':id' as placeholder for the VM UUID.",
				Elem: &schema.Schema{
					Type: schema.TypeList,
					Elem: &schema.Schema{Type: schema.TypeString},
				},
			},

			"storage_controller": {
				Type:        schema.TypeList,
				Optional:    true,
				Description: "Additional storage controllers. A default SATA controller is created automatically if not specified.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"name": {
							Type:        schema.TypeString,
							Required:    true,
							Description: "Controller name (e.g. SATA, IDE, NVMe)",
						},
						"type": {
							Type:        schema.TypeString,
							Required:    true,
							Description: "Controller bus type: ide, sata, scsi, floppy, sas, pcie, virtio",
						},
						"controller": {
							Type:        schema.TypeString,
							Optional:    true,
							Default:     "",
							Description: "Controller chipset: IntelAHCI, PIIX4, LSILogic, BusLogic, NVMe, VirtIO",
						},
						"port_count": {
							Type:        schema.TypeInt,
							Optional:    true,
							Default:     0,
							Description: "Number of ports (0 = auto)",
						},
						"host_io_cache": {
							Type:     schema.TypeBool,
							Optional: true,
							Default:  true,
						},
						"bootable": {
							Type:     schema.TypeBool,
							Optional: true,
							Default:  true,
						},
					},
				},
			},

			"disk_attachment": {
				Type:        schema.TypeList,
				Optional:    true,
				Description: "Attach additional disks to storage controllers",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"storage_controller": {
							Type:        schema.TypeString,
							Required:    true,
							Description: "Name of the storage controller to attach to",
						},
						"port": {
							Type:     schema.TypeInt,
							Required: true,
						},
						"device": {
							Type:     schema.TypeInt,
							Optional: true,
							Default:  0,
						},
						"drive_type": {
							Type:        schema.TypeString,
							Optional:    true,
							Default:     "hdd",
							Description: "Drive type: hdd, dvd, or floppy",
						},
						"medium": {
							Type:        schema.TypeString,
							Required:    true,
							Description: "Path to the disk image file, or 'none'/'emptydrive'/'additions'",
						},
						"non_rotational": {
							Type:        schema.TypeBool,
							Optional:    true,
							Default:     false,
							Description: "Mark as SSD (non-rotational)",
						},
						"hot_pluggable": {
							Type:     schema.TypeBool,
							Optional: true,
							Default:  false,
						},
					},
				},
			},

			"linked_clone": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				ForceNew:    true,
				Description: "Create VM as a linked clone (requires source_vm)",
			},

			"source_vm": {
				Type:        schema.TypeString,
				Optional:    true,
				ForceNew:    true,
				Description: "Name or UUID of source VM to clone from (used with linked_clone)",
			},

			"cpu_execution_cap": {
				Type:        schema.TypeInt,
				Optional:    true,
				Default:     100,
				Description: "CPU execution cap percentage (1-100, default 100)",
				ValidateFunc: func(val any, key string) (warns []string, errs []error) {
					v := val.(int)
					if v < 1 || v > 100 {
						errs = append(errs, fmt.Errorf("%q must be between 1 and 100, got: %d", key, v))
					}
					return
				},
			},

			"ioapic": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  true,
			},

			"pae": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  true,
			},

			"nested_hw_virt": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Enable nested hardware virtualization",
			},

			"largepages": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  true,
			},

			"vtx_vpid": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  true,
			},

			"shared_folder": {
				Type:        schema.TypeList,
				Optional:    true,
				Description: "Shared folders between host and guest",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"name": {
							Type:     schema.TypeString,
							Required: true,
						},
						"host_path": {
							Type:     schema.TypeString,
							Required: true,
						},
						"read_only": {
							Type:     schema.TypeBool,
							Optional: true,
							Default:  false,
						},
						"auto_mount": {
							Type:     schema.TypeBool,
							Optional: true,
							Default:  false,
						},
						"mount_point": {
							Type:        schema.TypeString,
							Optional:    true,
							Default:     "",
							Description: "Guest mount point (VirtualBox 6.1+)",
						},
					},
				},
			},

			"usb_controller": {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "",
				Description: "USB controller: '' (disabled), ohci (1.1), ehci (2.0), or xhci (3.0)",
				ValidateFunc: func(val any, key string) (warns []string, errs []error) {
					v := val.(string)
					valid := map[string]bool{"": true, "ohci": true, "ehci": true, "xhci": true}
					if !valid[v] {
						errs = append(errs, fmt.Errorf("%q must be one of '', ohci, ehci, xhci, got: %s", key, v))
					}
					return
				},
			},

			"clipboard_mode": {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "disabled",
				Description: "Shared clipboard mode: disabled, hosttoguest, guesttohost, or bidirectional",
			},

			"drag_and_drop": {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "disabled",
				Description: "Drag and drop mode: disabled, hosttoguest, guesttohost, or bidirectional",
			},

			"chipset": {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "piix3",
				Description: "Chipset type: piix3 (default) or ich9",
				ValidateFunc: func(val any, key string) (warns []string, errs []error) {
					v := val.(string)
					valid := map[string]bool{"piix3": true, "ich9": true}
					if !valid[v] {
						errs = append(errs, fmt.Errorf("%q must be one of piix3, ich9, got: %s", key, v))
					}
					return
				},
			},

			"serial_port": {
				Type:     schema.TypeList,
				Optional: true,
				MaxItems: 4,
				Description: "Serial port configuration (up to 4 ports)",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"slot": {
							Type:        schema.TypeInt,
							Required:    true,
							Description: "Port slot number (0-3)",
						},
						"mode": {
							Type:        schema.TypeString,
							Required:    true,
							Description: "Mode: disconnected, server, client, tcpserver, tcpclient, file, rawfile",
						},
						"path": {
							Type:        schema.TypeString,
							Optional:    true,
							Default:     "",
							Description: "Path or address for the serial port (file path, pipe name, or host:port)",
						},
					},
				},
			},
		},
	}
}
