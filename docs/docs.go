package docs

import (
	_ "embed"
)

// OpenAPISpec berisi seluruh definisi spesifikasi OpenAPI 3.0 Stoodify
// yang di-embed langsung ke dalam binary Go.
//
//go:embed openapi.yaml
var OpenAPISpec []byte
