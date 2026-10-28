package com.packpilot.android.data

import android.content.Context

/** 只持久化服务器与外观偏好，绝不保存页面密码或会话内容。 */
class SettingsStore(context: Context) {
    private val preferences = context.applicationContext.getSharedPreferences("packpilot_settings", Context.MODE_PRIVATE)

    /** 返回上次服务器地址，默认适配 Android 模拟器。 */
    fun serverURL(): String = preferences.getString("server_url", "http://10.0.2.2:8080")!!

    /** 返回界面风格，支持 MD3 和 MIUIX。 */
    fun style(): String = preferences.getString("style", "MIUIX")!!

    /** 返回主题预设名称。 */
    fun preset(): String = preferences.getString("preset", "EMERALD")!!

    /** 返回深色模式，支持 SYSTEM、LIGHT、DARK。 */
    fun darkMode(): String = preferences.getString("dark_mode", "SYSTEM")!!

    /** 保存服务器地址。 */
    fun saveServerURL(value: String) = preferences.edit().putString("server_url", normalizeServerURL(value)).apply()

    /** 保存界面风格、主题预设和深色模式。 */
    fun saveAppearance(style: String, preset: String, darkMode: String) = preferences.edit()
        .putString("style", style).putString("preset", preset).putString("dark_mode", darkMode).apply()
}
