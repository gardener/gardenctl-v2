package testdata

import "embed"

//go:embed templates azure gcp openstack test
var FS embed.FS
