%.pb.go: proto/%.proto
	protoc -I=./proto --go_out=. --go-grpc_out=. --go_opt=paths=source_relative proto/$*.proto

.PHONY: proto
proto: progress.pb.go
