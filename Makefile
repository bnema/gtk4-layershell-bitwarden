.PHONY: test race lint safety check

test:
	go test ./...

race:
	go test -race ./...

lint:
	golangci-lint run ./...

EXCLUDE_DISK_HELPERS := internal/adapters/fileutil/atomic.go
GREP_EXCLUDE := $(foreach p,$(EXCLUDE_DISK_HELPERS),| grep -v '$(p)')

safety:
	@if grep -RInE --exclude=Makefile --exclude='*_test.go' --exclude-dir=.git 'DumpRequest|DumpResponse|httputil|access_token|refresh_token|password=' .; then \
		echo "unsafe secret/body-dump pattern found"; \
		exit 1; \
	fi
	@if [ -d internal ] && grep -RInE --exclude='*_test.go' 'os\.WriteFile|CreateTemp' internal $(GREP_EXCLUDE); then \
		echo "unexpected disk persistence pattern found"; \
		exit 1; \
	fi
	@if [ -d internal ] && grep -RInE --exclude='*_test.go' 'MkdirAll' internal $(GREP_EXCLUDE); then \
		echo "unexpected directory creation outside cache/config adapters"; \
		exit 1; \
	fi

check: test lint safety
