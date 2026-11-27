package searchconsole

import "github.com/deliri/primitive/v2026/core"

type Dimension uint8

const (
	DimensionInvalid Dimension = iota
	DimensionDate
	DimensionPage
	DimensionQuery
)

func (Dimension) OffWireEnum()    {}
func (v Dimension) IsValid() bool { return v >= DimensionDate && v <= DimensionQuery }
func (v Dimension) Validate() error {
	if !v.IsValid() {
		return core.ErrSearchConsoleContract
	}
	return nil
}
func (v Dimension) String() string {
	if !v.IsValid() {
		return ""
	}
	return [...]string{"", "date", "page", "query"}[v]
}

type SearchType uint8

const (
	SearchTypeInvalid SearchType = iota
	SearchTypeWeb
)

func (SearchType) OffWireEnum()    {}
func (v SearchType) IsValid() bool { return v >= SearchTypeWeb && v <= SearchTypeWeb }
func (v SearchType) Validate() error {
	if !v.IsValid() {
		return core.ErrSearchConsoleContract
	}
	return nil
}
func (v SearchType) String() string {
	if !v.IsValid() {
		return ""
	}
	return [...]string{"", "web"}[v]
}

type Aggregation uint8

const (
	AggregationInvalid Aggregation = iota
	AggregationAuto
	AggregationPage
	AggregationProperty
)

func (Aggregation) OffWireEnum()    {}
func (v Aggregation) IsValid() bool { return v >= AggregationAuto && v <= AggregationProperty }
func (v Aggregation) Validate() error {
	if !v.IsValid() {
		return core.ErrSearchConsoleContract
	}
	return nil
}
func (v Aggregation) String() string {
	if !v.IsValid() {
		return ""
	}
	return [...]string{"", "auto", "byPage", "byProperty"}[v]
}

type DataState uint8

const (
	DataInvalid DataState = iota
	DataFinal
	DataAll
)

func (DataState) OffWireEnum()    {}
func (v DataState) IsValid() bool { return v >= DataFinal && v <= DataAll }
func (v DataState) Validate() error {
	if !v.IsValid() {
		return core.ErrSearchConsoleContract
	}
	return nil
}
func (v DataState) String() string {
	if !v.IsValid() {
		return ""
	}
	return [...]string{"", "final", "all"}[v]
}

type Permission uint8

const (
	PermissionInvalid Permission = iota
	PermissionRestricted
	PermissionFull
	PermissionOwner
	PermissionUnverified
)

func (Permission) OffWireEnum()    {}
func (v Permission) IsValid() bool { return v >= PermissionRestricted && v <= PermissionUnverified }
func (v Permission) Validate() error {
	if !v.IsValid() {
		return core.ErrSearchConsoleContract
	}
	return nil
}
func (v Permission) String() string {
	if !v.IsValid() {
		return ""
	}
	return [...]string{"", "siteRestrictedUser", "siteFullUser", "siteOwner", "siteUnverifiedUser"}[v]
}

type FilterOperator uint8

const (
	FilterInvalid FilterOperator = iota
	FilterEquals
)

func (FilterOperator) OffWireEnum()    {}
func (v FilterOperator) IsValid() bool { return v >= FilterEquals && v <= FilterEquals }
func (v FilterOperator) Validate() error {
	if !v.IsValid() {
		return core.ErrSearchConsoleContract
	}
	return nil
}
func (v FilterOperator) String() string {
	if !v.IsValid() {
		return ""
	}
	return [...]string{"", "equals"}[v]
}
