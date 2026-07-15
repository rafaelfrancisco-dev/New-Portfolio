# Generated files (*_templ.go, sqlc output, static/css/app.css) are
# committed, so the image builds without templ/sqlc/tailwind.
FROM golang:1.26 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags='-s -w' -o /server ./cmd/server \
    && mkdir /data

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build --chown=nonroot:nonroot /data /data
COPY --from=build /server /server
ENV ADDR=0.0.0.0:8080 APP_DB=/data/app.db DBOS_DB=/data/dbos.db
VOLUME /data
EXPOSE 8080
ENTRYPOINT ["/server"]
