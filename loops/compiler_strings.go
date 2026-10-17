package loops

// Provider wire spellings owned by this package. Query names and list values
// cross only through the typed Route projection.
const (
	apiPathPrefix          = "/api/v1/"
	webhookSecretPrefix    = "whsec_"
	queryEmail             = "email"
	queryUserID            = "userId"
	queryPropertyList      = "list"
	queryPerPage           = "perPage"
	queryCursor            = "cursor"
	propertyListAllText    = "all"
	propertyListCustomText = "custom"
	queryFieldSeparator    = "&"
	queryValueSeparator    = "="
)
