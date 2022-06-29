# 1. Stage one: build ontology
FROM golang:1.18.3 AS build

COPY . /src

WORKDIR /src

RUN GOPROXY=https://goproxy.cn make

FROM ubuntu:latest
WORKDIR /app
COPY --from=build /src/build/bin/poly /usr/local/bin/poly


EXPOSE 20334 20335 20336 20337 20338 20339
#NOTE! we highly recommand that you put data dir to a mounted volume, e.g. --data-dir /data/Chain
#write data to docker image is *not* a best practice
CMD ["/bin/bash"]