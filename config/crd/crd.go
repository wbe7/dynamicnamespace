package crd

import _ "embed"

var (
	//go:embed bases/platform.cloudnative.space_dynamicnamespaces.yaml
	DynamicNamespace []byte
)
