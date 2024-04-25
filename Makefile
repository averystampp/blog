build:
	@docker build .
run:
	@docker run --env-file=.env -d -p 443:8080 -v blog:/app $(id)
remove:
	@docker rm -f $(id)

