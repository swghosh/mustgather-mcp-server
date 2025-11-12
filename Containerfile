FROM golang:1.24 AS builder

WORKDIR /app

COPY . .

RUN go build -o mustgather-mcp-server .
RUN curl -L -O https://github.com/gmeghnag/omc/releases/download/v3.12.0/omc_Linux_x86_64
RUN chmod +x ./omc_Linux_x86_64

FROM registry.access.redhat.com/ubi9-minimal:9.2

WORKDIR /app

COPY --from=builder /app/omc_Linux_x86_64 ./omc
COPY --from=builder /app/mustgather-mcp-server ./mustgather-mcp-server

ENTRYPOINT ["/app/mustgather-mcp-server", "--sse-port", "8080"]
