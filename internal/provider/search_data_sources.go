package provider

import (
	"context"
	"fmt"
	"net/url"
	"strconv"

	"github.com/santos-edu/terraform-provider-xyops/internal/xyops"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Wrappers for the xyOps search actions. The three contracts share the request
// shaping (query, offset, limit) and the rows + list.length envelope, but the
// row types differ per action, so the mapping stays concrete per data source
// instead of hiding behind a generic abstraction.

type searchEnvelope[Row any] struct {
	Rows []Row `json:"rows"`
	List struct {
		Length int64 `json:"length"`
	} `json:"list"`
}

func searchParams(query types.String, offset, limit types.Int64) url.Values {
	values := url.Values{}
	if query.IsNull() || query.IsUnknown() || query.ValueString() == "" {
		values.Set("query", "*")
	} else {
		values.Set("query", query.ValueString())
	}
	if offset.IsNull() || offset.IsUnknown() {
		values.Set("offset", "0")
	} else {
		values.Set("offset", strconv.FormatInt(offset.ValueInt64(), 10))
	}
	// The API default limit is 1, which suits a UI page load, not a plan; the
	// provider pins 100 when the configuration omits the attribute.
	if limit.IsNull() || limit.IsUnknown() {
		values.Set("limit", "100")
	} else {
		values.Set("limit", strconv.FormatInt(limit.ValueInt64(), 10))
	}
	return values
}

func searchQueryAttribute(queryExamples string) schema.StringAttribute {
	return schema.StringAttribute{
		Optional:    true,
		Description: fmt.Sprintf("Search query in the xyOps index syntax, e.g. %s. Defaults to all records.", queryExamples),
	}
}

func searchOffsetAttribute() schema.Int64Attribute {
	return schema.Int64Attribute{
		Optional:    true,
		Description: "Zero-based row offset for pagination. Defaults to 0.",
	}
}

func searchLimitAttribute() schema.Int64Attribute {
	return schema.Int64Attribute{
		Optional:    true,
		Description: "Maximum number of rows to return. Defaults to 100.",
	}
}

func searchTotalAttribute() schema.Int64Attribute {
	return schema.Int64Attribute{
		Computed:    true,
		Description: "Total number of matching records, without pagination.",
	}
}

func searchConfigureClient(req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) *xyops.Client {
	if req.ProviderData == nil {
		return nil
	}
	client, ok := req.ProviderData.(*xyops.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *xyops.Client, got %T.", req.ProviderData),
		)
		return nil
	}
	return client
}

func searchClientUnavailable(resp *datasource.ReadResponse) {
	resp.Diagnostics.AddError(
		"xyOps client unavailable",
		"Configure the xyOps provider before reading xyOps data sources.",
	)
}

// searchRowStringList keeps null for an empty reference list so the state does
// not flip between null and [] on every read.
func searchRowStringList(ctx context.Context, values []string) (types.List, diag.Diagnostics) {
	if len(values) == 0 {
		return types.ListNull(types.StringType), nil
	}
	return types.ListValueFrom(ctx, types.StringType, nonNilStrings(values))
}

//
// xyops_servers (search_servers)
//

var _ datasource.DataSource = (*serversDataSource)(nil)

type serversDataSource struct{ client *xyops.Client }

type serverSearchPayload struct {
	ID        string   `json:"id"`
	Hostname  string   `json:"hostname"`
	IP        string   `json:"ip"`
	Enabled   bool     `json:"enabled"`
	AutoGroup bool     `json:"autoGroup"`
	Groups    []string `json:"groups"`
	Keywords  string   `json:"keywords"`
	Notes     string   `json:"notes"`
	Created   int64    `json:"created"`
	Modified  int64    `json:"modified"`
}

type serverSearchRow struct {
	ID        types.String `tfsdk:"id"`
	Hostname  types.String `tfsdk:"hostname"`
	IP        types.String `tfsdk:"ip"`
	Enabled   types.Bool   `tfsdk:"enabled"`
	AutoGroup types.Bool   `tfsdk:"auto_group"`
	Groups    types.List   `tfsdk:"groups"`
	Keywords  types.String `tfsdk:"keywords"`
	Notes     types.String `tfsdk:"notes"`
	Created   types.Int64  `tfsdk:"created"`
	Modified  types.Int64  `tfsdk:"modified"`
}

