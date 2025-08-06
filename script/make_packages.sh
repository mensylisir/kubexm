#!/bin/bash
set -e
set -o pipefail

SUDO=""
if [ "$(id -u)" -ne 0 ]; then
    SUDO="sudo"
fi
# 然后将脚本中所有的 `sudo` 替换为 `${SUDO}`
#${SUDO} apt-get update -qq

# ==============================================================================
#                      生产级离线软件包打包脚本
# ==============================================================================
#
# 功能:
#   1. 自动检测或通过参数指定操作系统、版本和架构。
#   2. 为 Debian/Ubuntu (.deb) 和 RHEL/CentOS (.rpm) 系统打包。
#   3. 下载指定的软件包及其所有必需的依赖项。
#   4. 生成结构化的输出目录和包含清单的 tar.gz 压缩包。
#
# 使用方法:
#   - 自动检测:
#     ./make-packages.sh
#
#   - 手动指定 (用于交叉打包或在 Docker 中运行):
#     OS_ID=ubuntu OS_VERSION=20.04 ARCH=amd64 ./make-packages.sh
#
# ==============================================================================

# --- 1. 脚本参数和环境变量 ---

# 允许通过环境变量覆盖自动检测
OS_ID="${OS_ID:-$(grep -oP '(?<=^ID=).+' /etc/os-release | tr -d '"')}"
OS_VERSION="${OS_VERSION:-$(grep -oP '(?<=^VERSION_ID=).+' /etc/os-release | tr -d '"')}"
ARCH="${ARCH:-$(dpkg --print-architecture 2>/dev/null || uname -m)}"

# 将 x86_64 统一为 amd64
if [ "$ARCH" = "x86_64" ]; then
    ARCH="amd64"
fi

echo "--- Deteckted/Specified Environment ---"
echo "Operating System : ${OS_ID}"
echo "Version          : ${OS_VERSION}"
echo "Architecture     : ${ARCH}"
echo "-------------------------------------"

# --- 2. 软件包列表定义 ---

# 基础软件包 (通用)
BASE_PACKAGES="socat conntrack ipset ebtables chrony ipvsadm"

# 负载均衡器软件包
LB_PACKAGES_KH="keepalived haproxy"
LB_PACKAGES_XN="keepalived nginx"

# 根据操作系统定义 iSCSI 和其他差异化软件包
declare -A OS_SPECIFIC_PACKAGES
OS_SPECIFIC_PACKAGES["ubuntu"]="open-iscsi"
OS_SPECIFIC_PACKAGES["debian"]="open-iscsi"
OS_SPECIFIC_PACKAGES["centos"]="iscsi-initiator-utils conntrack-tools"
OS_SPECIFIC_PACKAGES["rhel"]="iscsi-initiator-utils conntrack-tools"
OS_SPECIFIC_PACKAGES["almalinux"]="iscsi-initiator-utils conntrack-tools"
OS_SPECIFIC_PACKAGES["rocky"]="iscsi-initiator-utils conntrack-tools"

# 最终要打包的软件包列表
# 我们将所有可能用到的包都打包进去，在安装时按需选择
ALL_PACKAGES="${BASE_PACKAGES} ${LB_PACKAGES_KH} ${LB_PACKAGES_XN} ${OS_SPECIFIC_PACKAGES[${OS_ID}]}"
# 去重
ALL_PACKAGES=$(echo "${ALL_PACKAGES}" | tr ' ' '\n' | sort -u | tr '\n' ' ')

echo "Packages to be packed: ${ALL_PACKAGES}"

# --- 3. 准备输出目录 ---

WORKSPACE=$(pwd)
OUTPUT_DIR="${WORKSPACE}/packages"
PACKAGE_DIR="${OUTPUT_DIR}/${OS_ID}/${OS_VERSION}/${ARCH}"
rm -rf "${PACKAGE_DIR}"
mkdir -p "${PACKAGE_DIR}"

echo "Output will be saved in: ${PACKAGE_DIR}"

# --- 4. 打包逻辑 ---

