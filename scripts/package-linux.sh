#!/usr/bin/env bash
# 本地 Linux 打包：rpm（EL9 / webkit2gtk-4.0）与 deb（Ubuntu / webkit2gtk-4.1）。
#
#   scripts/package-linux.sh rpm [version]
#   scripts/package-linux.sh deb [version]
#   scripts/package-linux.sh all [version]
#
# WebKit 版本由 build tag 决定：rpm 用默认（webkit2gtk-4.0），deb 加
# `webkit41`（webkit2gtk-4.1），不再复制 .pc 文件。
# deb 在非 Debian/Ubuntu 主机上会自动进入 ubuntu:22.04 容器构建；版本默认取
# 最近一个 git tag（去掉 v 前缀），没有 tag 时用当天日期。
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
FORMAT="${1:-all}"
shift || true

INSIDE_DEB=0
INSIDE_RPM=0
if [ "$FORMAT" = "--inside-deb" ]; then
    INSIDE_DEB=1
    FORMAT="deb"
    VER="${1:?missing version for --inside-deb}"
    shift || true
elif [ "$FORMAT" = "--inside-rpm" ]; then
    INSIDE_RPM=1
    FORMAT="rpm"
    VER="${1:?missing version for --inside-rpm}"
    shift || true
else
    VER="${1:-}"
fi

if [ -z "${VER:-}" ]; then
    if tag="$(git -C "$ROOT" describe --tags --abbrev=0 2>/dev/null)"; then
        VER="${tag#v}"
    else
        VER="$(date +%Y%m%d)"
    fi
fi
if ! [[ "$VER" =~ ^[A-Za-z0-9._+]+$ ]]; then
    echo "invalid version '$VER': use letters, digits, dot, underscore or plus" >&2
    exit 1
fi

WORK="$ROOT/dist/package"
BIN="$WORK/little-timer"
RELEASE_RPM="1webkit40"
RELEASE_DEB="1webkit41"

die() {
    echo "error: $*" >&2
    exit 1
}

build_ui() {
    command -v pnpm >/dev/null 2>&1 || die "pnpm not found"
    [ -d "$ROOT/assets/src/bindings" ] || die "Wails bindings missing; run 'just bindings' first"
    cd "$ROOT/assets"
    [ -d node_modules ] || pnpm install
    pnpm run build
    "$ROOT/scripts/stage-embedded-ui.sh"
}

build_binary() {
    local webkit_tag="${1:-}"
    local webkit_pc="webkit2gtk-4.0"
    local tags="webview,embed_ui"
    if [ "$webkit_tag" = "webkit41" ]; then
        webkit_pc="webkit2gtk-4.1"
        tags="webview,embed_ui,webkit41"
    fi
    pkg-config --exists gtk+-3.0 || die "gtk+-3.0 development files not found"
    pkg-config --exists "$webkit_pc" || die "$webkit_pc pkg-config entry not found"
    mkdir -p "$WORK"
    cd "$ROOT/neo-src"
    CGO_ENABLED=1 go build -buildvcs=false -tags "$tags" \
        -ldflags "-s -w -X little-timer/internal/app.Version=$VER" \
        -o "$BIN" ./cmd/server
    echo "built $BIN ($(du -h "$BIN" | awk '{print $1}'), tags=$tags)"
}

assert_needed() {
    local want="$1"
    readelf -d "$BIN" | grep -q "NEEDED.*${want}" || {
        echo "binary NEEDED entries:" >&2
        readelf -d "$BIN" | awk '/NEEDED/ {print "  " $NF}' >&2
        die "binary does not link $want (check build tag / build image)"
    }
}

install_icons() {
    local dest_root="$1"
    install -Dm644 "$ROOT/android/app/src/main/res/mipmap-mdpi/ic_launcher.png" "$dest_root/48x48/apps/little-timer.png"
    install -Dm644 "$ROOT/android/app/src/main/res/mipmap-hdpi/ic_launcher.png" "$dest_root/72x72/apps/little-timer.png"
    install -Dm644 "$ROOT/android/app/src/main/res/mipmap-xhdpi/ic_launcher.png" "$dest_root/96x96/apps/little-timer.png"
    install -Dm644 "$ROOT/android/app/src/main/res/mipmap-xxhdpi/ic_launcher.png" "$dest_root/144x144/apps/little-timer.png"
    install -Dm644 "$ROOT/android/app/src/main/res/mipmap-xxxhdpi/ic_launcher.png" "$dest_root/192x192/apps/little-timer.png"
}

