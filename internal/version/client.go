package version

// Product identifies this CLI in the X-Tollbit-Client request header; backend
// version checks key off it, so treat it as a stable contract.
const Product = "search-cli"

// ClientHeader returns the X-Tollbit-Client value ("<product>/<semver>").
func ClientHeader() string {
	return Product + "/" + Version
}
