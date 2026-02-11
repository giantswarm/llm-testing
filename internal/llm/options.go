package llm

// Float64Ptr returns a pointer to the given float64 value.
// Useful for constructing ChatRequest with an explicit temperature.
func Float64Ptr(v float64) *float64 {
	return &v
}

// clientConfig holds configuration for an LLM client.
type clientConfig struct {
	baseURL     string
	apiKey      string
	model       string
	temperature *float64
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

// WithModel sets the default model name for requests.
// Per-request model settings in ChatRequest take precedence.
func WithModel(model string) Option {
	return func(c *clientConfig) {
		c.model = model
	}
}

// WithTemperature sets the default temperature for requests.
// Per-request temperature settings in ChatRequest take precedence.
func WithTemperature(temp float64) Option {
	return func(c *clientConfig) {
		c.temperature = &temp
	}
}
