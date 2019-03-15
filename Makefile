IMPORT_PATH=github.com/technicolor-research/pnyxdb

protoc:
	@protoc --go_out=. consensus/*.proto
	@protoc --go_out=Mconsensus/structures.proto=$(IMPORT_PATH)/consensus:. consensus/bbc/*.proto
	@protoc --go_out=plugins=grpc,Mconsensus/structures.proto=$(IMPORT_PATH)/consensus:. api/*.proto

test:
	go test -count 1 -p 1 ./...

lint:
	golangci-lint run
