/**
 * 页面头：标题 + 一句话说明 + 右侧操作区（v1.4 Phase 2）。
 *
 * 对齐范例的「标题 — 一句话说明 — 主操作」节奏；标题由路由表驱动，业务页面无需自带标题。
 */
import { Typography } from 'antd';
import type { ReactNode } from 'react';

export default function PageHeader({
  title,
  description,
  extra,
}: {
  title: string;
  description?: string;
  extra?: ReactNode;
}) {
  return (
    <div className="page-header">
      <div className="page-header__main">
        <Typography.Title level={4} className="page-header__title">
          {title}
        </Typography.Title>
        {description ? <div className="page-header__desc">{description}</div> : null}
      </div>
      {extra ? <div className="page-header__extra">{extra}</div> : null}
    </div>
  );
}
