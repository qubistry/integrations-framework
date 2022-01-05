package client

// ExternalAdapter external adapter
type ExternalAdapter struct {
	Config *ExternalAdapterConfig
}

// ExternalAdapterConfig holds config information for ExternalAdapter
type ExternalAdapterConfig struct {
	LocalURL   string
	ClusterURL string
}

// NewExternalAdapter returns an external adapter
func NewExternalAdapter(cfg *ExternalAdapterConfig) *ExternalAdapter {
	return &ExternalAdapter{
		Config: cfg,
	}
}
