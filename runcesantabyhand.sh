# run cesanta by hand on remote vm.

docker run --rm -it --name docker_auth -p 5001:5001 -v /var/log/docker_auth:/logs -v /home/vagrant/auth_server/config:/config -v /home/vagrant/auth_server/ssl:/ssl cesanta/docker_auth /config/auth_config.yml
