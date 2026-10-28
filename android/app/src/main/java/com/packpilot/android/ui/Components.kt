package com.packpilot.android.ui

import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.text.input.VisualTransformation
import androidx.compose.ui.unit.dp
import top.yukonga.miuix.kmp.basic.ButtonDefaults

/** 按当前风格渲染 Material 3 或 Miuix 按钮。 */
@Composable
fun AppButton(text: String, onClick: () -> Unit, modifier: Modifier = Modifier, primary: Boolean = true, enabled: Boolean = true) {
    if (LocalUiStyle.current == UiStyle.MIUIX) {
        top.yukonga.miuix.kmp.basic.Button(
            onClick = onClick, modifier = modifier, enabled = enabled,
            colors = if (primary) ButtonDefaults.buttonColorsPrimary() else ButtonDefaults.buttonColors(),
        ) { top.yukonga.miuix.kmp.basic.Text(text) }
    } else {
        Button(onClick = onClick, modifier = modifier, enabled = enabled) { Text(text) }
    }
}

/** 按当前风格渲染带统一内边距的卡片。 */
@Composable
fun AppCard(modifier: Modifier = Modifier, content: @Composable ColumnScope.() -> Unit) {
    if (LocalUiStyle.current == UiStyle.MIUIX) {
        top.yukonga.miuix.kmp.basic.Card(modifier = modifier, insideMargin = PaddingValues(16.dp), content = content)
    } else {
        Card(modifier = modifier) { androidx.compose.foundation.layout.Column(Modifier.padding(16.dp), content = content) }
    }
}

/** 按当前风格渲染文本输入框，支持密码隐藏。 */
@Composable
fun AppTextField(value: String, onValueChange: (String) -> Unit, label: String, modifier: Modifier = Modifier, password: Boolean = false) {
    if (LocalUiStyle.current == UiStyle.MIUIX) {
        top.yukonga.miuix.kmp.basic.TextField(
            value = value, onValueChange = onValueChange, label = label,
            useLabelAsPlaceholder = true, singleLine = true, modifier = modifier.fillMaxWidth(),
            visualTransformation = if (password) PasswordVisualTransformation() else VisualTransformation.None,
        )
    } else {
        OutlinedTextField(
            value = value, onValueChange = onValueChange, label = { Text(label) },
            singleLine = true, modifier = modifier.fillMaxWidth(),
            visualTransformation = if (password) PasswordVisualTransformation() else VisualTransformation.None,
        )
    }
}

/** 渲染任务状态对应的中文标签。 */
fun statusLabel(status: String): String = when (status) {
    "queued" -> "排队中"
    "discovering" -> "发现源码"
    "generating" -> "生成文件"
    "building" -> "容器构建"
    "succeeded" -> "验证通过"
    "failed" -> "执行失败"
    "canceled" -> "已取消"
    "publishing" -> "正在发布"
    "published" -> "已发布"
    else -> status
}
