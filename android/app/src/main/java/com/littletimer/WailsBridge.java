package com.littletimer;

import android.app.Activity;

/**
 * Thin facade: keeps {@code com.littletimer} as the surface WailsJSBridge,
 * WailsPathHandler, and MainActivity wire to, while delegating the JNI-bound
 * implementation to {@code com.wails.app.WailsBridge} — the class libwails.so
 * looks up by hardcoded symbol name. JVM virtual dispatch resolves inherited
 * methods on the parent so Go-side callbacks still run.
 */
public class WailsBridge extends com.wails.app.WailsBridge {
    public WailsBridge(Activity activity) {
        super(activity);
    }
}
