build:
	@go build -o ./bin/elk-alerts

run: build
	./bin/elk-alerts
