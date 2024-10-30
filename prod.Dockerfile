# syntax=docker/dockerfile:1

FROM golang:1.22 AS build

# Set destination for copying
WORKDIR /app

# copy config data and download go modules
COPY go.mod go.sum ./
RUN go mod download

# copy main source files
COPY ./cmd/main/*.go ./

# copy backends
COPY ./backend ./backend

# build
RUN CGO_ENABLED=0 GOOS=linux go build -o /docker-email-server

FROM scratch AS run

# copy essentials from the build stage
COPY --from=build /docker-email-server /
COPY config.json /

# expose port
EXPOSE 25

# Run
ENTRYPOINT [ "/docker-email-server" ]
