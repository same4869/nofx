package mcp

import "net/http"

const (
	ProviderLinkAI       = "linkai"
	DefaultLinkAIBaseURL = "https://api.link-ai.tech/v1"
	// LinkAI is an OpenAI-compatible gateway; use a commonly available default model as a sensible fallback.
	DefaultLinkAIModel = "deepseek-chat"
)

// LinkAIClient is an OpenAI-compatible client preset for LinkAI.
type LinkAIClient struct {
	*Client
}

func NewLinkAIClient() AIClient {
	return NewLinkAIClientWithOptions()
}

func NewLinkAIClientWithOptions(opts ...ClientOption) AIClient {
	linkaiOpts := []ClientOption{
		WithProvider(ProviderLinkAI),
		WithModel(DefaultLinkAIModel),
		WithBaseURL(DefaultLinkAIBaseURL),
	}

	allOpts := append(linkaiOpts, opts...)
	baseClient := NewClient(allOpts...).(*Client)

	linkaiClient := &LinkAIClient{
		Client: baseClient,
	}

	baseClient.hooks = linkaiClient
	return linkaiClient
}

func (c *LinkAIClient) SetAPIKey(apiKey string, customURL string, customModel string) {
	c.APIKey = apiKey

	if len(apiKey) > 8 {
		c.logger.Infof("🔧 [MCP] LinkAI API Key: %s...%s", apiKey[:4], apiKey[len(apiKey)-4:])
	}
	if customURL != "" {
		c.BaseURL = customURL
		c.logger.Infof("🔧 [MCP] LinkAI using custom BaseURL: %s", customURL)
	} else {
		c.logger.Infof("🔧 [MCP] LinkAI using default BaseURL: %s", c.BaseURL)
	}
	if customModel != "" {
		c.Model = customModel
		c.logger.Infof("🔧 [MCP] LinkAI using custom Model: %s", customModel)
	} else {
		c.logger.Infof("🔧 [MCP] LinkAI using default Model: %s", c.Model)
	}
}

// LinkAI uses standard Bearer auth (OpenAI-compatible).
func (c *LinkAIClient) setAuthHeader(reqHeaders http.Header) {
	c.Client.setAuthHeader(reqHeaders)
}

