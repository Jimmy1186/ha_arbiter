#!/bin/bash

sudo cp ./check_server_alive.sh /etc/keepalived/check_server_alive.sh
sudo cp ./check_server_alive.sh /etc/keepalived/chk_server_health.sh
sudo cp ./keepalived.conf /etc/keepalived/keepalived.conf
chmod 777 /etc/keepalived/check_server_alive.sh
chmod 777 /etc/keepalived/chk_server_health.sh
chmod 644 /etc/keepalived/keepalived.conf

# journalctl -u keepalived.service -f 