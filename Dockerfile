FROM golang:alpine AS builder
ARG VERSION
COPY . .
RUN go build -ldflags "-X main.version=$VERSION" -o /bin/app

FROM scratch
COPY --from=builder /bin/app /bin/app
ENTRYPOINT ["/bin/app"]