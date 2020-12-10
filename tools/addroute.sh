echo 1 > /proc/sys/net/ipv4/ip_forward
#清除转发，添加10.8.0.0/16 网段nat 
iptables -F POSTROUTING -tnat  
iptables -t nat -A POSTROUTING -s 10.8.0.0/16 -j MASQUERADE

#清除，并添加dns 重定向
iptables -F OUTPUT -tnat 
iptables -t nat -A OUTPUT -p udp -m udp --dport 53 -j DNAT --to-destination 8.8.8.8:53
