package searchconsole

import (
	"errors"
	"math"
	"net"
	"strings"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
)

const (
	propertyPrefix  = "sc-domain:"
	dnsMaximum      = 253
	labelMaximum    = 63
	filterMaximum   = 4096
	responseMaximum = 2 * 1024 * 1024
	PageMaximum     = 2001
	OffsetMaximum   = math.MaxInt32
)

type Property struct{ domain string }

func ParseProperty(text string) (Property, error) {
	host, ok := strings.CutPrefix(text, propertyPrefix)
	if !ok || len(host) > dnsMaximum || !strings.Contains(host, ".") || net.ParseIP(host) != nil {
		return Property{}, core.ErrSearchConsoleContract
	}
	for label := range strings.SplitSeq(host, ".") {
		if len(label) == 0 || len(label) > labelMaximum || label[0] == '-' || label[len(label)-1] == '-' {
			return Property{}, core.ErrSearchConsoleContract
		}
		for _, b := range label {
			if !(b >= 'a' && b <= 'z' || b >= '0' && b <= '9' || b == '-') {
				return Property{}, core.ErrSearchConsoleContract
			}
		}
	}
	return Property{domain: host}, nil
}

func (p Property) String() string {
	if p.domain == "" {
		return ""
	}
	return propertyPrefix + p.domain
}
func (p Property) Validate() error {
	_, err := ParseProperty(p.String())
	return err
}

type PageURL struct{ text string }

func ParsePageURL(text string) (PageURL, error) {
	if !validFilterText(text) {
		return PageURL{}, core.ErrSearchConsoleContract
	}
	if _, err := core.ParseHTTPEndpoint(text); err != nil {
		return PageURL{}, errors.Join(core.ErrSearchConsoleContract, err)
	}
	return PageURL{text: text}, nil
}
func (p PageURL) String() string { return p.text }
func (p PageURL) Validate() error {
	_, err := ParsePageURL(p.String())
	return err
}

type QueryText struct{ text string }

func ParseQueryText(text string) (QueryText, error) {
	if !validFilterText(text) {
		return QueryText{}, core.ErrSearchConsoleContract
	}
	return QueryText{text: text}, nil
}
func (q QueryText) String() string { return q.text }
func (q QueryText) Validate() error {
	_, err := ParseQueryText(q.String())
	return err
}

func validFilterText(s string) bool {
	if s == "" || len(s) > filterMaximum || !utf8.ValidString(s) {
		return false
	}
	for _, r := range s {
		if r < ' ' || r == 127 {
			return false
		}
	}
	return true
}

type Dimensions struct{ First, Second Dimension }

func (d Dimensions) Validate() error {
	if d.Second == DimensionInvalid {
		if d.First == DimensionInvalid {
			return nil
		}
		return d.First.Validate()
	}
	if d.First == DimensionPage && d.Second == DimensionQuery || d.First == DimensionQuery && d.Second == DimensionPage {
		return nil
	}
	return core.ErrSearchConsoleContract
}

func (d Dimensions) has(want Dimension) bool { return d.First == want || d.Second == want }

func (d Dimensions) count() int {
	if d.Second != DimensionInvalid {
		return 2
	}
	if d.First != DimensionInvalid {
		return 1
	}
	return 0
}

type DateRange struct{ Start, End temporal.CivilDate }

func (d DateRange) Validate() error {
	first, firstErr := d.Start.Text()
	last, lastErr := d.End.Text()
	if firstErr != nil || lastErr != nil || first > last {
		return errors.Join(core.ErrSearchConsoleContract, firstErr, lastErr)
	}
	return nil
}

func (d DateRange) contains(value temporal.CivilDate) bool {
	at, err := value.Text()
	first, firstErr := d.Start.Text()
	last, lastErr := d.End.Text()
	return err == nil && firstErr == nil && lastErr == nil && first <= at && at <= last
}

// ExactFilters applies case-sensitive equality to either or both dimensions.
type ExactFilters struct {
	Page  PageURL
	Query QueryText
}

func (f ExactFilters) Validate() error {
	var pageErr, queryErr error
	if f.Page != (PageURL{}) {
		pageErr = f.Page.Validate()
	}
	if f.Query != (QueryText{}) {
		queryErr = f.Query.Validate()
	}
	return errors.Join(pageErr, queryErr)
}

type PageWindow struct {
	Limit  uint16
	Offset uint32
}

func (p PageWindow) Validate() error {
	if p.Limit < 1 || p.Limit > PageMaximum || p.Offset > OffsetMaximum {
		return core.ErrSearchConsoleContract
	}
	return nil
}

type QueryRequest struct {
	Property    Property
	Dates       DateRange
	Dimensions  Dimensions
	Type        SearchType
	Aggregation Aggregation
	State       DataState
	Filters     ExactFilters
	Page        PageWindow
	Policy      exchange.OperationPolicy
}

