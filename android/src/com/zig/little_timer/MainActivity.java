package com.zig.little_timer;

import android.app.Activity;
import android.os.Bundle;
import android.webkit.WebView;
import android.webkit.WebSettings;

public class MainActivity extends Activity {
    static {
        // 加载 Zig 生成的 .so 库名（与 build.zig 中生成的库/包名一致）
        System.loadLibrary("little_timer");
    }

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);

        WebView webView = new WebView(this);
        WebSettings settings = webView.getSettings();
        settings.setJavaScriptEnabled(true);
        settings.setDomStorageEnabled(true);
        settings.setAllowFileAccess(true);
        settings.setDatabaseEnabled(true);
        setContentView(webView);

        // 启动 Zig 端逻辑（启动本地 WebUI Server）
        startZigLogic();

        // 连接到本地服务（端口在 Zig 端固定为 12889）
        webView.loadUrl("http://127.0.0.1:12889");
    }

    // 由 Zig 导出的本地方法
    public native void startZigLogic();
}
