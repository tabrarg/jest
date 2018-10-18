dist:
	@mkdir -p ./bin
	@rm -f ./bin/*
	GOOS=freebsd GOARCH=amd64 go build -o ./bin/Jest-freebsd64 .
	GOOS=freebsd GOARCH=386 go build -o ./bin/Jest-freebsd386 .
