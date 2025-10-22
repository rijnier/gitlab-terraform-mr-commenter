FROM gcr.io/distroless/static:nonroot

ARG TARGETPLATFORM

COPY $TARGETPLATFORM/gitlab-terraform-mr-commenter /gitlab-terraform-mr-commenter

ENTRYPOINT [ "/gitlab-terraform-mr-commenter" ]
