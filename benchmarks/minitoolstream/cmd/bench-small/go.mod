module bench-small

go 1.25.5

require (
	benchmarks/minitoolstream/pkg/metrics v0.0.0
	github.com/moroshma/MiniToolStreamConnector/minitoolstream_connector v0.1.3
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/moroshma/MiniToolStreamConnector/model v0.1.4 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/prometheus/client_golang v1.23.2 // indirect
	github.com/prometheus/client_model v0.6.2 // indirect
	github.com/prometheus/common v0.66.1 // indirect
	github.com/prometheus/procfs v0.16.1 // indirect
	go.yaml.in/yaml/v2 v2.4.2 // indirect
	golang.org/x/net v0.48.0 // indirect
	golang.org/x/sys v0.39.0 // indirect
	golang.org/x/text v0.32.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251213004720-97cd9d5aeac2 // indirect
	google.golang.org/grpc v1.77.0 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
)

replace benchmarks/minitoolstream/pkg/metrics => ../../pkg/metrics

replace github.com/moroshma/MiniToolStreamConnector/minitoolstream_connector => /Users/moroshma/go/DiplomaThesis/MiniToolStreamConnector/minitoolstream_connector

replace github.com/moroshma/MiniToolStreamConnector/model => /Users/moroshma/go/DiplomaThesis/MiniToolStreamConnector/model
