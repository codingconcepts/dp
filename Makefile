validate_version:
ifndef VERSION
	$(error VERSION is undefined)
endif

build: validate_version
	docker build -t codingconcepts/dp:${VERSION} \
		--build-arg version=${VERSION} \
		--no-cache .

run: validate_version
	docker run --rm -it \
		codingconcepts/dp:${VERSION} \
			--server "localhost:26001" \
			--server "localhost:26002" \
			--port 26257 \
			--ctl-port 3000 \
			--debug

push: validate_version
	docker push codingconcepts/dp:${VERSION}

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