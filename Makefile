%.pb.go: %.proto
	protoc -I=./ --go_out=. --go-grpc_out=. --go_opt=paths=source_relative $*.proto

.PHONY: proto
proto: progress.pb.go
