@echo off
echo Creating firewall rules for the distributed system...

netsh advfirewall firewall add rule name="Matrix Operations - TCP 1234" dir=in action=allow protocol=TCP localport=1234
netsh advfirewall firewall add rule name="Matrix Operations - TCP 1234" dir=out action=allow protocol=TCP localport=1234

echo Firewall rules have been created.
pause
