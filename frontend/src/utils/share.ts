/**
 * 分享工具函数
 */

/**
 * 复制文本到剪贴板
 */
export async function copyToClipboard(text: string): Promise<boolean> {
  try {
    // 现代浏览器 API
    if (navigator.clipboard && window.isSecureContext) {
      await navigator.clipboard.writeText(text);
      return true;
    }

    // 降级方案：使用 execCommand（已废弃但仍广泛支持）
    const textArea = document.createElement('textarea');
    textArea.value = text;
    textArea.style.position = 'fixed';
    textArea.style.left = '-9999px';
    textArea.style.top = '-9999px';
    document.body.appendChild(textArea);
    textArea.focus();
    textArea.select();

    const success = document.execCommand('copy');
    document.body.removeChild(textArea);
    return success;
  } catch {
    return false;
  }
}

/**
 * 分享当前页面
 * 优先使用 Web Share API（移动端），降级到复制链接
 */
export async function shareCurrentPage(): Promise<{ method: 'share' | 'copy' | 'cancelled'; success: boolean }> {
  const url = window.location.href;
  const title = document.title;

  // 尝试使用 Web Share API（主要用于移动端）
  // 注意：Safari/iOS 只有 navigator.share 没有 navigator.canShare，所以只检查 share
  if (navigator.share) {
    try {
      await navigator.share({ url, title });
      return { method: 'share', success: true };
    } catch (err) {
      // 用户取消分享是正常行为，静默处理
      if (err instanceof Error && err.name === 'AbortError') {
        return { method: 'cancelled', success: true };
      }
      // 其他错误（如不支持的数据类型）降级到复制
    }
  }

  // 降级到复制链接
  const success = await copyToClipboard(url);
  return { method: 'copy', success };
}
