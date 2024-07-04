FROM golang:1.22-alpine AS builder
COPY . .
RUN CGO_ENABLED=0 GO111MODULE=on go build -a -installsuffix nocgo -o /wait-for-tcp ./cmd/wait-for-tcp

FROM scratch
COPY --from=builder /wait-for-tcp /wait-for-tcp
ENTRYPOINT ["/wait-for-tcp"]

