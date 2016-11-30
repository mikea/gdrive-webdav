FROM debian:testing-slim
MAINTAINER mike.aizatsky@gmail.com

RUN apt-get update && apt-get install -y wget make autoconf build-essential

RUN mkdir /tmp/litmus && \
    cd /tmp/litmus && \
    wget http://www.webdav.org/neon/litmus/litmus-0.13.tar.gz && \
    tar -xvf *.gz --strip-components=1 && \
    ./autogen.sh && \
    ./configure && \
    make install && \
    rm -rf /tmp/litmus

ENTRYPOINT ["/usr/local/bin/litmus"]
