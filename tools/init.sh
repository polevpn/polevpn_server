echo 1 > /proc/sys/net/ipv4/ip_forward
sudo sysctl -w net.core.rmem_max=6500000
iptables -t nat -A POSTROUTING -s 10.8.0.0/16 -j MASQUERADE
ufw disable
