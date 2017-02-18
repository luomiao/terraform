package photon

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/hashicorp/terraform/helper/schema"
	"github.com/vmware/photon-controller-go-sdk/photon"
	"regexp"
)

const (
	MAC_OUI_VC         = "00:50:56"
	MAC_OUI_ESX        = "00:0c:29"
	checkSleepDuration = time.Second
	checkIPTimeout     = 90 * time.Second
)

type virtualMachine struct {
	name       string
	tenant     string
	project    string
	flavor     string
	diskFlavor string
	diskName   string
	image      string
	networks   string
}

func resourcePhotonVirtualMachine() *schema.Resource {
	return &schema.Resource{
		Create: resourcePhotonVirtualMachineCreate,
		Read:   resourcePhotonVirtualMachineRead,
		Delete: resourcePhotonVirtualMachineDelete,

		SchemaVersion: 1,

		Schema: map[string]*schema.Schema{
			"name": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},

			"tenant": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},

			"project": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},

			"flavor": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},

			"diskFlavor": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},

			"diskName": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},

			"image": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},

			"networks": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},

			"ip_address": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},

			"vmID": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},
		},
	}
}

func resourcePhotonVirtualMachineCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*photon.Client)

	vm := virtualMachine{
		name: d.Get("name").(string),
	}

	if v, ok := d.GetOk("tenant"); ok {
		tenants, err := client.Tenants.GetAll()
		if err != nil {
			return err
		}
		var found bool
		for _, tenant := range tenants.Items {
			if tenant.Name == v.(string) {
				found = true
				vm.tenant = tenant.ID
				break
			}
		}
		if !found {
			return fmt.Errorf("Tenant name '%s' not found", v)
		}
		log.Printf("[DEBUG] found Tenant ID %s for name %s", vm.tenant, v)
	}

	if v, ok := d.GetOk("project"); ok {
		tickets, err := client.Tenants.GetProjects(vm.tenant, &photon.ProjectGetOptions{Name: v.(string)})
		if err != nil {
			return err
		}
		pList := tickets.Items
		if len(pList) < 1 {
			return fmt.Errorf("Cannot find project named '%s'", v)
		}
		if len(pList) > 1 {
			return fmt.Errorf("Found more than 1 projects named '%s'", v)
		}

		vm.project = pList[0].ID
		log.Printf("[DEBUG] found Project ID %s for name %s", vm.project, v)
	}

	if v, ok := d.GetOk("flavor"); ok {
		vm.flavor = v.(string)
	}

	if v, ok := d.GetOk("diskFlavor"); ok {
		vm.diskFlavor = v.(string)
	}

	if v, ok := d.GetOk("diskName"); ok {
		vm.diskName = v.(string)
	}

	if v, ok := d.GetOk("image"); ok {
		vm.image = v.(string)
	}

	if v, ok := d.GetOk("networks"); ok {
		vm.networks = v.(string)
	}

	/* create disks list for the VM, which is only boot disk for now */
	var disksList []photon.AttachedDisk
	disksList = append(disksList, photon.AttachedDisk{Name: vm.diskName, Flavor: vm.diskFlavor, Kind: "ephemeral-disk", BootDisk: true})

	/* create networks list for the VM */
	var networkList []string
	if len(vm.networks) > 0 {
		networkList = regexp.MustCompile(`\s*,\s*`).Split(vm.networks, -1)
	}

	/* start creating virtual machine */
	vmSpec := photon.VmCreateSpec{}
	vmSpec.Name = vm.name
	vmSpec.Flavor = vm.flavor
	vmSpec.SourceImageID = vm.image
	vmSpec.AttachedDisks = disksList
	vmSpec.Subnets = networkList

	createTask, err := client.Projects.CreateVM(vm.project, &vmSpec)
	if err != nil {
		return err
	}

	waitTask, err := client.Tasks.Wait(createTask.ID)
	if err != nil {
		return err
	}

	vmid := waitTask.Entity.ID
	d.SetId(vmid)

	startTask, err := client.VMs.Start(vmid)
	if err != nil {
		return err
	}

	_, err = client.Tasks.Wait(startTask.ID)
	if err != nil {
		return err
	}

	log.Printf("[INFO] Created and started virtual machine: %s : %s", d.Id(), vm.name)

	return resourcePhotonVirtualMachineRead(d, meta)
}

func resourcePhotonVirtualMachineRead(d *schema.ResourceData, meta interface{}) error {
	log.Printf("[DEBUG] virtual machine resource data: %#v", d)
	client := meta.(*photon.Client)

	vmID := d.Id()
	d.Set("vmID", vmID)

	/* set IP address */
	/* IP address might not be ready yet, loop wait for a while */
	ticker := time.NewTicker(checkSleepDuration)
	defer ticker.Stop()
	timer := time.NewTimer(checkIPTimeout)
	defer timer.Stop()

	for {
		select {
		case <-timer.C:
			log.Printf("[DEBUG] Timeout to get valid IP address for VM %s", vmID)
			return fmt.Errorf("Timeout to get valid IP address for VM")
		case <-ticker.C:
			task, err := client.VMs.GetNetworks(vmID)
			if err != nil {
				log.Printf("[DEBUG] Failed to get networks for VM %s", vmID)
				return err
			} else {
				task, err = client.Tasks.Wait(task.ID)
				if err != nil {
					log.Printf("[DEBUG] Failed to wait for GetNetworks from VM %s", vmID)
					return err
				} else {
					networkConnections := task.ResourceProperties.(map[string]interface{})
					networks := networkConnections["networkConnections"].([]interface{})
					for _, nt := range networks {
						ipAddr := "-"
						macAddr := "-"
						network := nt.(map[string]interface{})
						if val, ok := network["ipAddress"]; ok && val != nil {
							ipAddr = val.(string)
						}
						if val, ok := network["macAddress"]; ok && val != nil {
							macAddr = val.(string)
						}
						if ipAddr != "-" {
							if strings.HasPrefix(macAddr, MAC_OUI_VC) ||
								strings.HasPrefix(macAddr, MAC_OUI_ESX) {
								d.Set("ip_address", ipAddr)
								log.Printf("[DEBUG] virtual machine %s get external IP address to %s", vmID, ipAddr)
								return nil
							}
						}
					}
				}
			}
		}
	}

}

func resourcePhotonVirtualMachineDelete(d *schema.ResourceData, meta interface{}) error {
	log.Printf("[DEBUG] Start to delete virtual machine %s", d.Id())
	client := meta.(*photon.Client)
	vmID := d.Id()

	/* Stop VM */
	task, err := client.VMs.Stop(vmID)
	if err != nil {
		return err
	}
	_, err = client.Tasks.Wait(task.ID)
	if err != nil {
		return err
	}

	/* Detach all attached non-boot disks */
	vm, err := client.VMs.Get(vmID)
	if err != nil {
		return err
	}
	for _, disk := range vm.AttachedDisks {
		if disk.BootDisk != true {
			operation := &photon.VmDiskOperation{
				DiskID: disk.ID,
			}

			task, err = client.VMs.DetachDisk(vmID, operation)
			if err != nil {
				return err
			}
			_, err = client.Tasks.Wait(task.ID)
			if err != nil {
				return err
			}
		}
	}

	/* Delete VM */
	task, err = client.VMs.Delete(vmID)
	if err != nil {
		return err
	}
	_, err = client.Tasks.Wait(task.ID)
	if err != nil {
		return err
	}

	log.Printf("[DEBUG] Successful deleted virtual machine %s", d.Id())
	return nil
}
