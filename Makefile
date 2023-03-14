.PHONY: build
build:
	go build "./cmd/xcm"

.PHONY: install
install:
	rm -f ${GOPATH}/bin/xcm
	go install ./cmd/xcm

.PHONY: clean
clean:
	rm -rf \
		$$(ls cmd) \
		$(NULL)
