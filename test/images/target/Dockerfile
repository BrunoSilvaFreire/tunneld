FROM alpine:3.20

RUN mkdir -p /www && printf '%s\n' 'hello-from-private-target' > /www/index.html

EXPOSE 8080
CMD ["busybox", "httpd", "-f", "-p", "8080", "-h", "/www"]
