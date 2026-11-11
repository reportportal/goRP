// Separate module on purpose: keeps github.com/onsi/ginkgo/v2 (and its transitive
// dependencies) out of the main goRP module, so users of pkg/gorp don't pull in
// a test framework they don't use.
module github.com/reportportal/goRP/pkg/ginkgo

go 1.25.0

require (
	github.com/google/uuid v1.6.0
	github.com/onsi/ginkgo/v2 v2.26.0
	github.com/onsi/gomega v1.38.2
	github.com/reportportal/goRP/v5 v5.0.0
)

require (
	github.com/Masterminds/semver/v3 v3.4.0 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-task/slim-sprig/v3 v3.0.0 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/google/pprof v0.0.0-20250403155104-27863c87afa6 // indirect
	go.uber.org/automaxprocs v1.6.0 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/mod v0.36.0 // indirect
	golang.org/x/net v0.56.0 // indirect
	golang.org/x/oauth2 v0.36.0 // indirect
	golang.org/x/sync v0.21.0 // indirect
	golang.org/x/sys v0.46.0 // indirect
	golang.org/x/text v0.38.0 // indirect
	golang.org/x/tools v0.45.0 // indirect
	resty.dev/v3 v3.0.0-rc.2 // indirect
)

// Local development: resolve goRP from this repository checkout.
replace github.com/reportportal/goRP/v5 => ../..
