package com.packpilot.android.ui

import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.staticCompositionLocalOf
import androidx.compose.ui.graphics.Color

/** 表示 Android 支持的组件风格。 */
enum class UiStyle(val label: String) { MD3("Material 3"), MIUIX("Miuix") }

/** 定义一套可跨 MD3 和 Miuix 使用的主题预设。 */
enum class ThemePreset(val label: String, val seed: Color) {
    EMERALD("翡翠绿", Color(0xFF1D6B4F)),
    OCEAN("海洋蓝", Color(0xFF2864DC)),
    VIOLET("暮光紫", Color(0xFF7656C8)),
    SUNSET("落日橙", Color(0xFFC85A32)),
}

/** 定义跟随系统、浅色和深色三种显示模式。 */
enum class DarkMode(val label: String) { SYSTEM("跟随系统"), LIGHT("浅色"), DARK("深色") }

val LocalUiStyle = staticCompositionLocalOf { UiStyle.MIUIX }

/** 根据模式、预设和组件风格同时提供 Material 与 Miuix 主题。 */
@Composable
fun PackPilotTheme(style: UiStyle, preset: ThemePreset, darkMode: DarkMode, content: @Composable () -> Unit) {
    val dark = when (darkMode) {
        DarkMode.SYSTEM -> isSystemInDarkTheme()
        DarkMode.LIGHT -> false
        DarkMode.DARK -> true
    }
    val materialColors = if (dark) darkColorScheme(primary = preset.seed) else lightColorScheme(primary = preset.seed)
    val miuixBase = if (dark) top.yukonga.miuix.kmp.theme.darkColorScheme() else top.yukonga.miuix.kmp.theme.lightColorScheme()
    val miuixColors = miuixBase.copy(primary = preset.seed, secondary = preset.seed)
    androidx.compose.runtime.CompositionLocalProvider(LocalUiStyle provides style) {
        MaterialTheme(colorScheme = materialColors) {
            top.yukonga.miuix.kmp.theme.MiuixTheme(colors = miuixColors, content = content)
        }
    }
}
