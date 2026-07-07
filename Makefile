.PHONY: validate-openapi hurl-fmt hurl-lint hurl-test

OPENAPI_SPECS := \
	internal/provider/cake/openapi.yaml \
	internal/provider/google/openapi.yaml \
	internal/provider/greenhouse/openapi.yaml \
	internal/provider/job104/openapi.yaml \
	internal/provider/lever/openapi.yaml \
	internal/provider/linkedin/openapi.yaml \
	internal/provider/nvidia/openapi.yaml \
	internal/provider/synopsys/openapi.yaml \
	internal/provider/tsmc/openapi.yaml \
	internal/provider/workday/openapi.yaml

validate-openapi: $(OPENAPI_SPECS)
	@for spec in $^; do \
		echo "Validating $$spec..."; \
		docker run --rm -v "$(PWD)/$$spec:/openapi.yaml" pythonopenapi/openapi-spec-validator /openapi.yaml; \
	done

HURL_FILES := $(shell find internal/provider -path '*/testdata/*.hurl')

hurl-fmt:
	@hurlfmt --in-place $(HURL_FILES)

hurl-lint:
	@hurlfmt --check $(HURL_FILES)

hurl-test:
	@hurl --test --max-time 30 $(HURL_FILES)
