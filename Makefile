run: build
	./bin/server
	
build:
	go build -o bin/server ./server

clean:
	rm -rf bin

lint:
	golangci-lint run

tidy:
	go mod tidy

client: build_client
	./bin/client --port 3000
client2:
	./bin/client --port 3001

build_client:
	go build -o bin/client ./client
