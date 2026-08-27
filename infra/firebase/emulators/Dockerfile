FROM eclipse-temurin:21-jre-noble AS java-env

# Firebase emulator suite in a container, so the host needs no JRE or
# firebase-tools. Auth, Firestore, and Storage emulators for local development.
FROM node:22-slim

# JRE for the Firestore/Storage emulators (headless — no GUI).
RUN apt-get update \
    && apt-get install -y --no-install-recommends \
    && rm -rf /var/lib/apt/lists/*

COPY --from=java-env /opt/java/openjdk /opt/java/openjdk

# Set environment variables so the system and Firebase can find Java
ENV JAVA_HOME=/opt/java/openjdk
ENV PATH="${JAVA_HOME}/bin:${PATH}"

RUN java -version && npm install -g firebase-tools

# Pre-download the emulator jars into the image so `docker compose up` needs no
# network and starts fast.
RUN firebase setup:emulators:firestore \
    && firebase setup:emulators:storage \
    && firebase setup:emulators:ui

WORKDIR /workspace
COPY entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/entrypoint.sh

# auth, firestore, storage, UI, hub, logging
EXPOSE 9099 8081 9199 4000 4400 4500

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
