FROM node:15.12
# WORKDIR set in base image
# hadolint ignore=DL3045
COPY viewer.js .

# Make sure to mount the logs you want to view here and pass as a parameter the one you want to view
VOLUME /logs

# Run as the non-root "node" user provided by the base image instead of root
USER node

# Intended usage: docker run --mount type=bind,source=...,target=/logs -it --rm <image> <filename>
ENTRYPOINT [ "node", "viewer.js" ]
CMD [ "" ]
