FROM alpine:3.22

RUN apk add --no-cache busybox-extras
WORKDIR /usr/share/httpd/html
COPY . .
EXPOSE 80
CMD ["httpd", "-f", "-p", "80", "-h", "/usr/share/httpd/html"]
