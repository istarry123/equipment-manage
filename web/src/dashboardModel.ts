/**
 * Dashboard 派生计算的唯一来源（v1.4 Phase 3）。
 *
 * 为什么单独成模块：Phase 3 的验收标准是「改版前后数值逐项一致」。
 * 把占比等派生计算抽成**无副作用的纯函数**，就能拿真实 `GET /api/dashboard` 响应
 * 直接逐项核验（见阶段报告），组件只负责呈现，不再内联算式。
 *
 * 硬约束：
 * 1. 本模块**不引入任何依赖、不发请求、无副作用**（刻意不含 import）——
 *    这样可以直接在 Node 里用真实 API 响应跑数值断言，而不必拉起浏览器或 antd。
 * 2. 遍历后端数组时**不得按固定状态清单过滤** —— 后端返回什么就呈现什么，绝不丢条目。
 * 3. 不新增、不改写任何业务数值；占比等纯派生值可 0 除兜底。
 * 4. 状态文案与配色不在此处：那是展示映射，归 status.ts；本模块只产出数值。
 */

export interface StatusCount {
  status: string;
  count: number;
}

export interface NameCount {
  name: string;
  count: number;
}

export interface FlowLine {
  equipment_id: number;
  equipment_no: string;
  display_no?: string;
  name: string;
  action_text: string;
  to_team_name: string;
  borrower_name: string;
  occurred_at: string;
  operator: string;
}

export interface BorrowLine {
  borrow_record_id: number;
  equipment_id: number;
  equipment_no: string;
  display_no?: string;
  name: string;
  borrower_name: string;
  borrow_date: string;
  expected_return_date: string;
  overdue_days: number;
}

export interface TeamCatRow {
  team: string;
  category: string;
  count: number;
}

export interface Dashboard {
  total: number;
  by_status: StatusCount[];
  by_category: NameCount[];
  by_team: NameCount[];
  recent_flows: FlowLine[];
  current_borrows: BorrowLine[];
  overdue_count: number;
  by_team_category: TeamCatRow[];
}

/** 百分比，保留 1 位小数；总数为 0/负数时返回 0（不产生 NaN / Infinity） */
export function percentOf(total: number, count: number): number {
  if (!total || total <= 0) return 0;
  return Math.round((count / total) * 1000) / 10;
}

export interface StatusKpi {
  /** 后端返回的状态码，原样保留 */
  status: string;
  /** 设备数（与后端 by_status[].count 完全一致） */
  count: number;
  /** 占设备总数百分比（1 位小数） */
  percent: number;
}

/** 由 by_status 构造 KPI：直接遍历后端数组，保证不丢失任何状态条目 */
export function buildStatusKpis(d: Dashboard | null | undefined): StatusKpi[] {
  if (!d) return [];
  const total = d.total ?? 0;
  return (d.by_status ?? []).map((s) => ({
    status: s.status,
    count: s.count,
    percent: percentOf(total, s.count),
  }));
}

/** 指定状态的设备数；后端未返回该状态时按 0 处理（仅用于摘要文案） */
export function countOfStatus(d: Dashboard | null | undefined, status: string): number {
  return (d?.by_status ?? []).find((s) => s.status === status)?.count ?? 0;
}

export interface OverdueSummary {
  count: number;
  hasOverdue: boolean;
}

/** 逾期摘要（口径与后端 overdue_count 一致，不做二次判断） */
export function overdueSummary(d: Dashboard | null | undefined): OverdueSummary {
  const count = d?.overdue_count ?? 0;
  return { count, hasOverdue: count > 0 };
}
