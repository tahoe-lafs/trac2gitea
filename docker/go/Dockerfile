# Start from the official Go image based on Alpine
FROM golang:1.22.4-alpine

LABEL Description="Trac2Gitea migration tool"

RUN apk --no-cache add gcc git libc-dev make

# Parameters for default user:group
ARG uid=1000
ARG user=appuser
ARG gid=1000
ARG group=appgroup

# Add user and group for build and runtime
RUN id ${user} > /dev/null 2>&1 || \
    { addgroup -g "${gid}" "${group}" && adduser -D -h /home/${user} -s /bin/bash -G "${group}" -u "${uid}" "${user}"; }

# Prepare directories
RUN DIRS="/src /app" && \
    mkdir -p ${DIRS} && \
    chown -R ${user}:${group} $DIRS

# Switch to non-root user
USER ${user}

# Install mockgen
#RUN go install go.uber.org/mock/mockgen@v0.4.0
#RUN ln -s /go /home/${user}/go

# Install pinned dependencies
#COPY go.mod go.sum /src
#WORKDIR /src
#RUN go mod download && \
#    go mod verify