make_rpm() {
    command -v rpmbuild >/dev/null 2>&1 || die "rpmbuild not found"
    if ! pkg-config --exists gtk+-3.0 webkit2gtk-4.0; then
        cat >&2 <<'EOF'
missing RPM build dependencies. Install them first:
  sudo dnf install -y gtk3-devel webkit2gtk3-devel sqlite-devel pkg-config
EOF
        exit 1
    fi

    build_binary
    assert_needed libwebkit2gtk-4.0.so.37

    local top="$WORK/rpmbuild"
    /bin/rm -rf "$top"
    mkdir -p "$top"/{SOURCES,SPECS,BUILD,BUILDROOT,RPMS,SRPMS}
    install -m755 "$BIN" "$top/SOURCES/little-timer"
    install -m755 "$ROOT/packaging/little-timer-desktop" "$top/SOURCES/little-timer-desktop"
    install -m644 "$ROOT/packaging/little-timer.desktop" "$top/SOURCES/little-timer.desktop"
    install -m644 "$ROOT/LICENSE" "$top/SOURCES/LICENSE"
    install -m644 "$ROOT/android/app/src/main/res/mipmap-mdpi/ic_launcher.png" "$top/SOURCES/little-timer-48.png"
    install -m644 "$ROOT/android/app/src/main/res/mipmap-hdpi/ic_launcher.png" "$top/SOURCES/little-timer-72.png"
    install -m644 "$ROOT/android/app/src/main/res/mipmap-xhdpi/ic_launcher.png" "$top/SOURCES/little-timer-96.png"
    install -m644 "$ROOT/android/app/src/main/res/mipmap-xxhdpi/ic_launcher.png" "$top/SOURCES/little-timer-144.png"
    install -m644 "$ROOT/android/app/src/main/res/mipmap-xxxhdpi/ic_launcher.png" "$top/SOURCES/little-timer-192.png"

    rpmbuild -bb \
        --define "_topdir $top" \
        --define "lt_version $VER" \
        "$ROOT/packaging/rpm/little-timer.spec"

    local rpmfile
    rpmfile="$(find "$top/RPMS" -name "little-timer-${VER}-${RELEASE_RPM}*.rpm" -print -quit)"
    [ -n "$rpmfile" ] || die "rpmbuild produced no little-timer rpm"
    cp "$rpmfile" "$ROOT/dist/"
    echo "✅ $ROOT/dist/$(basename "$rpmfile")"
}

make_deb() {
    command -v dpkg-deb >/dev/null 2>&1 || die "dpkg-deb not found"
    local debroot="$WORK/debroot"
    /bin/rm -rf "$debroot"
    mkdir -p "$debroot/DEBIAN" "$debroot/usr/bin" "$debroot/usr/share/applications" \
        "$debroot/usr/share/doc/little-timer"

    install -Dm755 "$BIN" "$debroot/usr/bin/little-timer"
    install -Dm755 "$ROOT/packaging/little-timer-desktop" "$debroot/usr/bin/little-timer-desktop"
    install -Dm644 "$ROOT/packaging/little-timer.desktop" "$debroot/usr/share/applications/little-timer.desktop"
    install -Dm644 "$ROOT/LICENSE" "$debroot/usr/share/doc/little-timer/copyright"
    install_icons "$debroot/usr/share/icons/hicolor"

    local depends=""
    local shlibdir="$WORK/shlibdeps"
    /bin/rm -rf "$shlibdir"
    mkdir -p "$shlibdir/debian"
    cat > "$shlibdir/debian/control" <<'EOF'
Source: little-timer
Section: utils
Priority: optional
Maintainer: Little Timer Maintainers <maintainers@example.invalid>
Standards-Version: 4.5.0

Package: little-timer
Architecture: any
Depends: ${shlibs:Depends}
Description: Countdown, stopwatch and habit timer
 Local-first countdown/stopwatch with habits and GTD tasks.
EOF
    if (cd "$shlibdir" && dpkg-shlibdeps -O "$debroot/usr/bin/little-timer" > "$WORK/shlibs.out" 2> "$WORK/shlibs.err"); then
        depends="$(sed -n 's/^shlibs:Depends=//p' "$WORK/shlibs.out")"
    else
        echo "warning: dpkg-shlibdeps failed; falling back to explicit Depends" >&2
        sed 's/^/  /' "$WORK/shlibs.err" >&2 || true
    fi
    if [ -z "$depends" ]; then
        depends="libc6, libgtk-3-0, libwebkit2gtk-4.1-0, libsqlite3-0"
    fi
    echo "$depends" | grep -q 'libwebkit2gtk-4\.1-0' || \
        die "generated Depends does not mention libwebkit2gtk-4.1-0: $depends"

    local installed_size
    installed_size="$(du -sk "$debroot/usr" | awk '{print $1}')"
    cat > "$debroot/DEBIAN/control" <<EOF
Package: little-timer
Version: ${VER}-${RELEASE_DEB}
Section: utils
Priority: optional
Architecture: amd64
Installed-Size: ${installed_size}
Depends: ${depends}
Recommends: gnome-keyring
Provides: little-timer-webkit41
Maintainer: Little Timer Maintainers <maintainers@example.invalid>
Description: Countdown, stopwatch and habit timer
 Local-first countdown/stopwatch with habits, GTD tasks, daily planning
 and backups. This build links WebKitGTK 4.1. The desktop entry starts
 the HTTP server and opens the system browser; a WebView window is
 available as a separate desktop action.
EOF

    local out="$ROOT/dist/little-timer_${VER}-${RELEASE_DEB}_amd64.deb"
    dpkg-deb --build --root-owner-group "$debroot" "$out" >/dev/null
    echo "✅ $out"
}

