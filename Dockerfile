FROM ubuntu:latest

WORKDIR /disk-tool/

COPY html html
COPY static static
COPY disk-tool .
COPY startup.sh .

# Copy empty config files
COPY config-files config-files

VOLUME /config/

EXPOSE 8192

CMD /disk-tool/startup.sh