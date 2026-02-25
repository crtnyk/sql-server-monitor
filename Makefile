build:
	go build -o bin/monitor cmd/monitor/main.go

build-windows:
	GOOS=windows GOARCH=amd64 go build -o bin/monitor.exe cmd/monitor/main.go

clean:
	rm -rf bin/
