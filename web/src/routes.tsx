/**
 * 路由与菜单的**唯一来源**（v1.4 Phase 2）。
 *
 * 一处定义，三处复用：侧栏菜单、页面头（标题 + 一句话说明）、document.title。
 * 新增页面时只需在此追加一条，并在 App.tsx 的 <Routes> 中登记对应组件。
 */
import {
  AppstoreOutlined,
  DatabaseOutlined,
  DashboardOutlined,
  ExportOutlined,
  FileAddOutlined,
  ImportOutlined,
  ProfileOutlined,
  SettingOutlined,
  SwapOutlined,
  TeamOutlined,
} from '@ant-design/icons';
import type { ReactNode } from 'react';

export interface RouteMeta {
  /** 菜单 key（与菜单选中态一致） */
  key: string;
  /** 路由路径（唯一，直接访问/刷新均可用） */
  path: string;
  /** 菜单与页面标题 */
  label: string;
  /** 一句话说明（取自 docs/user-guide.md §3 的实际用途，不新增未实现的能力描述） */
  description: string;
  icon: ReactNode;
}

export const ROUTES: RouteMeta[] = [
  {
    key: 'dashboard',
    path: '/dashboard',
    label: 'Dashboard',
    description: '设备总数、状态分布、班组占用、外借与逾期提醒的总览',
    icon: <DashboardOutlined />,
  },
  {
    key: 'equipment',
    path: '/equipment',
    label: '设备台账',
    description: '按编号/名称/型号检索设备，查看当前位置、当前状态与完整流转时间线',
    icon: <AppstoreOutlined />,
  },
  {
    key: 'teamview',
    path: '/teamview',
    label: '班组设备',
    description: '按「班组 → 设备类别 → 设备」查看各班组当前正在使用的设备',
    icon: <ProfileOutlined />,
  },
  {
    key: 'flow',
    path: '/flow',
    label: '设备流转',
    description: '出入库、班组转交、外借归还、送修与报废：在设备详情中按状态执行',
    icon: <SwapOutlined />,
  },
  {
    key: 'borrow',
    path: '/borrow',
    label: '外借管理',
    description: '外借单列表（逾期标红）、归还与延期（支持历史日期）、外借方维护',
    icon: <ExportOutlined />,
  },
  {
    key: 'team',
    path: '/team',
    label: '班组管理',
    description: '班组/内部单位的新增、修改、停用与受控删除',
    icon: <TeamOutlined />,
  },
  {
    key: 'import',
    path: '/import',
    label: '数据导入',
    description: 'Excel 台账解析预览 → REVIEW 审阅 → 疑似在借确认 → 导入 → 数据对账',
    icon: <ImportOutlined />,
  },
  {
    key: 'importDetail',
    path: '/import-detail',
    label: '外借明细导入',
    description: '历史外借明细补录：解析、匹配、按外借方分批写入与对账',
    icon: <FileAddOutlined />,
  },
  {
    key: 'backup',
    path: '/backup',
    label: '数据备份',
    description: '手动备份、备份列表与恢复（恢复需二次确认）',
    icon: <DatabaseOutlined />,
  },
  {
    key: 'settings',
    path: '/settings',
    label: '系统设置',
    description: '设备类别字典维护与系统信息',
    icon: <SettingOutlined />,
  },
];

/** 默认落地页（含全部未知路径的回退目标） */
export const DEFAULT_PATH = '/dashboard';

/** 路径 → 路由元数据；未登记路径返回 undefined（由调用方决定回退策略）。容忍结尾斜杠。 */
export function routeByPath(pathname: string): RouteMeta | undefined {
  const normalized = pathname.length > 1 && pathname.endsWith('/') ? pathname.slice(0, -1) : pathname;
  return ROUTES.find((r) => r.path === normalized);
}
