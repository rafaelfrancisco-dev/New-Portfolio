# Generated files (*_templ.go, sqlc output, static/css/app.css) are
# committed, so the image builds without templ/sqlc/tailwind.
FROM golang:1.27.1 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags='-s -w' -o /server ./cmd/server \
    && mkdir /data

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build --chown=nonroot:nonroot /data /data
COPY --from=build /server /server
ENV ADDR=127.0.0.1:8081 ADMIN_ADDR=127.0.0.1:8082 APP_DB=/data/app.db DBOS_DB=/data/dbos.db UPLOAD_DIR=/data/uploads ADMIN_USERNAME=admin
VOLUME /data
EXPOSE 8081 8082
ENTRYPOINT ["/server"]
