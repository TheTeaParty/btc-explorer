FROM alpine:latest

RUN apk --no-cache add ca-certificates

ENV IS_KUBERNETES "yes"

RUN mkdir /app
WORKDIR /app
COPY ./build/app .

ENTRYPOINT [ "./app" ]
