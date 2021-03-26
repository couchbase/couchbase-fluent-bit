FROM node:15.12
COPY viewer.js .

# Make sure to mount the logs you want to view here and pass as a parameter the one you want to view
VOLUME /logs

# Intended usage: docker run --mount type=bind,source=...,target=/logs -it --rm <image> <filename>
ENTRYPOINT [ "node", "viewer.js" ]
CMD [ "" ]
