// Copyright (c) 2026 thorsphere.
// All Rights Reserved. Use is governed by the Functional Source License v1.1
// (FSL-1.1-ALv2) that can be found in the LICENSE file.

package tscommit

type config struct {
	apiKey string
	model  string
}

const (
	defaultModel = "tencent/hy4-preview"
)

func DefaultConfig() *config {
	return &config{model: defaultModel}
}

func (cfg *config) withAPIKey(apiKey string) *config {
	cfg.apiKey = apiKey
	return cfg
}

func (cfg *config) withModel(model string) *config {
	cfg.model = model
	return cfg
}
