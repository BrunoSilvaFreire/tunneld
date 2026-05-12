 make package-deb || exit 1
 sudo apt purge tunneld && sudo apt install ./tunneld_0.1.0_amd64.deb || exit 2
 sudo systemctl daemon-reload  || exit 3
 sudo systemctl restart tunneld.service  || exit 4