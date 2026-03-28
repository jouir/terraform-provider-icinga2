package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/lrsmith/go-icinga2-api/iapi"
)

var (
	_ resource.Resource              = &hostResource{}
	_ resource.ResourceWithConfigure = &hostResource{}
)

func Host() resource.Resource {
	return &hostResource{}
}

type hostResourceModel struct {
	ID           types.String `tfsdk:"id"`
	LastUpdated  types.String `tfsdk:"last_updated"`
	Hostname     types.String `tfsdk:"hostname"`
	Address      types.String `tfsdk:"address"`
	CheckCommand types.String `tfsdk:"check_command"`
	Groups       types.List   `tfsdk:"groups"`
	Vars         types.Map    `tfsdk:"vars"`
	Templates    types.List   `tfsdk:"templates"`
	Zone         types.String `tfsdk:"zone"`
}

type hostResource struct {
	client *iapi.Server
}

func (r *hostResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_host"
}

func (r *hostResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
			},
			"last_updated": schema.StringAttribute{
				Computed: true,
			},
			"hostname": schema.StringAttribute{
				Required:    true,
				Description: "Hostname",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"address": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"check_command": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"groups": schema.ListAttribute{
				ElementType: types.StringType,
				Optional:    true,
			},
			"vars": schema.MapAttribute{
				ElementType: types.StringType,
				Optional:    true,
			},
			"templates": schema.ListAttribute{
				ElementType: types.StringType,
				Optional:    true,
			},
			"zone": schema.StringAttribute{
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Default: stringdefault.StaticString("master"),
			},
		},
	}
}

func (r *hostResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*iapi.Server)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *iapi.Server, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	r.client = client
}

func (r *hostResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan hostResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var groups []string
	if !plan.Groups.IsNull() && !plan.Groups.IsUnknown() {
		for _, group := range plan.Groups.Elements() {
			if strVal, ok := group.(types.String); ok {
				groups = append(groups, strVal.ValueString())
			} else {
				resp.Diagnostics.AddError(
					"Error creating Host",
					"Group not a string",
				)
			}
		}
	}

	vars := make(map[string]string)
	if !plan.Vars.IsNull() && !plan.Vars.IsUnknown() {
		for key, value := range plan.Vars.Elements() {
			if strVal, ok := value.(types.String); ok {
				vars[key] = strVal.ValueString()
			} else {
				resp.Diagnostics.AddError(
					"Error creating Host",
					"Variable not a string",
				)
			}
		}
	}

	var templates []string
	if !plan.Templates.IsNull() && !plan.Templates.IsUnknown() {
		for _, template := range plan.Templates.Elements() {
			if strVal, ok := template.(types.String); ok {
				templates = append(templates, strVal.ValueString())
			} else {
				resp.Diagnostics.AddError(
					"Error creating Host",
					"Template not a string",
				)
			}
		}
	}

	hosts, err := CreateHost(r.client, plan.Hostname.ValueString(), plan.Address.ValueString(), plan.CheckCommand.ValueString(), vars, templates, groups, plan.Zone.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating Host",
			"Could not create host unexpected error: "+err.Error(),
		)
		return
	}

	for _, host := range hosts {
		if host.Name == plan.Hostname.ValueString() {
			plan.ID = types.StringValue(host.Name)
		}
	}
	plan.LastUpdated = types.StringValue(time.Now().Format(time.RFC850))

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r *hostResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state hostResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	hosts, err := GetHost(r.client, state.Hostname.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Host",
			"Could not read host "+state.Hostname.ValueString()+": "+err.Error(),
		)
		return
	}

	for _, host := range hosts {
		if host.Name == state.Hostname.ValueString() {
			state.ID = types.StringValue(host.Name)
			state.Hostname = types.StringValue(host.Name)
			state.Address = types.StringValue(host.Attrs.Address)
			state.CheckCommand = types.StringValue(host.Attrs.CheckCommand)
			state.Zone = types.StringValue(host.Attrs.Zone)

			// Note: We might need to map vars back to state correctly for lists/maps. For simplicity keeping it string mapped to attributes if they existed directly.
		}
	}

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r *hostResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError(
		"Update not supported",
		"Updates are currently not supported for host resources",
	)
}

func (r *hostResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state hostResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteHost(state.Hostname.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting Host",
			"Could not delete host, unexpected error: "+err.Error(),
		)
		return
	}
}

// Host patch
// https://github.com/lrsmith/go-icinga2-api/pull/19
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
