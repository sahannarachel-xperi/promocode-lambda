FUNCTION_NAME := promocode-lambda

export FUNCTION_NAME

ifndef INSTANCE_NAME
INSTANCE_NAME := ${USER}
endif
export INSTANCE_NAME

ifndef IMAGE
IMAGE=docker.tivo.com/inception-serverless-docker-build:latest
endif
export IMAGE

export USE_OPENAPI_GENERATOR=true
export USE_OPENAPI_GENERATOR_GO=true

# this project is arm'ed
ifndef ARM
ARM=y
endif
export ARM

default:

# Don't remake the make file.
Makefile: ;

.PHONY: FORCE
FORCE:

%:: FORCE
ifndef SKIP_PULL
	docker pull ${IMAGE} 1>/dev/null
endif
	docker run --rm ${IMAGE} get-run-script > run.sh
	chmod +x run.sh
	./run.sh $@
