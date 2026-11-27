package searchconsole

import (
	"encoding/json/v2"
	"errors"
	"net/http"
	"slices"
	"strings"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

const siteMaximum = 4 * 1024

type queryWire struct {
	StartDate    string        `json:"startDate"`
	EndDate      string        `json:"endDate"`
	Type         string        `json:"type"`
	Aggregation  string        `json:"aggregationType"`
	State        string        `json:"dataState"`
	Dimensions   []string      `json:"dimensions,omitempty"`
	FilterGroups []filterGroup `json:"dimensionFilterGroups,omitempty"`
	Limit        uint16        `json:"rowLimit"`
	Offset       uint32        `json:"startRow"`
}

type filterGroup struct {
	Type    string       `json:"groupType"`
	Filters []filterWire `json:"filters"`
}

type filterWire struct {
	Dimension  string `json:"dimension"`
	Operator   string `json:"operator"`
	Expression string `json:"expression"`
}

func encodeQuery(r QueryRequest) ([]byte, error) {
	start, _ := r.Dates.Start.Text()
	end, _ := r.Dates.End.Text()
	wire := queryWire{
		StartDate: start, EndDate: end, Type: r.Type.String(),
		Aggregation: r.Aggregation.String(), State: r.State.String(),
		Limit: r.Page.Limit, Offset: r.Page.Offset,
	}
	for _, dimension := range [...]Dimension{r.Dimensions.First, r.Dimensions.Second} {
		if dimension.IsValid() {
			wire.Dimensions = append(wire.Dimensions, dimension.String())
		}
	}
	filters := []filterWire{}
	if r.Filters.Page != (PageURL{}) {
		filters = append(filters, filterWire{
			Dimension: DimensionPage.String(), Operator: FilterEquals.String(), Expression: r.Filters.Page.String(),
		})
	}
	if r.Filters.Query != (QueryText{}) {
		filters = append(filters, filterWire{
			Dimension: DimensionQuery.String(), Operator: FilterEquals.String(), Expression: r.Filters.Query.String(),
		})
	}
	if len(filters) > 0 {
		wire.FilterGroups = []filterGroup{{Type: "and", Filters: filters}}
	}
	data, err := json.Marshal(wire)
	if err != nil {
		return nil, errors.Join(core.ErrSearchConsoleContract, core.ErrJSONContract)
	}
	return data, nil
}

type pageWire struct {
	Rows        []rowWire     `json:"rows,omitempty"`
	Aggregation string        `json:"responseAggregationType,omitempty"`
	Metadata    *metadataWire `json:"metadata,omitempty"`
}

type metadataWire struct {
	FirstIncomplete string `json:"first_incomplete_date,omitempty"`
}

type rowWire struct {
	Keys        []string     `json:"keys,omitempty"`
	Clicks      *Clicks      `json:"clicks"`
	Impressions *Impressions `json:"impressions"`
	Rate        *ClickRate   `json:"ctr"`
	Position    *Position    `json:"position"`
}

type siteWire struct {
	Property   string `json:"siteUrl"`
	Permission string `json:"permissionLevel"`
}

func decodePage(data []byte, request QueryRequest) (QueryPage, error) {
	wire, err := decodeDocument[pageWire](data, responseMaximum, max(2, uint64(request.Page.Limit)))
	if err != nil {
		return QueryPage{}, err
	}
	if len(wire.Rows) > int(request.Page.Limit) {
		return QueryPage{}, core.ErrSearchConsoleResponse
	}

	got := QueryPage{Dates: request.Dates, State: request.State, Dimensions: request.Dimensions}
	for aggregation := AggregationAuto; aggregation <= AggregationProperty; aggregation++ {
		if aggregation.String() == wire.Aggregation {
			got.Aggregation = aggregation
			break
		}
	}
	if got.Aggregation == AggregationInvalid && wire.Aggregation != "" ||
		got.Aggregation != AggregationInvalid && request.Aggregation != AggregationAuto && got.Aggregation != request.Aggregation ||
		request.pageContext() && got.Aggregation == AggregationProperty {
		return QueryPage{}, core.ErrSearchConsoleResponse
	}
	if wire.Metadata != nil && wire.Metadata.FirstIncomplete != "" {
		got.FirstIncomplete, err = temporal.ParseCivilDate(wire.Metadata.FirstIncomplete)
		if err != nil {
			return QueryPage{}, errors.Join(core.ErrSearchConsoleResponse, err)
		}
	}
	if len(wire.Rows) > 0 {
		next := uint64(request.Page.Offset) + uint64(len(wire.Rows))
		if next <= OffsetMaximum {
			got.NextOffset = uint32(next)
		}
		got.Rows = make([]Row, 0, len(wire.Rows))
		for _, rowWire := range wire.Rows {
			row, err := rowWire.project(request)
			if err != nil {
				return QueryPage{}, err
			}
			got.Rows = append(got.Rows, row)
		}
	}
	if err := got.Validate(); err != nil {
		return QueryPage{}, errors.Join(core.ErrSearchConsoleResponse, err)
	}
	return got, nil
}

func (w rowWire) project(request QueryRequest) (Row, error) {
	if len(w.Keys) != request.Dimensions.count() || w.Clicks == nil || w.Impressions == nil || w.Rate == nil || w.Position == nil {
		return Row{}, core.ErrSearchConsoleResponse
	}
	row := Row{Clicks: *w.Clicks, Impressions: *w.Impressions, Rate: *w.Rate, Position: *w.Position}
	dimensions := [...]Dimension{request.Dimensions.First, request.Dimensions.Second}
	for i, key := range w.Keys {
		var err error
		switch dimensions[i] {
		case DimensionDate:
			row.Date, err = temporal.ParseCivilDate(key)
		case DimensionPage:
			row.Page, err = ParsePageURL(key)
			if request.Filters.Page != (PageURL{}) && row.Page != request.Filters.Page {
				return Row{}, core.ErrSearchConsoleResponse
			}
		case DimensionQuery:
			row.Query, err = ParseQueryText(key)
			if request.Filters.Query != (QueryText{}) && row.Query != request.Filters.Query {
				return Row{}, core.ErrSearchConsoleResponse
			}
		default:
			return Row{}, core.ErrSearchConsoleResponse
		}
		if err != nil {
			return Row{}, errors.Join(core.ErrSearchConsoleResponse, err)
		}
	}
	return row, nil
}

type rowKey struct {
	date  string
	page  PageURL
	query QueryText
}

func uniqueRowKeys(rows []Row) bool {
	if len(rows) < 2 {
		return true
	}
	keys := make([]rowKey, len(rows))
	for i, row := range rows {
		date, _ := row.Date.Text()
		keys[i] = rowKey{date: date, page: row.Page, query: row.Query}
	}
	slices.SortFunc(keys, func(a, b rowKey) int {
		switch {
		case a.date != b.date:
			return strings.Compare(a.date, b.date)
		case a.page != b.page:
			return strings.Compare(a.page.String(), b.page.String())
		default:
			return strings.Compare(a.query.String(), b.query.String())
		}
	})
	return len(slices.Compact(keys)) == len(rows)
}

func decodeSite(data []byte, wanted Property) (Site, error) {
	wire, err := decodeDocument[siteWire](data, siteMaximum, 1)
	if err != nil {
		return Site{}, err
	}
	property, err := ParseProperty(wire.Property)
	if err != nil || property != wanted {
		return Site{}, errors.Join(core.ErrSearchConsoleResponse, err)
	}
	for permission := PermissionRestricted; permission <= PermissionUnverified; permission++ {
		if permission.String() == wire.Permission {
			if permission == PermissionUnverified {
				return Site{}, core.ErrSearchConsoleProperty
			}
			return Site{Property: property, Permission: permission}, nil
		}
	}
	return Site{}, core.ErrSearchConsoleResponse
}

func decodeDocument[T any](data []byte, byteMaximum int, itemMaximum uint64) (T, error) {
	var zero T
	if len(data) > byteMaximum {
		return zero, errors.Join(core.ErrSearchConsoleResponse, core.ErrExchangeBodyLimit)
	}
	limits := providerJSONLimits(byteMaximum, itemMaximum)
	value, err := core.DecodeStrictJSONStructure[*T](data, limits)
	if err != nil || value == nil {
		return zero, errors.Join(core.ErrSearchConsoleResponse, core.ErrJSONContract)
	}
	return *value, nil
}

func providerJSONLimits(byteMaximum int, itemMaximum uint64) core.StrictJSONLimits {
	maximum, _ := core.NewByteCount(uint64(byteMaximum))
	return core.StrictJSONLimits{
		DocumentMaximumBytes: maximum, NestingDepthMaximum: 6, ObjectFieldMaximum: 16, ArrayItemMaximum: itemMaximum,
	}
}

type fault uint8

const (
	faultUnclassified fault = iota
	faultQuota
	faultProperty
)

type faultWire struct {
	Error *faultDetail `json:"error"`
}

type faultDetail struct {
	Reasons []faultReason `json:"errors"`
	Code    int           `json:"code"`
}

type faultReason struct {
	Reason string `json:"reason"`
}

func decodeFault(data []byte, status int) fault {
	if err := providerJSONLimits(failureMaximum, 16).ValidateDocument(data); err != nil {
		return faultUnclassified
	}
	var wire faultWire
	if err := json.Unmarshal(data, &wire); err != nil || wire.Error == nil || wire.Error.Code != status {
		return faultUnclassified
	}
	result := faultUnclassified
	for _, detail := range wire.Error.Reasons {
		switch detail.Reason {
		case "quotaExceeded", "rateLimitExceeded", "userRateLimitExceeded", "dailyLimitExceeded",
			"concurrentLimitExceeded", "servingLimitExceeded", "limitExceeded", "variableTermLimitExceeded",
			"variableTermExpiredDailyExceeded":
			if status == http.StatusForbidden {
				return faultQuota
			}
		case "forbidden":
			if status == http.StatusForbidden {
				result = faultProperty
			}
		case "notFound":
			if status == http.StatusNotFound {
				result = faultProperty
			}
		}
	}
	return result
}