var serversRowType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"id":         types.StringType,
	"hostname":   types.StringType,
	"ip":         types.StringType,
	"enabled":    types.BoolType,
	"auto_group": types.BoolType,
	"groups":     types.ListType{ElemType: types.StringType},
	"keywords":   types.StringType,
	"notes":      types.StringType,
	"created":    types.Int64Type,
	"modified":   types.Int64Type,
}}

type serversDataSourceModel struct {
	Query   types.String      `tfsdk:"query"`
	Offset  types.Int64       `tfsdk:"offset"`
	Limit   types.Int64       `tfsdk:"limit"`
	Total   types.Int64       `tfsdk:"total"`
	Servers []serverSearchRow `tfsdk:"servers"`
}

func NewServersDataSource() datasource.DataSource { return &serversDataSource{} }

func (d *serversDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_servers"
}

func (d *serversDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Searches xyOps server records with search_servers. Unlike the live registry, the matches include servers that are offline or were dropped from the registry, so references stay stable between refreshes.",
		Attributes: map[string]schema.Attribute{
			"query":  searchQueryAttribute("`groups:main`, `os_distro:ubuntu`, `os_arch:x86_64`"),
			"offset": searchOffsetAttribute(),
			"limit":  searchLimitAttribute(),
			"total":  searchTotalAttribute(),
			"servers": schema.ListAttribute{
				Computed:    true,
				ElementType: serversRowType,
				Description: "Matching server records without info, in the order xyOps returned.",
			},
		},
	}
}

func (d *serversDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = searchConfigureClient(req, resp)
}

func (d *serversDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config serversDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if d.client == nil {
		searchClientUnavailable(resp)
		return
	}
	var response searchEnvelope[serverSearchPayload]
	if err := d.client.GetQuery("search_servers", searchParams(config.Query, config.Offset, config.Limit), &response); err != nil {
		resp.Diagnostics.AddError("Search xyOps servers failed", err.Error())
		return
	}
	config.Total = types.Int64Value(response.List.Length)
	rows := make([]serverSearchRow, 0, len(response.Rows))
	for _, payload := range response.Rows {
		groups, groupsDiags := searchRowStringList(ctx, payload.Groups)
		if groupsDiags.HasError() {
			resp.Diagnostics.Append(groupsDiags...)
			return
		}
		rows = append(rows, serverSearchRow{
			ID:        types.StringValue(payload.ID),
			Hostname:  types.StringValue(payload.Hostname),
			IP:        types.StringValue(payload.IP),
			Enabled:   types.BoolValue(payload.Enabled),
			AutoGroup: types.BoolValue(payload.AutoGroup),
			Groups:    groups,
			Keywords:  types.StringValue(payload.Keywords),
			Notes:     types.StringValue(payload.Notes),
			Created:   types.Int64Value(payload.Created),
			Modified:  types.Int64Value(payload.Modified),
		})
	}
	config.Servers = rows
	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}

//
// xyops_jobs (search_jobs)
//

var _ datasource.DataSource = (*jobsDataSource)(nil)

type jobsDataSource struct{ client *xyops.Client }

