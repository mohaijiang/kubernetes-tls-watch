FROM golang:1.22 AS builder

COPY . /src
WORKDIR /src

RUN make build

FROM debian:stable-slim
WORKDIR /app
ENV TZ=Asia/Shanghai
ENV NAMESPACE=default
# statefulset or deployment
ENV DEPLOY_TYPE=statefulset
ENV DEPLOY_NAME=example
ENV SECRET_NAME=example-tls

COPY --from=builder /src/kubernetes-tls-watch /app

CMD ["kubernetes-tls-watch"]

