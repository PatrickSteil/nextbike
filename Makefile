BINARY  := nextbike
CMD     := ./cmd/nextbike
DB      := nextbike.db

.PHONY: all build run tidy clean reset db

all: build

build:
	go build -o $(BINARY) $(CMD)

run: build
	./$(BINARY)

tidy:
	go mod tidy

clean:
	rm -f $(BINARY)

reset: clean
	rm -f $(DB)

db:
	sqlite3 $(DB) "SELECT uid, name, bikes_available_to_rent, updated_at FROM stations ORDER BY bikes_available_to_rent DESC LIMIT 20;"

curl-example:
	curl -s http://localhost:8080/stations/19153166 | jq .
