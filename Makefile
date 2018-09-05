build:
	go build -i -o ./build/main ./main/main.go

clean:
	rm -rf ./build

test:
	go test -v -covermode=atomic ./...