package com.packpilot.android

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.NavigationBar
import androidx.compose.material3.NavigationBarItem
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.viewmodel.compose.viewModel
import com.packpilot.android.ui.CreateTaskScreen
import com.packpilot.android.ui.DarkMode
import com.packpilot.android.ui.LoginScreen
import com.packpilot.android.ui.PackPilotTheme
import com.packpilot.android.ui.SettingsScreen
import com.packpilot.android.ui.TasksScreen
import com.packpilot.android.ui.ThemePreset
import com.packpilot.android.ui.UiStyle
import java.net.CookieHandler
import java.net.CookieManager
import java.net.CookiePolicy
import kotlinx.coroutines.delay

/** Android 入口，安装系统 Cookie 会话并组合认证、主题和主导航。 */
class MainActivity : ComponentActivity() {
    /** 初始化 Compose 和全局 CookieManager。 */
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        CookieHandler.setDefault(CookieManager(null, CookiePolicy.ACCEPT_ORIGINAL_SERVER))
        setContent {
            val model: MainViewModel = viewModel()
            val state by model.state.collectAsStateWithLifecycle()
            val style = runCatching { UiStyle.valueOf(state.style) }.getOrDefault(UiStyle.MIUIX)
            val preset = runCatching { ThemePreset.valueOf(state.preset) }.getOrDefault(ThemePreset.EMERALD)
            val darkMode = runCatching { DarkMode.valueOf(state.darkMode) }.getOrDefault(DarkMode.SYSTEM)
            PackPilotTheme(style, preset, darkMode) {
                when {
                    state.checking -> Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) { CircularProgressIndicator() }
                    !state.authenticated -> LoginScreen(state, model)
                    else -> MainScreen(state, model)
                }
            }
        }
    }
}

/** 显示任务、新建和设置三个主页面，并在后台轮询任务状态。 */
@androidx.compose.runtime.Composable
private fun MainScreen(state: UiState, model: MainViewModel) {
    var destination by remember { mutableIntStateOf(0) }
    LaunchedEffect(Unit) {
        while (true) {
            delay(3000)
            model.refresh()
        }
    }
    Scaffold(
        bottomBar = {
            NavigationBar {
                NavigationBarItem(selected = destination == 0, onClick = { destination = 0 }, icon = { Text("任") }, label = { Text("任务") })
                NavigationBarItem(selected = destination == 1, onClick = { destination = 1 }, icon = { Text("＋") }, label = { Text("新建") })
                NavigationBarItem(selected = destination == 2, onClick = { destination = 2 }, icon = { Text("设") }, label = { Text("设置") })
            }
        },
    ) { padding ->
        Box(Modifier.padding(padding)) {
            when (destination) {
                0 -> TasksScreen(state, model)
                1 -> CreateTaskScreen(state, model) { destination = 0 }
                else -> SettingsScreen(state, model)
            }
        }
    }
}
