FROM alpine:3.20

RUN apk add --no-cache openssh-server && \
    adduser -D -s /bin/sh tunneld && \
    echo "tunneld:tunneld" | chpasswd && \
    mkdir -p /home/tunneld/.ssh /run/sshd && \
    chmod 700 /home/tunneld/.ssh && \
    ssh-keygen -A

RUN printf '%s\n' \
    'Port 22' \
    'ListenAddress 0.0.0.0' \
    'HostKey /etc/ssh/ssh_host_ed25519_key' \
    'HostKey /etc/ssh/ssh_host_rsa_key' \
    'PasswordAuthentication no' \
    'KbdInteractiveAuthentication no' \
    'PubkeyAuthentication yes' \
    'AuthorizedKeysFile .ssh/authorized_keys' \
    'AllowUsers tunneld' \
    'AllowTcpForwarding yes' \
    'DisableForwarding no' \
    'PermitOpen any' \
    'X11Forwarding no' \
    'PermitTunnel no' \
    'UsePAM no' \
    'LogLevel VERBOSE' \
    > /etc/ssh/sshd_config

EXPOSE 22
CMD ["sh", "-c", "cp /test/keys/id_ed25519.pub /home/tunneld/.ssh/authorized_keys && chown -R tunneld:tunneld /home/tunneld/.ssh && chmod 600 /home/tunneld/.ssh/authorized_keys && exec /usr/sbin/sshd -D -e"]
