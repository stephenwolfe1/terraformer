FROM golang:1.22.4-alpine3.20 AS builder

ARG TARGETPLATFORM
ENV TARGETPLATFORM=${TARGETPLATFORM:-linux/amd64}
ENV TERRAFORM_VERSION=1.5.5

RUN wget -nv -qO terraform.zip "https://releases.hashicorp.com/terraform/${TERRAFORM_VERSION}/terraform_${TERRAFORM_VERSION}_${TARGETPLATFORM////_}.zip" && \
  unzip terraform.zip

COPY examples/services/main/versions.tf .

RUN ./terraform providers mirror /providers

WORKDIR /terraformer

COPY go.mod .
COPY go.sum .

RUN go mod download

COPY . .

RUN go build -o ./out/terraformer src/main.go


FROM alpine:3.20 AS app
RUN apk add ca-certificates

COPY .terraformrc /root/.terraformrc

COPY --from=builder /go/terraform /usr/local/bin/terraform
COPY --from=builder /providers /providers
COPY --from=builder /terraformer/out/terraformer /usr/local/bin/terraformer

ENTRYPOINT ["terraformer"]