run_deb_container() {
    command -v docker >/dev/null 2>&1 || die "docker not found (needed for the 4.1 deb build)"
    local goroot gomodcache
    goroot="$(go env GOROOT)"
    gomodcache="$(go env GOMODCACHE)"
    echo "building deb in ubuntu:22.04 container (webkit2gtk-4.1)"
    docker run --rm \
        -v "$ROOT:/workspace" \
        -v "$goroot:/usr/local/go:ro" \
        -v "$gomodcache:/host-gomod:ro" \
        -v "little-timer-deb-gocache:/tmp/gocache" \
        -v "little-timer-deb-gomod:/tmp/gomod" \
        -e "HOST_UID=$(id -u)" -e "HOST_GID=$(id -g)" \
        -e GOROOT=/usr/local/go \
        -e PATH=/usr/local/go/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin \
        -e GOMODCACHE=/tmp/gomod -e GOCACHE=/tmp/gocache \
        -e GOPROXY=file:///host-gomod/cache/download -e GOSUMDB=off \
        -e GOFLAGS=-mod=readonly -e GOTOOLCHAIN=local -e CGO_ENABLED=1 \
        -w /workspace ubuntu:22.04 bash -lc '
            set -euo pipefail
            apt-get update -qq
            DEBIAN_FRONTEND=noninteractive apt-get install -y -qq --no-install-recommends \
                build-essential pkg-config libgtk-3-dev libwebkit2gtk-4.1-dev \
                libsqlite3-dev ca-certificates dpkg-dev
            /workspace/scripts/package-linux.sh --inside-deb '"$VER"'
            chown -R "$HOST_UID:$HOST_GID" /workspace/dist
        '
}

run_rpm_container() {
    command -v docker >/dev/null 2>&1 || die "docker not found (needed for the EL9 rpm build)"
    local goroot gomodcache
    goroot="$(go env GOROOT)"
    gomodcache="$(go env GOMODCACHE)"
    echo "building rpm in almalinux:9 container (webkit2gtk-4.0)"
    docker run --rm \
        -v "$ROOT:/workspace" \
        -v "$goroot:/usr/local/go:ro" \
        -v "$gomodcache:/host-gomod:ro" \
        -v "little-timer-rpm-gocache:/tmp/gocache" \
        -v "little-timer-rpm-gomod:/tmp/gomod" \
        -e "HOST_UID=$(id -u)" -e "HOST_GID=$(id -g)" \
        -e GOROOT=/usr/local/go \
        -e PATH=/usr/local/go/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin \
        -e GOMODCACHE=/tmp/gomod -e GOCACHE=/tmp/gocache \
        -e GOPROXY=file:///host-gomod/cache/download -e GOSUMDB=off \
        -e GOFLAGS=-mod=readonly -e GOTOOLCHAIN=local -e CGO_ENABLED=1 \
        -w /workspace almalinux:9 bash -lc '
            set -euo pipefail
            dnf install -y -q gcc gcc-c++ pkg-config gtk3-devel webkit2gtk3-devel \
                sqlite-devel rpm-build redhat-rpm-config binutils findutils ca-certificates
            /workspace/scripts/package-linux.sh --inside-rpm '"$VER"'
            chown -R "$HOST_UID:$HOST_GID" /workspace/dist
        '
}

run_rpm() {
    if [ "$INSIDE_RPM" = "1" ]; then
        make_rpm
        return
    fi

    build_ui
    if pkg-config --exists gtk+-3.0 webkit2gtk-4.0; then
        make_rpm
    else
        run_rpm_container
    fi
}

run_deb() {
    if [ "$INSIDE_DEB" = "1" ]; then
        build_binary webkit41
        assert_needed libwebkit2gtk-4.1.so.0
        make_deb
        return
    fi

    build_ui
    if command -v dpkg-deb >/dev/null 2>&1 && pkg-config --exists webkit2gtk-4.1; then
        build_binary webkit41
        assert_needed libwebkit2gtk-4.1.so.0
        make_deb
    else
        run_deb_container
    fi
}

case "$FORMAT" in
    rpm) run_rpm ;;
    deb) run_deb ;;
    all)
        rc=0
        run_rpm || rc=1
        run_deb || rc=1
        exit "$rc"
        ;;
    *)
        echo "usage: $0 {rpm|deb|all} [version]" >&2
        exit 2
        ;;
esac
