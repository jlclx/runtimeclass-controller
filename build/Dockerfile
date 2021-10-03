FROM golang:1.17 as builder
WORKDIR /build
COPY go.mod .
COPY go.sum .
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o runtimeclass-controller cmd/runtimeclass-controller/main.go

FROM gcr.io/distroless/base
COPY --from=builder /build/runtimeclass-controller /
EXPOSE 8443
CMD ["/runtimeclass-controller"]