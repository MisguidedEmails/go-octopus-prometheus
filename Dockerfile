FROM gcr.io/distroless/base-debian10
WORKDIR /
COPY /octopus-prometheus .

ENTRYPOINT ["/octopus-prometheus"]
