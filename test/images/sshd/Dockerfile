FROM alpine:3.20

RUN apk add --no-cache openssh-server && \
    adduser -D -s /bin/sh tunneld && \
    echo "tunneld:tunneld" | chpasswd && \
    mkdir -p /home/tunneld/.ssh /run/sshd && \
    chmod 700 /home/tunneld/.ssh && \
    ssh-keygen -A

RUN sed -i 's/#PasswordAuthentication yes/PasswordAuthentication no/' /etc/ssh/sshd_config && \
    sed -i 's/#PubkeyAuthentication yes/PubkeyAuthentication yes/' /etc/ssh/sshd_config && \
    echo "AllowUsers tunneld" >> /etc/ssh/sshd_config

EXPOSE 22
CMD ["sh", "-c", "cp /test/keys/id_ed25519.pub /home/tunneld/.ssh/authorized_keys && chown -R tunneld:tunneld /home/tunneld/.ssh && chmod 600 /home/tunneld/.ssh/authorized_keys && exec /usr/sbin/sshd -D -e"]