type jobSearchPayload struct {
	ID          string   `json:"id"`
	Event       string   `json:"event"`
	EventTitle  string   `json:"event_title"`
	Server      string   `json:"server"`
	Groups      []string `json:"groups"`
	Category    string   `json:"category"`
	Plugin      string   `json:"plugin"`
	State       string   `json:"state"`
	Started     float64  `json:"started"`
	Completed   float64  `json:"completed"`
	Elapsed     float64  `json:"elapsed"`
	Code        int64    `json:"code"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
}

type jobSearchRow struct {
	ID          types.String  `tfsdk:"id"`
	Event       types.String  `tfsdk:"event"`
	EventTitle  types.String  `tfsdk:"event_title"`
	Server      types.String  `tfsdk:"server"`
	Groups      types.List    `tfsdk:"groups"`
	Category    types.String  `tfsdk:"category"`
	Plugin      types.String  `tfsdk:"plugin"`
	State       types.String  `tfsdk:"state"`
	Started     types.Float64 `tfsdk:"started"`
	Completed   types.Float64 `tfsdk:"completed"`
	Elapsed     types.Float64 `tfsdk:"elapsed"`
	Code        types.Int64   `tfsdk:"code"`
	Description types.String  `tfsdk:"description"`
	Tags        types.List    `tfsdk:"tags"`
}

var jobsRowType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"id":          types.StringType,
	"event":       types.StringType,
	"event_title": types.StringType,
	"server":      types.StringType,
	"groups":      types.ListType{ElemType: types.StringType},
	"category":    types.StringType,
	"plugin":      types.StringType,
	"state":       types.StringType,
	"started":     types.Float64Type,
	"completed":   types.Float64Type,
	"elapsed":     types.Float64Type,
	"code":        types.Int64Type,
	"description": types.StringType,
	"tags":        types.ListType{ElemType: types.StringType},
}}

type jobsDataSourceModel struct {
	Query  types.String   `tfsdk:"query"`
	Offset types.Int64    `tfsdk:"offset"`
	Limit  types.Int64    `tfsdk:"limit"`
	Total  types.Int64    `tfsdk:"total"`
	Jobs   []jobSearchRow `tfsdk:"jobs"`
}

func NewJobsDataSource() datasource.DataSource { return &jobsDataSource{} }

func (d *jobsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_jobs"
}

func (d *jobsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Searches completed jobs with search_jobs, filtered by category and target access rights of the API key. Useful queries include `tags:_error` for failed jobs, `code:critical` and `event:<event id>`.",
		Attributes: map[string]schema.Attribute{
			"query":  searchQueryAttribute("`tags:_error`, `code:critical`, `event:<event id>`, `date:>=today`"),
			"offset": searchOffsetAttribute(),
			"limit":  searchLimitAttribute(),
			"total":  searchTotalAttribute(),
			"jobs": schema.ListAttribute{
				Computed:    true,
				ElementType: jobsRowType,
				Description: "Matching job records, pruned of runtime output, in the order xyOps returned.",
			},
		},
	}
}

func (d *jobsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = searchConfigureClient(req, resp)
}

func (d *jobsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config jobsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if d.client == nil {
		searchClientUnavailable(resp)
		return
	}
	var response searchEnvelope[jobSearchPayload]
	if err := d.client.GetQuery("search_jobs", searchParams(config.Query, config.Offset, config.Limit), &response); err != nil {
		resp.Diagnostics.AddError("Search xyOps jobs failed", err.Error())
		return
	}
	config.Total = types.Int64Value(response.List.Length)
	rows := make([]jobSearchRow, 0, len(response.Rows))
	for _, payload := range response.Rows {
		groups, gDiags := searchRowStringList(ctx, payload.Groups)
		if gDiags.HasError() {
			resp.Diagnostics.Append(gDiags...)
			return
		}
		tags, tDiags := searchRowStringList(ctx, payload.Tags)
		if tDiags.HasError() {
			resp.Diagnostics.Append(tDiags...)
			return
		}
		rows = append(rows, jobSearchRow{
			ID:          types.StringValue(payload.ID),
			Event:       types.StringValue(payload.Event),
			EventTitle:  types.StringValue(payload.EventTitle),
			Server:      types.StringValue(payload.Server),
			Groups:      groups,
			Category:    types.StringValue(payload.Category),
			Plugin:      types.StringValue(payload.Plugin),
			State:       types.StringValue(payload.State),
			Started:     types.Float64Value(payload.Started),
			Completed:   types.Float64Value(payload.Completed),
			Elapsed:     types.Float64Value(payload.Elapsed),
			Code:        types.Int64Value(payload.Code),
			Description: types.StringValue(payload.Description),
			Tags:        tags,
		})
	}
	config.Jobs = rows
	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}

//
// xyops_alerts (search_alerts)
//

var _ datasource.DataSource = (*alertsDataSource)(nil)

type alertsDataSource struct{ client *xyops.Client }

type alertSearchPayload struct {
	ID       string   `json:"id"`
	Alert    string   `json:"alert"`
	Server   string   `json:"server"`
	Groups   []string `json:"groups"`
	Date     float64  `json:"date"`
	Exp      string   `json:"exp"`
	Message  string   `json:"message"`
	Count    int64    `json:"count"`
	Active   bool     `json:"active"`
	Notified bool     `json:"notified"`
	Modified float64  `json:"modified"`
	Jobs     []string `json:"jobs"`
	Tickets  []string `json:"tickets"`
}

type alertSearchRow struct {
	ID       types.String  `tfsdk:"id"`
	Alert    types.String  `tfsdk:"alert"`
	Server   types.String  `tfsdk:"server"`
	Groups   types.List    `tfsdk:"groups"`
	Date     types.Float64 `tfsdk:"date"`
	Exp      types.String  `tfsdk:"exp"`
	Message  types.String  `tfsdk:"message"`
	Count    types.Int64   `tfsdk:"count"`
	Active   types.Bool    `tfsdk:"active"`
	Notified types.Bool    `tfsdk:"notified"`
	Modified types.Float64 `tfsdk:"modified"`
	Jobs     types.List    `tfsdk:"jobs"`
	Tickets  types.List    `tfsdk:"tickets"`
}

var alertsRowType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"id":       types.StringType,
	"alert":    types.StringType,
	"server":   types.StringType,
	"groups":   types.ListType{ElemType: types.StringType},
	"date":     types.Float64Type,
	"exp":      types.StringType,
	"message":  types.StringType,
	"count":    types.Int64Type,
	"active":   types.BoolType,
	"notified": types.BoolType,
	"modified": types.Float64Type,
	"jobs":     types.ListType{ElemType: types.StringType},
	"tickets":  types.ListType{ElemType: types.StringType},
}}

type alertsDataSourceModel struct {
	Query  types.String     `tfsdk:"query"`
	Offset types.Int64      `tfsdk:"offset"`
	Limit  types.Int64      `tfsdk:"limit"`
	Total  types.Int64      `tfsdk:"total"`
	Alerts []alertSearchRow `tfsdk:"alerts"`
}

func NewAlertsDataSource() datasource.DataSource { return &alertsDataSource{} }

func (d *alertsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_alerts"
}

func (d *alertsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Searches alert invocations (firing events of alert definitions) with search_alerts. Useful queries include `active:true`, `alert:<alert id>` and `server:<server id>`.",
		Attributes: map[string]schema.Attribute{
			"query":  searchQueryAttribute("`active:true`, `alert:<alert id>`, `server:<server id>`, `start:>=today`"),
			"offset": searchOffsetAttribute(),
			"limit":  searchLimitAttribute(),
			"total":  searchTotalAttribute(),
			"alerts": schema.ListAttribute{
				Computed:    true,
				ElementType: alertsRowType,
				Description: "Matching alert invocation records in the order xyOps returned.",
			},
		},
	}
}

func (d *alertsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = searchConfigureClient(req, resp)
}

func (d *alertsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config alertsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if d.client == nil {
		searchClientUnavailable(resp)
		return
	}
	var response searchEnvelope[alertSearchPayload]
	if err := d.client.GetQuery("search_alerts", searchParams(config.Query, config.Offset, config.Limit), &response); err != nil {
		resp.Diagnostics.AddError("Search xyOps alerts failed", err.Error())
		return
	}
	config.Total = types.Int64Value(response.List.Length)
	rows := make([]alertSearchRow, 0, len(response.Rows))
	for _, payload := range response.Rows {
		groups, gDiags := searchRowStringList(ctx, payload.Groups)
		if gDiags.HasError() {
			resp.Diagnostics.Append(gDiags...)
			return
		}
		jobs, jDiags := searchRowStringList(ctx, payload.Jobs)
		if jDiags.HasError() {
			resp.Diagnostics.Append(jDiags...)
			return
		}
		tickets, tDiags := searchRowStringList(ctx, payload.Tickets)
		if tDiags.HasError() {
			resp.Diagnostics.Append(tDiags...)
			return
		}
		rows = append(rows, alertSearchRow{
			ID:       types.StringValue(payload.ID),
			Alert:    types.StringValue(payload.Alert),
			Server:   types.StringValue(payload.Server),
			Groups:   groups,
			Date:     types.Float64Value(payload.Date),
			Exp:      types.StringValue(payload.Exp),
			Message:  types.StringValue(payload.Message),
			Count:    types.Int64Value(payload.Count),
			Active:   types.BoolValue(payload.Active),
			Notified: types.BoolValue(payload.Notified),
			Modified: types.Float64Value(payload.Modified),
			Jobs:     jobs,
			Tickets:  tickets,
		})
	}
	config.Alerts = rows
	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
