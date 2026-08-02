module github.com/dhcgn/paperless-ngx-privatemode-ai

go 1.26.0

// privatemode-public re-exports the internal continuum module under a different repo name.
replace github.com/edgelesssys/continuum => github.com/edgelesssys/privatemode-public v1.50.0

// contrast (pulled in via privatemode-proxy) expects these forks; replace directives in a
// dependency's own go.mod are not inherited, so they must be repeated here.
replace github.com/google/go-sev-guest => github.com/edgelesssys/go-sev-guest v0.0.0-20260625102846-50db367a5686

replace github.com/google/go-tdx-guest => github.com/edgelesssys/go-tdx-guest v0.0.0-20260625102850-ea481d3db249

require (
	github.com/edgelesssys/continuum v1.50.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	filippo.io/nistec v0.0.4 // indirect
	github.com/avast/retry-go/v5 v5.0.0 // indirect
	github.com/edgelesssys/contrast v1.22.0 // indirect
	github.com/evanphx/json-patch/v5 v5.9.11 // indirect
	github.com/google/go-sev-guest v0.14.2-0.20251119154202-af1c107a648f // indirect
	github.com/google/go-tdx-guest v0.3.2-0.20260104162950-32866d7a678f // indirect
	github.com/google/logger v1.1.2 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/mdlayher/socket v0.6.1 // indirect
	github.com/mdlayher/vsock v1.2.1 // indirect
	github.com/spf13/afero v1.15.0 // indirect
	github.com/spf13/cobra v1.10.2 // indirect
	github.com/spf13/pflag v1.0.10 // indirect
	github.com/tidwall/gjson v1.19.0 // indirect
	github.com/tidwall/match v1.1.1 // indirect
	github.com/tidwall/pretty v1.2.1 // indirect
	github.com/tidwall/sjson v1.2.5 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/crypto v0.54.0 // indirect
	golang.org/x/exp v0.0.0-20260410095643-746e56fc9e2f // indirect
	golang.org/x/mod v0.38.0 // indirect
	golang.org/x/net v0.57.0 // indirect
	golang.org/x/sync v0.22.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260706201446-f0a921348800 // indirect
	google.golang.org/grpc v1.82.1 // indirect
	google.golang.org/protobuf v1.36.12-0.20260120151049-f2248ac996af // indirect
	gopkg.in/natefinch/lumberjack.v2 v2.2.1 // indirect
	k8s.io/utils v0.0.0-20260707023825-cf1189d6abe3 // indirect
)

require (
	atomicgo.dev/cursor v0.2.0 // indirect
	atomicgo.dev/keyboard v0.2.9 // indirect
	atomicgo.dev/schedule v0.1.0 // indirect
	github.com/containerd/console v1.0.3 // indirect
	github.com/gookit/color v1.5.4 // indirect
	github.com/lithammer/fuzzysearch v1.1.8 // indirect
	github.com/mattn/go-runewidth v0.0.16 // indirect
	github.com/pterm/pterm v0.12.80
	github.com/rivo/uniseg v0.4.4 // indirect
	github.com/xo/terminfo v0.0.0-20220910002029-abceb7e1c41e // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/term v0.45.0 // indirect
	golang.org/x/text v0.40.0 // indirect
)
