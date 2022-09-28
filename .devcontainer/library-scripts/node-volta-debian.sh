#!/bin/bash
#-------------------------------------------------------------------------------------------------------------
# Copyright (c) Randall T. Vasquez
# Licensed under the MIT License.
#-------------------------------------------------------------------------------------------------------------
# Based on: https://github.com/microsoft/vscode-dev-containers/blob/main/containers/go/.devcontainer/library-scripts/node-debian.sh
# Docs: https://github.com/microsoft/vscode-dev-containers/blob/main/script-library/docs/node.md
#
# Copyright (c) Microsoft Corporation. All rights reserved.
# See https://go.microsoft.com/fwlink/?linkid=2090316 for license information.
#-------------------------------------------------------------------------------------------------------------
# Maintainer: Randall T. Vasquez
#
# Syntax: ./node-volta-debian.sh [directory to install Volta] [node version to install (use "none" to skip)] [non-root user] [Update rc files flag] [install node-gyp deps]

export VOLTA_DIR=${1:-"/usr/local/share/volta"}
export NODE_VERSION=${2:-"lts"}
USERNAME=${3:-"automatic"}
UPDATE_RC=${4:-"true"}
INSTALL_TOOLS_FOR_NODE_GYP="${5:-true}"

set -e

if [ "$(id -u)" -ne 0 ]; then
    echo -e 'Script must be run as root. Use sudo, su, or add "USER root" to your Dockerfile before running this script.'
    exit 1
fi

# Ensure that login shells get the correct path if the user updated the PATH using ENV.
rm -f /etc/profile.d/00-restore-env.sh
echo "export PATH=${PATH//$(sh -lc 'echo $PATH')/\$PATH}" > /etc/profile.d/00-restore-env.sh
chmod +x /etc/profile.d/00-restore-env.sh

# Determine the appropriate non-root user
if [ "${USERNAME}" = "auto" ] || [ "${USERNAME}" = "automatic" ]; then
    USERNAME=""
    POSSIBLE_USERS=("vscode" "node" "codespace" "$(awk -v val=1000 -F ":" '$3==val{print $1}' /etc/passwd)")
    for CURRENT_USER in ${POSSIBLE_USERS[@]}; do
        if id -u ${CURRENT_USER} > /dev/null 2>&1; then
            USERNAME=${CURRENT_USER}
            break
        fi
    done
    if [ "${USERNAME}" = "" ]; then
        USERNAME=root
    fi
elif [ "${USERNAME}" = "none" ] || ! id -u ${USERNAME} > /dev/null 2>&1; then
    USERNAME=root
fi

updaterc() {
    if [ "${UPDATE_RC}" = "true" ]; then
        echo "Updating /etc/bash.bashrc and /etc/zsh/zshrc..."
        if [[ "$(cat /etc/bash.bashrc)" != *"$1"* ]]; then
            echo -e "$1" >> /etc/bash.bashrc
        fi
        if [ -f "/etc/zsh/zshrc" ] && [[ "$(cat /etc/zsh/zshrc)" != *"$1"* ]]; then
            echo -e "$1" >> /etc/zsh/zshrc
        fi
    fi
}

# Function to run apt-get if needed
apt_get_update_if_needed()
{
    if [ ! -d "/var/lib/apt/lists" ] || [ "$(ls /var/lib/apt/lists/ | wc -l)" = "0" ]; then
        echo "Running apt-get update..."
        apt-get update
    else
        echo "Skipping apt-get update."
    fi
}

# Checks if packages are installed and installs them if not
check_packages() {
    if ! dpkg -s "$@" > /dev/null 2>&1; then
        apt_get_update_if_needed
        apt-get -y install --no-install-recommends "$@"
    fi
}

# Ensure apt is in non-interactive to avoid prompts
export DEBIAN_FRONTEND=noninteractive

# Install dependencies
check_packages apt-transport-https curl ca-certificates tar gnupg2 dirmngr

# Adjust node version if required
if [ "${NODE_VERSION}" = "none" ]; then
    export NODE_VERSION=
elif [ "${NODE_VERSION}" = "lts" ]; then
    export NODE_VERSION="lts"
fi

# Install the specified node version if Volta directory already exists, then exit
if [ -d "${VOLTA_DIR}" ]; then
    echo "Volta is already installed."
    if [ "${NODE_VERSION}" != "" ]; then
       su ${USERNAME} -c "volta install ${NODE_VERSION}"
    fi
    exit 0
fi

# Create volta group, volta dir, and set sticky bit
if ! cat /etc/group | grep -e "^volta:" > /dev/null 2>&1; then
    groupadd -r volta
fi
umask 0002
usermod -a -G volta ${USERNAME}
mkdir -p ${VOLTA_DIR}
chown :volta ${VOLTA_DIR}
chmod g+s ${VOLTA_DIR}
su ${USERNAME} -c "$(cat << EOF
    set -e
    umask 0002
    # Do not update profile - we'll do this manually
    export PROFILE=/dev/null
    curl -so- https://get.volta.sh | bash -s -- --skip-setup
    if [ "${NODE_VERSION}" != "" ]; then
        volta install ${NODE_VERSION}
    fi
EOF
)" 2>&1
# Update rc files
if [ "${UPDATE_RC}" = "true" ]; then
updaterc "$(cat <<EOF
export VOLTA_HOME="${VOLTA_DIR}"
export PATH="${VOLTA_HOME}/bin:${PATH}"
EOF
)"
fi

# If enabled, verify "python3", "make", "gcc", "g++" commands are available so node-gyp works - https://github.com/nodejs/node-gyp
if [ "${INSTALL_TOOLS_FOR_NODE_GYP}" = "true" ]; then
    echo "Verifying node-gyp OS requirements..."
    to_install=""
    if ! type make > /dev/null 2>&1; then
        to_install="${to_install} make"
    fi
    if ! type gcc > /dev/null 2>&1; then
        to_install="${to_install} gcc"
    fi
    if ! type g++ > /dev/null 2>&1; then
        to_install="${to_install} g++"
    fi
    if ! type python3 > /dev/null 2>&1; then
        to_install="${to_install} python3-minimal"
    fi
    if [ ! -z "${to_install}" ]; then
        apt_get_update_if_needed
        apt-get -y install ${to_install}
    fi
fi

echo "Done!"
