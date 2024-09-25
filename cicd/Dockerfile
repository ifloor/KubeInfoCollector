FROM golang:1.22.3-bookworm

# Imutable

RUN mkdir /kubeinfocollector

WORKDIR /kubeinfocollector

ENTRYPOINT ./KubeInfoCollector

# Mutable
COPY KubeInfoCollector KubeInfoCollector