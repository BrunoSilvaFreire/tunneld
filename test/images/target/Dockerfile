FROM alpine:3.20

RUN apk add --no-cache python3
RUN mkdir -p /www && printf '%s\n' 'hello-from-private-target' > /www/index.html

EXPOSE 8080
CMD ["python3", "-m", "http.server", "8080", "--directory", "/www"]
