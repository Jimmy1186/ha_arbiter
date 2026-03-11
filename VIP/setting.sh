#!/bin/bash

sudo cp ./check_server_alive.sh /etc/keepalived/check_server_alive.sh
sudo cp ./check_server_alive.sh /etc/keepalived/chk_server_health.sh
sudo cp ./notify_role.sh /etc/keepalived/notify_role.sh

sudo cp ./keepalived.conf /etc/keepalived/keepalived.conf
chmod 777 /etc/keepalived/check_server_alive.sh
chmod 777 /etc/keepalived/chk_server_health.sh
chmod 644 /etc/keepalived/keepalived.conf
chmod +x /etc/keepalived/notify_role.sh
# journalctl -u keepalived.service -f 