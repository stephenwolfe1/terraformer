FROM golang:1.16-alpine AS builder

ENV TERRAFORM_VERSION=1.1.7

RUN wget -O - "https://releases.hashicorp.com/terraform/${TERRAFORM_VERSION}/terraform_${TERRAFORM_VERSION}_linux_amd64.zip" | unzip - \
  && chmod +x terraform

COPY examples/services/main/versions.tf .

RUN ./terraform providers mirror /providers

WORKDIR /terraformer

COPY go.mod .
COPY go.sum .

RUN go mod download

COPY . .

RUN go build -o ./out/terraformer src/main.go


FROM alpine:3.13 AS app
RUN apk add ca-certificates

COPY .terraformrc /root/.terraformrc

COPY --from=builder /go/terraform /usr/local/bin/terraform
COPY --from=builder /providers /providers
COPY --from=builder /terraformer/out/terraformer /usr/local/bin/terraformer

ENTRYPOINT ["terraformer"]
