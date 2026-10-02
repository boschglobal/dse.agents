FROM amazon/aws-cli:latest

USER root
RUN dnf install -y unzip && dnf clean all

ENTRYPOINT ["/bin/sh", "-c"]