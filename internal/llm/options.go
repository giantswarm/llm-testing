package llm

// clientConfig holds configuration for an LLM client.
type clientConfig struct {
	baseURL string
	apiKey  string
}

// Option is a functional option for configuring an LLM client.
type Option func(*clientConfig)

// WithBaseURL sets the base URL for the API.
func WithBaseURL(url string) Option {
	return func(c *clientConfig) {
		c.baseURL = url
	}
}

// WithAPIKey sets the API key.
func WithAPIKey(key string) Option {
	return func(c *clientConfig) {
		c.apiKey = key
	}
}
