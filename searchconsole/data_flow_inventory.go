package searchconsole

func (Property) searchProtocolFact()           {}
func (PageURL) searchProtocolFact()            {}
func (QueryText) searchProtocolFact()          {}
func (Dimensions) searchProtocolFact()         {}
func (DateRange) searchProtocolFact()          {}
func (ExactFilters) searchProtocolFact()       {}
func (PageWindow) searchProtocolFact()         {}
func (QueryRequest) searchProtocolFact()       {}
func (SiteRequest) searchProtocolFact()        {}
func (Site) searchSealedProjection()           {}
func (Row) searchSealedProjection()            {}
func (QueryPage) searchSealedProjection()      {}
func (Client) searchCapability()               {}
func (Server) searchCapability()               {}
func (Operation[R, S]) searchCapability()      {}
func (ServiceAccountSource) searchCapability() {}
func (queryWire) searchInternalFlow()          {}
func (filterGroup) searchInternalFlow()        {}
func (filterWire) searchInternalFlow()         {}
func (pageWire) searchInternalFlow()           {}
func (metadataWire) searchInternalFlow()       {}
func (rowWire) searchInternalFlow()            {}
func (siteWire) searchInternalFlow()           {}
func (rowKey) searchInternalFlow()             {}
func (providerRead) searchInternalFlow()       {}
func (responseBuffer) searchInternalFlow()     {}
func (faultWire) searchInternalFlow()          {}
func (faultDetail) searchInternalFlow()        {}
func (faultReason) searchInternalFlow()        {}
