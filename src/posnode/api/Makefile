export MOCKGEN?=mockgen
export PROTOC?=protoc
export RM?=rm

PROTOS:=$(wildcard *proto)
PBGOS:=$(PROTOS:.proto=.pb.go)
INTERWIRE:=../../inter/wire

.PHONY: build clean proto buildtests

clean:
	$(RM) -rf *.pb.go

proto: $(PROTOS)
	$(PROTOC) --proto_path=. -I "$(INTERWIRE)" --proto_path="$(GOPATH)/src" --go_out=plugins=grpc:. $(PROTOS)

build: mock.go

buildtests: mock.go

mock.go: service.pb.go
	$(MOCKGEN) -package=api -source=service.pb.go -destination=mock.go NodeServer
