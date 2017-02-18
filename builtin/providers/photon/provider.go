package photon

import (
	"fmt"

	"github.com/hashicorp/terraform/helper/schema"
	"github.com/hashicorp/terraform/terraform"
)

// Provider returns a terraform.ResourceProvider.
func Provider() terraform.ResourceProvider {
	return &schema.Provider{
		Schema: map[string]*schema.Schema{
			/*
				"user": &schema.Schema{
					Type:        schema.TypeString,
					Required:    true,
					DefaultFunc: schema.EnvDefaultFunc("PHOTON_USER", nil),
					Description: "The user name.",
				},

				"password": &schema.Schema{
					Type:        schema.TypeString,
					Required:    true,
					DefaultFunc: schema.EnvDefaultFunc("PHOTON_PASSWORD", nil),
					Description: "The user password.",
				},
			*/

			"photon_server": &schema.Schema{
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("PHOTON_SERVER", nil),
				Description: "The Photon Endpoint URL for Photon API operations.",
			},
			"photon_ignoreCertificate": &schema.Schema{
				Type:        schema.TypeBool,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("PHOTON_IGNORE_CERTIFICATE", nil),
				Description: "The Photon ignoreCertificate.",
			},
			"photon_tenant": &schema.Schema{
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("PHOTON_TENANT", nil),
				Description: "The Photon Tenant.",
			},
			"photon_project": &schema.Schema{
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("PHOTON_PROJECT", nil),
				Description: "The Photon Project.",
			},
			"photon_overrideIP": &schema.Schema{
				Type:        schema.TypeBool,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("PHOTON_OVERRIDE_IP", nil),
				Description: "The Photon overrideIP.",
			},
		},

		ResourcesMap: map[string]*schema.Resource{
			"photon_virtual_machine": resourcePhotonVirtualMachine(),
		},

		ConfigureFunc: providerConfigure,
	}
}

func providerConfigure(d *schema.ResourceData) (interface{}, error) {
	// Handle backcompat support for vcenter_server; once that is removed,
	// vsphere_server can just become a Required field that is referenced inline
	// in Config below.
	server := d.Get("photon_server").(string)

	if server == "" {
		return nil, fmt.Errorf(
			"One of photon_server must be provided.")
	}

	config := Config{
		CloudTarget:       server,
		IgnoreCertificate: d.Get("photon_ignoreCertificate").(bool),
		Tenant:            d.Get("photon_tenant").(string),
		Project:           d.Get("photon_project").(string),
		OverrideIP:        d.Get("photon_overrideIP").(bool),
	}

	return config.Client()
}
