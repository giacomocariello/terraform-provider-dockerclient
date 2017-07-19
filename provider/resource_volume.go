// Implementation of resourceDockerVolume is derived from github.com/terraform-providers/terraform-provider-docker.
// See https://github.com/terraform-providers/terraform-provider-docker/blob/master/LICENSE for original licensing details.

package provider

import (
	"fmt"

	dc "github.com/fsouza/go-dockerclient"
	"github.com/hashicorp/terraform/helper/schema"
)

func resourceDockerVolume() *schema.Resource {
	return &schema.Resource{
		Create: resourceDockerVolumeCreate,
		Read:   resourceDockerVolumeRead,
		Delete: resourceDockerVolumeDelete,
		Exists: resourceDockerVolumeExists,

		Schema: map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
				ForceNew: true,
			},
                        "host": {
                                Type:     schema.TypeString,
                                Optional: true,
                                ForceNew: true,
                        },
                        "machine_name": {
                                Type:     schema.TypeString,
                                Optional: true,
                                ForceNew: true,
                        },
			"driver": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
				ForceNew: true,
			},
			"driver_opts": {
				Type:     schema.TypeMap,
				Optional: true,
				ForceNew: true,
			},
			"mountpoint": {
				Type:     schema.TypeString,
				Computed: true,
			},
		},
	}
}

func resourceDockerVolumeCreate(d *schema.ResourceData, meta interface{}) error {
	providerConfig := meta.(*ProviderConfig)
	resolvedConfig, _, err := providerConfig.GetResolvedConfig(d)
	if err != nil {
		return err
	}
	client, err := resolvedConfig.NewClient()
	if err != nil {
		return err
	}

	createOpts := dc.CreateVolumeOptions{}
	if v, ok := d.GetOk("name"); ok {
		createOpts.Name = v.(string)
	}
	if v, ok := d.GetOk("driver"); ok {
		createOpts.Driver = v.(string)
	}
	if v, ok := d.GetOk("driver_opts"); ok {
		createOpts.DriverOpts = mapTypeMapValsToString(v.(map[string]interface{}))
	}

	var retVolume *dc.Volume
	if retVolume, err = client.CreateVolume(createOpts); err != nil {
		return fmt.Errorf("Unable to create volume: %s", err)
	}
	if retVolume == nil {
		return fmt.Errorf("Returned volume is nil")
	}

	d.SetId(retVolume.Name)
	d.Set("name", retVolume.Name)
	d.Set("driver", retVolume.Driver)
	d.Set("mountpoint", retVolume.Mountpoint)

	return nil
}

func resourceDockerVolumeRead(d *schema.ResourceData, meta interface{}) error {
	providerConfig := meta.(*ProviderConfig)
	resolvedConfig, _, err := providerConfig.GetResolvedConfig(d)
	if err != nil {
		return err
	}
	client, err := resolvedConfig.NewClient()
	if err != nil {
		return err
	}

	var retVolume *dc.Volume
	if retVolume, err = client.InspectVolume(d.Id()); err != nil && err != dc.ErrNoSuchVolume {
		return fmt.Errorf("Unable to inspect volume: %s", err)
	}
	if retVolume == nil {
		d.SetId("")
		return nil
	}

	d.Set("name", retVolume.Name)
	d.Set("driver", retVolume.Driver)
	d.Set("mountpoint", retVolume.Mountpoint)

	return nil
}

func resourceDockerVolumeDelete(d *schema.ResourceData, meta interface{}) error {
	providerConfig := meta.(*ProviderConfig)
	resolvedConfig, _, err := providerConfig.GetResolvedConfig(d)
	if err != nil {
		return err
	}
	client, err := resolvedConfig.NewClient()
	if err != nil {
		return err
	}

	if err := client.RemoveVolume(d.Id()); err != nil && err != dc.ErrNoSuchVolume {
		return fmt.Errorf("Error deleting volume %s: %s", d.Id(), err)
	}

	d.SetId("")
	return nil
}

func resourceDockerVolumeExists(d *schema.ResourceData, meta interface{}) (bool, error) {
	providerConfig := meta.(*ProviderConfig)
	resolvedConfig, deferred, err := providerConfig.GetResolvedConfig(d)
	if deferred {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	client, err := resolvedConfig.NewClient()
	if err != nil {
		return false, err
	}

	if retVolume, err := client.InspectVolume(d.Id()); err != nil && err != dc.ErrNoSuchVolume {
		return false, fmt.Errorf("Unable to inspect volume: %s", err)
	} else if retVolume == nil {
		return false, nil
	}

	return true, nil
}
