package timematch

// Every production struct declares one data-flow role. The architecture test
// refuses an unclassified struct.
type protocolFact interface{ timematchProtocolFact() }
type internalFlow interface{ timematchInternalFlow() }
type capabilityWrapper interface{ timematchCapabilityWrapper() }

// Public typed SDK facts crossing the Primitive/Kernel Bridge wall.
func (Application) timematchProtocolFact()          {}
func (AuthorizationRequest) timematchProtocolFact() {}
func (CodeExchange) timematchProtocolFact()         {}
func (RefreshRequest) timematchProtocolFact()       {}
func (RevokeRequest) timematchProtocolFact()        {}
func (CalendarID) timematchProtocolFact()           {}
func (Calendar) timematchProtocolFact()             {}
func (CalendarListRequest) timematchProtocolFact()  {}
func (CalendarList) timematchProtocolFact()         {}
func (AvailabilityRequest) timematchProtocolFact()  {}
func (CalendarAvailability) timematchProtocolFact() {}
func (Availability) timematchProtocolFact()         {}
func (EventID) timematchProtocolFact()              {}
func (EventSummary) timematchProtocolFact()         {}
func (EventIntent) timematchProtocolFact()          {}
func (EventRef) timematchProtocolFact()             {}
func (EventReceipt) timematchProtocolFact()         {}
func (EventWrite) timematchProtocolFact()           {}
func (EventUpdate) timematchProtocolFact()          {}
func (EventDelete) timematchProtocolFact()          {}

// Credential and transport capabilities.
func (Secret) timematchCapabilityWrapper()       {}
func (AppleAccount) timematchCapabilityWrapper() {}
func (Grant) timematchCapabilityWrapper()        {}
func (Connection) timematchCapabilityWrapper()   {}
func (Client) timematchCapabilityWrapper()       {}

// Provider wire projections and internal execution carriers.
func (call) timematchInternalFlow()                   {}
func (outcome) timematchInternalFlow()                {}
func (formPair) timematchInternalFlow()               {}
func (tokenWire) timematchInternalFlow()              {}
func (wireSecret) timematchInternalFlow()             {}
func (googleCalendarPage) timematchInternalFlow()     {}
func (googleFreeBusyItem) timematchInternalFlow()     {}
func (googleFreeBusyBody) timematchInternalFlow()     {}
func (googleBusyCalendar) timematchInternalFlow()     {}
func (googleFreeBusyResponse) timematchInternalFlow() {}
func (googleEventTime) timematchInternalFlow()        {}
func (googleEventBody) timematchInternalFlow()        {}
func (googleEventObservation) timematchInternalFlow() {}
func (graphCalendarPage) timematchInternalFlow()      {}
func (graphDateTime) timematchInternalFlow()          {}
func (graphViewPage) timematchInternalFlow()          {}
func (graphEventBody) timematchInternalFlow()         {}
func (graphEventObservation) timematchInternalFlow()  {}
func (icsEvent) timematchInternalFlow()               {}
func (icsProperty) timematchInternalFlow()            {}
func (icsRawEvent) timematchInternalFlow()            {}
func (davMultistatusWire) timematchInternalFlow()     {}
func (davResponseWire) timematchInternalFlow()        {}
func (davPropstatWire) timematchInternalFlow()        {}
func (davHrefWire) timematchInternalFlow()            {}
func (davPresenceWire) timematchInternalFlow()        {}
func (davPrivilegeWire) timematchInternalFlow()       {}
func (davComponentWire) timematchInternalFlow()       {}
func (davPropWire) timematchInternalFlow()            {}

var (
	_ protocolFact      = Availability{}
	_ internalFlow      = call{}
	_ capabilityWrapper = Secret{}
)
