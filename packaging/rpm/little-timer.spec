%global debug_package %{nil}

Name:           little-timer
Version:        %{?lt_version}%{!?lt_version:2.0.0}
Release:        1webkit40%{?dist}
Summary:        Countdown, stopwatch and habit timer
License:        Apache-2.0
URL:            https://github.com/daedalusys/little_timer
Source0:        little-timer
Source1:        little-timer-desktop
Source2:        little-timer.desktop
Source3:        little-timer-48.png
Source4:        little-timer-72.png
Source5:        little-timer-96.png
Source6:        little-timer-144.png
Source7:        little-timer-192.png
Source8:        LICENSE

Recommends:     gnome-keyring
Provides:       little-timer(webkit40) = %{version}-%{release}

%description
Little Timer is a local-first countdown/stopwatch application with habits,
GTD tasks, daily planning and backups. This build links WebKitGTK 4.0
(EL8/EL9). The desktop entry starts the HTTP server and opens the system
browser; a WebView window is available as a separate desktop action.

%prep

%build

%install
install -Dm755 %{SOURCE0} %{buildroot}%{_bindir}/little-timer
install -Dm755 %{SOURCE1} %{buildroot}%{_bindir}/little-timer-desktop
install -Dm644 %{SOURCE2} %{buildroot}%{_datadir}/applications/little-timer.desktop
install -Dm644 %{SOURCE8} %{buildroot}%{_licensedir}/%{name}/LICENSE

install -Dm644 %{SOURCE3} %{buildroot}%{_datadir}/icons/hicolor/48x48/apps/little-timer.png
install -Dm644 %{SOURCE4} %{buildroot}%{_datadir}/icons/hicolor/72x72/apps/little-timer.png
install -Dm644 %{SOURCE5} %{buildroot}%{_datadir}/icons/hicolor/96x96/apps/little-timer.png
install -Dm644 %{SOURCE6} %{buildroot}%{_datadir}/icons/hicolor/144x144/apps/little-timer.png
install -Dm644 %{SOURCE7} %{buildroot}%{_datadir}/icons/hicolor/192x192/apps/little-timer.png

%files
%{_bindir}/little-timer
%{_bindir}/little-timer-desktop
%{_datadir}/applications/little-timer.desktop
%{_datadir}/icons/hicolor/*/apps/little-timer.png
%license %{_licensedir}/%{name}/LICENSE

%changelog
* Sat Oct 31 2026 Little Timer Maintainers - %{version}-%{release}
- Local package built from the little_timer repository.
