//go:generate go tool github.com/ogen-go/ogen/cmd/ogen --target . -package realtek --clean openapi.yaml

package realtek

// DefaultBaseURL is the public API base URL for Realtek.
const DefaultBaseURL = "https://recruit.realtek.com"
