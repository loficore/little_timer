// 丢给真前端的持续帧率计:不是探针页,而是贴在 dist/index.html 尾巴上的旁路脚本。
// 每 30 帧报一次空闲节奏;pointerdown 时切换成 15 帧的短窗,单独报「点下去之后」的帧间隔,
// 这样能把「空闲就送不了 60Hz」和「交互那一下画不完」分开。
(() => {
  const IDLE_WINDOW = 30;
  const CLICK_WINDOW = 15;
  let stamps = [];
  let pendingClick = false;
  let windowSize = IDLE_WINDOW;

  function emit(line) {
    console.log(line);
    // LT_WEBKIT_DEBUG=1 时 native.go 绑了 window.ltProfLog,把同一行写回终端;
    // 绑定不存在(浏览器 / 未开 debug)就只留 console,静默降级。
    const bound = window.ltProfLog;
    if (typeof bound === "function") {
      try {
        const r = bound(line);
        if (r && typeof r.catch === "function") r.catch(() => {});
      } catch (e) { /* 回传失败不影响计时 */ }
    }
  }

  function report(mode) {
    const d = [];
    for (let i = 1; i < stamps.length; i++) d.push(stamps[i] - stamps[i - 1]);
    if (!d.length) return;
    const sorted = [...d].sort((a, b) => a - b);
    emit(
      `[CAD] ${mode} avgMs=${(d.reduce((a, b) => a + b, 0) / d.length).toFixed(1)} ` +
      `medianMs=${sorted[sorted.length >> 1].toFixed(1)} maxMs=${sorted[sorted.length - 1].toFixed(1)}`,
    );
  }

  function frame(now) {
    stamps.push(now);
    if (stamps.length > windowSize) {
      report(pendingClick ? "click" : "idle");
      pendingClick = false;
      windowSize = IDLE_WINDOW;
      stamps = [now];
    }
    requestAnimationFrame(frame);
  }

  addEventListener(
    "pointerdown",
    () => {
      if (pendingClick) return;
      pendingClick = true;
      windowSize = CLICK_WINDOW;
      stamps = [performance.now()];
    },
    true,
  );

  requestAnimationFrame(frame);
})();
