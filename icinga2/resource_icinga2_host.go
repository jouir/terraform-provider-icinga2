package icinga2

import (
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/lrsmith/go-icinga2-api/iapi"
)

func resourceIcinga2Host() *schema.Resource {

	return &schema.Resource{
		Create: resourceIcinga2HostCreate,
		Read:   resourceIcinga2HostRead,
		Delete: resourceIcinga2HostDelete,
		Schema: map[string]*schema.Schema{
			"hostname": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Hostname",
				ForceNew:    true,
			},
			"address": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"check_command": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"groups": {
				Type:     schema.TypeList,
				Optional: true,
				ForceNew: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
			"vars": {
				Type:     schema.TypeMap,
				Optional: true,
				ForceNew: true,
			},
			"templates": {
				Type:     schema.TypeList,
				Optional: true,
				ForceNew: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
			"zone": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},
		},
	}
}

func resourceIcinga2HostCreate(d *schema.ResourceData, meta interface{}) error {

	client := meta.(*iapi.Server)

	hostname := d.Get("hostname").(string)
	address := d.Get("address").(string)
	checkCommand := d.Get("check_command").(string)
	zone := d.Get("zone").(string)

	vars := make(map[string]string)

	groups := make([]string, len(d.Get("groups").([]interface{})))
	for i, v := range d.Get("groups").([]interface{}) {
		groups[i] = v.(string)
	}

	// Normalize from map[string]interface{} to map[string]string
	iterator := d.Get("vars").(map[string]interface{})
	for key, value := range iterator {
		vars[key] = value.(string)
	}

	templates := make([]string, len(d.Get("templates").([]interface{})))
	for i, v := range d.Get("templates").([]interface{}) {
		templates[i] = v.(string)
	}

	// Call CreateHost with normalized data
	hosts, err := CreateHost(client, hostname, address, checkCommand, vars, templates, groups, zone)
	if err != nil {
		return err
	}

	found := false
	for _, host := range hosts {
		if host.Name == hostname {
			d.SetId(hostname)
			found = true
		}
	}

	if !found {
		return fmt.Errorf("Failed to Create Host %s : %s", hostname, err)
	}

	return nil
}

func resourceIcinga2HostRead(d *schema.ResourceData, meta interface{}) error {

	client := meta.(*iapi.Server)

	hostname := d.Get("hostname").(string)

	hosts, err := GetHost(client, hostname)
	if err != nil {
		return err
	}

	found := false
	for _, host := range hosts {
		if host.Name == hostname {
			d.SetId(hostname)
			_ = d.Set("hostname", host.Name)
			_ = d.Set("address", host.Attrs.Address)
			_ = d.Set("check_command", host.Attrs.CheckCommand)
			_ = d.Set("vars", host.Attrs.Vars)
			_ = d.Set("zone", host.Attrs.Zone)
			found = true
		}
	}

	if !found {
		return fmt.Errorf("Failed to Read Host %s : %s", hostname, err)
	}

	return nil
}

func resourceIcinga2HostDelete(d *schema.ResourceData, meta interface{}) error {

	client := meta.(*iapi.Server)
	hostname := d.Get("hostname").(string)

	err := client.DeleteHost(hostname)
	if err != nil {
		return fmt.Errorf("Failed to Delete Host %s : %s", hostname, err)
	}

	return nil

}

// Patch for zone
type NewHostStruct struct {
	Name      string      `json:"name"`
	Type      string      `json:"type"`
	Attrs     interface{} `json:"attrs"`
	Templates []string    `json:"templates"`
}

type HostStruct struct {
	Name      string    `json:"name"`
	Type      string    `json:"type"`
	Attrs     HostAttrs `json:"attrs"`
	Meta      struct{}  `json:"meta"`
	Joins     struct{}  `json:"stuct"`
	Templates []string  `json:"templates"`
}

type HostAttrs struct {
	ActionURL    string      `json:"action_url"`
	Address      string      `json:"address"`
	Address6     string      `json:"address6"`
	CheckCommand string      `json:"check_command"`
	DisplayName  string      `json:"display_name"`
	Groups       []string    `json:"groups"`
	Notes        string      `json:"notes"`
	NotesURL     string      `json:"notes_url"`
	Vars         interface{} `json:"vars,omitempty"`
	Templates    []string    `json:"templates"`
	Zone         string      `json:"zone,omitempty"`
}

func CreateHost(server *iapi.Server, hostname, address, checkCommand string, variables map[string]string, templates []string, groups []string, zone string) ([]HostStruct, error) {
	attrs := map[string]interface{}{}
	attrs["address"] = address
	attrs["check_command"] = checkCommand

	if variables != nil {
		for key, value := range variables {
			attrs["vars."+key] = value
		}
	}
	if groups != nil {
		attrs["groups"] = groups
	}
	attrs["zone"] = zone

	var newHost NewHostStruct
	newHost.Name = hostname
	newHost.Type = "Host"
	newHost.Attrs = attrs
	newHost.Templates = templates

	// Create JSON from completed struct
	payloadJSON, marshalErr := json.Marshal(newHost)
	if marshalErr != nil {
		return nil, marshalErr
	}

	// Make the API request to create the hosts.
	results, err := server.NewAPIRequest("PUT", "/objects/hosts/"+hostname, []byte(payloadJSON))
	if err != nil {
		return nil, err
	}

	if results.Code == 200 {
		hosts, err := GetHost(server, hostname)
		return hosts, err
	}

	return nil, fmt.Errorf("%s", results.ErrorString)
}

func GetHost(server *iapi.Server, hostname string) ([]HostStruct, error) {
	var hosts []HostStruct

	results, err := server.NewAPIRequest("GET", "/objects/hosts/"+hostname, nil)
	if err != nil {
		return nil, err
	}

	// Contents of the results is an interface object. Need to convert it to json first.
	jsonStr, marshalErr := json.Marshal(results.Results)
	if marshalErr != nil {
		return nil, marshalErr
	}

	// then the JSON can be pushed into the appropriate struct.
	// Note : Results is a slice so much push into a slice.

	if unmarshalErr := json.Unmarshal(jsonStr, &hosts); unmarshalErr != nil {
		return nil, unmarshalErr
	}

	return hosts, err
}
