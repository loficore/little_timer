import type { FunctionalComponent, ComponentChildren } from "preact";

interface FormSectionProps {
  /** 分组标题 */
  title?: string;
  /** 分组描述 */
  description?: string;
  /** 分组内容 */
  children: ComponentChildren;
  /** 自定义 className */
  className?: string;
}

/**
 * 表单分段容器 - 用于组织设置界面的表单分组
 *
 * @example
 * ```tsx
 * <FormSection title="基本设置" description="应用全局设置">
 *   <FormGroup label="时区">
 *     <NumberInput {...props} />
 *   </FormGroup>
 * </FormSection>
 * ```
 */
export const FormSection: FunctionalComponent<FormSectionProps> = ({
  title,
  description,
  children,
  className = "",
}) => {
  return (
    <div className={`mb-6 sm:mb-8 md:mb-10 animate-slideUp ${className}`}>
      {title && (
        <div className="mb-3 sm:mb-4">
          <h3 className="text-sm sm:text-base font-semibold text-text-primary-dark">
            {title}
          </h3>
          {description && (
            <p className="text-xs sm:text-sm text-text-secondary-dark italic mt-1">
              {description}
            </p>
          )}
        </div>
      )}
      <div className="space-y-4 sm:space-y-6">{children}</div>
    </div>
  );
};
