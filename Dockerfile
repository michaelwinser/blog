FROM golang:1.23-alpine AS build
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ cmd/
RUN go build -o /usr/local/bin/blog ./cmd/generate

FROM alpine:latest
RUN apk add --no-cache pandoc curl
COPY --from=build /usr/local/bin/blog /usr/local/bin/blog
COPY templates/ /site/defaults/templates/
COPY static/css/ /site/defaults/static/css/
ENV BLOG_DEFAULTS_DIR=/site/defaults
WORKDIR /site
ENTRYPOINT ["blog"]
CMD ["generate"]
