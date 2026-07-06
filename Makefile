.PHONY: validate-openapi

OPENAPI_SPECS := \
	internal/provider/cake/openapi.yaml \
	internal/provider/google/openapi.yaml \
	internal/provider/job104/openapi.yaml \
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
