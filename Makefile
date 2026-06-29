.PHONY: ut validate-openapi

OPENAPI_SPECS := \
	internal/provider/cake/openapi.yaml \
	internal/provider/google/openapi.yaml \
	internal/provider/synopsys/openapi.yaml \
	internal/provider/tsmc/openapi.yaml

ut:
	go test -race -vet=all $(shell go list ./... | grep -vE '/cmd($|/)')

validate-openapi: $(OPENAPI_SPECS)
	@for spec in $^; do \
		echo "Validating $$spec..."; \
		docker run --rm -v "$(PWD)/$$spec:/openapi.yaml" pythonopenapi/openapi-spec-validator /openapi.yaml; \
	done
