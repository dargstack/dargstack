package compose

// Error prefixes shared across compose operations.
const (
	ErrParseComposeForFiltering = "parse compose for filtering"
	ErrNoServicesSection        = "compose file has no services section"
	ErrSerializeFilteredCompose = "serialize filtered compose"
	ErrParseCompose             = "parse compose"
)
