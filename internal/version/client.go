package version

// Product identifies this CLI in the X-Tollbit-Client request header; backend
// version checks key off it, so treat it as a stable contract.
const Product = "search-cli"

// HTTPProduct identifies this CLI in the HTTP User-Agent request header.
const HTTPProduct = "Tollbit-CLI"

// ClientHeader returns the X-Tollbit-Client value ("<product>/<semver>").
func ClientHeader() string {
	return Product + "/" + Version
}

// HTTPUserAgent returns the HTTP User-Agent value ("<product>/v<semver>").
func HTTPUserAgent() string {
	return HTTPProduct + "/v" + Version
}