func (r QueryRequest) Validate() error {
	if r.Aggregation != AggregationAuto && r.pageContext() ||
		r.Dimensions.has(DimensionDate) && r.Filters != (ExactFilters{}) {
		return core.ErrSearchConsoleContract
	}
	return errors.Join(r.Property.Validate(), r.Dates.Validate(), r.Dimensions.Validate(), r.Type.Validate(),
		r.Aggregation.Validate(), r.State.Validate(), r.Filters.Validate(), r.Page.Validate(), validatePolicy(r.Policy))
}

func (r QueryRequest) pageContext() bool {
	return r.Dimensions.has(DimensionPage) || r.Filters.Page != (PageURL{})
}

type SiteRequest struct {
	Property Property
	Policy   exchange.OperationPolicy
}

func (r SiteRequest) Validate() error {
	return errors.Join(r.Property.Validate(), validatePolicy(r.Policy))
}

func validatePolicy(p exchange.OperationPolicy) error {
	if p.Retry.MaximumAttempts != 1 || p.Redirect.Mode != exchange.RedirectReject {
		return core.ErrSearchConsoleContract
	}
	if err := p.Validate(); err != nil {
		return errors.Join(core.ErrSearchConsoleContract, err)
	}
	return nil
}

type Site struct {
	Property   Property
	Permission Permission
}

func (s Site) Validate() error { return errors.Join(s.Property.Validate(), s.Permission.Validate()) }

type Clicks float64
type Impressions float64
type ClickRate float64
type Position float64

func (c Clicks) Validate() error      { return nonnegative(float64(c)) }
func (i Impressions) Validate() error { return nonnegative(float64(i)) }
func (p Position) Validate() error {
	if p > 0 && p < 1 {
		return core.ErrSearchConsoleResponse
	}
	return nonnegative(float64(p))
}
func (r ClickRate) Validate() error {
	if r > 1 {
		return core.ErrSearchConsoleResponse
	}
	return nonnegative(float64(r))
}
func nonnegative(v float64) error {
	if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 {
		return core.ErrSearchConsoleResponse
	}
	return nil
}

type Row struct {
	Date        temporal.CivilDate
	Page        PageURL
	Query       QueryText
	Clicks      Clicks
	Impressions Impressions
	Rate        ClickRate
	Position    Position
}

func (r Row) Validate() error {
	if r.Impressions > 0 && r.Position == 0 {
		return core.ErrSearchConsoleResponse
	}
	return errors.Join(r.Clicks.Validate(), r.Impressions.Validate(), r.Rate.Validate(), r.Position.Validate())
}

type QueryPage struct {
	Rows            []Row
	Dates           DateRange
	Dimensions      Dimensions
	State           DataState
	Aggregation     Aggregation
	FirstIncomplete temporal.CivilDate
	NextOffset      uint32
}

func (p QueryPage) Validate() error {
	if len(p.Rows) > PageMaximum || p.NextOffset > OffsetMaximum ||
		(p.FirstIncomplete != (temporal.CivilDate{}) &&
			(p.State != DataAll || !p.Dimensions.has(DimensionDate) || !p.Dates.contains(p.FirstIncomplete))) {
		return core.ErrSearchConsoleResponse
	}
	var aggregationErr error
	if p.Aggregation != AggregationInvalid || len(p.Rows) > 0 {
		aggregationErr = p.Aggregation.Validate()
	}
	if err := errors.Join(p.Dates.Validate(), p.Dimensions.Validate(), p.State.Validate(), aggregationErr); err != nil {
		return err
	}
	for _, row := range p.Rows {
		if err := row.Validate(); err != nil {
			return err
		}
		if err := p.validateRowKeys(row); err != nil {
			return err
		}
	}
	if !uniqueRowKeys(p.Rows) {
		return core.ErrSearchConsoleResponse
	}
	return nil
}

func (p QueryPage) validateRowKeys(row Row) error {
	if p.Dimensions.has(DimensionDate) != (row.Date != (temporal.CivilDate{})) ||
		p.Dimensions.has(DimensionPage) != (row.Page != (PageURL{})) ||
		p.Dimensions.has(DimensionQuery) != (row.Query != (QueryText{})) ||
		row.Date != (temporal.CivilDate{}) && !p.Dates.contains(row.Date) {
		return core.ErrSearchConsoleResponse
	}
	if row.Page != (PageURL{}) && row.Page.Validate() != nil || row.Query != (QueryText{}) && row.Query.Validate() != nil {
		return core.ErrSearchConsoleResponse
	}
	return nil
}

// FinalDate checks observed finality for one date in an unfiltered date probe.
// A missing date or an all-data observation without a cutoff is unknown.
func (p QueryPage) FinalDate(day temporal.CivilDate) error {
	if !p.Dimensions.has(DimensionDate) || !p.Dates.contains(day) {
		return core.ErrSearchConsoleContract
	}
	dayText, _ := day.Text()
	cutoff, _ := p.FirstIncomplete.Text()
	if cutoff != "" && dayText >= cutoff {
		return core.ErrSearchConsoleNotReady
	}
	for _, row := range p.Rows {
		if row.Date == day && (p.State == DataFinal || cutoff != "") {
			return nil
		}
	}
	return core.ErrSearchConsoleCoverageUnknown
}
