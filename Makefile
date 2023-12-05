validate_version:
ifndef VERSION
	$(error VERSION is undefined)
endif

release: validate_version
	# linux
	GOOS=linux go build -ldflags "-X main.version=${VERSION}" -o lb ;\
	tar -zcvf ./releases/lb_${VERSION}_linux.tar.gz ./lb ;\

	# macos (arm)
	GOOS=darwin GOARCH=arm64 go build -ldflags "-X main.version=${VERSION}" -o lb ;\
	tar -zcvf ./releases/lb_${VERSION}_macos_arm64.tar.gz ./lb ;\

	# macos (amd)
	GOOS=darwin GOARCH=amd64 go build -ldflags "-X main.version=${VERSION}" -o lb ;\
	tar -zcvf ./releases/lb_${VERSION}_macos_amd64.tar.gz ./lb ;\

	# windows
	GOOS=windows go build -ldflags "-X main.version=${VERSION}" -o lb ;\
	tar -zcvf ./releases/lb_${VERSION}_windows.tar.gz ./lb ;\

	rm ./lb