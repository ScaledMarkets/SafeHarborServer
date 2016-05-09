source $( dirname "${BASH_SOURCE[0]}" )/env.sh

sudo docker stop safeharborserver
sudo docker stop redis
sudo docker stop registry
sudo docker stop clair
sudo docker stop postgres
