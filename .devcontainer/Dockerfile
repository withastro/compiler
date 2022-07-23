# See here for image contents: https://github.com/microsoft/vscode-dev-containers/tree/v0.241.1/containers/go/.devcontainer/base.Dockerfile

# [Choice] Go version (use -bullseye variants on local arm64/Apple Silicon): 1, 1.18, 1.17, 1-bullseye, 1.17-bullseye, 1.17-bullseye, 1-buster, 1.18-buster, 1.17-buster
ARG VARIANT="1.17-bullseye"
FROM mcr.microsoft.com/vscode/devcontainers/go:0-${VARIANT}

# [Choice] Node.js version: none, lts/*, 16, 14, 12, 10
ARG NODE_VERSION="lts"
RUN if [ "${NODE_VERSION}" != "none" ]; then su vscode -c "umask 0002 && . /usr/local/share/nvm/nvm.sh && nvm install ${NODE_VERSION} 2>&1"; fi

# Install anything else you need
RUN npm i --location=global pnpm
USER vscode
RUN curl https://get.volta.sh | bash
