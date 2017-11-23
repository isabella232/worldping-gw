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
clean:
	rm -rf ./build
all: test build docker
