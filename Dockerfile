FROM golang:1.16-alpine AS builder

ENV TERRAFORM_VER v0.15.0

RUN apk add --no-cache git && \
  git clone -c advice.detachedHead=false https://github.com/hashicorp/terraform.git -b ${TERRAFORM_VER}

RUN cd terraform/tools/terraform-bundle && \
  go install .

COPY terraform-bundle.hcl .

RUN terraform-bundle package -os=linux -arch=amd64 terraform-bundle.hcl

WORKDIR /terraformer

COPY go.mod .
COPY go.sum .

RUN go mod download

COPY . .

RUN go build -o ./out/terraformer src/main.go

FROM alpine:3.13 AS app
RUN apk add ca-certificates

COPY --from=builder /go/terraform_*.zip /tmp/

RUN mkdir -p /providers/ && \
  unzip -d /providers /tmp/terraform_*.zip && \
  mv /providers/terraform /bin/terraform && \
  rm /tmp/terraform_*.zip

COPY --from=builder /terraformer/out/terraformer /usr/local/bin/terraformer

ENTRYPOINT ["terraformer"]
