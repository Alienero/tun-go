FROM ubuntu:latest

MAINTAINER Yim "yiyan.lu@me.com"

# RUN apt-get update
RUN apt-get install -y iptables

# Create TUN interface config file.
RUN mkdir -p /dev/net && \
	mknod /dev/net/tun c 10 200 && \
	chmod 600 /dev/net/tun

ADD server /bin/server