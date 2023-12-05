validate_version:
ifndef VERSION
	$(error VERSION is undefined)
endif

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