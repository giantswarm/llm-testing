FROM gsoci.azurecr.io/giantswarm/alpine:3.20.3-giantswarm
FROM scratch

COPY --from=0 /etc/passwd /etc/passwd
COPY --from=0 /etc/group /etc/group
COPY --from=0 /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

ADD llm-testing /
USER giantswarm

ENTRYPOINT ["/llm-testing"]
