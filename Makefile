default:
	$(MAKE) all
test:
	bash -c "./scripts/test.sh"
build:
	bash -c "./scripts/build.sh"
docker:
	bash -c "./scripts/build_docker.sh"
deploy:
	bash -c "./scripts/deploy.sh"
all: test build docker
