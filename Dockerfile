FROM golang:1.26.2-alpine3.23 AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .

RUN CGO_ENABLED=0 go build \
  -trimpath \
  -ldflags="-s -w" \
  -o /out/butler \
  .

FROM alpine:3.23 AS runtime

RUN apk add --no-cache ca-certificates tzdata \
  && addgroup -S -g 8637 butler \
  && adduser -S -u 8637 -G butler -h /home/butler butler \
  && mkdir -p /home/butler/.config/butler \
  && chown -R 8637:8637 /home/butler

ENV HOME=/home/butler
ENV TZ=Asia/Shanghai

COPY --from=builder /out/butler /usr/local/bin/butler

USER 8637:8637 

ENTRYPOINT [ "/usr/local/bin/butler" ]
CMD ["serve"]
