// Implementation of resourceDockerNetwork is derived from github.com/terraform-providers/terraform-provider-docker.
// See https://github.com/terraform-providers/terraform-provider-docker/blob/master/LICENSE for original licensing details.

package provider

import (
	"bytes"
	"fmt"
	"sort"

	"github.com/hashicorp/terraform/helper/hashcode"
	"github.com/hashicorp/terraform/helper/schema"

	dc "github.com/fsouza/go-dockerclient"
)

func resourceDockerNetwork() *schema.Resource {
	return &schema.Resource{
		Create: resourceDockerNetworkCreate,
		Read:   resourceDockerNetworkRead,
		Delete: resourceDockerNetworkDelete,
		Exists: resourceDockerNetworkExists,

		Schema: map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},

			"check_duplicate": {
				Type:     schema.TypeBool,
				Optional: true,
				ForceNew: true,
			},

			"driver": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
				Computed: true,
			},

			"options": {
				Type:     schema.TypeMap,
				Optional: true,
				ForceNew: true,
				Computed: true,
			},

			"internal": {
				Type:     schema.TypeBool,
				Optional: true,
				Computed: true,
				ForceNew: true,
			},

			"ipam_driver": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},

			"ipam_config": {
				Type:     schema.TypeSet,
				Optional: true,
				ForceNew: true,
				Elem:     getIpamConfigElem(),
				Set:      resourceDockerIpamConfigHash,
			},

			"id": {
				Type:     schema.TypeString,
				Computed: true,
			},

			"scope": {
				Type:     schema.TypeString,
				Computed: true,
			},
		},
	}
}

func getIpamConfigElem() *schema.Resource {
	return &schema.Resource{
		Schema: map[string]*schema.Schema{
			"subnet": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},

			"ip_range": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},

			"gateway": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},

			"aux_address": {
				Type:     schema.TypeMap,
				Optional: true,
				ForceNew: true,
			},
		},
	}
}

func resourceDockerIpamConfigHash(v interface{}) int {
	var buf bytes.Buffer
	m := v.(map[string]interface{})

	if v, ok := m["subnet"]; ok {
		buf.WriteString(fmt.Sprintf("%v-", v.(string)))
	}

	if v, ok := m["ip_range"]; ok {
		buf.WriteString(fmt.Sprintf("%v-", v.(string)))
	}

	if v, ok := m["gateway"]; ok {
		buf.WriteString(fmt.Sprintf("%v-", v.(string)))
	}

	if v, ok := m["aux_address"]; ok {
		auxAddress := v.(map[string]interface{})

		keys := make([]string, len(auxAddress))
		i := 0
		for k := range auxAddress {
			keys[i] = k
			i++
		}
		sort.Strings(keys)

		for _, k := range keys {
			buf.WriteString(fmt.Sprintf("%v-%v-", k, auxAddress[k].(string)))
		}
	}

	return hashcode.String(buf.String())
}

func resourceDockerNetworkCreate(d *schema.ResourceData, meta interface{}) error {
	providerConfig := meta.(*ProviderConfig)
	resolvedConfig, _, err := providerConfig.GetResolvedConfig(d)
	if err != nil {
		return err
	}
	client, err := resolvedConfig.NewClient()
	if err != nil {
		return err
	}

	createOpts := dc.CreateNetworkOptions{
		Name: d.Get("name").(string),
	}
	if v, ok := d.GetOk("check_duplicate"); ok {
		createOpts.CheckDuplicate = v.(bool)
	}
	if v, ok := d.GetOk("driver"); ok {
		createOpts.Driver = v.(string)
	}
	if v, ok := d.GetOk("options"); ok {
		createOpts.Options = v.(map[string]interface{})
	}
	if v, ok := d.GetOk("internal"); ok {
		createOpts.Internal = v.(bool)
	}

	ipamOpts := dc.IPAMOptions{}
	ipamOptsSet := false
	if v, ok := d.GetOk("ipam_driver"); ok {
		ipamOpts.Driver = v.(string)
		ipamOptsSet = true
	}
	if v, ok := d.GetOk("ipam_config"); ok {
		ipamOpts.Config = ipamConfigSetToIpamConfigs(v.(*schema.Set))
		ipamOptsSet = true
	}

	if ipamOptsSet {
		createOpts.IPAM = ipamOpts
	}

	var retNetwork *dc.Network
	if retNetwork, err = client.CreateNetwork(createOpts); err != nil {
		return fmt.Errorf("Unable to create network: %s", err)
	}
	if retNetwork == nil {
		return fmt.Errorf("Returned network is nil")
	}

	d.SetId(retNetwork.ID)
	d.Set("name", retNetwork.Name)
	d.Set("scope", retNetwork.Scope)
	d.Set("driver", retNetwork.Driver)
	d.Set("options", retNetwork.Options)

	// The 'internal' property is not send back when create network
	d.Set("internal", createOpts.Internal)

	return nil
}

