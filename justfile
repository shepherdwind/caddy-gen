default:
  just --list

build:
  docker build -t gera2ld/caddy-gen:latest .

push:
  docker push gera2ld/caddy-gen:latest
