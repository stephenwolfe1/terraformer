FROM golang:1.25.1-alpine3.22 AS builder

ARG TERRAFORM_VERSION="1.12.2" 
ENV TERRAFORM_URL="https://releases.hashicorp.com/terraform/${TERRAFORM_VERSION}/terraform_${TERRAFORM_VERSION}_linux_arm64.zip"

RUN wget --quiet ${TERRAFORM_URL} \
  && unzip terraform_${TERRAFORM_VERSION}_linux_arm64.zip \
  && mv terraform /usr/local/bin/ \
  && rm terraform_${TERRAFORM_VERSION}_linux_arm64.zip

COPY examples/services/main/versions.tf .

RUN terraform providers mirror /providers

WORKDIR /terraformer

COPY go.mod .
COPY go.sum .

RUN go mod download

COPY . .

RUN go build -o ./out/terraformer src/main.go


FROM alpine:3.22 AS app
RUN apk add ca-certificates

COPY .terraformrc /root/.terraformrc

COPY --from=builder /usr/local/bin/terraform /usr/local/bin/terraform
COPY --from=builder /providers /providers
COPY --from=builder /terraformer/out/terraformer /usr/local/bin/terraformer

ENTRYPOINT ["terraformer"]
