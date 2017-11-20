FROM alpine
MAINTAINER Anthony Woods awoods@raintank.io

RUN apk --update add ca-certificates
RUN mkdir -p /etc/raintank
COPY config/gw.ini /etc/worldping/gw.ini

COPY build/worldping-gw /usr/bin/worldping-gw
COPY entrypoint.sh /usr/bin/

EXPOSE 80
EXPOSE 443

ENTRYPOINT ["/usr/bin/entrypoint.sh"]
CMD ["/usr/bin/worldping-gw", "-config=/etc/worldping/gw.ini"]
