FROM alpine:3.4

RUN apk add --no-cache ca-certificates

WORKDIR /app

COPY config.json /app/
COPY fofou_linux /app/fofou
COPY scripts/entrypoint.sh /app/entrypoint.sh
COPY static /app/static/
COPY img /app/img/
COPY tmpl /app/tmpl/
COPY forums /app/forums/

EXPOSE 80 443

CMD ["./entrypoint.sh"]
