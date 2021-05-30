module github.com/bluele/ibc-multisig-client

go 1.15

replace github.com/gogo/protobuf => github.com/regen-network/protobuf v1.3.3-alpha.regen.1

require (
	github.com/confio/ics23/go v0.6.3
	github.com/cosmos/cosmos-sdk v0.42.0-alpha1.0.20210301172302-05ce78935a9b
	github.com/cosmos/ibc-go v0.0.0-20210330211706-07b6a97b67d1
	github.com/gogo/protobuf v1.3.3
	github.com/gorilla/mux v1.8.0
	github.com/grpc-ecosystem/grpc-gateway v1.16.0
	github.com/spf13/cobra v1.1.3
)
