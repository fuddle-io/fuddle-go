module github.com/fuddle-io/fuddle-go

go 1.20

require (
	github.com/fuddle-io/fuddle-rpc/go v0.0.0-20230322065350-85501b751765
	github.com/google/uuid v1.3.0
	github.com/hashicorp/go-multierror v1.1.1
	github.com/stretchr/testify v1.8.2
	google.golang.org/grpc v1.53.0
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/hashicorp/errwrap v1.0.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	golang.org/x/net v0.5.0 // indirect
	golang.org/x/sys v0.4.0 // indirect
	golang.org/x/text v0.6.0 // indirect
	google.golang.org/genproto v0.0.0-20230110181048-76db0878b65f // indirect
	google.golang.org/protobuf v1.30.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/fuddle-io/fuddle-rpc/go v0.0.0-20230321084119-bb863b3c13f6 => ../fuddle-rpc/go