# 函数：为 Debian/Ubuntu 打包
package_deb() {
    echo "Starting packaging for Debian/Ubuntu..."

    # 确保 apt-cache 和 apt-get 可用
    command -v apt-get >/dev/null 2>&1 || { echo >&2 "apt-get not found. Aborting."; exit 1; }

    # 更新包列表
    echo "Running apt-get update..."
    ${SUDO} apt-get update -qq

    echo "Collecting dependencies..."
    # 使用 apt-rdepends 来获取更精确的依赖列表（如果没有，可以安装 apt-rdepends）
    # 这里我们还是使用内建的 apt-cache，因为它更通用
    DEPS=$(apt-cache depends --recurse --no-recommends --no-suggests \
      --no-conflicts --no-breaks --no-replaces --no-enhances \
      ${ALL_PACKAGES} | grep "^\w" | sort -u)

    if [ -z "$DEPS" ]; then
        echo >&2 "Failed to collect dependencies. Please check package names."
        exit 1
    fi

    echo "Downloading packages and dependencies..."
    cd "${PACKAGE_DIR}"
    # 下载所有 .deb 文件
    apt-get download ${DEPS}
    if [ $? -ne 0 ]; then
        echo >&2 "Error downloading one of the packages or dependencies for: ${ALL_PACKAGES}"
        exit 1
    fi

    # 生成清单文件
    echo "Generating manifest..."
    echo "--- Precise Package Info from downloaded files ---" > manifest.txt
    for pkg in *.deb; do
        dpkg-deb -f "$pkg" Package Version Architecture >> manifest.txt
        echo "---" >> manifest.txt
    done
    echo "" >> manifest.txt
    echo "--- Packed Files ---" >> manifest.txt
    ls -1 *.deb >> manifest.txt

    # 打包
    echo "Creating archive..."
    cd "${WORKSPACE}"
    TARBALL_NAME="packages-${OS_ID}-${OS_VERSION}-${ARCH}.tar.gz"
    tar -zcvf "${TARBALL_NAME}" -C "${OUTPUT_DIR}" "${OS_ID}/${OS_VERSION}/${ARCH}"

    echo "SUCCESS: Offline package created at ${WORKSPACE}/${TARBALL_NAME}"
}

# 函数：为 RHEL/CentOS 打包
package_rpm() {
    echo "Starting packaging for RHEL/CentOS..."

    # 确定包管理器
    if command -v dnf >/dev/null 2>&1; then
        PKG_MANAGER="dnf"
        ${SUDO} ${PKG_MANAGER} install -y 'dnf-command(download)'
    elif command -v yum >/dev/null 2>&1; then
        PKG_MANAGER="yum"
        ${SUDO} ${PKG_MANAGER} install -y yum-utils
    else
        echo >&2 "Neither yum nor dnf found. Aborting."
        exit 1
    fi

    echo "Downloading packages and dependencies with ${PKG_MANAGER}..."
    cd "${PACKAGE_DIR}"

    # 使用 download 命令下载
    ${SUDO} ${PKG_MANAGER} download --resolve --destdir=. ${ALL_PACKAGES}

    # 生成清单文件
    echo "Generating manifest..."
    echo "--- Precise Package Info from downloaded files ---" > manifest.txt
    for pkg in *.rpm; do
        rpm -qip "$pkg" >> manifest.txt
        echo "---" >> manifest.txt
    done
    echo "" >> manifest.txt
    echo "--- Packed Files ---" >> manifest.txt
    ls -1 *.rpm >> manifest.txt

    # 打包
    echo "Creating archive..."
    cd "${WORKSPACE}"
    TARBALL_NAME="packages-${OS_ID}-${OS_VERSION}-${ARCH}.tar.gz"
    tar -zcvf "${TARBALL_NAME}" -C "${OUTPUT_DIR}" "${OS_ID}/${OS_VERSION}/${ARCH}"

    echo "SUCCESS: Offline package created at ${WORKSPACE}/${TARBALL_NAME}"
}


# --- 5. 主逻辑 ---

case "${OS_ID}" in
    ubuntu|debian)
        package_deb
        ;;
    centos|rhel|almalinux|rocky)
        package_rpm
        ;;
    *)
        echo >&2 "Unsupported Operating System: ${OS_ID}"
        exit 1
        ;;
esac

exit 0


#docker run --rm -v "$(pwd):/workspace" -w /workspace ubuntu:20.04 \
#  bash -c "apt-get update && apt-get install -y apt-utils && ./make-packages.sh"


#docker run --rm -v "$(pwd):/workspace" -w /workspace centos:7 \
#  bash -c "yum install -y epel-release && ./make-packages.sh"