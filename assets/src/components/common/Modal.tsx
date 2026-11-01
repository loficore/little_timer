import { createPortal } from "preact/compat";
import { useEffect } from "preact/hooks";
import type { ComponentChildren } from "preact";
import { t } from "../../utils/i18n";

/** 内联 X 图标：避免 heroicons 的 forwardRef 在 preact/compat 下被当成对象标签。 */
function CloseIcon() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} className="h-5 w-5" aria-hidden="true">
      <path strokeLinecap="round" strokeLinejoin="round" d="M6 18 18 6M6 6l12 12" />
    </svg>
  );
}

export interface ModalProps {
  /** header 标题（T2 层级，纯文本，不叠辉光）。 */
  title: string;
  onClose: () => void;
  children?: ComponentChildren;
  /** footer 操作区；内容区滚动时保持可见。 */
  footer?: ComponentChildren;
  /** 面板最大宽度类，默认 max-w-lg。 */
  widthClass?: string;
  testId?: string;
}

/**
 * Modal —— 全站共用弹窗外壳（visual-design-spec R7）。
 *
 * - 通过 portal 挂到 document.body：避免祖先的 backdrop-filter / transform
 *   建立包含块，把 position: fixed 限制在内容区（移动端顶栏浮在遮罩上的根因）；
 * - 遮罩走 .my-overlay-backdrop，面板走 .my-surface-modal（主题 token）；
 * - header（T2 + X）/ body（超高内部滚动）/ footer（取消 + primary）三段固定；
 * - Escape / 遮罩点击 / X 关闭。
 */
export function Modal({
  title,
  onClose,
  children,
  footer,
  widthClass = "max-w-lg",
  testId,
}: ModalProps) {
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);

  return createPortal(
    <div
      className="my-overlay-backdrop fixed inset-0 z-[100] flex items-center justify-center p-3 sm:p-6"
      data-testid={testId}
    >
      <div
        className="absolute inset-0"
        data-testid="modal-backdrop"
        aria-hidden="true"
        onClick={onClose}
      />
      <div
        role="dialog"
        aria-modal="true"
        aria-label={title}
        className={`my-surface-modal relative flex max-h-[calc(100dvh-1.5rem)] w-full ${widthClass} flex-col overflow-hidden sm:max-h-[90vh]`}
      >
        <header className="flex items-center justify-between gap-3 border-b border-[color-mix(in_oklab,var(--my-outline)_45%,transparent)] px-5 py-4 sm:px-6">
          <h2 className="m-0 text-lg font-semibold text-[var(--my-on-surface)]">{title}</h2>
          <button
            type="button"
            className="inline-flex h-11 w-11 shrink-0 items-center justify-center rounded-xl text-[var(--my-on-surface-variant)] transition-colors hover:bg-[color-mix(in_oklab,var(--my-outline)_22%,transparent)] hover:text-[var(--my-on-surface)]"
            aria-label={t("button.close")}
            data-testid="modal-close"
            onClick={onClose}
          >
            <CloseIcon />
          </button>
        </header>

        <div className="flex-1 overflow-y-auto px-5 py-4 sm:px-6 sm:py-5" data-testid="modal-body">
          {children}
        </div>

        {footer && (
          <footer
            className="flex flex-wrap items-center justify-end gap-2 border-t border-[color-mix(in_oklab,var(--my-outline)_45%,transparent)] px-5 py-4 sm:px-6"
            data-testid="modal-footer"
          >
            {footer}
          </footer>
        )}
      </div>
    </div>,
    document.body,
  );
}
