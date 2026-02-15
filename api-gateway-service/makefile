# ----------------------------
# Variables
# ----------------------------
PROTO_DIR=proto
PROTO_FILE=$(PROTO_DIR)/shipment.proto

GO_OUT=.
GRPC_OUT=.

BINARY_NAME=shipment-service

.PHONY: proto clean-proto build run test clean

# ----------------------------
# Protobuf Generation
# ----------------------------
proto:
	protoc \
	--go_out=$(GO_OUT) \
	--go-grpc_out=$(GRPC_OUT) \
	$(PROTO_FILE)

# ----------------------------
# Clean Generated Files
# ----------------------------
clean-proto:
	rm -f $(PROTO_DIR)/*.pb.go

# ----------------------------
# Build Binary
# ----------------------------
build:
	go build -o bin/$(BINARY_NAME) main.go

# ----------------------------
# Run Service
# ----------------------------
run:
	go run cmd/main.go

# ----------------------------
# Test
# ----------------------------
test:
	go test ./...

# ----------------------------
# Clean Build
# ----------------------------
clean:
	rm -rf bin/