FROM gcr.io/distroless/static-debian12
COPY octopus-prometheus /usr/bin/

ENTRYPOINT ["/usr/bin/octopus-prometheus"]
