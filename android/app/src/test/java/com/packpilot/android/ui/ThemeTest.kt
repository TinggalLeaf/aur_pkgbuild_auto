package com.packpilot.android.ui

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotEquals
import org.junit.Test

/** 验证主题预设和中文状态映射。 */
class ThemeTest {
    /** 验证四套主题名称和种子颜色各不相同。 */
    @Test fun presetsAreDistinct() {
        assertEquals(4, ThemePreset.entries.size)
        assertEquals(4, ThemePreset.entries.map { it.seed }.toSet().size)
        assertNotEquals(ThemePreset.EMERALD.seed, ThemePreset.OCEAN.seed)
    }

    /** 验证两种组件风格和三种显示模式完整可用。 */
    @Test fun appearanceModes() {
        assertEquals(setOf("MD3", "MIUIX"), UiStyle.entries.map { it.name }.toSet())
        assertEquals(setOf("SYSTEM", "LIGHT", "DARK"), DarkMode.entries.map { it.name }.toSet())
    }

    /** 验证所有任务状态都能显示中文。 */
    @Test fun statusLabels() {
        assertEquals("容器构建", statusLabel("building"))
        assertEquals("验证通过", statusLabel("succeeded"))
        assertEquals("已发布", statusLabel("published"))
    }
}
