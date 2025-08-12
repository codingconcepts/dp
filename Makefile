validate_version:
ifndef VERSION
	$(error VERSION is undefined)
endif

test:
	go test ./... -v -cover

build_and_push: validate_version
	- docker buildx create --name multiarch --use

	- docker buildx build \
		--platform linux/amd64,linux/arm64 \
		-t codingconcepts/dp:${VERSION} \
		--build-arg version=${VERSION} \
		--push \
		.

run: validate_version
	docker run --rm -it \
		codingconcepts/dp:${VERSION} \
			--port 26257 \
			--ctl-port 3000

release: validate_version
	# linux
	GOOS=linux go build -ldflags "-X main.version=${VERSION}" -o dp ;\
	tar -zcvf ./releases/dp_${VERSION}_linux.tar.gz ./dp ;\

	# macos (arm)
	GOOS=darwin GOARCH=arm64 go build -ldflags "-X main.version=${VERSION}" -o dp ;\
	tar -zcvf ./releases/dp_${VERSION}_macos_arm64.tar.gz ./dp ;\

	# macos (amd)
	GOOS=darwin GOARCH=amd64 go build -ldflags "-X main.version=${VERSION}" -o dp ;\
	tar -zcvf ./releases/dp_${VERSION}_macos_amd64.tar.gz ./dp ;\

	# windows
	GOOS=windows go build -ldflags "-X main.version=${VERSION}" -o dp ;\
	tar -zcvf ./releases/dp_${VERSION}_windows.tar.gz ./dp ;\

	rm ./dp
	open releases