SOURCES ?= $(shell find . -path "./vendor" -prune -o -type f -name "*.go" -print)

TEST_DB_PORT := 3100
TBLS_VERSION := 1.38.3
SPECTRAL_VERSION := 5.4.0

traQ: $(SOURCES)
	CGO_ENABLED=0 go build -o traQ -ldflags "-s -w -X main.version=Dev -X main.revision=Local"

.PHONY: init
init:
	go mod download
	go install github.com/google/wire/cmd/wire
	go install github.com/golang/mock/mockgen

.PHONY: genkey
genkey:
	mkdir -p ./dev/keys
	cd ./dev/keys && go run ../bin/gen_ec_pem.go

.PHONY: test
test:
	MARIADB_PORT=$(TEST_DB_PORT) go test ./... -race

.PHONY: up-test-db
up-test-db:
	@TEST_DB_PORT=$(TEST_DB_PORT) ./dev/bin/up-test-db.sh

.PHONY: rm-test-db
rm-test-db:
	@./dev/bin/down-test-db.sh

.PHONY: lint
lint:
	-@make golangci-lint
	-@make swagger-lint

.PHONY: golangci-lint
golangci-lint:
	@golangci-lint run

.PHONY: swagger-lint
swagger-lint:
	@docker run --rm -it -v $$PWD:/tmp stoplight/spectral:$(SPECTRAL_VERSION) lint -r /tmp/.spectral.yml -q /tmp/docs/v3-api.yaml

.PHONY: db-gen-docs
db-gen-docs:
	@if [ -d "./docs/dbschema" ]; then \
		rm -r ./docs/dbschema; \
	fi
	TRAQ_MARIADB_PORT=$(TEST_DB_PORT) go run main.go migrate --reset
	docker run --rm --net=host -e TBLS_DSN="mysql://root:password@127.0.0.1:$(TEST_DB_PORT)/traq" -v $$PWD:/work k1low/tbls:$(TBLS_VERSION) doc

.PHONY: db-diff-docs
db-diff-docs:
	TRAQ_MARIADB_PORT=$(TEST_DB_PORT) go run main.go migrate --reset
	docker run --rm --net=host -e TBLS_DSN="mysql://root:password@127.0.0.1:$(TEST_DB_PORT)/traq" -v $$PWD:/work k1low/tbls:$(TBLS_VERSION) diff

.PHONY: db-lint
db-lint:
	TRAQ_MARIADB_PORT=$(TEST_DB_PORT) go run main.go migrate --reset
	docker run --rm --net=host -e TBLS_DSN="mysql://root:password@127.0.0.1:$(TEST_DB_PORT)/traq" -v $$PWD:/work k1low/tbls:$(TBLS_VERSION) lint

.PHONY: goreleaser-snapshot
goreleaser-snapshot:
	@docker run --rm -it -v $$PWD:/src -w /src goreleaser/goreleaser --snapshot --skip-publish --rm-dist

.PHONY: update-frontend
update-frontend:
	@mkdir -p ./dev/frontend
	@curl -L -Ss https://github.com/laminne/traQ_S-UI/releases/latest/download/dist.tar.gz | tar zxv -C ./dev/frontend/ --strip-components=2

.PHONY: reset-frontend
reset-frontend:
	@if [ -d "./dev/frontend" ]; then \
		rm -r ./dev/frontend; \
	fi
	@make update-frontend

.PHONY: up
up:
	@docker-compose up -d --build

.PHONY: down
down:
	@docker-compose down -v

.PHONY: gogen
gogen:
	go generate ./...