func resourceDockerNetworkRead(d *schema.ResourceData, meta interface{}) error {
	providerConfig := meta.(*ProviderConfig)
	resolvedConfig, _, err := providerConfig.GetResolvedConfig(d)
	if err != nil {
		return err
	}
	client, err := resolvedConfig.NewClient()
	if err != nil {
		return err
	}

	var retNetwork *dc.Network
	if retNetwork, err = client.NetworkInfo(d.Id()); err != nil {
		if _, ok := err.(*dc.NoSuchNetwork); !ok {
			return fmt.Errorf("Unable to inspect network: %s", err)
		}
	}
	if retNetwork == nil {
		d.SetId("")
		return nil
	}

	d.Set("scope", retNetwork.Scope)
	d.Set("driver", retNetwork.Driver)
	d.Set("options", retNetwork.Options)
	d.Set("internal", retNetwork.Internal)

	return nil
}

func resourceDockerNetworkDelete(d *schema.ResourceData, meta interface{}) error {
	providerConfig := meta.(*ProviderConfig)
	resolvedConfig, _, err := providerConfig.GetResolvedConfig(d)
	if err != nil {
		return err
	}
	client, err := resolvedConfig.NewClient()
	if err != nil {
		return err
	}

	if err := client.RemoveNetwork(d.Id()); err != nil {
		if _, ok := err.(*dc.NoSuchNetwork); !ok {
			return fmt.Errorf("Error deleting network %s: %s", d.Id(), err)
		}
	}

	d.SetId("")
	return nil
}

func resourceDockerNetworkExists(d *schema.ResourceData, meta interface{}) (bool, error) {
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

	var retNetwork *dc.Network
	if retNetwork, err = client.NetworkInfo(d.Id()); err != nil {
		if _, ok := err.(*dc.NoSuchNetwork); !ok {
			return false, fmt.Errorf("Unable to inspect network: %s", err)
		}
	}
	if retNetwork == nil {
		return false, nil
	}

	return true, nil
}

func ipamConfigSetToIpamConfigs(ipamConfigSet *schema.Set) []dc.IPAMConfig {
	ipamConfigs := make([]dc.IPAMConfig, ipamConfigSet.Len())

	for i, ipamConfigInt := range ipamConfigSet.List() {
		ipamConfigRaw := ipamConfigInt.(map[string]interface{})

		ipamConfig := dc.IPAMConfig{}
		ipamConfig.Subnet = ipamConfigRaw["subnet"].(string)
		ipamConfig.IPRange = ipamConfigRaw["ip_range"].(string)
		ipamConfig.Gateway = ipamConfigRaw["gateway"].(string)

		auxAddressRaw := ipamConfigRaw["aux_address"].(map[string]interface{})
		ipamConfig.AuxAddress = make(map[string]string, len(auxAddressRaw))
		for k, v := range auxAddressRaw {
			ipamConfig.AuxAddress[k] = v.(string)
		}

		ipamConfigs[i] = ipamConfig
	}

	return ipamConfigs
}
